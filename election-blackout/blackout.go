// Package blackout implements Section 126 RP Act compliance.
// Open-source so election blackout logic is auditable and verifiable.
package blackout

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Error definitions
var (
	ErrBlackoutActive       = errors.New("operation blocked: election blackout active")
	ErrNoElectionFound      = errors.New("no election found for specified criteria")
	ErrInvalidElection      = errors.New("invalid election data")
	ErrOverrideNotApproved  = errors.New("override request not approved")
	ErrInsufficientApprovals = errors.New("insufficient approvals for override")
	ErrElectionNotFound     = errors.New("election not found")
	ErrACNotInElection      = errors.New("assembly constituency not in election scope")
)

// Constants
const (
	// BlackoutDuration is 48 hours before poll close (as per Section 126)
	BlackoutDuration = 48 * time.Hour

	// RequiredApprovals for override (2 founders + legal)
	RequiredApprovals = 3

	// MaxPhases is the maximum number of phases for a multi-phase election
	MaxPhases = 10

	// DefaultPollingStartHour is the default polling start time
	DefaultPollingStartHour = 7

	// DefaultPollingEndHour is the default polling end time
	DefaultPollingEndHour = 18
)

// ElectionType defines types of elections
type ElectionType string

const (
	ElectionGeneral    ElectionType = "general"
	ElectionAssembly   ElectionType = "assembly"
	ElectionByElection ElectionType = "by_election"
	ElectionLocalBody  ElectionType = "local_body"
	ElectionPanchayat  ElectionType = "panchayat"
)

// BlackoutStatus indicates the current status of a blackout period
type BlackoutStatus string

const (
	StatusScheduled  BlackoutStatus = "scheduled"
	StatusActive     BlackoutStatus = "active"
	StatusCompleted  BlackoutStatus = "completed"
	StatusCancelled  BlackoutStatus = "cancelled"
)

// BlockedAction defines actions blocked during blackout
type BlockedAction string

const (
	ActionPollCreate     BlockedAction = "poll_create"
	ActionResultsView    BlockedAction = "results_view"
	ActionAnalyticsView  BlockedAction = "analytics_view"
	ActionSentimentView  BlockedAction = "sentiment_view"
	ActionHistoricalData BlockedAction = "historical_data"
	ActionPollTarget     BlockedAction = "poll_target"
	ActionExportData     BlockedAction = "export_data"
)

// AllowedAction defines actions allowed during blackout
type AllowedAction string

const (
	ActionIssueReport       AllowedAction = "issue_report"
	ActionIssueVerification AllowedAction = "issue_verification"
	ActionIssueLeaderboard  AllowedAction = "issue_leaderboard"
	ActionIssueFix          AllowedAction = "issue_fix"
	ActionProfile           AllowedAction = "profile"
	ActionWallet            AllowedAction = "wallet"
)

// ElectionPhase represents a single phase of voting
type ElectionPhase struct {
	PhaseNumber     int       `json:"phase_number"`
	PollingDate     time.Time `json:"polling_date"`
	PollingStartTime time.Time `json:"polling_start_time"`
	PollingEndTime  time.Time `json:"polling_end_time"`
	ACIDs           []int     `json:"ac_ids"` // ACs voting in this phase
	BlackoutStart   time.Time `json:"blackout_start"`
	BlackoutEnd     time.Time `json:"blackout_end"`
}

// Election represents an election event
type Election struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Type             ElectionType   `json:"type"`
	StateID          *int           `json:"state_id,omitempty"`
	StateName        string         `json:"state_name,omitempty"`
	TotalPhases      int            `json:"total_phases"`
	Phases           []ElectionPhase `json:"phases"`
	Status           BlackoutStatus `json:"status"`
	SourceURL        string         `json:"source_url,omitempty"` // ECI notification
	VerifiedBy       string         `json:"verified_by,omitempty"`
	VerifiedAt       *time.Time     `json:"verified_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`

	// Legacy single-phase support
	PollingDate     time.Time `json:"polling_date,omitempty"`
	PollingEndTime  time.Time `json:"polling_end_time,omitempty"`
	BlackoutStartsAt time.Time `json:"blackout_starts_at,omitempty"`
	BlackoutEndsAt   time.Time `json:"blackout_ends_at,omitempty"`
	ACIDs           []int     `json:"ac_ids,omitempty"`
}

