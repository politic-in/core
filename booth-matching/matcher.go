// Package boothmatching implements fuzzy matching for the Polling Station Challenge.
// Open-source so the community can improve accuracy for regional language variations.
package boothmatching

import (
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// Error definitions
var (
	ErrNoBoothsLoaded  = errors.New("no booths loaded in matcher")
	ErrInvalidInput    = errors.New("invalid input for matching")
	ErrACIDRequired    = errors.New("assembly constituency ID is required")
	ErrNoMatchFound    = errors.New("no matching booth found")
	ErrBelowConfidence = errors.New("match confidence below threshold")
)

// Constants
const (
	// MinConfidence is the minimum confidence required to accept a match
	MinConfidence = 0.7

	// HighConfidence threshold for exact or near-exact matches
	HighConfidence = 0.9

	// VeryHighConfidence for matches that are almost certainly correct
	VeryHighConfidence = 0.95

	// DefaultCandidateLimit is the default number of candidates to return
	DefaultCandidateLimit = 5

	// MaxInputLength prevents DoS via extremely long inputs
	MaxInputLength = 500
)

// MatchResult represents the result of a booth name match
type MatchResult struct {
	BoothID     int     `json:"booth_id"`
	BoothName   string  `json:"booth_name"`
	BoothNumber string  `json:"booth_number"`
	ACID        int     `json:"ac_id"`
	Confidence  float64 `json:"confidence"` // 0.0 to 1.0
	Distance    int     `json:"distance"`   // Levenshtein distance
	MatchType   string  `json:"match_type"` // "exact", "fuzzy", "phonetic"
}

// Booth represents a polling booth for matching
type Booth struct {
	ID             int
	Number         string // Booth number within AC
	Name           string
	NameNormalized string
	NamePhonetic   string // Phonetic encoding for sound-alike matching
	ACID           int
	Keywords       []string // Extracted keywords for partial matching
}

// Matcher provides booth name matching functionality
type Matcher struct {
	mu            sync.RWMutex
	booths        []Booth
	boothsByAC    map[int][]int    // AC ID -> booth indices
	exactIndex    map[string][]int // normalized name -> booth indices
	phoneticIndex map[string][]int // phonetic encoding -> booth indices
	keywordIndex  map[string][]int // keyword -> booth indices
	config        MatcherConfig
}

// MatcherConfig holds configuration for the matcher
type MatcherConfig struct {
	MinConfidence      float64
	MaxCandidates      int
	EnablePhonetic     bool
	EnableKeywordMatch bool
	CaseSensitive      bool
}

// DefaultMatcherConfig returns the default configuration
func DefaultMatcherConfig() MatcherConfig {
	return MatcherConfig{
		MinConfidence:      MinConfidence,
		MaxCandidates:      DefaultCandidateLimit,
		EnablePhonetic:     true,
		EnableKeywordMatch: true,
		CaseSensitive:      false,
	}
}

// NewMatcher creates a new booth matcher with the given booths
func NewMatcher(booths []Booth) *Matcher {
	return NewMatcherWithConfig(booths, DefaultMatcherConfig())
}

// NewMatcherWithConfig creates a matcher with custom configuration
func NewMatcherWithConfig(booths []Booth, config MatcherConfig) *Matcher {
	m := &Matcher{
		booths:        make([]Booth, len(booths)),
		boothsByAC:    make(map[int][]int),
		exactIndex:    make(map[string][]int),
		phoneticIndex: make(map[string][]int),
		keywordIndex:  make(map[string][]int),
		config:        config,
	}

	// Process and index booths
	for i, booth := range booths {
		// Ensure normalized name is set
		if booth.NameNormalized == "" {
			booth.NameNormalized = Normalize(booth.Name)
		}

		// Generate phonetic encoding
		if config.EnablePhonetic && booth.NamePhonetic == "" {
			booth.NamePhonetic = PhoneticEncode(booth.Name)
		}

		// Extract keywords
		if config.EnableKeywordMatch && len(booth.Keywords) == 0 {
			booth.Keywords = ExtractKeywords(booth.Name)
		}

		m.booths[i] = booth

		// Index by AC
		m.boothsByAC[booth.ACID] = append(m.boothsByAC[booth.ACID], i)

		// Exact index
		m.exactIndex[booth.NameNormalized] = append(m.exactIndex[booth.NameNormalized], i)

		// Phonetic index
		if booth.NamePhonetic != "" {
			m.phoneticIndex[booth.NamePhonetic] = append(m.phoneticIndex[booth.NamePhonetic], i)
		}

		// Keyword index
		for _, kw := range booth.Keywords {
			m.keywordIndex[kw] = append(m.keywordIndex[kw], i)
		}
	}

	return m
}

// Match finds the best matching booth for the given user input within an AC
func (m *Matcher) Match(userInput string, acID int) (*MatchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.booths) == 0 {
		return nil, ErrNoBoothsLoaded
	}

	if userInput == "" {
		return nil, ErrInvalidInput
	}

	if len(userInput) > MaxInputLength {
		userInput = userInput[:MaxInputLength]
	}

	candidates, err := m.MatchWithCandidates(userInput, acID, 1)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, ErrNoMatchFound
	}

	best := candidates[0]
	if best.Confidence < m.config.MinConfidence {
		return nil, ErrBelowConfidence
	}

	return &best, nil
}

