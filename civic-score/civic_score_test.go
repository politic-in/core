package civicscore

import (
	"testing"
	"time"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name    string
		current int
		actions []Action
		want    int
	}{
		{
			name:    "no actions",
			current: 20,
			actions: []Action{},
			want:    20,
		},
		{
			name:    "single positive action",
			current: 20,
			actions: []Action{{Type: KYCCompleted}},
			want:    30,
		},
		{
			name:    "single negative action",
			current: 20,
			actions: []Action{{Type: FakeVerification}},
			want:    10,
		},
		{
			name:    "multiple actions",
			current: 20,
			actions: []Action{
				{Type: KYCCompleted},
				{Type: BoothChallengePassed},
				{Type: IssueVerified, Count: 2},
			},
			want: 55, // 20 + 10 + 15 + 10 = 55
		},
		{
			name:    "max score cap",
			current: 90,
			actions: []Action{
				{Type: KYCCompleted},
				{Type: BoothChallengePassed},
				{Type: TopContributor},
			},
			want: MaxScore, // Should cap at 100
		},
		{
			name:    "min score cap",
			current: 10,
			actions: []Action{
				{Type: FakeIssueReported},
				{Type: FakeVerification},
			},
			want: MinScore, // Should not go below 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calculate(tt.current, tt.actions)
			if got != tt.want {
				t.Errorf("Calculate() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCalculateSingle(t *testing.T) {
	tests := []struct {
		name      string
		current   int
		action    ActionType
		count     int
		wantScore int
		wantDelta int
	}{
		{
			name:      "KYC completed",
			current:   20,
			action:    KYCCompleted,
			count:     1,
			wantScore: 30,
			wantDelta: 10,
		},
		{
			name:      "multiple polls",
			current:   50,
			action:    PollCompleted,
			count:     5,
			wantScore: 55,
			wantDelta: 5,
		},
		{
			name:      "penalty",
			current:   30,
			action:    LowQualityResponse,
			count:     1,
			wantScore: 25,
			wantDelta: -5,
		},
		{
			name:      "invalid action",
			current:   30,
			action:    "invalid_action",
			count:     1,
			wantScore: 30,
			wantDelta: 0,
		},
		{
			name:      "capped at max",
			current:   98,
			action:    KYCCompleted,
			count:     1,
			wantScore: MaxScore,
			wantDelta: 2,
		},
		{
			name:      "capped at min",
			current:   5,
			action:    FakeIssueReported,
			count:     1,
			wantScore: MinScore,
			wantDelta: -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, delta := CalculateSingle(tt.current, tt.action, tt.count)
			if score != tt.wantScore {
				t.Errorf("score = %d, want %d", score, tt.wantScore)
			}
			if delta != tt.wantDelta {
				t.Errorf("delta = %d, want %d", delta, tt.wantDelta)
			}
		})
	}
}

func TestApplyDecay(t *testing.T) {
	tests := []struct {
		name         string
		current      int
		daysInactive int
		wantScore    int
		wantWeeks    int
	}{
		{
			name:         "no decay - active",
			current:      50,
			daysInactive: 30,
			wantScore:    50,
			wantWeeks:    0,
		},
		{
			name:         "no decay - at threshold",
			current:      50,
			daysInactive: 60,
			wantScore:    50,
			wantWeeks:    0,
		},
		{
			name:         "one week decay",
			current:      50,
			daysInactive: 67,
			wantScore:    48,
			wantWeeks:    1,
		},
		{
			name:         "two weeks decay",
			current:      50,
			daysInactive: 74,
			wantScore:    46,
			wantWeeks:    2,
		},
		{
			name:         "capped at min",
			current:      10,
			daysInactive: 180,
			wantScore:    MinScore,
			wantWeeks:    17, // (180-60)/7 = 17 weeks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			lastActive := now.Add(-time.Duration(tt.daysInactive) * 24 * time.Hour)

			score, weeks := ApplyDecay(tt.current, lastActive, now)
			if score != tt.wantScore {
				t.Errorf("score = %d, want %d", score, tt.wantScore)
			}
			if weeks != tt.wantWeeks {
				t.Errorf("weeks = %d, want %d", weeks, tt.wantWeeks)
			}
		})
	}
}

func TestCalculateAgeBonus(t *testing.T) {
	tests := []struct {
		name           string
		daysOld        int
		wantBonus      int
		wantMilestones int
	}{
		{
			name:           "new account",
			daysOld:        10,
			wantBonus:      0,
			wantMilestones: 0,
		},
		{
			name:           "30 day account",
			daysOld:        30,
			wantBonus:      5,
			wantMilestones: 1,
		},
		{
			name:           "90 day account",
			daysOld:        90,
			wantBonus:      10,
			wantMilestones: 2,
		},
		{
			name:           "180+ day account",
			daysOld:        200,
			wantBonus:      15,
			wantMilestones: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			created := now.Add(-time.Duration(tt.daysOld) * 24 * time.Hour)

			bonus, milestones := CalculateAgeBonus(created, now)
			if bonus != tt.wantBonus {
				t.Errorf("bonus = %d, want %d", bonus, tt.wantBonus)
			}
			if len(milestones) != tt.wantMilestones {
				t.Errorf("milestones = %d, want %d", len(milestones), tt.wantMilestones)
			}
		})
	}
}