// GetBlackoutForAC returns the blackout period for a specific AC
func (e *Election) GetBlackoutForAC(acID int) (start, end time.Time, found bool) {
	// Check phases first
	for _, phase := range e.Phases {
		for _, id := range phase.ACIDs {
			if id == acID {
				return phase.BlackoutStart, phase.BlackoutEnd, true
			}
		}
	}

	// Fall back to legacy single-phase
	if len(e.ACIDs) == 0 || containsInt(e.ACIDs, acID) {
		return e.BlackoutStartsAt, e.BlackoutEndsAt, true
	}

	return time.Time{}, time.Time{}, false
}

// IsACInScope checks if an AC is within this election's scope
func (e *Election) IsACInScope(acID int) bool {
	// Check phases
	for _, phase := range e.Phases {
		if containsInt(phase.ACIDs, acID) {
			return true
		}
	}

	// Legacy: If no phases defined, check top-level ACIDs
	if len(e.Phases) == 0 {
		// Empty ACIDs means all ACs in state
		if len(e.ACIDs) == 0 {
			return true
		}
		return containsInt(e.ACIDs, acID)
	}

	return false
}

// Checker provides blackout checking functionality
type Checker struct {
	mu        sync.RWMutex
	elections []Election
	overrides map[string]*Override // election_id:ac_id -> override
}

// NewChecker creates a new blackout checker
func NewChecker(elections []Election) *Checker {
	return &Checker{
		elections: elections,
		overrides: make(map[string]*Override),
	}
}

// AddElection adds a new election to the checker
func (c *Checker) AddElection(election Election) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.elections = append(c.elections, election)
}

// RemoveElection removes an election by ID
func (c *Checker) RemoveElection(electionID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, e := range c.elections {
		if e.ID == electionID {
			c.elections = append(c.elections[:i], c.elections[i+1:]...)
			return true
		}
	}
	return false
}

// IsBlackoutActive checks if blackout is active for a given AC at the given time
func (c *Checker) IsBlackoutActive(acID int, at time.Time) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check for override first
	if c.hasActiveOverride(acID, at) {
		return false
	}

	for _, election := range c.elections {
		if !election.IsACInScope(acID) {
			continue
		}

		start, end, found := election.GetBlackoutForAC(acID)
		if !found {
			continue
		}

		if at.After(start) && at.Before(end) {
			return true
		}
	}
	return false
}

// IsActionBlocked checks if a specific action is blocked for an AC
func (c *Checker) IsActionBlocked(acID int, action BlockedAction, at time.Time) bool {
	if !c.IsBlackoutActive(acID, at) {
		return false
	}

	// All poll/results/analytics actions are blocked during blackout
	switch action {
	case ActionPollCreate, ActionResultsView, ActionAnalyticsView,
		ActionSentimentView, ActionHistoricalData, ActionPollTarget, ActionExportData:
		return true
	default:
		return false
	}
}

// IsActionAllowed checks if a specific action is allowed for an AC
func (c *Checker) IsActionAllowed(acID int, action AllowedAction, at time.Time) bool {
	// Civic actions are always allowed, even during blackout
	switch action {
	case ActionIssueReport, ActionIssueVerification, ActionIssueLeaderboard,
		ActionIssueFix, ActionProfile, ActionWallet:
		return true
	default:
		return false
	}
}

// GetActiveBlackouts returns all active blackouts at the given time
func (c *Checker) GetActiveBlackouts(at time.Time) []Election {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var active []Election
	for _, election := range c.elections {
		// Check each phase
		for _, phase := range election.Phases {
			if at.After(phase.BlackoutStart) && at.Before(phase.BlackoutEnd) {
				active = append(active, election)
				break // Don't add same election multiple times
			}
		}

		// Check legacy single-phase
		if len(election.Phases) == 0 {
			if at.After(election.BlackoutStartsAt) && at.Before(election.BlackoutEndsAt) {
				active = append(active, election)
			}
		}
	}
	return active
}

// GetBlackoutForAC returns the active blackout for a specific AC, if any
func (c *Checker) GetBlackoutForAC(acID int, at time.Time) *Election {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, election := range c.elections {
		if !election.IsACInScope(acID) {
			continue
		}

		start, end, found := election.GetBlackoutForAC(acID)
		if !found {
			continue
		}

		if at.After(start) && at.Before(end) {
			return &election
		}
	}
	return nil
}

// GetBlackoutEndTime returns when blackout ends for an AC
func (c *Checker) GetBlackoutEndTime(acID int, at time.Time) *time.Time {
	election := c.GetBlackoutForAC(acID, at)
	if election == nil {
		return nil
	}

	_, end, _ := election.GetBlackoutForAC(acID)
	return &end
}

