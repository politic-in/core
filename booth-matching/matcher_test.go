package boothmatching

import (
	"strings"
	"sync"
	"testing"
)

func createTestBooths() []Booth {
	return []Booth{
		BoothFromDB(1, "1", "Government Primary School, 5th Block Jayanagar", 176),
		BoothFromDB(2, "2", "Govt. Higher Secondary School, Koramangala", 176),
		BoothFromDB(3, "3", "Community Hall, BTM Layout", 176),
		BoothFromDB(4, "4", "Municipal Corporation Office, Banashankari", 176),
		BoothFromDB(5, "5", "Sarkar Prathamik Vidyalaya, HSR Layout", 176),
		BoothFromDB(6, "1", "Primary School, Whitefield", 177),
		BoothFromDB(7, "2", "Gram Panchayat Office, Varthur", 177),
		BoothFromDB(8, "3", "Community Center, Marathahalli", 177),
	}
}

func TestNewMatcher(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	if m.GetBoothCount() != len(booths) {
		t.Errorf("booth count = %d, want %d", m.GetBoothCount(), len(booths))
	}

	if m.GetACCount() != 2 {
		t.Errorf("AC count = %d, want 2", m.GetACCount())
	}
}

func TestMatcher_Match_ExactMatch(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	// Test exact match with normalization
	result, err := m.Match("Government Primary School, 5th Block Jayanagar", 176)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BoothID != 1 {
		t.Errorf("booth ID = %d, want 1", result.BoothID)
	}
	if result.Confidence != 1.0 {
		t.Errorf("confidence = %f, want 1.0", result.Confidence)
	}
	if result.MatchType != "exact" {
		t.Errorf("match type = %s, want exact", result.MatchType)
	}
}

func TestMatcher_Match_FuzzyMatch(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	// Test fuzzy match (slight typo) - intentional misspellings for testing
	result, err := m.Match("Govenment Primary Scool 5th Blok Jayanagar", 176) //nolint:misspell
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BoothID != 1 {
		t.Errorf("booth ID = %d, want 1", result.BoothID)
	}
	if result.Confidence < MinConfidence {
		t.Errorf("confidence = %f, should be >= %f", result.Confidence, MinConfidence)
	}
}

func TestMatcher_Match_AbbreviationExpansion(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	// "Govt." should match "Government"
	result, err := m.Match("Govt Primary School 5th Block Jayanagar", 176)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BoothID != 1 {
		t.Errorf("booth ID = %d, want 1", result.BoothID)
	}
}

func TestMatcher_Match_HindiAbbreviations(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	// "Sarkar Prathamik Vidyalaya" should match government primary school
	result, err := m.Match("Sarkar Prathamik Vidyalaya HSR Layout", 176)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BoothID != 5 {
		t.Errorf("booth ID = %d, want 5", result.BoothID)
	}
}

func TestMatcher_Match_WrongAC(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	// Booth exists but in different AC
	result, err := m.Match("Government Primary School, 5th Block Jayanagar", 177)
	if err != ErrNoMatchFound && err != ErrBelowConfidence {
		// Should either not find or have low confidence
		if result != nil && result.Confidence >= MinConfidence {
			t.Error("should not match booth from different AC with high confidence")
		}
	}
}

func TestMatcher_Match_NoBooths(t *testing.T) {
	m := NewMatcher([]Booth{})

	_, err := m.Match("Some School", 176)
	if err != ErrNoBoothsLoaded {
		t.Errorf("expected ErrNoBoothsLoaded, got %v", err)
	}
}

