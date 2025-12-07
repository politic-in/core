// Package civicscore implements the transparent Civic Score calculation formula.
// This is open-source so users can verify exactly how their score is computed.
package civicscore

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Error definitions
var (
	ErrInvalidScore    = errors.New("invalid score value")
	ErrInvalidAction   = errors.New("invalid action type")
	ErrScoreOutOfRange = errors.New("score out of valid range")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidUserID   = errors.New("invalid user ID")
)

// ActionType defines the types of actions that affect civic score
type ActionType string

const (
	KYCCompleted         ActionType = "kyc_completed"
	BoothChallengePassed ActionType = "booth_challenge_passed"
	IssueVerified        ActionType = "issue_verified"
	VerificationGiven    ActionType = "verification_given"
	PollCompleted        ActionType = "poll_completed"
	IssueFixed           ActionType = "issue_fixed"
	AccountAge30         ActionType = "account_age_30"
	AccountAge90         ActionType = "account_age_90"
	AccountAge180        ActionType = "account_age_180"
	FakeVerification     ActionType = "fake_verification"
	FakeIssueReported    ActionType = "fake_issue_reported"
	LowQualityResponse   ActionType = "low_quality_response"
	Inactive60Days       ActionType = "inactive_60_days"
	DailyLogin           ActionType = "daily_login"
	StreakBonus7Days     ActionType = "streak_bonus_7_days"
	StreakBonus30Days    ActionType = "streak_bonus_30_days"
	ReferralBonus        ActionType = "referral_bonus"
	FirstIssueBonus      ActionType = "first_issue_bonus"
	FirstPollBonus       ActionType = "first_poll_bonus"
	TopContributor       ActionType = "top_contributor"
)

// Points defines the point value for each action type
// These are public constants so anyone can verify the formula
var Points = map[ActionType]int{
	KYCCompleted:         10,
	BoothChallengePassed: 15,
	IssueVerified:        5,  // Per issue
	VerificationGiven:    2,  // Per verification (if confirmed real)
	PollCompleted:        1,  // Per poll
	IssueFixed:           10, // When reporter's issue is fixed
	AccountAge30:         5,
	AccountAge90:         5,
	AccountAge180:        5,
	FakeVerification:     -10,
	FakeIssueReported:    -15,
	LowQualityResponse:   -5,
	Inactive60Days:       -10,
	DailyLogin:           1,
	StreakBonus7Days:     3,
	StreakBonus30Days:    10,
	ReferralBonus:        5,
	FirstIssueBonus:      5,
	FirstPollBonus:       3,
	TopContributor:       15,
}

// Score boundaries
const (
	// MinScore is the minimum possible civic score
	MinScore = 0

	// MaxScore is the maximum possible civic score
	MaxScore = 100

	// DefaultStartScore is the score new users start with
	DefaultStartScore = 20

	// TopResponderThreshold is the minimum score to be a Top Responder
	TopResponderThreshold = 60

	// PowerUserThreshold is the minimum score to be a Power User
	PowerUserThreshold = 85

	// InactivityDays is the number of days of inactivity before decay kicks in
	InactivityDays = 60

	// DecayPerWeek is the score decay per week after inactivity threshold
	DecayPerWeek = 2
)

// Level represents a user's level based on their civic score
type Level string

const (
	LevelNewUser      Level = "new_user"
	LevelActive       Level = "active"
	LevelTopResponder Level = "top_responder"
	LevelPowerUser    Level = "power_user"
)

// Badge represents achievements users can earn
type Badge string

const (
	BadgeVerified       Badge = "verified"        // Completed KYC
	BadgeLocalExpert    Badge = "local_expert"    // Passed booth challenge
	BadgeIssueReporter  Badge = "issue_reporter"  // Reported 5+ verified issues
	BadgeVerifier       Badge = "verifier"        // Verified 20+ issues
	BadgePollster       Badge = "pollster"        // Completed 50+ polls
	BadgeFixer          Badge = "fixer"           // Fixed 3+ issues
	BadgeStreaker       Badge = "streaker"        // 30-day login streak
	BadgeTopContributor Badge = "top_contributor" // Ranked top 10% in AC
	BadgeFounder        Badge = "founder"         // Early adopter (first 1000 users)
)