// GetUpcomingBlackouts returns blackouts starting within the next n days
func (c *Checker) GetUpcomingBlackouts(days int, now time.Time) []Election {
	c.mu.RLock()
	defer c.mu.RUnlock()

	deadline := now.Add(time.Duration(days) * 24 * time.Hour)
	var upcoming []Election

	for _, election := range c.elections {
		for _, phase := range election.Phases {
			if phase.BlackoutStart.After(now) && phase.BlackoutStart.Before(deadline) {
				upcoming = append(upcoming, election)
				break
			}
		}

		// Legacy
		if len(election.Phases) == 0 && election.BlackoutStartsAt.After(now) && election.BlackoutStartsAt.Before(deadline) {
			upcoming = append(upcoming, election)
		}
	}

	// Sort by start time
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].BlackoutStartsAt.Before(upcoming[j].BlackoutStartsAt)
	})

	return upcoming
}

// GetBlackoutsForACs returns upcoming blackout info for multiple ACs
func (c *Checker) GetBlackoutsForACs(acIDs []int, now time.Time) map[int]*BlackoutInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[int]*BlackoutInfo)

	for _, acID := range acIDs {
		info := &BlackoutInfo{ACID: acID}

		// Check current blackout
		if c.IsBlackoutActive(acID, now) {
			info.IsActive = true
			if election := c.GetBlackoutForAC(acID, now); election != nil {
				info.ElectionName = election.Name
				_, end, _ := election.GetBlackoutForAC(acID)
				info.EndsAt = &end
			}
		}

		// Find next upcoming blackout
		for _, election := range c.elections {
			start, end, found := election.GetBlackoutForAC(acID)
			if !found {
				continue
			}

			if start.After(now) && (info.NextStart == nil || start.Before(*info.NextStart)) {
				info.NextStart = &start
				info.NextEnd = &end
				info.NextElection = election.Name
			}
		}

		result[acID] = info
	}

	return result
}

// BlackoutInfo contains blackout information for an AC
type BlackoutInfo struct {
	ACID         int        `json:"ac_id"`
	IsActive     bool       `json:"is_active"`
	ElectionName string     `json:"election_name,omitempty"`
	EndsAt       *time.Time `json:"ends_at,omitempty"`
	NextStart    *time.Time `json:"next_start,omitempty"`
	NextEnd      *time.Time `json:"next_end,omitempty"`
	NextElection string     `json:"next_election,omitempty"`
}

// CalculateBlackoutPeriod calculates blackout start/end from polling schedule
func CalculateBlackoutPeriod(pollingDate time.Time, pollingEndHour, pollingEndMinute int) (start, end time.Time) {
	// Polling end time on polling date
	pollingEnd := time.Date(
		pollingDate.Year(),
		pollingDate.Month(),
		pollingDate.Day(),
		pollingEndHour,
		pollingEndMinute,
		0,
		0,
		pollingDate.Location(),
	)

	// Blackout starts 48 hours before polling ends
	start = pollingEnd.Add(-BlackoutDuration)
	end = pollingEnd

	return start, end
}

