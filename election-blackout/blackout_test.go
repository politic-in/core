package blackout

import (
	"testing"
	"time"
)

func createTestElection() *Election {
	pollingDate := time.Now().Add(3 * 24 * time.Hour) // 3 days from now
	stateID := 29 // Karnataka
	return CreateElection("election-1", "Karnataka Assembly 2028", ElectionAssembly, &stateID, pollingDate, []int{176, 177, 178})
}

func createActiveBlackoutElection() *Election {
	// Create an election where blackout is currently active
	pollingDate := time.Now().Add(24 * time.Hour) // Tomorrow
	stateID := 29
	return CreateElection("election-active", "Active Election", ElectionAssembly, &stateID, pollingDate, []int{176, 177})
}

func TestCreateElection(t *testing.T) {
	pollingDate := time.Date(2028, 5, 10, 0, 0, 0, 0, time.Local)
	stateID := 29
	election := CreateElection("test-1", "Test Election", ElectionAssembly, &stateID, pollingDate, []int{176, 177})

	if election.ID != "test-1" {
		t.Errorf("ID = %s, want test-1", election.ID)
	}
	if election.Name != "Test Election" {
		t.Errorf("Name = %s, want Test Election", election.Name)
	}
	if election.Type != ElectionAssembly {
		t.Errorf("Type = %s, want %s", election.Type, ElectionAssembly)
	}
	if len(election.ACIDs) != 2 {
		t.Errorf("ACIDs length = %d, want 2", len(election.ACIDs))
	}
	if election.Status != StatusScheduled {
		t.Errorf("Status = %s, want %s", election.Status, StatusScheduled)
	}

	// Check blackout calculation (48 hours before 18:00 on polling day)
	expectedEnd := time.Date(2028, 5, 10, DefaultPollingEndHour, 0, 0, 0, time.Local)
	expectedStart := expectedEnd.Add(-BlackoutDuration)

	if !election.BlackoutStartsAt.Equal(expectedStart) {
		t.Errorf("BlackoutStart = %v, want %v", election.BlackoutStartsAt, expectedStart)
	}
	if !election.BlackoutEndsAt.Equal(expectedEnd) {
		t.Errorf("BlackoutEnd = %v, want %v", election.BlackoutEndsAt, expectedEnd)
	}
}

func TestCreateMultiPhaseElection(t *testing.T) {
	phases := []ElectionPhase{
		{
			PhaseNumber: 1,
			PollingDate: time.Date(2028, 4, 19, 0, 0, 0, 0, time.Local),
			PollingEndTime: time.Date(2028, 4, 19, 18, 0, 0, 0, time.Local),
			ACIDs: []int{1, 2, 3},
		},
		{
			PhaseNumber: 2,
			PollingDate: time.Date(2028, 4, 26, 0, 0, 0, 0, time.Local),
			PollingEndTime: time.Date(2028, 4, 26, 18, 0, 0, 0, time.Local),
			ACIDs: []int{4, 5, 6},
		},
	}

	election, err := CreateMultiPhaseElection("multi-1", "Multi-Phase Election", ElectionGeneral, nil, phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if election.TotalPhases != 2 {
		t.Errorf("TotalPhases = %d, want 2", election.TotalPhases)
	}
	if len(election.Phases) != 2 {
		t.Errorf("Phases length = %d, want 2", len(election.Phases))
	}

	// Check that blackout periods were calculated
	for i, phase := range election.Phases {
		if phase.BlackoutStart.IsZero() {
			t.Errorf("Phase %d BlackoutStart not set", i+1)
		}
		if phase.BlackoutEnd.IsZero() {
			t.Errorf("Phase %d BlackoutEnd not set", i+1)
		}
	}
}

func TestCreateMultiPhaseElection_TooManyPhases(t *testing.T) {
	phases := make([]ElectionPhase, MaxPhases+1)
	for i := range phases {
		phases[i] = ElectionPhase{
			PollingDate: time.Now(),
			PollingEndTime: time.Now(),
			ACIDs: []int{i + 1},
		}
	}

	_, err := CreateMultiPhaseElection("multi-2", "Too Many Phases", ElectionGeneral, nil, phases)
	if err == nil {
		t.Error("expected error for too many phases")
	}
}

func TestElection_GetBlackoutForAC(t *testing.T) {
	election := createTestElection()

	// AC in election
	start, end, found := election.GetBlackoutForAC(176)
	if !found {
		t.Error("should find blackout for AC 176")
	}
	if start.IsZero() || end.IsZero() {
		t.Error("start and end should be set")
	}

	// AC not in election
	_, _, found = election.GetBlackoutForAC(999)
	if found {
		t.Error("should not find blackout for AC not in election")
	}
}

func TestElection_IsACInScope(t *testing.T) {
	election := createTestElection()

	if !election.IsACInScope(176) {
		t.Error("AC 176 should be in scope")
	}
	if !election.IsACInScope(177) {
		t.Error("AC 177 should be in scope")
	}
	if election.IsACInScope(999) {
		t.Error("AC 999 should not be in scope")
	}
}

func TestChecker_IsBlackoutActive(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	// During blackout
	now := time.Now()
	if !checker.IsBlackoutActive(176, now) {
		t.Error("blackout should be active for AC 176")
	}

	// AC not in election
	if checker.IsBlackoutActive(999, now) {
		t.Error("blackout should not be active for AC not in election")
	}
}

func TestChecker_IsBlackoutActive_NoActiveBlackout(t *testing.T) {
	// Create election far in the future
	pollingDate := time.Now().Add(30 * 24 * time.Hour)
	stateID := 29
	election := CreateElection("future", "Future Election", ElectionAssembly, &stateID, pollingDate, []int{176})
	checker := NewChecker([]Election{*election})

	if checker.IsBlackoutActive(176, time.Now()) {
		t.Error("blackout should not be active for future election")
	}
}

func TestChecker_IsActionBlocked(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})
	now := time.Now()

	// Blocked actions during blackout
	blockedActions := []BlockedAction{
		ActionPollCreate, ActionResultsView, ActionAnalyticsView,
		ActionSentimentView, ActionHistoricalData,
	}

	for _, action := range blockedActions {
		if !checker.IsActionBlocked(176, action, now) {
			t.Errorf("action %s should be blocked during blackout", action)
		}
	}
}

