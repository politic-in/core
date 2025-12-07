package anonymization

import (
	"strings"
	"testing"
	"time"
)

func TestGeneratePayoutToken(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		pollID  string
		amount  int64
		wantErr bool
	}{
		{
			name:    "valid inputs",
			userID:  "user-123",
			pollID:  "poll-456",
			amount:  1000,
			wantErr: false,
		},
		{
			name:    "empty user ID",
			userID:  "",
			pollID:  "poll-456",
			amount:  1000,
			wantErr: true,
		},
		{
			name:    "empty poll ID",
			userID:  "user-123",
			pollID:  "",
			amount:  1000,
			wantErr: true,
		},
		{
			name:    "zero amount is valid",
			userID:  "user-123",
			pollID:  "poll-456",
			amount:  0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, mapping, err := GeneratePayoutToken(tt.userID, tt.pollID, tt.amount)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify token
			if token.TokenHash == "" {
				t.Error("token hash should not be empty")
			}
			if token.PollID != tt.pollID {
				t.Errorf("poll ID mismatch: got %s, want %s", token.PollID, tt.pollID)
			}
			if token.Amount != tt.amount {
				t.Errorf("amount mismatch: got %d, want %d", token.Amount, tt.amount)
			}
			if token.ExpiresAt.Before(token.CreatedAt) {
				t.Error("expiry should be after creation")
			}

			// Verify mapping
			if mapping.UserID != tt.userID {
				t.Errorf("user ID mismatch: got %s, want %s", mapping.UserID, tt.userID)
			}
			if mapping.TokenHash != token.TokenHash {
				t.Error("token hash should match between token and mapping")
			}
			if mapping.Salt == "" {
				t.Error("salt should not be empty")
			}
		})
	}
}

func TestGeneratePayoutToken_Uniqueness(t *testing.T) {
	// Same inputs should produce different tokens due to random salt
	token1, _, _ := GeneratePayoutToken("user-1", "poll-1", 100)
	token2, _, _ := GeneratePayoutToken("user-1", "poll-1", 100)

	if token1.TokenHash == token2.TokenHash {
		t.Error("tokens should be unique even with same inputs due to random salt")
	}
}