// Action represents a user action that affects civic score
type Action struct {
	Type        ActionType
	Count       int       // For actions that can occur multiple times
	Timestamp   time.Time // When the action occurred
	ReferenceID string    // Optional reference (issue_id, poll_id, etc.)
}

// ScoreLog represents a log entry for score changes
type ScoreLog struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Action        ActionType `json:"action"`
	Points        int        `json:"points"`
	ScoreBefore   int        `json:"score_before"`
	ScoreAfter    int        `json:"score_after"`
	ReferenceType string     `json:"reference_type,omitempty"`
	ReferenceID   string     `json:"reference_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// UserScore represents a user's complete score state
type UserScore struct {
	UserID             string    `json:"user_id"`
	Score              int       `json:"score"`
	Level              Level     `json:"level"`
	Badges             []Badge   `json:"badges"`
	IssuesReported     int       `json:"issues_reported"`
	IssuesVerified     int       `json:"issues_verified"`
	VerificationsGiven int       `json:"verifications_given"`
	PollsCompleted     int       `json:"polls_completed"`
	IssuesFixed        int       `json:"issues_fixed"`
	LoginStreak        int       `json:"login_streak"`
	LastActiveAt       time.Time `json:"last_active_at"`
	AccountCreatedAt   time.Time `json:"account_created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ScoreBreakdown shows how a score is composed
type ScoreBreakdown struct {
	BaseScore    int `json:"base_score"`
	KYCBonus     int `json:"kyc_bonus"`
	BoothBonus   int `json:"booth_bonus"`
	IssuePoints  int `json:"issue_points"`
	VerifyPoints int `json:"verify_points"`
	PollPoints   int `json:"poll_points"`
	AgeBonus     int `json:"age_bonus"`
	StreakBonus  int `json:"streak_bonus"`
	Penalties    int `json:"penalties"`
	TotalScore   int `json:"total_score"`
}

// Calculator handles civic score calculations
type Calculator struct {
	mu           sync.RWMutex
	customPoints map[ActionType]int
}

// NewCalculator creates a new score calculator
func NewCalculator() *Calculator {
	return &Calculator{
		customPoints: make(map[ActionType]int),
	}
}

// NewCalculatorWithCustomPoints creates a calculator with custom point values
func NewCalculatorWithCustomPoints(points map[ActionType]int) *Calculator {
	c := NewCalculator()
	for k, v := range points {
		c.customPoints[k] = v
	}
	return c
}

// GetPoints returns the point value for an action type
func (c *Calculator) GetPoints(action ActionType) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if pts, ok := c.customPoints[action]; ok {
		return pts
	}
	if pts, ok := Points[action]; ok {
		return pts
	}
	return 0
}

// Calculate computes the new civic score given a starting score and actions
func Calculate(currentScore int, actions []Action) int {
	calc := NewCalculator()
	return calc.CalculateWithActions(currentScore, actions)
}

// CalculateWithActions computes the new civic score
func (c *Calculator) CalculateWithActions(currentScore int, actions []Action) int {
	score := currentScore

	for _, action := range actions {
		points := c.GetPoints(action.Type)

		// For countable actions, multiply by count
		if action.Count > 0 {
			points *= action.Count
		}

		score += points
	}

	// Clamp to valid range
	return clamp(score, MinScore, MaxScore)
}

// CalculateSingle computes the score change for a single action
func CalculateSingle(currentScore int, actionType ActionType, count int) (newScore int, delta int) {
	calc := NewCalculator()
	return calc.ApplyAction(currentScore, actionType, count)
}

// ApplyAction applies a single action and returns the new score and delta
func (c *Calculator) ApplyAction(currentScore int, actionType ActionType, count int) (newScore int, delta int) {
	points := c.GetPoints(actionType)
	if points == 0 {
		return currentScore, 0
	}

	if count > 0 {
		points *= count
	}

	newScore = clamp(currentScore+points, MinScore, MaxScore)
	delta = newScore - currentScore

	return newScore, delta
}