func TestChecker_IsActionAllowed(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})
	now := time.Now()

	// Allowed actions even during blackout
	allowedActions := []AllowedAction{
		ActionIssueReport, ActionIssueVerification, ActionIssueLeaderboard,
		ActionIssueFix, ActionProfile, ActionWallet,
	}

	for _, action := range allowedActions {
		if !checker.IsActionAllowed(176, action, now) {
			t.Errorf("action %s should be allowed during blackout", action)
		}
	}
}

func TestChecker_GetActiveBlackouts(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	active := checker.GetActiveBlackouts(time.Now())
	if len(active) != 1 {
		t.Errorf("active blackouts = %d, want 1", len(active))
	}
}

func TestChecker_GetBlackoutForAC(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	result := checker.GetBlackoutForAC(176, time.Now())
	if result == nil {
		t.Error("should find blackout for AC 176")
	}
	if result != nil && result.ID != election.ID {
		t.Errorf("election ID = %s, want %s", result.ID, election.ID)
	}
}

func TestChecker_GetBlackoutEndTime(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	endTime := checker.GetBlackoutEndTime(176, time.Now())
	if endTime == nil {
		t.Error("should return end time for active blackout")
	}

	// No blackout for AC not in election
	endTime = checker.GetBlackoutEndTime(999, time.Now())
	if endTime != nil {
		t.Error("should not return end time for AC not in blackout")
	}
}

func TestChecker_AddElection(t *testing.T) {
	checker := NewChecker([]Election{})
	if checker.GetElectionCount() != 0 {
		t.Error("initial election count should be 0")
	}

	election := createTestElection()
	checker.AddElection(*election)

	if checker.GetElectionCount() != 1 {
		t.Errorf("election count = %d, want 1", checker.GetElectionCount())
	}
}

func TestChecker_RemoveElection(t *testing.T) {
	election := createTestElection()
	checker := NewChecker([]Election{*election})

	removed := checker.RemoveElection(election.ID)
	if !removed {
		t.Error("should return true for successful removal")
	}
	if checker.GetElectionCount() != 0 {
		t.Error("election count should be 0 after removal")
	}

	// Try to remove non-existent election
	removed = checker.RemoveElection("non-existent")
	if removed {
		t.Error("should return false for non-existent election")
	}
}

func TestChecker_GetElectionByID(t *testing.T) {
	election := createTestElection()
	checker := NewChecker([]Election{*election})

	found := checker.GetElectionByID(election.ID)
	if found == nil {
		t.Error("should find election by ID")
	}

	found = checker.GetElectionByID("non-existent")
	if found != nil {
		t.Error("should not find non-existent election")
	}
}

func TestCalculateBlackoutPeriod(t *testing.T) {
	pollingDate := time.Date(2028, 5, 10, 0, 0, 0, 0, time.Local)
	start, end := CalculateBlackoutPeriod(pollingDate, 18, 0)

	expectedEnd := time.Date(2028, 5, 10, 18, 0, 0, 0, time.Local)
	expectedStart := expectedEnd.Add(-48 * time.Hour)

	if !start.Equal(expectedStart) {
		t.Errorf("start = %v, want %v", start, expectedStart)
	}
	if !end.Equal(expectedEnd) {
		t.Errorf("end = %v, want %v", end, expectedEnd)
	}
}