// MatchWithCandidates returns top N matching booths
func (m *Matcher) MatchWithCandidates(userInput string, acID int, limit int) ([]MatchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.booths) == 0 {
		return nil, ErrNoBoothsLoaded
	}

	if userInput == "" {
		return nil, ErrInvalidInput
	}

	if len(userInput) > MaxInputLength {
		userInput = userInput[:MaxInputLength]
	}

	if limit <= 0 {
		limit = m.config.MaxCandidates
	}

	normalized := Normalize(userInput)
	phonetic := ""
	if m.config.EnablePhonetic {
		phonetic = PhoneticEncode(userInput)
	}
	keywords := []string{}
	if m.config.EnableKeywordMatch {
		keywords = ExtractKeywords(userInput)
	}

	// Get candidate booths from this AC
	boothIndices, ok := m.boothsByAC[acID]
	if !ok || len(boothIndices) == 0 {
		return []MatchResult{}, nil
	}

	var results []MatchResult

	// Check for exact match first
	if indices, ok := m.exactIndex[normalized]; ok {
		for _, idx := range indices {
			booth := m.booths[idx]
			if booth.ACID == acID {
				results = append(results, MatchResult{
					BoothID:     booth.ID,
					BoothName:   booth.Name,
					BoothNumber: booth.Number,
					ACID:        booth.ACID,
					Confidence:  1.0,
					Distance:    0,
					MatchType:   "exact",
				})
			}
		}
		if len(results) > 0 {
			return results[:min(len(results), limit)], nil
		}
	}

	// Score all booths in AC
	scored := make(map[int]float64) // booth index -> score
	matchTypes := make(map[int]string)

	for _, idx := range boothIndices {
		booth := m.booths[idx]

		// Fuzzy string matching
		distance := fuzzy.LevenshteinDistance(normalized, booth.NameNormalized)
		maxLen := max(len(normalized), len(booth.NameNormalized))

		var confidence float64
		if maxLen == 0 {
			continue
		}
		confidence = 1.0 - (float64(distance) / float64(maxLen))

		// Boost confidence for phonetic matches
		if m.config.EnablePhonetic && phonetic != "" && booth.NamePhonetic != "" {
			if phonetic == booth.NamePhonetic {
				confidence = math.Max(confidence, 0.85) // Phonetic match guarantees at least 0.85
				matchTypes[idx] = "phonetic"
			}
		}

		// Boost for keyword matches
		if m.config.EnableKeywordMatch && len(keywords) > 0 {
			matchedKeywords := 0
			for _, kw := range keywords {
				for _, bkw := range booth.Keywords {
					if kw == bkw || strings.Contains(bkw, kw) || strings.Contains(kw, bkw) {
						matchedKeywords++
						break
					}
				}
			}
			if matchedKeywords > 0 {
				keywordBonus := float64(matchedKeywords) / float64(len(keywords)) * 0.1
				confidence = math.Min(confidence+keywordBonus, 1.0)
			}
		}

		if confidence > 0 {
			scored[idx] = confidence
			if matchTypes[idx] == "" {
				matchTypes[idx] = "fuzzy"
			}
		}
	}

	// Convert to results and sort
	for idx, conf := range scored {
		booth := m.booths[idx]
		results = append(results, MatchResult{
			BoothID:     booth.ID,
			BoothName:   booth.Name,
			BoothNumber: booth.Number,
			ACID:        booth.ACID,
			Confidence:  conf,
			Distance:    fuzzy.LevenshteinDistance(normalized, booth.NameNormalized),
			MatchType:   matchTypes[idx],
		})
	}

	// Sort by confidence descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// MatchMultiple matches multiple inputs in batch (more efficient than individual calls)
func (m *Matcher) MatchMultiple(inputs []string, acID int) ([]*MatchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.booths) == 0 {
		return nil, ErrNoBoothsLoaded
	}

	results := make([]*MatchResult, len(inputs))

	for i, input := range inputs {
		// Temporarily release lock for each match to allow concurrent reads
		m.mu.RUnlock()
		result, err := m.Match(input, acID)
		m.mu.RLock()
		if err != nil {
			results[i] = nil
		} else {
			results[i] = result
		}
	}

	return results, nil
}