func TestCalculateStreakBonus(t *testing.T) {
	tests := []struct {
		streak    int
		wantBonus int
		wantType  ActionType
	}{
		{0, 0, ""},
		{5, 0, ""},
		{7, 3, StreakBonus7Days},
		{15, 3, StreakBonus7Days},
		{30, 10, StreakBonus30Days},
		{100, 10, StreakBonus30Days},
	}

	for _, tt := range tests {
		bonus, milestone := CalculateStreakBonus(tt.streak)
		if bonus != tt.wantBonus {
			t.Errorf("CalculateStreakBonus(%d) bonus = %d, want %d", tt.streak, bonus, tt.wantBonus)
		}
		if milestone != tt.wantType {
			t.Errorf("CalculateStreakBonus(%d) type = %s, want %s", tt.streak, milestone, tt.wantType)
		}
	}
}

func TestIsTopResponder(t *testing.T) {
	tests := []struct {
		score  int
		issues int
		verifs int
		days   int
		want   bool
	}{
		{60, 5, 10, 30, true},    // All requirements met
		{59, 5, 10, 30, false},   // Score too low
		{60, 4, 10, 30, false},   // Not enough issues
		{60, 5, 9, 30, false},    // Not enough verifications
		{60, 5, 10, 29, false},   // Account too new
		{100, 20, 50, 180, true}, // All requirements exceeded
	}

	for _, tt := range tests {
		got := IsTopResponder(tt.score, tt.issues, tt.verifs, tt.days)
		if got != tt.want {
			t.Errorf("IsTopResponder(%d, %d, %d, %d) = %v, want %v",
				tt.score, tt.issues, tt.verifs, tt.days, got, tt.want)
		}
	}
}

func TestIsPowerUser(t *testing.T) {
	tests := []struct {
		score  int
		issues int
		verifs int
		polls  int
		want   bool
	}{
		{85, 15, 50, 100, true},   // All requirements met
		{84, 15, 50, 100, false},  // Score too low
		{85, 14, 50, 100, false},  // Not enough issues
		{85, 15, 49, 100, false},  // Not enough verifications
		{85, 15, 50, 99, false},   // Not enough polls
		{100, 30, 100, 200, true}, // All requirements exceeded
	}

	for _, tt := range tests {
		got := IsPowerUser(tt.score, tt.issues, tt.verifs, tt.polls)
		if got != tt.want {
			t.Errorf("IsPowerUser(%d, %d, %d, %d) = %v, want %v",
				tt.score, tt.issues, tt.verifs, tt.polls, got, tt.want)
		}
	}
}