// ApplyDecay applies inactivity decay to a score
func ApplyDecay(currentScore int, lastActiveAt time.Time, now time.Time) (newScore int, weeksDecayed int) {
	daysSinceActive := int(now.Sub(lastActiveAt).Hours() / 24)

	if daysSinceActive < InactivityDays {
		return currentScore, 0
	}

	// Calculate weeks of inactivity after threshold
	weeksInactive := (daysSinceActive - InactivityDays) / 7
	if weeksInactive <= 0 {
		return currentScore, 0
	}

	totalDecay := weeksInactive * DecayPerWeek
	newScore = clamp(currentScore-totalDecay, MinScore, MaxScore)

	return newScore, weeksInactive
}

// CalculateAgeBonus calculates the bonus based on account age
func CalculateAgeBonus(accountCreatedAt time.Time, now time.Time) (bonus int, milestones []ActionType) {
	days := int(now.Sub(accountCreatedAt).Hours() / 24)

	if days >= 180 {
		bonus = Points[AccountAge30] + Points[AccountAge90] + Points[AccountAge180]
		milestones = []ActionType{AccountAge30, AccountAge90, AccountAge180}
	} else if days >= 90 {
		bonus = Points[AccountAge30] + Points[AccountAge90]
		milestones = []ActionType{AccountAge30, AccountAge90}
	} else if days >= 30 {
		bonus = Points[AccountAge30]
		milestones = []ActionType{AccountAge30}
	}

	return bonus, milestones
}

// CalculateStreakBonus calculates bonus based on login streak
func CalculateStreakBonus(streakDays int) (bonus int, milestone ActionType) {
	if streakDays >= 30 {
		return Points[StreakBonus30Days], StreakBonus30Days
	}
	if streakDays >= 7 {
		return Points[StreakBonus7Days], StreakBonus7Days
	}
	return 0, ""
}

// IsTopResponder checks if a user qualifies as a Top Responder
func IsTopResponder(civicScore int, issuesReported int, verificationsGiven int, accountAgeDays int) bool {
	return civicScore >= TopResponderThreshold &&
		issuesReported >= 5 &&
		verificationsGiven >= 10 &&
		accountAgeDays >= 30
}

// IsPowerUser checks if a user qualifies as a Power User
func IsPowerUser(civicScore int, issuesReported int, verificationsGiven int, pollsCompleted int) bool {
	return civicScore >= PowerUserThreshold &&
		issuesReported >= 15 &&
		verificationsGiven >= 50 &&
		pollsCompleted >= 100
}

// GetLevel returns the user level based on civic score
func GetLevel(score int) Level {
	switch {
	case score >= PowerUserThreshold:
		return LevelPowerUser
	case score >= TopResponderThreshold:
		return LevelTopResponder
	case score >= 31:
		return LevelActive
	default:
		return LevelNewUser
	}
}

// GetPollAccess returns what poll types a user can access
func GetPollAccess(score int) []string {
	switch {
	case score >= TopResponderThreshold:
		return []string{"micro", "detailed"}
	case score >= 31:
		return []string{"micro", "some_detailed"}
	default:
		return []string{"micro"}
	}
}

// GetEarningMultiplier returns the earning multiplier based on level
func GetEarningMultiplier(level Level) float64 {
	switch level {
	case LevelPowerUser:
		return 1.5 // 50% bonus
	case LevelTopResponder:
		return 1.25 // 25% bonus
	case LevelActive:
		return 1.1 // 10% bonus
	default:
		return 1.0
	}
}

// CalculateBadges determines which badges a user has earned
func CalculateBadges(user *UserScore) []Badge {
	var badges []Badge

	// Verified badge (KYC completed)
	// This should be determined by checking if KYC action exists
	if user.Score >= 30 { // Has at least KYC bonus
		badges = append(badges, BadgeVerified)
	}

	// Issue Reporter badge (5+ verified issues)
	if user.IssuesVerified >= 5 {
		badges = append(badges, BadgeIssueReporter)
	}

	// Verifier badge (20+ verifications)
	if user.VerificationsGiven >= 20 {
		badges = append(badges, BadgeVerifier)
	}

	// Pollster badge (50+ polls)
	if user.PollsCompleted >= 50 {
		badges = append(badges, BadgePollster)
	}

	// Fixer badge (3+ fixes)
	if user.IssuesFixed >= 3 {
		badges = append(badges, BadgeFixer)
	}

	// Streaker badge (30-day streak)
	if user.LoginStreak >= 30 {
		badges = append(badges, BadgeStreaker)
	}

	return badges
}

