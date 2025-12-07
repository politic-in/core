// Package types provides common types and errors for the Politic core packages.
package types

import (
	"errors"
	"testing"
	"time"
)

func TestWrapError(t *testing.T) {
	// Wrap a nil error
	if WrapError(nil, "context") != nil {
		t.Error("expected nil for nil error")
	}

	// Wrap an error
	err := WrapError(ErrInvalidInput, "processing")
	if err == nil {
		t.Error("expected non-nil error")
	}

	// Verify the error chain
	if !errors.Is(err, ErrInvalidInput) {
		t.Error("wrapped error should contain original error")
	}

	expectedMsg := "processing: invalid input"
	if err.Error() != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, err.Error())
	}
}

func TestIsError(t *testing.T) {
	wrappedErr := WrapError(ErrUserNotFound, "lookup")

	if !IsError(wrappedErr, ErrUserNotFound) {
		t.Error("expected IsError to return true for wrapped error")
	}

	if IsError(wrappedErr, ErrInvalidInput) {
		t.Error("expected IsError to return false for different error")
	}
}

func TestBoundingBox_Contains(t *testing.T) {
	bb := BoundingBox{
		MinLat: 28.0, MaxLat: 29.0,
		MinLng: 77.0, MaxLng: 78.0,
	}

	tests := []struct {
		name     string
		lat, lng float64
		want     bool
	}{
		{"inside", 28.5, 77.5, true},
		{"on min edge", 28.0, 77.0, true},
		{"on max edge", 29.0, 78.0, true},
		{"below min lat", 27.9, 77.5, false},
		{"above max lat", 29.1, 77.5, false},
		{"below min lng", 28.5, 76.9, false},
		{"above max lng", 28.5, 78.1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bb.Contains(tt.lat, tt.lng); got != tt.want {
				t.Errorf("Contains(%f, %f) = %v, want %v", tt.lat, tt.lng, got, tt.want)
			}
		})
	}
}

