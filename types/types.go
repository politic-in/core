// Package types provides common types and errors for the Politic core packages.
// This package defines the fundamental data structures and error types used
// across anonymization, booth-matching, civic-score, election-blackout, and h3-utils.
package types

import (
	"errors"
	"fmt"
	"time"
)

// Common Error Definitions
var (
	// General errors
	ErrInvalidInput      = errors.New("invalid input")
	ErrNotFound          = errors.New("not found")
	ErrAlreadyExists     = errors.New("already exists")
	ErrOperationFailed   = errors.New("operation failed")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrRateLimited       = errors.New("rate limited")
	ErrTimeout           = errors.New("operation timed out")
	ErrNotImplemented    = errors.New("not implemented")
	ErrMaintenanceMode   = errors.New("system in maintenance mode")

	// User-related errors
	ErrInvalidUserID     = errors.New("invalid user ID")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserSuspended     = errors.New("user suspended")
	ErrInvalidCredentials = errors.New("invalid credentials")

	// Survey/Response errors
	ErrSurveyNotFound    = errors.New("survey not found")
	ErrSurveyClosed      = errors.New("survey closed")
	ErrAlreadyResponded  = errors.New("already responded")
	ErrInvalidResponse   = errors.New("invalid response")
	ErrResponseRequired  = errors.New("response required")

	// Location errors
	ErrInvalidLocation   = errors.New("invalid location")
	ErrLocationRequired  = errors.New("location required")
	ErrOutOfBounds       = errors.New("location out of bounds")
	ErrGeocodingFailed   = errors.New("geocoding failed")

	// Election errors
	ErrElectionNotFound  = errors.New("election not found")
	ErrBlackoutActive    = errors.New("election blackout active")
	ErrInvalidPhase      = errors.New("invalid election phase")
	ErrInvalidDate       = errors.New("invalid date")

	// Privacy errors
	ErrAnonymizationFailed   = errors.New("anonymization failed")
	ErrDecryptionFailed      = errors.New("decryption failed")
	ErrInsufficientResponses = errors.New("insufficient responses for anonymity")
	ErrPrivacyViolation      = errors.New("privacy violation detected")
)

// WrapError wraps an error with additional context
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// IsError checks if an error is of a specific type
func IsError(err, target error) bool {
	return errors.Is(err, target)
}

// UserID represents a unique user identifier
type UserID string

// SurveyID represents a unique survey identifier
type SurveyID string

// ResponseID represents a unique response identifier
type ResponseID string

// ElectionID represents a unique election identifier
type ElectionID string

// ACID represents an Assembly Constituency identifier
type ACID string

// PCID represents a Parliamentary Constituency identifier
type PCID string

// BoothID represents a polling booth identifier
type BoothID string

// H3Cell represents an H3 hexagonal cell ID
type H3Cell string

// LatLng represents a geographic coordinate
type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// BoundingBox represents a geographic bounding box
type BoundingBox struct {
	MinLat float64 `json:"min_lat"`
	MinLng float64 `json:"min_lng"`
	MaxLat float64 `json:"max_lat"`
	MaxLng float64 `json:"max_lng"`
}

// Contains checks if a point is within the bounding box
func (bb BoundingBox) Contains(lat, lng float64) bool {
	return lat >= bb.MinLat && lat <= bb.MaxLat &&
		lng >= bb.MinLng && lng <= bb.MaxLng
}

// IsValid checks if the bounding box is valid
func (bb BoundingBox) IsValid() bool {
	return bb.MinLat <= bb.MaxLat && bb.MinLng <= bb.MaxLng &&
		bb.MinLat >= -90 && bb.MaxLat <= 90 &&
		bb.MinLng >= -180 && bb.MaxLng <= 180
}

// GeoPolygon represents a geographic polygon
type GeoPolygon struct {
	ExteriorRing []LatLng   `json:"exterior_ring"`
	Holes        [][]LatLng `json:"holes,omitempty"`
}