// CalculateBreakdown provides a detailed breakdown of score components
func CalculateBreakdown(user *UserScore) *ScoreBreakdown {
	breakdown := &ScoreBreakdown{
		BaseScore: DefaultStartScore,
	}

	// Estimate component contributions
	if user.Score >= DefaultStartScore+Points[KYCCompleted] {
		breakdown.KYCBonus = Points[KYCCompleted]
	}

	if user.Score >= DefaultStartScore+Points[KYCCompleted]+Points[BoothChallengePassed] {
		breakdown.BoothBonus = Points[BoothChallengePassed]
	}

	breakdown.IssuePoints = user.IssuesVerified * Points[IssueVerified]
	breakdown.VerifyPoints = user.VerificationsGiven * Points[VerificationGiven]
	breakdown.PollPoints = user.PollsCompleted * Points[PollCompleted]

	// Age bonus
	breakdown.AgeBonus, _ = CalculateAgeBonus(user.AccountCreatedAt, time.Now())

	// Streak bonus
	breakdown.StreakBonus, _ = CalculateStreakBonus(user.LoginStreak)

	// Calculate total
	breakdown.TotalScore = breakdown.BaseScore +
		breakdown.KYCBonus +
		breakdown.BoothBonus +
		breakdown.IssuePoints +
		breakdown.VerifyPoints +
		breakdown.PollPoints +
		breakdown.AgeBonus +
		breakdown.StreakBonus +
		breakdown.Penalties

	// Clamp to actual score (there might be other factors)
	if breakdown.TotalScore > user.Score {
		// Adjust for penalties we couldn't track
		breakdown.Penalties = user.Score - (breakdown.TotalScore - breakdown.Penalties)
		breakdown.TotalScore = user.Score
	}

	return breakdown
}

// GetNextMilestone returns the next achievable milestone and points needed
func GetNextMilestone(user *UserScore) (milestone string, pointsNeeded int) {
	score := user.Score

	if score < 31 {
		return "Active User", 31 - score
	}
	if score < TopResponderThreshold {
		return "Top Responder", TopResponderThreshold - score
	}
	if score < PowerUserThreshold {
		return "Power User", PowerUserThreshold - score
	}
	return "Maximum Score", MaxScore - score
}

// GetRankPercentile estimates a user's percentile based on score
// This is a simplified model; actual percentiles would come from database
func GetRankPercentile(score int) float64 {
	// Assuming normal distribution centered around 35 with std dev of 15
	// This is a rough approximation
	if score >= 90 {
		return 99.0
	}
	if score >= 80 {
		return 95.0
	}
	if score >= 70 {
		return 85.0
	}
	if score >= 60 {
		return 70.0
	}
	if score >= 50 {
		return 50.0
	}
	if score >= 40 {
		return 30.0
	}
	if score >= 30 {
		return 15.0
	}
	return 5.0
}

// ValidateAction checks if an action type is valid
func ValidateAction(action ActionType) error {
	if _, ok := Points[action]; ok {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInvalidAction, action)
}

// ValidateScore checks if a score is within valid range
func ValidateScore(score int) error {
	if score < MinScore || score > MaxScore {
		return fmt.Errorf("%w: %d (must be %d-%d)", ErrScoreOutOfRange, score, MinScore, MaxScore)
	}
	return nil
}

// CreateScoreLog creates a new score log entry
func CreateScoreLog(userID string, action ActionType, points, scoreBefore, scoreAfter int, refType, refID string) *ScoreLog {
	return &ScoreLog{
		UserID:        userID,
		Action:        action,
		Points:        points,
		ScoreBefore:   scoreBefore,
		ScoreAfter:    scoreAfter,
		ReferenceType: refType,
		ReferenceID:   refID,
		CreatedAt:     time.Now(),
	}
}

// NewUserScore creates a new user score with default values
func NewUserScore(userID string) *UserScore {
	now := time.Now()
	return &UserScore{
		UserID:           userID,
		Score:            DefaultStartScore,
		Level:            LevelNewUser,
		Badges:           []Badge{},
		LastActiveAt:     now,
		AccountCreatedAt: now,
		UpdatedAt:        now,
	}
}

