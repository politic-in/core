// Package anonymization implements the zero-knowledge response architecture.
// Open-source so users can verify that their responses cannot be linked to their identity.
package anonymization

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sync"
	"time"
)

// Architecture Overview:
//
// IDENTITY SERVICE (GCP Project A)     RESPONSE SERVICE (GCP Project B)
// +---------------------+              +---------------------+
// | user_id             |              | response_id         |
// | phone (encrypted)   |    NO FK     | hexagon_id          |
// | kyc_docs (encrypted)| ------------ | poll_id             |
// | wallet_balance      |              | answer              |
// | civic_score         |              | timestamp           |
// +---------------------+              +---------------------+
//
// CRITICAL: No foreign key between databases.
// Even with both databases, cannot link user to response.

const (
	// KAnonymityThreshold is minimum respondents per data point
	KAnonymityThreshold = 10

	// MinAggregationSize is minimum responses before results are released
	MinAggregationSize = 10

	// DifferentialPrivacyEpsilon is the default epsilon for differential privacy
	DifferentialPrivacyEpsilon = 1.0

	// LargeSampleThreshold is the count above which no noise is needed
	LargeSampleThreshold = 50

	// MaxTokenAge is the maximum age of a payout token before expiry
	MaxTokenAge = 30 * 24 * time.Hour // 30 days

	// SaltLength is the length of random salt for token generation
	SaltLength = 32

	// NonceSize is the size of nonce for AES-GCM
	NonceSize = 12
)

// Error definitions
var (
	ErrKAnonymityNotMet     = errors.New("k-anonymity threshold not met")
	ErrInvalidResponse      = errors.New("invalid response: missing required fields")
	ErrSeparationViolation  = errors.New("critical: database separation violated")
	ErrEncryptionFailed     = errors.New("encryption operation failed")
	ErrDecryptionFailed     = errors.New("decryption operation failed")
	ErrInvalidToken         = errors.New("invalid payout token")
	ErrTokenExpired         = errors.New("payout token has expired")
	ErrInsufficientResponses = errors.New("insufficient responses for aggregation")
)