// IsExactMatch checks if the normalized input matches exactly
func (m *Matcher) IsExactMatch(userInput string, acID int) *MatchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalized := Normalize(userInput)

	indices, ok := m.exactIndex[normalized]
	if !ok {
		return nil
	}

	for _, idx := range indices {
		booth := m.booths[idx]
		if booth.ACID == acID {
			return &MatchResult{
				BoothID:     booth.ID,
				BoothName:   booth.Name,
				BoothNumber: booth.Number,
				ACID:        booth.ACID,
				Confidence:  1.0,
				Distance:    0,
				MatchType:   "exact",
			}
		}
	}

	return nil
}

// GetBoothsByAC returns all booths for a given AC
func (m *Matcher) GetBoothsByAC(acID int) []Booth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indices := m.boothsByAC[acID]
	booths := make([]Booth, len(indices))
	for i, idx := range indices {
		booths[i] = m.booths[idx]
	}
	return booths
}

// GetBoothCount returns total number of booths loaded
func (m *Matcher) GetBoothCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.booths)
}

// GetACCount returns number of unique ACs with booths
func (m *Matcher) GetACCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.boothsByAC)
}

// AddBooth adds a new booth to the matcher (thread-safe)
func (m *Matcher) AddBooth(booth Booth) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure normalized name is set
	if booth.NameNormalized == "" {
		booth.NameNormalized = Normalize(booth.Name)
	}

	// Generate phonetic encoding
	if m.config.EnablePhonetic && booth.NamePhonetic == "" {
		booth.NamePhonetic = PhoneticEncode(booth.Name)
	}

	// Extract keywords
	if m.config.EnableKeywordMatch && len(booth.Keywords) == 0 {
		booth.Keywords = ExtractKeywords(booth.Name)
	}

	idx := len(m.booths)
	m.booths = append(m.booths, booth)

	// Update indices
	m.boothsByAC[booth.ACID] = append(m.boothsByAC[booth.ACID], idx)
	m.exactIndex[booth.NameNormalized] = append(m.exactIndex[booth.NameNormalized], idx)
	if booth.NamePhonetic != "" {
		m.phoneticIndex[booth.NamePhonetic] = append(m.phoneticIndex[booth.NamePhonetic], idx)
	}
	for _, kw := range booth.Keywords {
		m.keywordIndex[kw] = append(m.keywordIndex[kw], idx)
	}
}

// Normalize prepares a string for comparison
// - Lowercase
// - Remove punctuation
// - Collapse whitespace
// - Handle common abbreviations
func Normalize(s string) string {
	s = strings.ToLower(s)

	// Apply abbreviation expansion
	s = ExpandAbbreviations(s)

	// Remove punctuation and extra whitespace
	var result strings.Builder
	lastWasSpace := false

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
			lastWasSpace = false
		} else if unicode.IsSpace(r) && !lastWasSpace {
			result.WriteRune(' ')
			lastWasSpace = true
		}
	}

	return strings.TrimSpace(result.String())
}

// Common abbreviations in Indian booth names (multi-lingual)
var abbreviations = map[string]string{
	// English
	"govt":      "government",
	"gov":       "government",
	"govt.":     "government",
	"gov.":      "government",
	"pri":       "primary",
	"pry":       "primary",
	"prim":      "primary",
	"sec":       "secondary",
	"sr":        "senior",
	"jr":        "junior",
	"sch":       "school",
	"schl":      "school",
	"bldg":      "building",
	"rd":        "road",
	"st":        "street",
	"no.":       "number",
	"blk":       "block",
	"comm":      "community",
	"cmty":      "community",
	"hosp":      "hospital",
	"hosp.":     "hospital",
	"dispensry": "dispensary",
	"disp":      "dispensary",
	"elem":      "elementary",
	"coll":      "college",
	"univ":      "university",
	"mun":       "municipal",
	"corp":      "corporation",
	"corpn":     "corporation",
	"dist":      "district",
	"hq":        "headquarters",
	"hdqtrs":    "headquarters",
	"opp":       "opposite",
	"nr":        "near",
	"adj":       "adjacent",
	"betw":      "between",
	"bhd":       "behind",
	"beh":       "behind",
	"frt":       "front",

	// Hindi/Common Indian
	"sarkar":    "government",
	"sarkari":   "government",
	"vidyalaya": "school",
	"vidya":     "school",
	"prathamik": "primary",
	"prath":     "primary",
	"madhyamik": "secondary",
	"uchcha":    "higher",
	"ucch":      "higher",
	"kendra":    "center",
	"bhavan":    "building",
	"marg":      "road",
	"sadak":     "road",
	"gali":      "lane",
	"mohalla":   "locality",
	"nagar":     "town",
	"puram":     "town",
	"abad":      "city",
	"gram":      "village",
	"gaon":      "village",
	"tal":       "taluka",
	"mandal":    "block",
	"panchayat": "council",
	"samiti":    "committee",
}