func TestMatcher_Match_EmptyInput(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	_, err := m.Match("", 176)
	if err != ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestMatcher_Match_LongInput(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	// Test that long input is truncated and still works
	longInput := strings.Repeat("Government Primary School ", 100)
	_, err := m.Match(longInput, 176)
	// Should not panic or error due to length
	if err != nil && err != ErrBelowConfidence && err != ErrNoMatchFound {
		t.Errorf("unexpected error for long input: %v", err)
	}
}

func TestMatcher_MatchWithCandidates(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	candidates, err := m.MatchWithCandidates("School", 176, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) == 0 {
		t.Error("expected at least one candidate")
	}

	// Check candidates are sorted by confidence
	for i := 1; i < len(candidates); i++ {
		if candidates[i].Confidence > candidates[i-1].Confidence {
			t.Error("candidates should be sorted by confidence descending")
		}
	}
}

func TestMatcher_IsExactMatch(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	t.Run("exact match found", func(t *testing.T) {
		result := m.IsExactMatch("Government Primary School, 5th Block Jayanagar", 176)
		if result == nil {
			t.Error("expected to find exact match")
		}
		if result != nil && result.BoothID != 1 {
			t.Errorf("booth ID = %d, want 1", result.BoothID)
		}
	})

	t.Run("no exact match", func(t *testing.T) {
		result := m.IsExactMatch("Some Random School", 176)
		if result != nil {
			t.Error("expected no exact match")
		}
	})
}

func TestMatcher_GetBoothsByAC(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	ac176Booths := m.GetBoothsByAC(176)
	if len(ac176Booths) != 5 {
		t.Errorf("AC 176 booths = %d, want 5", len(ac176Booths))
	}

	ac177Booths := m.GetBoothsByAC(177)
	if len(ac177Booths) != 3 {
		t.Errorf("AC 177 booths = %d, want 3", len(ac177Booths))
	}

	ac999Booths := m.GetBoothsByAC(999)
	if len(ac999Booths) != 0 {
		t.Errorf("AC 999 booths = %d, want 0", len(ac999Booths))
	}
}

func TestMatcher_AddBooth(t *testing.T) {
	m := NewMatcher([]Booth{})

	m.AddBooth(BoothFromDB(1, "1", "Test School", 100))
	m.AddBooth(BoothFromDB(2, "2", "Test College", 100))

	if m.GetBoothCount() != 2 {
		t.Errorf("booth count = %d, want 2", m.GetBoothCount())
	}

	result, err := m.Match("Test School", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BoothID != 1 {
		t.Errorf("booth ID = %d, want 1", result.BoothID)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  Hello   World  ", "hello world"},
		{"GOVT. PRIMARY SCHOOL", "government primary school"},
		{"School, 5th Block", "school 5th block"},
		{"Govt Primary Sch", "government primary school"},
		{"Sarkar Vidyalaya", "government school"},
		{"", ""},
		{"Special!@#$%Chars", "specialchars"},
	}

	for _, tt := range tests {
		got := Normalize(tt.input)
		if got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExpandAbbreviations(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"govt school", "government school"},
		{"pri sch", "primary school"},
		{"sarkar vidyalaya", "government school"},
		{"no changes needed", "no changes needed"},
		{"GOVT HOSP", "government hospital"},
	}

	for _, tt := range tests {
		got := ExpandAbbreviations(tt.input)
		if got != tt.want {
			t.Errorf("ExpandAbbreviations(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		input       string
		wantMinLen  int
		wantContain string
	}{
		{"Government Primary School, Jayanagar", 2, "jayanagar"},
		{"The Office of Municipal Corporation", 1, "municipal"},
		{"A Hall", 0, ""},
	}

	for _, tt := range tests {
		keywords := ExtractKeywords(tt.input)
		if len(keywords) < tt.wantMinLen {
			t.Errorf("ExtractKeywords(%q) returned %d keywords, want at least %d",
				tt.input, len(keywords), tt.wantMinLen)
		}
		if tt.wantContain != "" {
			found := false
			for _, kw := range keywords {
				if kw == tt.wantContain {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ExtractKeywords(%q) should contain %q", tt.input, tt.wantContain)
			}
		}
	}
}

func TestPhoneticEncode(t *testing.T) {
	tests := []struct {
		input1 string
		input2 string
		match  bool
	}{
		// Similar sounding names should have same encoding
		{"Robert", "Rupert", true},
		{"Ashok", "Ashok", true},
		// Different names should have different encoding
		{"School", "Hospital", false},
		{"Primary", "Secondary", false},
	}

	for _, tt := range tests {
		enc1 := PhoneticEncode(tt.input1)
		enc2 := PhoneticEncode(tt.input2)

		if tt.match && enc1 != enc2 {
			t.Errorf("PhoneticEncode(%q) = %q, PhoneticEncode(%q) = %q; expected match",
				tt.input1, enc1, tt.input2, enc2)
		}
		if !tt.match && enc1 == enc2 {
			t.Errorf("PhoneticEncode(%q) = %q, PhoneticEncode(%q) = %q; expected no match",
				tt.input1, enc1, tt.input2, enc2)
		}
	}

	// Test minimum length padding
	enc := PhoneticEncode("A")
	if len(enc) < 4 {
		t.Errorf("PhoneticEncode should pad to at least 4 chars, got %d", len(enc))
	}

	// Test empty input
	if PhoneticEncode("") != "" {
		t.Error("PhoneticEncode of empty string should be empty")
	}
}

func TestBoothFromDB(t *testing.T) {
	booth := BoothFromDB(123, "45", "Govt Primary School", 176)

	if booth.ID != 123 {
		t.Errorf("ID = %d, want 123", booth.ID)
	}
	if booth.Number != "45" {
		t.Errorf("Number = %s, want 45", booth.Number)
	}
	if booth.Name != "Govt Primary School" {
		t.Error("Name should be preserved")
	}
	if booth.NameNormalized == "" {
		t.Error("NameNormalized should be set")
	}
	if booth.NamePhonetic == "" {
		t.Error("NamePhonetic should be set")
	}
	if len(booth.Keywords) == 0 {
		t.Error("Keywords should be extracted")
	}
	if booth.ACID != 176 {
		t.Errorf("ACID = %d, want 176", booth.ACID)
	}
}

func TestMatcher_EvaluateChallenge(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	t.Run("high confidence pass", func(t *testing.T) {
		result, err := m.EvaluateChallenge("Government Primary School, 5th Block Jayanagar", 176)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("challenge should pass for exact match")
		}
		if result.ConfidenceLevel != "high" {
			t.Errorf("confidence level = %s, want high", result.ConfidenceLevel)
		}
	})

	t.Run("medium confidence pass", func(t *testing.T) {
		// Slight variation should still pass with medium confidence
		result, err := m.EvaluateChallenge("Govt Primary School Jayanagar", 176)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("challenge should pass for close match")
		}
	})

	t.Run("low confidence fail", func(t *testing.T) {
		result, err := m.EvaluateChallenge("Random Text That Does Not Match", 176)
		if err != nil && err != ErrNoMatchFound && err != ErrBelowConfidence {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil && result.Passed {
			t.Error("challenge should fail for non-matching input")
		}
	})

	t.Run("candidates returned", func(t *testing.T) {
		result, _ := m.EvaluateChallenge("School", 176)
		if result != nil && len(result.Candidates) == 0 {
			t.Error("should return candidates even for partial matches")
		}
	})
}

func TestMatcherConfig(t *testing.T) {
	booths := createTestBooths()

	t.Run("default config", func(t *testing.T) {
		config := DefaultMatcherConfig()
		if config.MinConfidence != MinConfidence {
			t.Errorf("default min confidence = %f, want %f", config.MinConfidence, MinConfidence)
		}
		if !config.EnablePhonetic {
			t.Error("phonetic should be enabled by default")
		}
		if !config.EnableKeywordMatch {
			t.Error("keyword match should be enabled by default")
		}
	})

	t.Run("custom min confidence", func(t *testing.T) {
		config := MatcherConfig{
			MinConfidence:      0.9,
			MaxCandidates:      3,
			EnablePhonetic:     false,
			EnableKeywordMatch: false,
		}
		m := NewMatcherWithConfig(booths, config)

		// With high min confidence, fuzzy matches might not pass
		_, err := m.Match("Govt School Jayanagar", 176)
		if err == nil {
			// If it passes, confidence should be >= 0.9
			result, _ := m.Match("Govt School Jayanagar", 176)
			if result.Confidence < 0.9 {
				t.Error("result confidence should be >= configured minimum")
			}
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	booths := createTestBooths()
	m := NewMatcher(booths)

	var wg sync.WaitGroup

	// Concurrent reads - each goroutine does a few operations then exits
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Just a few operations, not a tight loop
			m.Match("Government Primary School", 176)
			m.GetBoothCount()
			m.GetBoothsByAC(176)
		}()
	}

	// Wait for all reads to complete before starting writes
	wg.Wait()

	// Test writes sequentially (no concurrent reads/writes to avoid RWMutex contention)
	for j := 0; j < 10; j++ {
		m.AddBooth(BoothFromDB(100+j, "X", "New School", 176))
	}

	// Verify writes succeeded
	if m.GetBoothCount() != len(booths)+10 {
		t.Errorf("expected %d booths, got %d", len(booths)+10, m.GetBoothCount())
	}
}

// Benchmark tests
func BenchmarkMatch(b *testing.B) {
	booths := make([]Booth, 1000)
	for i := 0; i < 1000; i++ {
		booths[i] = BoothFromDB(i, "X", "Government Primary School Number "+string(rune('A'+i%26)), i%10)
	}
	m := NewMatcher(booths)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match("Government Primary School", 5)
	}
}

func BenchmarkMatchWithCandidates(b *testing.B) {
	booths := make([]Booth, 1000)
	for i := 0; i < 1000; i++ {
		booths[i] = BoothFromDB(i, "X", "Government Primary School Number "+string(rune('A'+i%26)), i%10)
	}
	m := NewMatcher(booths)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.MatchWithCandidates("Government Primary School", 5, 5)
	}
}

func BenchmarkNormalize(b *testing.B) {
	input := "GOVT. Primary School, 5th Block Jayanagar"
	for i := 0; i < b.N; i++ {
		Normalize(input)
	}
}

func BenchmarkPhoneticEncode(b *testing.B) {
	input := "Government Primary School"
	for i := 0; i < b.N; i++ {
		PhoneticEncode(input)
	}
}

func BenchmarkBoothFromDB(b *testing.B) {
	for i := 0; i < b.N; i++ {
		BoothFromDB(123, "45", "Government Primary School Jayanagar", 176)
	}
}