// UpdateLevel updates the user's level based on current score
func (u *UserScore) UpdateLevel() {
	u.Level = GetLevel(u.Score)
}

// UpdateBadges updates the user's badges based on current stats
func (u *UserScore) UpdateBadges() {
	u.Badges = CalculateBadges(u)
}

// ApplyAction applies an action to the user score
func (u *UserScore) ApplyAction(action ActionType, count int) (delta int) {
	var newScore int
	newScore, delta = CalculateSingle(u.Score, action, count)
	u.Score = newScore
	u.UpdateLevel()
	u.LastActiveAt = time.Now()
	u.UpdatedAt = time.Now()
	return delta
}

// ApplyDecay applies inactivity decay to user score
func (u *UserScore) ApplyDecay(now time.Time) (weeksDecayed int) {
	u.Score, weeksDecayed = ApplyDecay(u.Score, u.LastActiveAt, now)
	if weeksDecayed > 0 {
		u.UpdateLevel()
		u.UpdatedAt = now
	}
	return weeksDecayed
}

// IncrementStreak increments login streak
func (u *UserScore) IncrementStreak() {
	u.LoginStreak++
	u.LastActiveAt = time.Now()
	u.UpdatedAt = time.Now()
}

// ResetStreak resets login streak to 0
func (u *UserScore) ResetStreak() {
	u.LoginStreak = 0
	u.UpdatedAt = time.Now()
}

// Helper functions

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// PointsDescription returns a human-readable description of point values
func PointsDescription() map[ActionType]string {
	return map[ActionType]string{
		KYCCompleted:         "+10: Complete KYC verification",
		BoothChallengePassed: "+15: Pass Polling Station Challenge",
		IssueVerified:        "+5: Issue verified by neighbors (per issue)",
		VerificationGiven:    "+2: Verify a neighbor's issue (per verification)",
		PollCompleted:        "+1: Complete a poll (per poll)",
		IssueFixed:           "+10: Your reported issue was fixed",
		AccountAge30:         "+5: Account age reaches 30 days",
		AccountAge90:         "+5: Account age reaches 90 days",
		AccountAge180:        "+5: Account age reaches 180 days",
		FakeVerification:     "-10: Verified an issue that was fake",
		FakeIssueReported:    "-15: Reported issue flagged as fake",
		LowQualityResponse:   "-5: Poll response flagged as low quality",
		Inactive60Days:       "-10: Inactive for 60+ days",
		DailyLogin:           "+1: Daily login bonus",
		StreakBonus7Days:     "+3: 7-day login streak",
		StreakBonus30Days:    "+10: 30-day login streak",
		ReferralBonus:        "+5: Successful referral",
		FirstIssueBonus:      "+5: First issue reported",
		FirstPollBonus:       "+3: First poll completed",
		TopContributor:       "+15: Top contributor in your AC",
	}
}

// LevelDescription returns descriptions for each level
func LevelDescription() map[Level]string {
	return map[Level]string{
		LevelNewUser:      "New User (0-30): Limited poll access, building trust",
		LevelActive:       "Active (31-59): Access to most micro polls",
		LevelTopResponder: "Top Responder (60-84): Access to detailed polls, earning bonus",
		LevelPowerUser:    "Power User (85-100): Maximum access and earnings",
	}
}

// BadgeDescription returns descriptions for each badge
func BadgeDescription() map[Badge]string {
	return map[Badge]string{
		BadgeVerified:       "Verified: Completed identity verification",
		BadgeLocalExpert:    "Local Expert: Passed the Polling Station Challenge",
		BadgeIssueReporter:  "Issue Reporter: Reported 5+ verified issues",
		BadgeVerifier:       "Verifier: Verified 20+ neighbor issues",
		BadgePollster:       "Pollster: Completed 50+ polls",
		BadgeFixer:          "Fixer: Fixed 3+ community issues",
		BadgeStreaker:       "Streaker: Maintained 30-day login streak",
		BadgeTopContributor: "Top Contributor: Ranked top 10% in your AC",
		BadgeFounder:        "Founder: Among the first 1000 users",
	}
}