func TestOverride_IsFullyApproved(t *testing.T) {
	override := &Override{}

	if override.IsFullyApproved() {
		t.Error("should not be approved with no approvals")
	}

	now := time.Now()
	override.Approval1At = &now
	if override.IsFullyApproved() {
		t.Error("should not be approved with 1 approval")
	}

	override.Approval2At = &now
	if override.IsFullyApproved() {
		t.Error("should not be approved with 2 approvals")
	}

	override.LegalApprovalAt = &now
	if !override.IsFullyApproved() {
		t.Error("should be approved with 3 approvals")
	}
}

func TestChecker_RequestOverride(t *testing.T) {
	checker := NewChecker([]Election{})

	override, err := checker.RequestOverride(
		"election-1",
		[]int{176},
		"Emergency maintenance",
		"admin@politic.in",
		time.Now(),
		time.Now().Add(2*time.Hour),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if override.ElectionID != "election-1" {
		t.Error("election ID should be set")
	}
	if !override.Approved {
		// Should not be approved yet
	}
}

func TestChecker_RequestOverride_MissingFields(t *testing.T) {
	checker := NewChecker([]Election{})

	_, err := checker.RequestOverride("", []int{176}, "reason", "admin", time.Now(), time.Now())
	if err == nil {
		t.Error("should error for missing election ID")
	}

	_, err = checker.RequestOverride("election-1", []int{}, "reason", "admin", time.Now(), time.Now())
	if err == nil {
		t.Error("should error for empty AC IDs")
	}
}

func TestChecker_ApproveOverride(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	override, _ := checker.RequestOverride(
		election.ID,
		[]int{176},
		"Emergency",
		"admin",
		time.Now(),
		time.Now().Add(2*time.Hour),
	)

	// Add approvals
	checker.ApproveOverride(override, "founder_1", "Founder One")
	checker.ApproveOverride(override, "founder_2", "Founder Two")
	checker.ApproveOverride(override, "legal", "Legal Team")

	if !override.Approved {
		t.Error("override should be approved after 3 approvals")
	}

	// With override, blackout should no longer be active
	if checker.IsBlackoutActive(176, time.Now()) {
		t.Error("blackout should not be active with approved override")
	}
}

func TestChecker_CheckAndLog(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	userID := "user-123"
	result, log := checker.CheckAndLog(176, ActionPollCreate, &userID, "192.168.1.1", "Mozilla/5.0")

	if !result.IsBlocked {
		t.Error("action should be blocked")
	}
	if result.ElectionID != election.ID {
		t.Errorf("election ID = %s, want %s", result.ElectionID, election.ID)
	}
	if log == nil {
		t.Error("log should be created")
	}
	if log != nil && log.ActionBlocked != ActionPollCreate {
		t.Errorf("action blocked = %s, want %s", log.ActionBlocked, ActionPollCreate)
	}
}

func TestChecker_CheckAndLog_NotBlocked(t *testing.T) {
	// Future election (no active blackout)
	pollingDate := time.Now().Add(30 * 24 * time.Hour)
	stateID := 29
	election := CreateElection("future", "Future", ElectionAssembly, &stateID, pollingDate, []int{176})
	checker := NewChecker([]Election{*election})

	result, log := checker.CheckAndLog(176, ActionPollCreate, nil, "192.168.1.1", "")

	if result.IsBlocked {
		t.Error("action should not be blocked")
	}
	if log != nil {
		t.Error("log should not be created when not blocked")
	}
}

func TestValidateElection(t *testing.T) {
	tests := []struct {
		name    string
		election *Election
		wantErr bool
	}{
		{
			name:    "valid election",
			election: createTestElection(),
			wantErr: false,
		},
		{
			name:    "missing ID",
			election: &Election{Name: "Test", Type: ElectionAssembly, PollingDate: time.Now()},
			wantErr: true,
		},
		{
			name:    "missing name",
			election: &Election{ID: "test", Type: ElectionAssembly, PollingDate: time.Now()},
			wantErr: true,
		},
		{
			name:    "missing type",
			election: &Election{ID: "test", Name: "Test", PollingDate: time.Now()},
			wantErr: true,
		},
		{
			name:    "missing polling date (no phases)",
			election: &Election{ID: "test", Name: "Test", Type: ElectionAssembly},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateElection(tt.election)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestElection_ToJSON_FromJSON(t *testing.T) {
	election := createTestElection()

	data, err := election.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	parsed, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}

	if parsed.ID != election.ID {
		t.Errorf("ID = %s, want %s", parsed.ID, election.ID)
	}
	if parsed.Name != election.Name {
		t.Errorf("Name = %s, want %s", parsed.Name, election.Name)
	}
}

func TestChecker_ACBlackoutSchedule(t *testing.T) {
	election1 := createTestElection()
	pollingDate2 := time.Now().Add(60 * 24 * time.Hour)
	stateID := 29
	election2 := CreateElection("election-2", "Another Election", ElectionAssembly, &stateID, pollingDate2, []int{176, 177})

	checker := NewChecker([]Election{*election1, *election2})

	schedule := checker.ACBlackoutSchedule(176)
	if len(schedule) != 2 {
		t.Errorf("schedule length = %d, want 2", len(schedule))
	}

	// Should be sorted by start time
	if len(schedule) >= 2 && schedule[0].Start.After(schedule[1].Start) {
		t.Error("schedule should be sorted by start time")
	}
}

func TestChecker_GetUpcomingBlackouts(t *testing.T) {
	election := createTestElection()
	checker := NewChecker([]Election{*election})

	upcoming := checker.GetUpcomingBlackouts(7, time.Now())
	if len(upcoming) != 1 {
		t.Errorf("upcoming blackouts = %d, want 1", len(upcoming))
	}
}

func TestChecker_GetBlackoutsForACs(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	result := checker.GetBlackoutsForACs([]int{176, 999}, time.Now())

	if len(result) != 2 {
		t.Errorf("result length = %d, want 2", len(result))
	}

	info176 := result[176]
	if info176 == nil {
		t.Fatal("should have info for AC 176")
	}
	if !info176.IsActive {
		t.Error("AC 176 should have active blackout")
	}

	info999 := result[999]
	if info999 == nil {
		t.Fatal("should have info for AC 999")
	}
	if info999.IsActive {
		t.Error("AC 999 should not have active blackout")
	}
}

func TestNewEnforcementLog(t *testing.T) {
	userID := "user-123"
	log := NewEnforcementLog("election-1", "Test Election", 176, ActionPollCreate, &userID, "192.168.1.1", "Mozilla/5.0")

	if log.ID == "" {
		t.Error("ID should be set")
	}
	if log.ElectionID != "election-1" {
		t.Errorf("ElectionID = %s, want election-1", log.ElectionID)
	}
	if log.ACID != 176 {
		t.Errorf("ACID = %d, want 176", log.ACID)
	}
	if log.ActionBlocked != ActionPollCreate {
		t.Errorf("ActionBlocked = %s, want %s", log.ActionBlocked, ActionPollCreate)
	}
	if log.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestChecker_IsDateInBlackout(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	today := time.Now()
	if !checker.IsDateInBlackout(176, today) {
		t.Error("today should be in blackout")
	}

	farFuture := time.Now().Add(365 * 24 * time.Hour)
	if checker.IsDateInBlackout(176, farFuture) {
		t.Error("far future should not be in blackout")
	}
}

func TestChecker_GetBlockedDates(t *testing.T) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})

	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now().Add(7 * 24 * time.Hour)

	blockedDates := checker.GetBlockedDates(176, startDate, endDate)

	// Should have at least today and possibly tomorrow blocked
	if len(blockedDates) == 0 {
		t.Error("should have some blocked dates")
	}
}

// Concurrent access test
func TestChecker_ConcurrentAccess(t *testing.T) {
	election := createTestElection()
	checker := NewChecker([]Election{*election})

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				checker.IsBlackoutActive(176, time.Now())
				checker.GetElectionCount()
				checker.GetActiveBlackouts(time.Now())
			}
			done <- true
		}()
	}

	// Concurrent writes
	go func() {
		for j := 0; j < 10; j++ {
			pollingDate := time.Now().Add(time.Duration(30+j) * 24 * time.Hour)
			stateID := 29
			checker.AddElection(*CreateElection("new-"+string(rune('A'+j)), "New Election", ElectionAssembly, &stateID, pollingDate, []int{176}))
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkIsBlackoutActive(b *testing.B) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.IsBlackoutActive(176, now)
	}
}

func BenchmarkIsActionBlocked(b *testing.B) {
	election := createActiveBlackoutElection()
	checker := NewChecker([]Election{*election})
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.IsActionBlocked(176, ActionPollCreate, now)
	}
}

func BenchmarkCalculateBlackoutPeriod(b *testing.B) {
	pollingDate := time.Date(2028, 5, 10, 0, 0, 0, 0, time.Local)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateBlackoutPeriod(pollingDate, 18, 0)
	}
}