// IsValid checks if the polygon is valid (at least 3 points in exterior ring)
func (p GeoPolygon) IsValid() bool {
	return len(p.ExteriorRing) >= 3
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Contains checks if a time is within the range
func (tr TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.Start) && !t.After(tr.End)
}

// Duration returns the duration of the time range
func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// Overlaps checks if two time ranges overlap
func (tr TimeRange) Overlaps(other TimeRange) bool {
	return !tr.End.Before(other.Start) && !other.End.Before(tr.Start)
}

// Demographics represents user demographic information
type Demographics struct {
	Gender    string `json:"gender,omitempty"`
	AgeGroup  string `json:"age_group,omitempty"`
	Education string `json:"education,omitempty"`
	Income    string `json:"income_bracket,omitempty"`
	Religion  string `json:"religion,omitempty"`
	Caste     string `json:"caste,omitempty"`
	Language  string `json:"language,omitempty"`
}

// SurveyMetadata contains survey metadata
type SurveyMetadata struct {
	ID             SurveyID  `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	ClientID       string    `json:"client_id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	TargetResponses int      `json:"target_responses"`
	CurrentResponses int     `json:"current_responses"`
	Status         string    `json:"status"`
	IsPaid         bool      `json:"is_paid"`
	PayoutPerResponse float64 `json:"payout_per_response,omitempty"`
	Categories     []string  `json:"categories,omitempty"`
}

// ResponseMetadata contains response metadata (without actual response data)
type ResponseMetadata struct {
	ResponseID   ResponseID `json:"response_id"`
	SurveyID     SurveyID   `json:"survey_id"`
	SubmittedAt  time.Time  `json:"submitted_at"`
	LatencyMs    int64      `json:"latency_ms,omitempty"`
	DeviceType   string     `json:"device_type,omitempty"`
	AppVersion   string     `json:"app_version,omitempty"`
}

// UserProfile represents a user's public profile
type UserProfile struct {
	UserID       UserID    `json:"user_id"`
	DisplayName  string    `json:"display_name"`
	CivicScore   int       `json:"civic_score"`
	Level        string    `json:"level"`
	Badges       []string  `json:"badges,omitempty"`
	JoinedAt     time.Time `json:"joined_at"`
	ResponseCount int      `json:"response_count"`
	IsVerified   bool      `json:"is_verified"`
}

// AssemblyConstituency represents an Assembly Constituency
type AssemblyConstituency struct {
	ID           ACID              `json:"id"`
	Name         string            `json:"name"`
	NameLocal    string            `json:"name_local,omitempty"`
	State        string            `json:"state"`
	PCID         PCID              `json:"pc_id"`
	VoterCount   int               `json:"voter_count,omitempty"`
	BoothCount   int               `json:"booth_count,omitempty"`
	Boundary     *GeoPolygon       `json:"boundary,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ParliamentaryConstituency represents a Parliamentary Constituency
type ParliamentaryConstituency struct {
	ID           PCID              `json:"id"`
	Name         string            `json:"name"`
	NameLocal    string            `json:"name_local,omitempty"`
	State        string            `json:"state"`
	ACIDs        []ACID            `json:"ac_ids"`
	VoterCount   int               `json:"voter_count,omitempty"`
	Boundary     *GeoPolygon       `json:"boundary,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// PollingBooth represents a polling booth/station
type PollingBooth struct {
	ID           BoothID           `json:"id"`
	Name         string            `json:"name"`
	NameLocal    string            `json:"name_local,omitempty"`
	ACID         ACID              `json:"ac_id"`
	Address      string            `json:"address,omitempty"`
	Location     *LatLng           `json:"location,omitempty"`
	VoterCount   int               `json:"voter_count,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Action    string                 `json:"action"`
	ActorID   string                 `json:"actor_id,omitempty"`
	ActorType string                 `json:"actor_type,omitempty"`
	ResourceType string              `json:"resource_type"`
	ResourceID string                `json:"resource_id,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	IPAddress string                 `json:"ip_address,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
}

// Pagination contains pagination parameters
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// HasMore returns true if there are more pages
func (p Pagination) HasMore() bool {
	return p.Page*p.PageSize < p.Total
}

// Offset returns the offset for database queries
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// TotalPages returns the total number of pages
func (p Pagination) TotalPages() int {
	if p.PageSize <= 0 {
		return 0
	}
	return (p.Total + p.PageSize - 1) / p.PageSize
}

// SortOrder represents sort order
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// Filter represents a generic filter
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, ne, gt, gte, lt, lte, in, like
	Value    interface{} `json:"value"`
}

