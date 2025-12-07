package data

import (
	"fmt"

	boothmatching "github.com/politic-in/core/booth-matching"
)

// BoothMatcherForAC creates a booth matcher for a specific AC using indexed data
func (g *GeoIndex) BoothMatcherForAC(stateSlug string, acNumber int) (*boothmatching.Matcher, error) {
	booths, err := g.GetBoothsForAC(stateSlug, acNumber)
	if err != nil {
		return nil, err
	}

	if len(booths) == 0 {
		return nil, fmt.Errorf("%w: no booths for AC %d in %s", ErrBoothNotFound, acNumber, stateSlug)
	}

	// Convert to booth-matching package format
	matchBooths := make([]boothmatching.Booth, len(booths))
	for i, booth := range booths {
		matchBooths[i] = boothmatching.Booth{
			ID:     booth.PartID,
			Number: fmt.Sprintf("%d", booth.PartNumber),
			Name:   booth.PartName,
			ACID:   booth.ACNumber,
		}
	}

	return boothmatching.NewMatcher(matchBooths), nil
}

// BoothMatcherForState creates a booth matcher for all booths in a state
func (g *GeoIndex) BoothMatcherForState(stateSlug string) (*boothmatching.Matcher, error) {
	booths, err := g.GetBoothsForState(stateSlug)
	if err != nil {
		return nil, err
	}

	if len(booths) == 0 {
		return nil, fmt.Errorf("%w: no booths for state %s", ErrBoothNotFound, stateSlug)
	}

	matchBooths := make([]boothmatching.Booth, len(booths))
	for i, booth := range booths {
		matchBooths[i] = boothmatching.Booth{
			ID:     booth.PartID,
			Number: fmt.Sprintf("%d", booth.PartNumber),
			Name:   booth.PartName,
			ACID:   booth.ACNumber,
		}
	}

	return boothmatching.NewMatcher(matchBooths), nil
}

// MatchBooth matches user input to a booth within an AC
func (g *GeoIndex) MatchBooth(stateSlug string, acNumber int, userInput string) (*boothmatching.MatchResult, error) {
	matcher, err := g.BoothMatcherForAC(stateSlug, acNumber)
	if err != nil {
		return nil, err
	}

	result, err := matcher.Match(userInput, acNumber)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// MatchBoothWithCandidates returns multiple match candidates for user review
func (g *GeoIndex) MatchBoothWithCandidates(stateSlug string, acNumber int, userInput string, limit int) ([]boothmatching.MatchResult, error) {
	matcher, err := g.BoothMatcherForAC(stateSlug, acNumber)
	if err != nil {
		return nil, err
	}

	results, err := matcher.MatchWithCandidates(userInput, acNumber, limit)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// EvaluateBoothChallenge evaluates a booth challenge attempt
func (g *GeoIndex) EvaluateBoothChallenge(stateSlug string, acNumber int, userInput string) (*boothmatching.ChallengeResult, error) {
	matcher, err := g.BoothMatcherForAC(stateSlug, acNumber)
	if err != nil {
		return nil, err
	}

	return matcher.EvaluateChallenge(userInput, acNumber)
}