// CreateElection creates a new election with calculated blackout periods
func CreateElection(id, name string, electionType ElectionType, stateID *int, pollingDate time.Time, acIDs []int) *Election {
	blackoutStart, blackoutEnd := CalculateBlackoutPeriod(pollingDate, DefaultPollingEndHour, 0)

	now := time.Now()
	return &Election{
		ID:               id,
		Name:             name,
		Type:             electionType,
		StateID:          stateID,
		TotalPhases:      1,
		PollingDate:      pollingDate,
		PollingEndTime:   time.Date(pollingDate.Year(), pollingDate.Month(), pollingDate.Day(), DefaultPollingEndHour, 0, 0, 0, pollingDate.Location()),
		BlackoutStartsAt: blackoutStart,
		BlackoutEndsAt:   blackoutEnd,
		ACIDs:            acIDs,
		Status:           StatusScheduled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// CreateMultiPhaseElection creates an election with multiple phases
func CreateMultiPhaseElection(id, name string, electionType ElectionType, stateID *int, phases []ElectionPhase) (*Election, error) {
	if len(phases) == 0 {
		return nil, errors.New("at least one phase is required")
	}
	if len(phases) > MaxPhases {
		return nil, fmt.Errorf("too many phases: max %d", MaxPhases)
	}

	// Calculate blackout periods for each phase
	for i := range phases {
		phases[i].BlackoutStart, phases[i].BlackoutEnd = CalculateBlackoutPeriod(
			phases[i].PollingDate,
			phases[i].PollingEndTime.Hour(),
			phases[i].PollingEndTime.Minute(),
		)
	}

	now := time.Now()
	return &Election{
		ID:          id,
		Name:        name,
		Type:        electionType,
		StateID:     stateID,
		TotalPhases: len(phases),
		Phases:      phases,
		Status:      StatusScheduled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Override represents a blackout override request
type Override struct {
	ID              string     `json:"id"`
	ElectionID      string     `json:"election_id"`
	ACIDs           []int      `json:"ac_ids"`
	Reason          string     `json:"reason"`
	RequestedBy     string     `json:"requested_by"`
	RequestedAt     time.Time  `json:"requested_at"`

	// Approvals (need 2 founders + legal)
	Approval1By     string     `json:"approval_1_by,omitempty"`
	Approval1At     *time.Time `json:"approval_1_at,omitempty"`
	Approval2By     string     `json:"approval_2_by,omitempty"`
	Approval2At     *time.Time `json:"approval_2_at,omitempty"`
	LegalApprovalBy string     `json:"legal_approval_by,omitempty"`
	LegalApprovalAt *time.Time `json:"legal_approval_at,omitempty"`

	// Override period
	Approved        bool       `json:"approved"`
	OverrideStart   time.Time  `json:"override_start"`
	OverrideEnd     time.Time  `json:"override_end"`
}

// IsFullyApproved checks if override has all required approvals
func (o *Override) IsFullyApproved() bool {
	approvalCount := 0
	if o.Approval1At != nil {
		approvalCount++
	}
	if o.Approval2At != nil {
		approvalCount++
	}
	if o.LegalApprovalAt != nil {
		approvalCount++
	}
	return approvalCount >= RequiredApprovals
}

// RequestOverride creates a new override request
func (c *Checker) RequestOverride(electionID string, acIDs []int, reason, requestedBy string, overrideStart, overrideEnd time.Time) (*Override, error) {
	if electionID == "" || len(acIDs) == 0 || reason == "" || requestedBy == "" {
		return nil, errors.New("all fields are required")
	}

	override := &Override{
		ID:            fmt.Sprintf("override-%d", time.Now().UnixNano()),
		ElectionID:    electionID,
		ACIDs:         acIDs,
		Reason:        reason,
		RequestedBy:   requestedBy,
		RequestedAt:   time.Now(),
		OverrideStart: overrideStart,
		OverrideEnd:   overrideEnd,
	}

	return override, nil
}

// ApproveOverride adds an approval to an override request
func (c *Checker) ApproveOverride(override *Override, approverType, approverName string) error {
	now := time.Now()

	switch approverType {
	case "founder_1":
		override.Approval1By = approverName
		override.Approval1At = &now
	case "founder_2":
		override.Approval2By = approverName
		override.Approval2At = &now
	case "legal":
		override.LegalApprovalBy = approverName
		override.LegalApprovalAt = &now
	default:
		return errors.New("invalid approver type")
	}

	// Check if fully approved
	if override.IsFullyApproved() {
		override.Approved = true

		// Register the override
		c.mu.Lock()
		for _, acID := range override.ACIDs {
			key := fmt.Sprintf("%s:%d", override.ElectionID, acID)
			c.overrides[key] = override
		}
		c.mu.Unlock()
	}

	return nil
}

// hasActiveOverride checks if there's an active override for an AC
func (c *Checker) hasActiveOverride(acID int, at time.Time) bool {
	for _, override := range c.overrides {
		if !override.Approved {
			continue
		}
		if !containsInt(override.ACIDs, acID) {
			continue
		}
		if at.After(override.OverrideStart) && at.Before(override.OverrideEnd) {
			return true
		}
	}
	return false
}

// EnforcementLog records a blackout enforcement event
type EnforcementLog struct {
	ID            string        `json:"id"`
	ElectionID    string        `json:"election_id"`
	ElectionName  string        `json:"election_name"`
	ACID          int           `json:"ac_id"`
	ActionBlocked BlockedAction `json:"action_blocked"`
	UserID        *string       `json:"user_id,omitempty"`
	IPAddress     string        `json:"ip_address"`
	UserAgent     string        `json:"user_agent,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

// NewEnforcementLog creates a new enforcement log entry
func NewEnforcementLog(electionID, electionName string, acID int, action BlockedAction, userID *string, ip, userAgent string) *EnforcementLog {
	return &EnforcementLog{
		ID:            fmt.Sprintf("log-%d", time.Now().UnixNano()),
		ElectionID:    electionID,
		ElectionName:  electionName,
		ACID:          acID,
		ActionBlocked: action,
		UserID:        userID,
		IPAddress:     ip,
		UserAgent:     userAgent,
		Timestamp:     time.Now(),
	}
}

// CheckResult contains the result of a blackout check
type CheckResult struct {
	IsBlocked       bool           `json:"is_blocked"`
	ElectionID      string         `json:"election_id,omitempty"`
	ElectionName    string         `json:"election_name,omitempty"`
	BlackoutEnds    *time.Time     `json:"blackout_ends,omitempty"`
	BlockedAction   BlockedAction  `json:"blocked_action,omitempty"`
	Message         string         `json:"message,omitempty"`
}

// CheckAndLog checks if an action is blocked and creates a log if so
func (c *Checker) CheckAndLog(acID int, action BlockedAction, userID *string, ip, userAgent string) (*CheckResult, *EnforcementLog) {
	now := time.Now()
	result := &CheckResult{
		BlockedAction: action,
	}

	if !c.IsActionBlocked(acID, action, now) {
		result.IsBlocked = false
		return result, nil
	}

	election := c.GetBlackoutForAC(acID, now)
	if election == nil {
		result.IsBlocked = false
		return result, nil
	}

	_, blackoutEnd, _ := election.GetBlackoutForAC(acID)

	result.IsBlocked = true
	result.ElectionID = election.ID
	result.ElectionName = election.Name
	result.BlackoutEnds = &blackoutEnd
	result.Message = fmt.Sprintf("Action blocked due to election blackout for %s. Blackout ends at %s",
		election.Name, blackoutEnd.Format(time.RFC3339))

	log := NewEnforcementLog(election.ID, election.Name, acID, action, userID, ip, userAgent)

	return result, log
}

// ValidateElection validates election data
func ValidateElection(e *Election) error {
	if e.ID == "" {
		return fmt.Errorf("%w: missing ID", ErrInvalidElection)
	}
	if e.Name == "" {
		return fmt.Errorf("%w: missing name", ErrInvalidElection)
	}
	if e.Type == "" {
		return fmt.Errorf("%w: missing type", ErrInvalidElection)
	}

	// Validate phases
	for i, phase := range e.Phases {
		if phase.PollingDate.IsZero() {
			return fmt.Errorf("%w: phase %d missing polling date", ErrInvalidElection, i+1)
		}
		if len(phase.ACIDs) == 0 {
			return fmt.Errorf("%w: phase %d has no ACs", ErrInvalidElection, i+1)
		}
	}

	// Validate legacy single-phase
	if len(e.Phases) == 0 {
		if e.PollingDate.IsZero() {
			return fmt.Errorf("%w: missing polling date", ErrInvalidElection)
		}
	}

	return nil
}

// ToJSON serializes election to JSON
func (e *Election) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes election from JSON
func FromJSON(data []byte) (*Election, error) {
	var e Election
	err := json.Unmarshal(data, &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Helper functions

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// ACBlackoutSchedule returns all blackout periods for an AC across elections
func (c *Checker) ACBlackoutSchedule(acID int) []struct {
	ElectionName string
	Start        time.Time
	End          time.Time
} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var schedule []struct {
		ElectionName string
		Start        time.Time
		End          time.Time
	}

	for _, election := range c.elections {
		start, end, found := election.GetBlackoutForAC(acID)
		if found {
			schedule = append(schedule, struct {
				ElectionName string
				Start        time.Time
				End          time.Time
			}{
				ElectionName: election.Name,
				Start:        start,
				End:          end,
			})
		}
	}

	// Sort by start time
	sort.Slice(schedule, func(i, j int) bool {
		return schedule[i].Start.Before(schedule[j].Start)
	})

	return schedule
}

// GetElectionByID returns an election by its ID
func (c *Checker) GetElectionByID(id string) *Election {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := range c.elections {
		if c.elections[i].ID == id {
			return &c.elections[i]
		}
	}
	return nil
}

// GetElectionCount returns the total number of elections
func (c *Checker) GetElectionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.elections)
}

// IsDateInBlackout checks if a specific date falls within any blackout period for an AC
func (c *Checker) IsDateInBlackout(acID int, date time.Time) bool {
	// Check at noon on the specified date
	checkTime := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, date.Location())
	return c.IsBlackoutActive(acID, checkTime)
}

// GetBlockedDates returns all dates with blackouts for an AC within a date range
func (c *Checker) GetBlockedDates(acID int, startDate, endDate time.Time) []time.Time {
	var blockedDates []time.Time

	current := startDate
	for current.Before(endDate) || current.Equal(endDate) {
		if c.IsDateInBlackout(acID, current) {
			blockedDates = append(blockedDates, current)
		}
		current = current.Add(24 * time.Hour)
	}

	return blockedDates
}