func TestGetLevel(t *testing.T) {
	tests := []struct {
		score int
		want  Level
	}{
		{0, LevelNewUser},
		{20, LevelNewUser},
		{30, LevelNewUser},
		{31, LevelActive},
		{59, LevelActive},
		{60, LevelTopResponder},
		{84, LevelTopResponder},
		{85, LevelPowerUser},
		{100, LevelPowerUser},
	}

	for _, tt := range tests {
		got := GetLevel(tt.score)
		if got != tt.want {
			t.Errorf("GetLevel(%d) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestGetPollAccess(t *testing.T) {
	tests := []struct {
		score   int
		wantLen int
	}{
		{0, 1},  // Only micro
		{30, 1}, // Only micro
		{31, 2}, // micro + some_detailed
		{60, 2}, // micro + detailed
		{100, 2},
	}

	for _, tt := range tests {
		got := GetPollAccess(tt.score)
		if len(got) != tt.wantLen {
			t.Errorf("GetPollAccess(%d) len = %d, want %d", tt.score, len(got), tt.wantLen)
		}
	}
}

func TestGetEarningMultiplier(t *testing.T) {
	tests := []struct {
		level Level
		want  float64
	}{
		{LevelNewUser, 1.0},
		{LevelActive, 1.1},
		{LevelTopResponder, 1.25},
		{LevelPowerUser, 1.5},
	}

	for _, tt := range tests {
		got := GetEarningMultiplier(tt.level)
		if got != tt.want {
			t.Errorf("GetEarningMultiplier(%s) = %f, want %f", tt.level, got, tt.want)
		}
	}
}

func TestCalculateBadges(t *testing.T) {
	tests := []struct {
		name      string
		user      *UserScore
		wantLen   int
		wantBadge Badge
	}{
		{
			name:    "no badges",
			user:    &UserScore{Score: 20},
			wantLen: 0,
		},
		{
			name:      "verified badge",
			user:      &UserScore{Score: 30},
			wantLen:   1,
			wantBadge: BadgeVerified,
		},
		{
			name:    "issue reporter badge",
			user:    &UserScore{Score: 50, IssuesVerified: 5},
			wantLen: 2, // Verified + IssueReporter
		},
		{
			name:    "verifier badge",
			user:    &UserScore{Score: 50, VerificationsGiven: 20},
			wantLen: 2, // Verified + Verifier
		},
		{
			name:    "pollster badge",
			user:    &UserScore{Score: 50, PollsCompleted: 50},
			wantLen: 2, // Verified + Pollster
		},
		{
			name:    "fixer badge",
			user:    &UserScore{Score: 50, IssuesFixed: 3},
			wantLen: 2, // Verified + Fixer
		},
		{
			name:    "streaker badge",
			user:    &UserScore{Score: 50, LoginStreak: 30},
			wantLen: 2, // Verified + Streaker
		},
		{
			name: "all badges",
			user: &UserScore{
				Score:              50,
				IssuesVerified:     5,
				VerificationsGiven: 20,
				PollsCompleted:     50,
				IssuesFixed:        3,
				LoginStreak:        30,
			},
			wantLen: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			badges := CalculateBadges(tt.user)
			if len(badges) != tt.wantLen {
				t.Errorf("got %d badges, want %d", len(badges), tt.wantLen)
			}
			if tt.wantBadge != "" {
				found := false
				for _, b := range badges {
					if b == tt.wantBadge {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected badge %s not found", tt.wantBadge)
				}
			}
		})
	}
}

func TestGetNextMilestone(t *testing.T) {
	tests := []struct {
		score      int
		wantName   string
		wantPoints int
	}{
		{20, "Active User", 11},
		{31, "Top Responder", 29},
		{60, "Power User", 25},
		{85, "Maximum Score", 15},
		{100, "Maximum Score", 0},
	}

	for _, tt := range tests {
		user := &UserScore{Score: tt.score}
		name, points := GetNextMilestone(user)
		if name != tt.wantName {
			t.Errorf("GetNextMilestone(%d) name = %s, want %s", tt.score, name, tt.wantName)
		}
		if points != tt.wantPoints {
			t.Errorf("GetNextMilestone(%d) points = %d, want %d", tt.score, points, tt.wantPoints)
		}
	}
}

func TestGetRankPercentile(t *testing.T) {
	// Higher scores should have higher percentiles
	p1 := GetRankPercentile(20)
	p2 := GetRankPercentile(50)
	p3 := GetRankPercentile(90)

	if p1 >= p2 || p2 >= p3 {
		t.Error("percentile should increase with score")
	}

	if p3 != 99.0 {
		t.Errorf("score 90+ should be 99th percentile, got %f", p3)
	}
}

func TestValidateAction(t *testing.T) {
	// Valid actions
	for action := range Points {
		if err := ValidateAction(action); err != nil {
			t.Errorf("ValidateAction(%s) returned error for valid action: %v", action, err)
		}
	}

	// Invalid action
	if err := ValidateAction("invalid_action"); err == nil {
		t.Error("ValidateAction should return error for invalid action")
	}
}

func TestValidateScore(t *testing.T) {
	tests := []struct {
		score   int
		wantErr bool
	}{
		{-1, true},
		{0, false},
		{50, false},
		{100, false},
		{101, true},
	}

	for _, tt := range tests {
		err := ValidateScore(tt.score)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateScore(%d) error = %v, wantErr %v", tt.score, err, tt.wantErr)
		}
	}
}

func TestNewUserScore(t *testing.T) {
	user := NewUserScore("user-123")

	if user.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", user.UserID)
	}
	if user.Score != DefaultStartScore {
		t.Errorf("Score = %d, want %d", user.Score, DefaultStartScore)
	}
	if user.Level != LevelNewUser {
		t.Errorf("Level = %s, want %s", user.Level, LevelNewUser)
	}
	if len(user.Badges) != 0 {
		t.Error("new user should have no badges")
	}
}

func TestUserScore_ApplyAction(t *testing.T) {
	user := NewUserScore("user-123")
	initialScore := user.Score

	delta := user.ApplyAction(KYCCompleted, 1)

	if delta != Points[KYCCompleted] {
		t.Errorf("delta = %d, want %d", delta, Points[KYCCompleted])
	}
	if user.Score != initialScore+Points[KYCCompleted] {
		t.Errorf("score = %d, want %d", user.Score, initialScore+Points[KYCCompleted])
	}
}

func TestUserScore_ApplyDecay(t *testing.T) {
	user := NewUserScore("user-123")
	user.Score = 50
	user.LastActiveAt = time.Now().Add(-90 * 24 * time.Hour) // 90 days ago

	weeks := user.ApplyDecay(time.Now())

	if weeks == 0 {
		t.Error("should have some decay after 90 days")
	}
	if user.Score >= 50 {
		t.Error("score should have decreased")
	}
}

func TestUserScore_IncrementStreak(t *testing.T) {
	user := NewUserScore("user-123")
	initialStreak := user.LoginStreak

	user.IncrementStreak()

	if user.LoginStreak != initialStreak+1 {
		t.Errorf("streak = %d, want %d", user.LoginStreak, initialStreak+1)
	}
}

func TestUserScore_ResetStreak(t *testing.T) {
	user := NewUserScore("user-123")
	user.LoginStreak = 15

	user.ResetStreak()

	if user.LoginStreak != 0 {
		t.Errorf("streak = %d, want 0", user.LoginStreak)
	}
}

func TestCalculateBreakdown(t *testing.T) {
	now := time.Now()
	user := &UserScore{
		Score:              50,
		IssuesVerified:     3,
		VerificationsGiven: 10,
		PollsCompleted:     5,
		LoginStreak:        7,
		AccountCreatedAt:   now.Add(-100 * 24 * time.Hour),
	}

	breakdown := CalculateBreakdown(user)

	if breakdown.BaseScore != DefaultStartScore {
		t.Errorf("base score = %d, want %d", breakdown.BaseScore, DefaultStartScore)
	}
	if breakdown.IssuePoints != 15 { // 3 * 5
		t.Errorf("issue points = %d, want 15", breakdown.IssuePoints)
	}
	if breakdown.VerifyPoints != 20 { // 10 * 2
		t.Errorf("verify points = %d, want 20", breakdown.VerifyPoints)
	}
	if breakdown.PollPoints != 5 { // 5 * 1
		t.Errorf("poll points = %d, want 5", breakdown.PollPoints)
	}
}

func TestCreateScoreLog(t *testing.T) {
	log := CreateScoreLog("user-123", KYCCompleted, 10, 20, 30, "kyc", "kyc-456")

	if log.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", log.UserID)
	}
	if log.Action != KYCCompleted {
		t.Errorf("Action = %s, want %s", log.Action, KYCCompleted)
	}
	if log.Points != 10 {
		t.Errorf("Points = %d, want 10", log.Points)
	}
	if log.ScoreBefore != 20 {
		t.Errorf("ScoreBefore = %d, want 20", log.ScoreBefore)
	}
	if log.ScoreAfter != 30 {
		t.Errorf("ScoreAfter = %d, want 30", log.ScoreAfter)
	}
}