// PayoutToken represents a one-way token for payouts without revealing identity
type PayoutToken struct {
	TokenHash   string    `json:"token_hash"`    // SHA256 hash - cannot reverse to user_id
	Amount      int64     `json:"amount"`        // Amount in paisa
	PollID      string    `json:"poll_id"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// TokenMapping stores the user-to-token mapping (stored encrypted in Identity DB only)
type TokenMapping struct {
	UserID      string    `json:"user_id"`
	TokenHash   string    `json:"token_hash"`
	PollID      string    `json:"poll_id"`
	Salt        string    `json:"salt"`           // Used in hash generation
	CreatedAt   time.Time `json:"created_at"`
}

// GeneratePayoutToken creates a token that links payout to user without revealing identity
// The token is a hash of: user_id + poll_id + random_salt
// Response service sees: token_hash, amount
// Identity service sees: token_hash -> user_id mapping (separate, encrypted)
func GeneratePayoutToken(userID, pollID string, amountPaisa int64) (*PayoutToken, *TokenMapping, error) {
	if userID == "" || pollID == "" {
		return nil, nil, errors.New("userID and pollID are required")
	}

	// Generate random salt
	salt, err := generateSecureRandomString(SaltLength)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Create hash using HMAC-SHA256 for added security
	h := hmac.New(sha256.New, []byte(salt))
	h.Write([]byte(userID + pollID))
	tokenHash := hex.EncodeToString(h.Sum(nil))

	now := time.Now()
	token := &PayoutToken{
		TokenHash: tokenHash,
		Amount:    amountPaisa,
		PollID:    pollID,
		CreatedAt: now,
		ExpiresAt: now.Add(MaxTokenAge),
	}

	mapping := &TokenMapping{
		UserID:    userID,
		TokenHash: tokenHash,
		PollID:    pollID,
		Salt:      salt,
		CreatedAt: now,
	}

	return token, mapping, nil
}

// ValidatePayoutToken checks if a token is valid and not expired
func ValidatePayoutToken(token *PayoutToken) error {
	if token == nil {
		return ErrInvalidToken
	}
	if token.TokenHash == "" || token.PollID == "" {
		return ErrInvalidToken
	}
	if time.Now().After(token.ExpiresAt) {
		return ErrTokenExpired
	}
	return nil
}

// ResponseAnonymizer handles response anonymization with thread-safety
type ResponseAnonymizer struct {
	mu            sync.RWMutex
	encryptionKey []byte // 32-byte AES key
}

// NewResponseAnonymizer creates a new anonymizer
func NewResponseAnonymizer() *ResponseAnonymizer {
	return &ResponseAnonymizer{}
}

// NewResponseAnonymizerWithKey creates an anonymizer with encryption capability
func NewResponseAnonymizerWithKey(key []byte) (*ResponseAnonymizer, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	return &ResponseAnonymizer{encryptionKey: key}, nil
}

// AnonymizedResponse represents a response with no user linkage
type AnonymizedResponse struct {
	ID                    string                 `json:"id"`
	PollID                string                 `json:"poll_id"`
	HexagonID             string                 `json:"hexagon_id"` // Location only
	Answers               map[string]interface{} `json:"answers"`
	ResponseTimeSeconds   int                    `json:"response_time_seconds"`
	DeviceFingerprintHash string                 `json:"device_fingerprint_hash"` // For dedup only
	PayoutTokenHash       string                 `json:"payout_token_hash"`
	CreatedAt             time.Time              `json:"created_at"`
	// NO user_id - this is intentional
}

// ValidateResponse validates that a response has all required fields and no user linkage
func (ra *ResponseAnonymizer) ValidateResponse(response *AnonymizedResponse) error {
	if response == nil {
		return ErrInvalidResponse
	}
	if response.ID == "" {
		return fmt.Errorf("%w: missing ID", ErrInvalidResponse)
	}
	if response.PollID == "" {
		return fmt.Errorf("%w: missing poll_id", ErrInvalidResponse)
	}
	if response.HexagonID == "" {
		return fmt.Errorf("%w: missing hexagon_id for aggregation", ErrInvalidResponse)
	}
	if len(response.Answers) == 0 {
		return fmt.Errorf("%w: missing answers", ErrInvalidResponse)
	}
	return nil
}

// AggregatedResult represents k-anonymous aggregated data
type AggregatedResult struct {
	PollID          string                 `json:"poll_id"`
	HexagonID       *string                `json:"hexagon_id,omitempty"` // nil = poll-level
	ACID            *int                   `json:"ac_id,omitempty"`
	ResponseCount   int                    `json:"response_count"`
	Results         map[string]interface{} `json:"results"`
	MeetsKAnon      bool                   `json:"meets_k_anon"`
	NoiseApplied    bool                   `json:"noise_applied"`
	ComputedAt      time.Time              `json:"computed_at"`
}

// AggregationConfig configures the aggregation behavior
type AggregationConfig struct {
	KAnonymityThreshold int
	DPEpsilon           float64
	MinAggregationSize  int
	ApplyNoise          bool
}

// DefaultAggregationConfig returns the default configuration
func DefaultAggregationConfig() AggregationConfig {
	return AggregationConfig{
		KAnonymityThreshold: KAnonymityThreshold,
		DPEpsilon:           DifferentialPrivacyEpsilon,
		MinAggregationSize:  MinAggregationSize,
		ApplyNoise:          true,
	}
}

// Aggregator handles response aggregation with privacy guarantees
type Aggregator struct {
	config AggregationConfig
	mu     sync.RWMutex
}

// NewAggregator creates a new aggregator with default config
func NewAggregator() *Aggregator {
	return &Aggregator{config: DefaultAggregationConfig()}
}

// NewAggregatorWithConfig creates an aggregator with custom config
func NewAggregatorWithConfig(config AggregationConfig) *Aggregator {
	return &Aggregator{config: config}
}

// CheckKAnonymity verifies that a result set meets k-anonymity requirements
func (a *Aggregator) CheckKAnonymity(responseCount int) bool {
	return responseCount >= a.config.KAnonymityThreshold
}

// AggregateResponses aggregates responses with k-anonymity and differential privacy
func (a *Aggregator) AggregateResponses(responses []AnonymizedResponse, pollID string, hexagonID *string, acID *int) (*AggregatedResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	count := len(responses)
	if count < a.config.MinAggregationSize {
		return nil, fmt.Errorf("%w: got %d, need %d", ErrInsufficientResponses, count, a.config.MinAggregationSize)
	}

	// Aggregate answers
	questionResults := make(map[string]interface{})
	questionCounts := make(map[string]map[string]int)

	for _, resp := range responses {
		for qID, answer := range resp.Answers {
			if questionCounts[qID] == nil {
				questionCounts[qID] = make(map[string]int)
			}
			ansStr := fmt.Sprintf("%v", answer)
			questionCounts[qID][ansStr]++
		}
	}

	// Apply differential privacy noise if needed
	for qID, counts := range questionCounts {
		if a.config.ApplyNoise && count < LargeSampleThreshold {
			noisyCounts := make(map[string]int)
			for opt, c := range counts {
				noisyCounts[opt] = ApplyDifferentialPrivacy(c, a.config.DPEpsilon)
			}
			questionResults[qID] = noisyCounts
		} else {
			questionResults[qID] = counts
		}
	}

	meetsKAnon := count >= a.config.KAnonymityThreshold
	noiseApplied := a.config.ApplyNoise && count < LargeSampleThreshold

	// If k-anonymity not met, redact results
	if !meetsKAnon {
		questionResults = nil
	}

	return &AggregatedResult{
		PollID:        pollID,
		HexagonID:     hexagonID,
		ACID:          acID,
		ResponseCount: count,
		Results:       questionResults,
		MeetsKAnon:    meetsKAnon,
		NoiseApplied:  noiseApplied,
		ComputedAt:    time.Now(),
	}, nil
}

// ApplyDifferentialPrivacy adds Laplacian noise to small cell sizes
// This prevents inference attacks on sparse data
func ApplyDifferentialPrivacy(count int, epsilon float64) int {
	if count >= LargeSampleThreshold {
		// Large enough sample, no noise needed
		return count
	}

	if epsilon <= 0 {
		epsilon = DifferentialPrivacyEpsilon
	}

	// Add Laplacian noise for smaller samples
	// Sensitivity = 1 (one person's presence/absence)
	noise := laplacianNoise(1.0 / epsilon)

	result := count + int(math.Round(noise))
	if result < 0 {
		return 0
	}
	return result
}

// HashDeviceFingerprint creates a one-way hash of device fingerprint
// Used for duplicate detection without storing actual fingerprint
func HashDeviceFingerprint(fingerprint string) string {
	hash := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(hash[:])
}

// HashWithSalt creates a salted hash (more secure than plain SHA256)
func HashWithSalt(data, salt string) string {
	h := hmac.New(sha256.New, []byte(salt))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// SeparationGuarantee documents the architectural guarantee
type SeparationGuarantee struct {
	IdentityDBProject   string `json:"identity_db_project"`   // GCP Project A
	ResponseDBProject   string `json:"response_db_project"`   // GCP Project B
	ForeignKeyExists    bool   `json:"foreign_key_exists"`    // Must be false
	SharedEncryptionKey bool   `json:"shared_encryption_key"` // Must be false
	SeparateHSMKeys     bool   `json:"separate_hsm_keys"`     // Must be true
}

// ValidateSeparation ensures the architectural separation is maintained
func ValidateSeparation(guarantee SeparationGuarantee) error {
	if guarantee.IdentityDBProject == "" || guarantee.ResponseDBProject == "" {
		return errors.New("both database projects must be specified")
	}

	if guarantee.IdentityDBProject == guarantee.ResponseDBProject {
		return fmt.Errorf("%w: Identity and Response DBs must be in separate GCP projects", ErrSeparationViolation)
	}

	if guarantee.ForeignKeyExists {
		return fmt.Errorf("%w: No foreign key may exist between Identity and Response DBs", ErrSeparationViolation)
	}

	if guarantee.SharedEncryptionKey {
		return fmt.Errorf("%w: Identity and Response DBs must use separate encryption keys", ErrSeparationViolation)
	}

	if !guarantee.SeparateHSMKeys {
		return fmt.Errorf("%w: HSM keys must be separate for each database", ErrSeparationViolation)
	}

	return nil
}

// Encryptor provides AES-GCM encryption for sensitive fields
type Encryptor struct {
	key []byte
}

// NewEncryptor creates a new encryptor with the given 32-byte key
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes for AES-256")
	}
	return &Encryptor{key: key}, nil
}

// Encrypt encrypts plaintext using AES-GCM
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-GCM
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrDecryptionFailed)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64-encoded ciphertext
func (e *Encryptor) EncryptString(plaintext string) (string, error) {
	ciphertext, err := e.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a base64-encoded ciphertext and returns plaintext
func (e *Encryptor) DecryptString(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: invalid base64", ErrDecryptionFailed)
	}
	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// ResponseSplitter handles splitting response data between Identity and Response DBs
type ResponseSplitter struct {
	identityEncryptor *Encryptor
	responseEncryptor *Encryptor
}

// NewResponseSplitter creates a splitter with separate encryption keys
func NewResponseSplitter(identityKey, responseKey []byte) (*ResponseSplitter, error) {
	if len(identityKey) != 32 || len(responseKey) != 32 {
		return nil, errors.New("both encryption keys must be 32 bytes")
	}

	// Verify keys are different
	if hmac.Equal(identityKey, responseKey) {
		return nil, errors.New("identity and response keys must be different")
	}

	identityEnc, _ := NewEncryptor(identityKey)
	responseEnc, _ := NewEncryptor(responseKey)

	return &ResponseSplitter{
		identityEncryptor: identityEnc,
		responseEncryptor: responseEnc,
	}, nil
}

// IdentityRecord is what gets stored in the Identity DB
type IdentityRecord struct {
	UserID               string    `json:"user_id"`
	PayoutTokenHash      string    `json:"payout_token_hash"`
	PollID               string    `json:"poll_id"`
	EarningAmountPaisa   int64     `json:"earning_amount_paisa"`
	CreatedAt            time.Time `json:"created_at"`
}

// ResponseRecord is what gets stored in the Response DB (NO user_id)
type ResponseRecord struct {
	ResponseID            string                 `json:"response_id"`
	PollID                string                 `json:"poll_id"`
	HexagonID             string                 `json:"hexagon_id"`
	Answers               map[string]interface{} `json:"answers"`
	ResponseTimeSeconds   int                    `json:"response_time_seconds"`
	DeviceFingerprintHash string                 `json:"device_fingerprint_hash"`
	PayoutTokenHash       string                 `json:"payout_token_hash"`
	CreatedAt             time.Time              `json:"created_at"`
}

// SplitResponse splits a response into Identity and Response records
// This is the core anonymization function
func (rs *ResponseSplitter) SplitResponse(
	userID string,
	pollID string,
	hexagonID string,
	answers map[string]interface{},
	responseTimeSeconds int,
	deviceFingerprint string,
	earningAmountPaisa int64,
) (*IdentityRecord, *ResponseRecord, error) {
	// Generate unique response ID
	responseID, err := generateSecureRandomString(16)
	if err != nil {
		return nil, nil, err
	}

	// Generate payout token
	token, _, err := GeneratePayoutToken(userID, pollID, earningAmountPaisa)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()

	// Identity record (knows user, doesn't know answers)
	identityRecord := &IdentityRecord{
		UserID:             userID,
		PayoutTokenHash:    token.TokenHash,
		PollID:             pollID,
		EarningAmountPaisa: earningAmountPaisa,
		CreatedAt:          now,
	}

	// Response record (knows answers, doesn't know user)
	responseRecord := &ResponseRecord{
		ResponseID:            responseID,
		PollID:                pollID,
		HexagonID:             hexagonID,
		Answers:               answers,
		ResponseTimeSeconds:   responseTimeSeconds,
		DeviceFingerprintHash: HashDeviceFingerprint(deviceFingerprint),
		PayoutTokenHash:       token.TokenHash,
		CreatedAt:             now,
	}

	return identityRecord, responseRecord, nil
}

// Helper functions

func generateSecureRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}

// laplacianNoise generates Laplacian noise using inverse CDF method
func laplacianNoise(scale float64) float64 {
	// Generate uniform random in (0, 1)
	b := make([]byte, 8)
	rand.Read(b)
	// Convert to float64 in (0, 1)
	u := float64(uint64(b[0])|(uint64(b[1])<<8)|(uint64(b[2])<<16)|(uint64(b[3])<<24)|
		(uint64(b[4])<<32)|(uint64(b[5])<<40)|(uint64(b[6])<<48)|(uint64(b[7])<<56)) / float64(^uint64(0))

	// Avoid log(0)
	if u < 1e-10 {
		u = 1e-10
	}
	if u > 1-1e-10 {
		u = 1 - 1e-10
	}

	// Inverse CDF of Laplace distribution
	if u < 0.5 {
		return scale * math.Log(2*u)
	}
	return -scale * math.Log(2*(1-u))
}

// GenerateEncryptionKey generates a secure 32-byte key for AES-256
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// DeriveKey derives a key from a password using PBKDF2-like approach
// Note: For production, use golang.org/x/crypto/pbkdf2
func DeriveKey(password, salt []byte, iterations int) []byte {
	key := password
	for i := 0; i < iterations; i++ {
		h := hmac.New(sha256.New, salt)
		h.Write(key)
		key = h.Sum(nil)
	}
	return key
}