// HealthStatus represents system health status
type HealthStatus struct {
	Status    string            `json:"status"` // healthy, degraded, unhealthy
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version,omitempty"`
	Components map[string]string `json:"components,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// RateLimitInfo contains rate limiting information
type RateLimitInfo struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
}

// IsExhausted returns true if rate limit is exhausted
func (r RateLimitInfo) IsExhausted() bool {
	return r.Remaining <= 0
}

// Language codes for supported Indian languages
const (
	LangHindi    = "hi"
	LangEnglish  = "en"
	LangBengali  = "bn"
	LangTelugu   = "te"
	LangMarathi  = "mr"
	LangTamil    = "ta"
	LangGujarati = "gu"
	LangUrdu     = "ur"
	LangKannada  = "kn"
	LangOdia     = "or"
	LangMalayalam = "ml"
	LangPunjabi  = "pa"
)

// SupportedLanguages returns all supported language codes
func SupportedLanguages() []string {
	return []string{
		LangHindi, LangEnglish, LangBengali, LangTelugu,
		LangMarathi, LangTamil, LangGujarati, LangUrdu,
		LangKannada, LangOdia, LangMalayalam, LangPunjabi,
	}
}

// IsLanguageSupported checks if a language is supported
func IsLanguageSupported(lang string) bool {
	for _, l := range SupportedLanguages() {
		if l == lang {
			return true
		}
	}
	return false
}

// Indian states and UTs
var (
	IndianStates = []string{
		"Andhra Pradesh", "Arunachal Pradesh", "Assam", "Bihar", "Chhattisgarh",
		"Goa", "Gujarat", "Haryana", "Himachal Pradesh", "Jharkhand",
		"Karnataka", "Kerala", "Madhya Pradesh", "Maharashtra", "Manipur",
		"Meghalaya", "Mizoram", "Nagaland", "Odisha", "Punjab",
		"Rajasthan", "Sikkim", "Tamil Nadu", "Telangana", "Tripura",
		"Uttar Pradesh", "Uttarakhand", "West Bengal",
	}

	IndianUTs = []string{
		"Andaman and Nicobar Islands", "Chandigarh", "Dadra and Nagar Haveli and Daman and Diu",
		"Delhi", "Jammu and Kashmir", "Ladakh", "Lakshadweep", "Puducherry",
	}
)

// IsValidState checks if a state name is valid
func IsValidState(state string) bool {
	for _, s := range IndianStates {
		if s == state {
			return true
		}
	}
	for _, s := range IndianUTs {
		if s == state {
			return true
		}
	}
	return false
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors map[string]string `json:"errors,omitempty"`
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:  true,
		Errors: make(map[string]string),
	}
}

// AddError adds a validation error
func (v *ValidationResult) AddError(field, message string) {
	v.Valid = false
	v.Errors[field] = message
}

// HasErrors returns true if there are validation errors
func (v *ValidationResult) HasErrors() bool {
	return len(v.Errors) > 0
}

// Merge merges another validation result into this one
func (v *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}
	for field, msg := range other.Errors {
		v.AddError(field, msg)
	}
}