func TestCalculator_CustomPoints(t *testing.T) {
	customPoints := map[ActionType]int{
		KYCCompleted: 20, // Override default 10
	}
	calc := NewCalculatorWithCustomPoints(customPoints)

	pts := calc.GetPoints(KYCCompleted)
	if pts != 20 {
		t.Errorf("custom points = %d, want 20", pts)
	}

	// Non-overridden should use default
	pts = calc.GetPoints(BoothChallengePassed)
	if pts != 15 {
		t.Errorf("default points = %d, want 15", pts)
	}
}

func TestPointsDescription(t *testing.T) {
	desc := PointsDescription()

	// Should have description for all defined points
	for action := range Points {
		if _, ok := desc[action]; !ok {
			t.Errorf("missing description for action: %s", action)
		}
	}
}

func TestLevelDescription(t *testing.T) {
	desc := LevelDescription()

	levels := []Level{LevelNewUser, LevelActive, LevelTopResponder, LevelPowerUser}
	for _, level := range levels {
		if _, ok := desc[level]; !ok {
			t.Errorf("missing description for level: %s", level)
		}
	}
}

func TestBadgeDescription(t *testing.T) {
	desc := BadgeDescription()

	badges := []Badge{
		BadgeVerified, BadgeLocalExpert, BadgeIssueReporter,
		BadgeVerifier, BadgePollster, BadgeFixer, BadgeStreaker,
		BadgeTopContributor, BadgeFounder,
	}
	for _, badge := range badges {
		if _, ok := desc[badge]; !ok {
			t.Errorf("missing description for badge: %s", badge)
		}
	}
}

// Benchmark tests
func BenchmarkCalculate(b *testing.B) {
	actions := []Action{
		{Type: KYCCompleted},
		{Type: BoothChallengePassed},
		{Type: IssueVerified, Count: 5},
		{Type: PollCompleted, Count: 10},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(20, actions)
	}
}

func BenchmarkCalculateSingle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateSingle(50, PollCompleted, 1)
	}
}

func BenchmarkCalculateBadges(b *testing.B) {
	user := &UserScore{
		Score:              80,
		IssuesVerified:     10,
		VerificationsGiven: 30,
		PollsCompleted:     60,
		IssuesFixed:        5,
		LoginStreak:        45,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateBadges(user)
	}
}

func BenchmarkGetLevel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetLevel(65)
	}
}