// ExpandAbbreviations expands common abbreviations in the input
func ExpandAbbreviations(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if expanded, ok := abbreviations[strings.ToLower(word)]; ok {
			words[i] = expanded
		}
	}
	return strings.Join(words, " ")
}

// ExtractKeywords extracts meaningful keywords from booth name
func ExtractKeywords(name string) []string {
	// Stopwords to ignore
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "of": true, "in": true,
		"at": true, "to": true, "for": true, "and": true, "or": true,
		"with": true, "by": true, "from": true, "is": true, "on": true,
		"part": true, "room": true, "hall": true, "building": true,
		// Hindi common words
		"ka": true, "ki": true, "ke": true, "se": true, "me": true,
		"par": true, "ko": true, "ne": true, "hai": true,
	}

	normalized := Normalize(name)
	words := strings.Fields(normalized)
	keywords := make([]string, 0, len(words))

	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		if stopwords[word] {
			continue
		}
		// Only include if it's likely a proper noun or significant word
		if len(word) >= 4 || unicode.IsUpper(rune(word[0])) {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// PhoneticEncode creates a phonetic encoding for sound-alike matching
// This is a simplified Soundex-like algorithm adapted for Indian names
func PhoneticEncode(s string) string {
	if s == "" {
		return ""
	}

	s = strings.ToLower(s)

	// Keep first letter
	var result strings.Builder
	if len(s) > 0 {
		result.WriteByte(s[0])
	}

	// Phonetic replacements for Indian English
	replacements := map[rune]byte{
		// Vowels are largely ignored (except first letter)
		'a': '0', 'e': '0', 'i': '0', 'o': '0', 'u': '0',
		// Labials
		'b': '1', 'f': '1', 'p': '1', 'v': '1',
		// Gutturals
		'c': '2', 'g': '2', 'j': '2', 'k': '2', 'q': '2', 's': '2', 'x': '2', 'z': '2',
		// Dentals
		'd': '3', 't': '3',
		// Long liquids
		'l': '4',
		// Nasals
		'm': '5', 'n': '5',
		// Short liquids
		'r': '6',
		// Special: handled differently
		'h': '0', 'w': '0', 'y': '0',
	}

	lastCode := byte('0')
	for i, r := range s {
		if i == 0 {
			if code, ok := replacements[r]; ok {
				lastCode = code
			}
			continue
		}

		if code, ok := replacements[r]; ok && code != '0' && code != lastCode {
			result.WriteByte(code)
			lastCode = code
		}

		// Limit length
		if result.Len() >= 6 {
			break
		}
	}

	// Pad to minimum length
	for result.Len() < 4 {
		result.WriteByte('0')
	}

	return result.String()
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// BoothFromDB is a helper to create a Booth from database row
func BoothFromDB(id int, number, name string, acID int) Booth {
	return Booth{
		ID:             id,
		Number:         number,
		Name:           name,
		NameNormalized: Normalize(name),
		NamePhonetic:   PhoneticEncode(name),
		ACID:           acID,
		Keywords:       ExtractKeywords(name),
	}
}

// ChallengeResult represents the outcome of a booth challenge attempt
type ChallengeResult struct {
	Passed          bool          `json:"passed"`
	BestMatch       *MatchResult  `json:"best_match,omitempty"`
	Candidates      []MatchResult `json:"candidates,omitempty"`
	AttemptedInput  string        `json:"attempted_input"`
	ACID            int           `json:"ac_id"`
	ConfidenceLevel string        `json:"confidence_level"` // "high", "medium", "low"
}

// EvaluateChallenge evaluates a user's booth challenge attempt
func (m *Matcher) EvaluateChallenge(userInput string, acID int) (*ChallengeResult, error) {
	candidates, err := m.MatchWithCandidates(userInput, acID, 3)
	if err != nil {
		return &ChallengeResult{
			Passed:         false,
			AttemptedInput: userInput,
			ACID:           acID,
		}, err
	}

	result := &ChallengeResult{
		AttemptedInput: userInput,
		ACID:           acID,
		Candidates:     candidates,
	}

	if len(candidates) == 0 {
		result.Passed = false
		result.ConfidenceLevel = "low"
		return result, nil
	}

	best := candidates[0]
	result.BestMatch = &best

	// Determine if passed and confidence level
	switch {
	case best.Confidence >= VeryHighConfidence:
		result.Passed = true
		result.ConfidenceLevel = "high"
	case best.Confidence >= HighConfidence:
		result.Passed = true
		result.ConfidenceLevel = "high"
	case best.Confidence >= MinConfidence:
		result.Passed = true
		result.ConfidenceLevel = "medium"
	default:
		result.Passed = false
		result.ConfidenceLevel = "low"
	}

	return result, nil
}