func TestValidatePayoutToken(t *testing.T) {
	tests := []struct {
		name    string
		token   *PayoutToken
		wantErr error
	}{
		{
			name:    "nil token",
			token:   nil,
			wantErr: ErrInvalidToken,
		},
		{
			name: "empty token hash",
			token: &PayoutToken{
				TokenHash: "",
				PollID:    "poll-1",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			wantErr: ErrInvalidToken,
		},
		{
			name: "empty poll ID",
			token: &PayoutToken{
				TokenHash: "hash123",
				PollID:    "",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			wantErr: ErrInvalidToken,
		},
		{
			name: "expired token",
			token: &PayoutToken{
				TokenHash: "hash123",
				PollID:    "poll-1",
				ExpiresAt: time.Now().Add(-time.Hour),
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "valid token",
			token: &PayoutToken{
				TokenHash: "hash123",
				PollID:    "poll-1",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePayoutToken(tt.token)
			if err != tt.wantErr {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestResponseAnonymizer_ValidateResponse(t *testing.T) {
	ra := NewResponseAnonymizer()

	tests := []struct {
		name     string
		response *AnonymizedResponse
		wantErr  bool
	}{
		{
			name:     "nil response",
			response: nil,
			wantErr:  true,
		},
		{
			name: "missing ID",
			response: &AnonymizedResponse{
				PollID:    "poll-1",
				HexagonID: "hex-1",
				Answers:   map[string]interface{}{"q1": "a"},
			},
			wantErr: true,
		},
		{
			name: "missing poll ID",
			response: &AnonymizedResponse{
				ID:        "resp-1",
				HexagonID: "hex-1",
				Answers:   map[string]interface{}{"q1": "a"},
			},
			wantErr: true,
		},
		{
			name: "missing hexagon ID",
			response: &AnonymizedResponse{
				ID:      "resp-1",
				PollID:  "poll-1",
				Answers: map[string]interface{}{"q1": "a"},
			},
			wantErr: true,
		},
		{
			name: "missing answers",
			response: &AnonymizedResponse{
				ID:        "resp-1",
				PollID:    "poll-1",
				HexagonID: "hex-1",
				Answers:   map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "valid response",
			response: &AnonymizedResponse{
				ID:        "resp-1",
				PollID:    "poll-1",
				HexagonID: "hex-1",
				Answers:   map[string]interface{}{"q1": "a"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ra.ValidateResponse(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAggregator_CheckKAnonymity(t *testing.T) {
	agg := NewAggregator()

	tests := []struct {
		count int
		want  bool
	}{
		{0, false},
		{5, false},
		{9, false},
		{10, true},
		{100, true},
	}

	for _, tt := range tests {
		got := agg.CheckKAnonymity(tt.count)
		if got != tt.want {
			t.Errorf("CheckKAnonymity(%d) = %v, want %v", tt.count, got, tt.want)
		}
	}
}

func TestAggregator_AggregateResponses(t *testing.T) {
	agg := NewAggregatorWithConfig(AggregationConfig{
		KAnonymityThreshold: 10,
		DPEpsilon:           1.0,
		MinAggregationSize:  5,
		ApplyNoise:          false, // Disable noise for deterministic testing
	})

	// Create test responses
	responses := make([]AnonymizedResponse, 15)
	for i := 0; i < 15; i++ {
		answer := "option_a"
		if i >= 10 {
			answer = "option_b"
		}
		responses[i] = AnonymizedResponse{
			ID:        "resp-" + string(rune('0'+i)),
			PollID:    "poll-1",
			HexagonID: "hex-1",
			Answers:   map[string]interface{}{"q1": answer},
		}
	}

	result, err := agg.AggregateResponses(responses, "poll-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ResponseCount != 15 {
		t.Errorf("response count = %d, want 15", result.ResponseCount)
	}
	if !result.MeetsKAnon {
		t.Error("should meet k-anonymity with 15 responses")
	}
	if result.Results == nil {
		t.Error("results should not be nil when k-anonymity is met")
	}
}

func TestAggregator_AggregateResponses_InsufficientResponses(t *testing.T) {
	agg := NewAggregator()

	responses := make([]AnonymizedResponse, 5)
	for i := 0; i < 5; i++ {
		responses[i] = AnonymizedResponse{
			ID:        "resp-" + string(rune('0'+i)),
			PollID:    "poll-1",
			HexagonID: "hex-1",
			Answers:   map[string]interface{}{"q1": "a"},
		}
	}

	_, err := agg.AggregateResponses(responses, "poll-1", nil, nil)
	if err == nil {
		t.Error("expected error for insufficient responses")
	}
}

func TestAggregator_AggregateResponses_KAnonymityNotMet(t *testing.T) {
	agg := NewAggregatorWithConfig(AggregationConfig{
		KAnonymityThreshold: 20,
		DPEpsilon:           1.0,
		MinAggregationSize:  5,
		ApplyNoise:          false,
	})

	responses := make([]AnonymizedResponse, 10)
	for i := 0; i < 10; i++ {
		responses[i] = AnonymizedResponse{
			ID:        "resp-" + string(rune('0'+i)),
			PollID:    "poll-1",
			HexagonID: "hex-1",
			Answers:   map[string]interface{}{"q1": "a"},
		}
	}

	result, err := agg.AggregateResponses(responses, "poll-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MeetsKAnon {
		t.Error("should not meet k-anonymity with threshold 20 and 10 responses")
	}
	if result.Results != nil {
		t.Error("results should be nil when k-anonymity is not met")
	}
}

func TestApplyDifferentialPrivacy(t *testing.T) {
	// Test that large samples return unchanged
	result := ApplyDifferentialPrivacy(100, 1.0)
	if result != 100 {
		t.Errorf("large sample should be unchanged, got %d", result)
	}

	// Test that small samples get noise (run multiple times to check randomness)
	original := 20
	sameCount := 0
	for i := 0; i < 100; i++ {
		result := ApplyDifferentialPrivacy(original, 1.0)
		if result == original {
			sameCount++
		}
	}
	// Should have some variation (not all the same)
	if sameCount == 100 {
		t.Error("differential privacy should add noise to small samples")
	}

	// Test that results are never negative
	for i := 0; i < 100; i++ {
		result := ApplyDifferentialPrivacy(1, 0.5)
		if result < 0 {
			t.Errorf("result should never be negative, got %d", result)
		}
	}
}

func TestHashDeviceFingerprint(t *testing.T) {
	fingerprint := "device-123-abc"
	hash1 := HashDeviceFingerprint(fingerprint)
	hash2 := HashDeviceFingerprint(fingerprint)

	// Same input should produce same hash
	if hash1 != hash2 {
		t.Error("same fingerprint should produce same hash")
	}

	// Different input should produce different hash
	hash3 := HashDeviceFingerprint("different-device")
	if hash1 == hash3 {
		t.Error("different fingerprints should produce different hashes")
	}

	// Hash should be 64 characters (hex encoded SHA256)
	if len(hash1) != 64 {
		t.Errorf("hash length should be 64, got %d", len(hash1))
	}
}

func TestHashWithSalt(t *testing.T) {
	data := "sensitive-data"
	salt1 := "salt1"
	salt2 := "salt2"

	hash1 := HashWithSalt(data, salt1)
	hash2 := HashWithSalt(data, salt1)
	hash3 := HashWithSalt(data, salt2)

	// Same inputs should produce same hash
	if hash1 != hash2 {
		t.Error("same data and salt should produce same hash")
	}

	// Different salt should produce different hash
	if hash1 == hash3 {
		t.Error("different salts should produce different hashes")
	}
}

func TestValidateSeparation(t *testing.T) {
	tests := []struct {
		name      string
		guarantee SeparationGuarantee
		wantErr   bool
	}{
		{
			name: "valid separation",
			guarantee: SeparationGuarantee{
				IdentityDBProject:   "project-identity",
				ResponseDBProject:   "project-response",
				ForeignKeyExists:    false,
				SharedEncryptionKey: false,
				SeparateHSMKeys:     true,
			},
			wantErr: false,
		},
		{
			name: "same project",
			guarantee: SeparationGuarantee{
				IdentityDBProject:   "same-project",
				ResponseDBProject:   "same-project",
				ForeignKeyExists:    false,
				SharedEncryptionKey: false,
				SeparateHSMKeys:     true,
			},
			wantErr: true,
		},
		{
			name: "foreign key exists",
			guarantee: SeparationGuarantee{
				IdentityDBProject:   "project-identity",
				ResponseDBProject:   "project-response",
				ForeignKeyExists:    true,
				SharedEncryptionKey: false,
				SeparateHSMKeys:     true,
			},
			wantErr: true,
		},
		{
			name: "shared encryption key",
			guarantee: SeparationGuarantee{
				IdentityDBProject:   "project-identity",
				ResponseDBProject:   "project-response",
				ForeignKeyExists:    false,
				SharedEncryptionKey: true,
				SeparateHSMKeys:     true,
			},
			wantErr: true,
		},
		{
			name: "shared HSM keys",
			guarantee: SeparationGuarantee{
				IdentityDBProject:   "project-identity",
				ResponseDBProject:   "project-response",
				ForeignKeyExists:    false,
				SharedEncryptionKey: false,
				SeparateHSMKeys:     false,
			},
			wantErr: true,
		},
		{
			name: "empty project names",
			guarantee: SeparationGuarantee{
				IdentityDBProject:   "",
				ResponseDBProject:   "",
				ForeignKeyExists:    false,
				SharedEncryptionKey: false,
				SeparateHSMKeys:     true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSeparation(tt.guarantee)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncryptor(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	t.Run("encrypt and decrypt bytes", func(t *testing.T) {
		plaintext := []byte("Hello, World!")
		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		// Ciphertext should be different from plaintext
		if string(ciphertext) == string(plaintext) {
			t.Error("ciphertext should differ from plaintext")
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if string(decrypted) != string(plaintext) {
			t.Errorf("decrypted = %s, want %s", string(decrypted), string(plaintext))
		}
	})

	t.Run("encrypt and decrypt string", func(t *testing.T) {
		plaintext := "Sensitive data here"
		encoded, err := enc.EncryptString(plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		decrypted, err := enc.DecryptString(encoded)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("decrypted = %s, want %s", decrypted, plaintext)
		}
	})

	t.Run("different encryptions produce different ciphertexts", func(t *testing.T) {
		plaintext := []byte("Same message")
		cipher1, _ := enc.Encrypt(plaintext)
		cipher2, _ := enc.Encrypt(plaintext)

		// Due to random nonce, ciphertexts should differ
		if string(cipher1) == string(cipher2) {
			t.Error("encrypting same plaintext twice should produce different ciphertexts")
		}
	})

	t.Run("decrypt with wrong key fails", func(t *testing.T) {
		plaintext := []byte("Secret")
		ciphertext, _ := enc.Encrypt(plaintext)

		wrongKey, _ := GenerateEncryptionKey()
		wrongEnc, _ := NewEncryptor(wrongKey)

		_, err := wrongEnc.Decrypt(ciphertext)
		if err == nil {
			t.Error("decryption with wrong key should fail")
		}
	})
}

func TestEncryptor_InvalidKey(t *testing.T) {
	_, err := NewEncryptor([]byte("too-short"))
	if err == nil {
		t.Error("should reject key that's not 32 bytes")
	}
}

func TestResponseSplitter(t *testing.T) {
	identityKey, _ := GenerateEncryptionKey()
	responseKey, _ := GenerateEncryptionKey()

	splitter, err := NewResponseSplitter(identityKey, responseKey)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	identityRecord, responseRecord, err := splitter.SplitResponse(
		"user-123",
		"poll-456",
		"hex-789",
		map[string]interface{}{"q1": "answer1", "q2": 5},
		45,
		"device-fingerprint-abc",
		1000,
	)

	if err != nil {
		t.Fatalf("split failed: %v", err)
	}

	// Verify identity record has user info but not answers
	if identityRecord.UserID != "user-123" {
		t.Error("identity record should have user ID")
	}
	if identityRecord.PollID != "poll-456" {
		t.Error("identity record should have poll ID")
	}

	// Verify response record has answers but not user info
	if responseRecord.ResponseID == "" {
		t.Error("response record should have a response ID")
	}
	if responseRecord.HexagonID != "hex-789" {
		t.Error("response record should have hexagon ID")
	}
	if len(responseRecord.Answers) != 2 {
		t.Error("response record should have answers")
	}

	// Both records should share the same payout token hash
	if identityRecord.PayoutTokenHash != responseRecord.PayoutTokenHash {
		t.Error("payout token hash should match between records")
	}

	// Device fingerprint should be hashed, not plain
	if responseRecord.DeviceFingerprintHash == "device-fingerprint-abc" {
		t.Error("device fingerprint should be hashed")
	}
	if len(responseRecord.DeviceFingerprintHash) != 64 {
		t.Error("device fingerprint hash should be 64 characters")
	}
}

func TestResponseSplitter_SameKeysRejected(t *testing.T) {
	key, _ := GenerateEncryptionKey()

	_, err := NewResponseSplitter(key, key)
	if err == nil {
		t.Error("should reject same key for both databases")
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	key1, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("key length should be 32, got %d", len(key1))
	}

	key2, _ := GenerateEncryptionKey()
	if string(key1) == string(key2) {
		t.Error("generated keys should be unique")
	}
}

func TestDeriveKey(t *testing.T) {
	password := []byte("my-password")
	salt := []byte("random-salt")

	key1 := DeriveKey(password, salt, 1000)
	key2 := DeriveKey(password, salt, 1000)

	// Same inputs should produce same key
	if string(key1) != string(key2) {
		t.Error("same password and salt should produce same key")
	}

	// Different password should produce different key
	key3 := DeriveKey([]byte("different-password"), salt, 1000)
	if string(key1) == string(key3) {
		t.Error("different passwords should produce different keys")
	}

	// Different salt should produce different key
	key4 := DeriveKey(password, []byte("different-salt"), 1000)
	if string(key1) == string(key4) {
		t.Error("different salts should produce different keys")
	}

	// Key should be 32 bytes
	if len(key1) != 32 {
		t.Errorf("derived key should be 32 bytes, got %d", len(key1))
	}
}

func TestDefaultAggregationConfig(t *testing.T) {
	config := DefaultAggregationConfig()

	if config.KAnonymityThreshold != KAnonymityThreshold {
		t.Errorf("KAnonymityThreshold = %d, want %d", config.KAnonymityThreshold, KAnonymityThreshold)
	}
	if config.DPEpsilon != DifferentialPrivacyEpsilon {
		t.Errorf("DPEpsilon = %f, want %f", config.DPEpsilon, DifferentialPrivacyEpsilon)
	}
	if config.MinAggregationSize != MinAggregationSize {
		t.Errorf("MinAggregationSize = %d, want %d", config.MinAggregationSize, MinAggregationSize)
	}
	if !config.ApplyNoise {
		t.Error("ApplyNoise should be true by default")
	}
}

func TestResponseAnonymizerWithKey(t *testing.T) {
	t.Run("valid key", func(t *testing.T) {
		key := make([]byte, 32)
		ra, err := NewResponseAnonymizerWithKey(key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ra == nil {
			t.Error("anonymizer should not be nil")
		}
	})

	t.Run("invalid key length", func(t *testing.T) {
		key := make([]byte, 16) // Wrong size
		_, err := NewResponseAnonymizerWithKey(key)
		if err == nil {
			t.Error("should reject invalid key length")
		}
	})
}

// Benchmark tests
func BenchmarkGeneratePayoutToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GeneratePayoutToken("user-123", "poll-456", 1000)
	}
}

func BenchmarkHashDeviceFingerprint(b *testing.B) {
	fingerprint := "device-123-abc-def-ghi"
	for i := 0; i < b.N; i++ {
		HashDeviceFingerprint(fingerprint)
	}
}

func BenchmarkEncryption(b *testing.B) {
	key, _ := GenerateEncryptionKey()
	enc, _ := NewEncryptor(key)
	plaintext := []byte(strings.Repeat("Hello, World! ", 100))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.Encrypt(plaintext)
	}
}

func BenchmarkDecryption(b *testing.B) {
	key, _ := GenerateEncryptionKey()
	enc, _ := NewEncryptor(key)
	plaintext := []byte(strings.Repeat("Hello, World! ", 100))
	ciphertext, _ := enc.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.Decrypt(ciphertext)
	}
}

func BenchmarkApplyDifferentialPrivacy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ApplyDifferentialPrivacy(25, 1.0)
	}
}