func TestBoundingBox_IsValid(t *testing.T) {
	tests := []struct {
		name string
		bb   BoundingBox
		want bool
	}{
		{"valid", BoundingBox{28.0, 77.0, 29.0, 78.0}, true},
		{"inverted lat", BoundingBox{29.0, 77.0, 28.0, 78.0}, false},
		{"inverted lng", BoundingBox{28.0, 78.0, 29.0, 77.0}, false},
		{"lat below -90", BoundingBox{-91.0, 0.0, 0.0, 0.0}, false},
		{"lat above 90", BoundingBox{0.0, 0.0, 91.0, 0.0}, false},
		{"lng below -180", BoundingBox{0.0, -181.0, 0.0, 0.0}, false},
		{"lng above 180", BoundingBox{0.0, 0.0, 0.0, 181.0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bb.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGeoPolygon_IsValid(t *testing.T) {
	validPoly := GeoPolygon{
		ExteriorRing: []LatLng{{28.0, 77.0}, {28.0, 78.0}, {29.0, 77.5}},
	}

	invalidPoly := GeoPolygon{
		ExteriorRing: []LatLng{{28.0, 77.0}, {29.0, 78.0}},
	}

	if !validPoly.IsValid() {
		t.Error("expected valid polygon to be valid")
	}

	if invalidPoly.IsValid() {
		t.Error("expected polygon with < 3 points to be invalid")
	}
}

func TestTimeRange_Contains(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	tr := TimeRange{Start: start, End: end}

	tests := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"before start", start.Add(-time.Hour), false},
		{"at start", start, true},
		{"middle", start.Add(180 * 24 * time.Hour), true},
		{"at end", end, true},
		{"after end", end.Add(time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tr.Contains(tt.t); got != tt.want {
				t.Errorf("Contains(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestTimeRange_Duration(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	tr := TimeRange{Start: start, End: end}

	if tr.Duration() != 24*time.Hour {
		t.Errorf("expected 24 hours, got %v", tr.Duration())
	}
}

func TestTimeRange_Overlaps(t *testing.T) {
	jan := TimeRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
	}

	tests := []struct {
		name  string
		other TimeRange
		want  bool
	}{
		{
			"overlapping",
			TimeRange{
				Start: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
			},
			true,
		},
		{
			"contained",
			TimeRange{
				Start: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			},
			true,
		},
		{
			"before",
			TimeRange{
				Start: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			false,
		},
		{
			"after",
			TimeRange{
				Start: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
			},
			false,
		},
		{
			"touching at end",
			TimeRange{
				Start: time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
				End:   time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jan.Overlaps(tt.other); got != tt.want {
				t.Errorf("Overlaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPagination_HasMore(t *testing.T) {
	tests := []struct {
		name string
		p    Pagination
		want bool
	}{
		{"has more", Pagination{Page: 1, PageSize: 10, Total: 25}, true},
		{"exact last page", Pagination{Page: 3, PageSize: 10, Total: 30}, false},
		{"no more", Pagination{Page: 3, PageSize: 10, Total: 25}, false},
		{"single page", Pagination{Page: 1, PageSize: 10, Total: 5}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.HasMore(); got != tt.want {
				t.Errorf("HasMore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPagination_Offset(t *testing.T) {
	tests := []struct {
		name string
		p    Pagination
		want int
	}{
		{"first page", Pagination{Page: 1, PageSize: 10}, 0},
		{"second page", Pagination{Page: 2, PageSize: 10}, 10},
		{"third page", Pagination{Page: 3, PageSize: 25}, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Offset(); got != tt.want {
				t.Errorf("Offset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPagination_TotalPages(t *testing.T) {
	tests := []struct {
		name string
		p    Pagination
		want int
	}{
		{"exact division", Pagination{PageSize: 10, Total: 30}, 3},
		{"with remainder", Pagination{PageSize: 10, Total: 25}, 3},
		{"single page", Pagination{PageSize: 10, Total: 5}, 1},
		{"empty", Pagination{PageSize: 10, Total: 0}, 0},
		{"zero page size", Pagination{PageSize: 0, Total: 10}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.TotalPages(); got != tt.want {
				t.Errorf("TotalPages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateLimitInfo_IsExhausted(t *testing.T) {
	exhausted := RateLimitInfo{Limit: 100, Remaining: 0}
	notExhausted := RateLimitInfo{Limit: 100, Remaining: 50}

	if !exhausted.IsExhausted() {
		t.Error("expected exhausted to be true")
	}

	if notExhausted.IsExhausted() {
		t.Error("expected exhausted to be false")
	}
}

func TestIsLanguageSupported(t *testing.T) {
	supported := []string{"hi", "en", "bn", "te", "mr", "ta", "gu", "ur", "kn", "or", "ml", "pa"}

	for _, lang := range supported {
		if !IsLanguageSupported(lang) {
			t.Errorf("expected %s to be supported", lang)
		}
	}

	unsupported := []string{"fr", "de", "es", "zh", "ja"}
	for _, lang := range unsupported {
		if IsLanguageSupported(lang) {
			t.Errorf("expected %s to be unsupported", lang)
		}
	}
}

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()

	if len(langs) != 12 {
		t.Errorf("expected 12 supported languages, got %d", len(langs))
	}

	// Verify Hindi and English are included
	hasHindi := false
	hasEnglish := false
	for _, l := range langs {
		if l == LangHindi {
			hasHindi = true
		}
		if l == LangEnglish {
			hasEnglish = true
		}
	}

	if !hasHindi {
		t.Error("expected Hindi to be in supported languages")
	}
	if !hasEnglish {
		t.Error("expected English to be in supported languages")
	}
}

func TestIsValidState(t *testing.T) {
	validStates := []string{"Delhi", "Maharashtra", "Karnataka", "Gujarat"}
	invalidStates := []string{"California", "London", "NotAState"}

	for _, state := range validStates {
		if !IsValidState(state) {
			t.Errorf("expected %s to be valid", state)
		}
	}

	for _, state := range invalidStates {
		if IsValidState(state) {
			t.Errorf("expected %s to be invalid", state)
		}
	}
}

func TestValidationResult(t *testing.T) {
	v := NewValidationResult()

	if !v.Valid {
		t.Error("new validation result should be valid")
	}

	if v.HasErrors() {
		t.Error("new validation result should not have errors")
	}

	// Add an error
	v.AddError("email", "invalid email format")

	if v.Valid {
		t.Error("validation result with error should not be valid")
	}

	if !v.HasErrors() {
		t.Error("validation result should have errors")
	}

	if v.Errors["email"] != "invalid email format" {
		t.Error("error message not stored correctly")
	}

	// Merge another validation result
	v2 := NewValidationResult()
	v2.AddError("phone", "invalid phone number")

	v.Merge(v2)

	if len(v.Errors) != 2 {
		t.Errorf("expected 2 errors after merge, got %d", len(v.Errors))
	}

	if v.Errors["phone"] != "invalid phone number" {
		t.Error("merged error not present")
	}

	// Merge nil should be safe
	v.Merge(nil)

	if len(v.Errors) != 2 {
		t.Error("merge nil should not change errors")
	}
}

func TestIndianStatesAndUTs(t *testing.T) {
	// Verify counts
	if len(IndianStates) != 28 {
		t.Errorf("expected 28 states, got %d", len(IndianStates))
	}

	if len(IndianUTs) != 8 {
		t.Errorf("expected 8 UTs, got %d", len(IndianUTs))
	}

	// Verify some known states
	expectedStates := []string{"Maharashtra", "Karnataka", "Tamil Nadu", "Uttar Pradesh"}
	for _, state := range expectedStates {
		found := false
		for _, s := range IndianStates {
			if s == state {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s in IndianStates", state)
		}
	}

	// Verify Delhi is a UT
	found := false
	for _, ut := range IndianUTs {
		if ut == "Delhi" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Delhi in IndianUTs")
	}
}

func TestErrorDefinitions(t *testing.T) {
	// Verify all error definitions are unique
	errs := []error{
		ErrInvalidInput, ErrNotFound, ErrAlreadyExists, ErrOperationFailed,
		ErrUnauthorized, ErrForbidden, ErrRateLimited, ErrTimeout,
		ErrInvalidUserID, ErrUserNotFound, ErrUserSuspended,
		ErrSurveyNotFound, ErrSurveyClosed, ErrAlreadyResponded,
		ErrInvalidLocation, ErrLocationRequired, ErrOutOfBounds,
		ErrElectionNotFound, ErrBlackoutActive, ErrInvalidPhase,
		ErrAnonymizationFailed, ErrDecryptionFailed, ErrInsufficientResponses,
	}

	seen := make(map[string]bool)
	for _, err := range errs {
		msg := err.Error()
		if seen[msg] {
			t.Errorf("duplicate error message: %s", msg)
		}
		seen[msg] = true
	}
}
