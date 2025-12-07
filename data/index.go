package data

import (
	"fmt"
	"sync"
)

// GeoIndex provides fast O(1) lookups for Indian geographic and electoral data.
// It builds hierarchical indices: State → District → AC → Booth
type GeoIndex struct {
	dataDir string

	// State indices
	statesByID   map[string]*State // "AP" -> State
	statesByName map[string]*State // "andhra_pradesh" (slug) -> State
	statesBySlug map[string]*State // "andhra_pradesh" -> State

	// District indices
	districtsByID      map[int]*District      // 1 -> District
	districtsByState   map[string][]*District // state slug -> districts
	districtsByNameMap map[string]*District   // "state_slug:district_slug" -> District

	// AC indices
	acsByState   map[string][]*AssemblyConstituency // state slug -> ACs
	acsByID      map[string]*AssemblyConstituency   // "state_slug:ac_1" -> AC
	acsByNumber  map[string]*AssemblyConstituency   // "state_slug:123" -> AC
	acsByNameMap map[string]*AssemblyConstituency   // "state_slug:ac_slug" -> AC

	// Booth indices
	boothsByState    map[string][]*PollingBooth // state slug -> booths
	boothsByAC       map[string][]*PollingBooth // "state_slug:ac_number" -> booths
	boothsByDistrict map[string][]*PollingBooth // "state_slug:district_slug" -> booths
	boothByPartID    map[string]*PollingBooth   // "state_slug:ac:part_id" -> booth

	// Boundary indices
	boundariesByState map[string][]*ACBoundary // state slug -> boundaries
	boundaryByAC      map[string]*ACBoundary   // "state_slug:cons_code" -> boundary

	// Party indices
	partiesByID        map[int]*Party
	partiesByShortName map[string]*Party // "BJP" -> Party

	// Lookup table for coordinate -> AC mapping
	constituencyLookup []ConstituencyBoundaryLookup

	// Load state tracking
	loadedStates map[string]bool
	loadedBounds map[string]bool
	mu           sync.RWMutex
}

// NewGeoIndex creates a new geographic index from the given data directory
func NewGeoIndex(dataDir string) *GeoIndex {
	return &GeoIndex{
		dataDir:            dataDir,
		statesByID:         make(map[string]*State),
		statesByName:       make(map[string]*State),
		statesBySlug:       make(map[string]*State),
		districtsByID:      make(map[int]*District),
		districtsByState:   make(map[string][]*District),
		districtsByNameMap: make(map[string]*District),
		acsByState:         make(map[string][]*AssemblyConstituency),
		acsByID:            make(map[string]*AssemblyConstituency),
		acsByNumber:        make(map[string]*AssemblyConstituency),
		acsByNameMap:       make(map[string]*AssemblyConstituency),
		boothsByState:      make(map[string][]*PollingBooth),
		boothsByAC:         make(map[string][]*PollingBooth),
		boothsByDistrict:   make(map[string][]*PollingBooth),
		boothByPartID:      make(map[string]*PollingBooth),
		boundariesByState:  make(map[string][]*ACBoundary),
		boundaryByAC:       make(map[string]*ACBoundary),
		partiesByID:        make(map[int]*Party),
		partiesByShortName: make(map[string]*Party),
		loadedStates:       make(map[string]bool),
		loadedBounds:       make(map[string]bool),
	}
}

// LoadAll loads all available data into the index
func (g *GeoIndex) LoadAll() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Load states
	if err := g.loadStatesLocked(); err != nil {
		return fmt.Errorf("loading states: %w", err)
	}

	// Load districts
	if err := g.loadDistrictsLocked(); err != nil {
		return fmt.Errorf("loading districts: %w", err)
	}

	// Load constituencies
	if err := g.loadConstituenciesLocked(); err != nil {
		return fmt.Errorf("loading constituencies: %w", err)
	}

	// Load parties
	if err := g.loadPartiesLocked(); err != nil {
		return fmt.Errorf("loading parties: %w", err)
	}

	// Load constituency lookup
	if err := g.loadConstituencyLookupLocked(); err != nil {
		// Non-fatal - lookup file might not exist
		_ = err
	}

	return nil
}

// loadStatesLocked loads states (must hold lock)
func (g *GeoIndex) loadStatesLocked() error {
	states, err := LoadStates(g.dataDir)
	if err != nil {
		return err
	}

	for i := range states {
		state := &states[i]
		g.statesByID[state.StateID] = state
		g.statesByName[state.Name] = state
		g.statesBySlug[state.Slug()] = state
	}

	return nil
}

// loadDistrictsLocked loads districts (must hold lock)
func (g *GeoIndex) loadDistrictsLocked() error {
	districts, err := LoadDistricts(g.dataDir)
	if err != nil {
		return err
	}

	for i := range districts {
		district := &districts[i]
		g.districtsByID[district.ID] = district

		stateSlug := ToSlug(district.State)
		g.districtsByState[stateSlug] = append(g.districtsByState[stateSlug], district)

		key := fmt.Sprintf("%s:%s", stateSlug, district.Slug())
		g.districtsByNameMap[key] = district
	}

	return nil
}

// loadConstituenciesLocked loads constituencies (must hold lock)
func (g *GeoIndex) loadConstituenciesLocked() error {
	acMap, err := LoadConstituencies(g.dataDir)
	if err != nil {
		return err
	}

	for stateName, acs := range acMap {
		stateSlug := ToSlug(stateName)
		acList := make([]*AssemblyConstituency, len(acs))

		for i := range acs {
			ac := &acs[i]
			acList[i] = ac

			// Index by various keys
			idKey := fmt.Sprintf("%s:%s", stateSlug, ac.ID)
			g.acsByID[idKey] = ac

			numKey := fmt.Sprintf("%s:%d", stateSlug, ac.ACNumber)
			g.acsByNumber[numKey] = ac

			nameKey := fmt.Sprintf("%s:%s", stateSlug, ToSlug(ac.Name))
			g.acsByNameMap[nameKey] = ac
		}

		g.acsByState[stateSlug] = acList
	}

	return nil
}

// loadPartiesLocked loads parties (must hold lock)
func (g *GeoIndex) loadPartiesLocked() error {
	parties, err := LoadParties(g.dataDir)
	if err != nil {
		return err
	}

	for i := range parties {
		party := &parties[i]
		g.partiesByID[party.ID] = party
		g.partiesByShortName[party.ShortName] = party
	}

	return nil
}

// loadConstituencyLookupLocked loads constituency lookup (must hold lock)
func (g *GeoIndex) loadConstituencyLookupLocked() error {
	lookup, err := LoadConstituencyLookup(g.dataDir)
	if err != nil {
		return err
	}
	g.constituencyLookup = lookup
	return nil
}

// LoadBoothsForState lazily loads booths for a state
func (g *GeoIndex) LoadBoothsForState(stateSlug string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.loadedStates[stateSlug] {
		return nil
	}

	booths, err := LoadBoothsForState(g.dataDir, stateSlug)
	if err != nil {
		return err
	}

	for i := range booths {
		booth := &booths[i]
		g.boothsByState[stateSlug] = append(g.boothsByState[stateSlug], booth)

		// Index by AC
		acKey := fmt.Sprintf("%s:%d", stateSlug, booth.ACNumber)
		g.boothsByAC[acKey] = append(g.boothsByAC[acKey], booth)

		// Index by district
		distKey := fmt.Sprintf("%s:%s", stateSlug, ToSlug(booth.DistrictName))
		g.boothsByDistrict[distKey] = append(g.boothsByDistrict[distKey], booth)

		// Index by part ID
		partKey := fmt.Sprintf("%s:%d:%d", stateSlug, booth.ACNumber, booth.PartID)
		g.boothByPartID[partKey] = booth
	}

	g.loadedStates[stateSlug] = true
	return nil
}

// LoadBoundariesForState lazily loads AC boundaries for a state
func (g *GeoIndex) LoadBoundariesForState(stateSlug string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.loadedBounds[stateSlug] {
		return nil
	}

	boundaries, err := LoadBoundariesForState(g.dataDir, stateSlug)
	if err != nil {
		return err
	}

	for i := range boundaries {
		boundary := &boundaries[i]
		g.boundariesByState[stateSlug] = append(g.boundariesByState[stateSlug], boundary)

		key := fmt.Sprintf("%s:%d", stateSlug, boundary.ConsCode)
		g.boundaryByAC[key] = boundary
	}

	g.loadedBounds[stateSlug] = true
	return nil
}

// --- State Lookups ---

// GetState returns a state by ID (e.g., "AP", "KA")
func (g *GeoIndex) GetState(stateID string) (*State, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	state, ok := g.statesByID[stateID]
	return state, ok
}

// GetStateBySlug returns a state by slug (e.g., "andhra_pradesh")
func (g *GeoIndex) GetStateBySlug(slug string) (*State, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	state, ok := g.statesBySlug[slug]
	return state, ok
}

// GetStateByName returns a state by name (e.g., "Andhra Pradesh")
func (g *GeoIndex) GetStateByName(name string) (*State, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	state, ok := g.statesByName[name]
	return state, ok
}

// ListStates returns all states
func (g *GeoIndex) ListStates() []*State {
	g.mu.RLock()
	defer g.mu.RUnlock()

	states := make([]*State, 0, len(g.statesByID))
	for _, state := range g.statesByID {
		states = append(states, state)
	}
	return states
}

// --- District Lookups ---

// GetDistrict returns a district by ID
func (g *GeoIndex) GetDistrict(id int) (*District, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	district, ok := g.districtsByID[id]
	return district, ok
}

// GetDistrictByName returns a district by state and district slug
func (g *GeoIndex) GetDistrictByName(stateSlug, districtSlug string) (*District, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", stateSlug, districtSlug)
	district, ok := g.districtsByNameMap[key]
	return district, ok
}

// GetDistrictsForState returns all districts for a state
func (g *GeoIndex) GetDistrictsForState(stateSlug string) []*District {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.districtsByState[stateSlug]
}

// --- AC Lookups ---

// GetAC returns an AC by state slug and AC ID (e.g., "ac_1")
func (g *GeoIndex) GetAC(stateSlug, acID string) (*AssemblyConstituency, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", stateSlug, acID)
	ac, ok := g.acsByID[key]
	return ac, ok
}

// GetACByNumber returns an AC by state slug and AC number
func (g *GeoIndex) GetACByNumber(stateSlug string, acNumber int) (*AssemblyConstituency, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%d", stateSlug, acNumber)
	ac, ok := g.acsByNumber[key]
	return ac, ok
}

// GetACByName returns an AC by state slug and AC name slug
func (g *GeoIndex) GetACByName(stateSlug, acNameSlug string) (*AssemblyConstituency, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", stateSlug, acNameSlug)
	ac, ok := g.acsByNameMap[key]
	return ac, ok
}

// GetACsForState returns all ACs for a state
func (g *GeoIndex) GetACsForState(stateSlug string) []*AssemblyConstituency {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.acsByState[stateSlug]
}

// --- Booth Lookups ---

// GetBoothsForState returns all booths for a state (loads if needed)
func (g *GeoIndex) GetBoothsForState(stateSlug string) ([]*PollingBooth, error) {
	if err := g.LoadBoothsForState(stateSlug); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.boothsByState[stateSlug], nil
}

// GetBoothsForAC returns all booths for an AC (loads state if needed)
func (g *GeoIndex) GetBoothsForAC(stateSlug string, acNumber int) ([]*PollingBooth, error) {
	if err := g.LoadBoothsForState(stateSlug); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%d", stateSlug, acNumber)
	return g.boothsByAC[key], nil
}

// GetBoothsForDistrict returns all booths for a district (loads state if needed)
func (g *GeoIndex) GetBoothsForDistrict(stateSlug, districtSlug string) ([]*PollingBooth, error) {
	if err := g.LoadBoothsForState(stateSlug); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", stateSlug, districtSlug)
	return g.boothsByDistrict[key], nil
}

// GetBooth returns a specific booth by state, AC, and part ID
func (g *GeoIndex) GetBooth(stateSlug string, acNumber, partID int) (*PollingBooth, error) {
	if err := g.LoadBoothsForState(stateSlug); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%d:%d", stateSlug, acNumber, partID)
	booth, ok := g.boothByPartID[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s AC:%d Part:%d", ErrBoothNotFound, stateSlug, acNumber, partID)
	}
	return booth, nil
}

// --- Boundary Lookups ---

// GetBoundariesForState returns all AC boundaries for a state (loads if needed)
func (g *GeoIndex) GetBoundariesForState(stateSlug string) ([]*ACBoundary, error) {
	if err := g.LoadBoundariesForState(stateSlug); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.boundariesByState[stateSlug], nil
}

// GetBoundaryForAC returns the boundary for a specific AC
func (g *GeoIndex) GetBoundaryForAC(stateSlug string, consCode int) (*ACBoundary, error) {
	if err := g.LoadBoundariesForState(stateSlug); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	key := fmt.Sprintf("%s:%d", stateSlug, consCode)
	boundary, ok := g.boundaryByAC[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%d", ErrBoundaryNotFound, stateSlug, consCode)
	}
	return boundary, nil
}

// --- Party Lookups ---

// GetParty returns a party by ID
func (g *GeoIndex) GetParty(id int) (*Party, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	party, ok := g.partiesByID[id]
	return party, ok
}

// GetPartyByShortName returns a party by short name (e.g., "BJP")
func (g *GeoIndex) GetPartyByShortName(shortName string) (*Party, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	party, ok := g.partiesByShortName[shortName]
	return party, ok
}

// ListParties returns all parties
func (g *GeoIndex) ListParties() []*Party {
	g.mu.RLock()
	defer g.mu.RUnlock()

	parties := make([]*Party, 0, len(g.partiesByID))
	for _, party := range g.partiesByID {
		parties = append(parties, party)
	}
	return parties
}

// --- Geospatial Lookups ---

// FindACAtPoint finds the AC that contains the given point
func (g *GeoIndex) FindACAtPoint(stateSlug string, lat, lng float64) (*ACBoundary, error) {
	boundaries, err := g.GetBoundariesForState(stateSlug)
	if err != nil {
		return nil, err
	}

	for _, boundary := range boundaries {
		if boundary.ContainsPoint(lat, lng) {
			return boundary, nil
		}
	}

	return nil, fmt.Errorf("%w: no AC found at (%.6f, %.6f)", ErrACNotFound, lat, lng)
}

// FindACAtPointAllStates searches all states for an AC containing the point
// This is slower but useful when state is unknown
func (g *GeoIndex) FindACAtPointAllStates(lat, lng float64) (*ACBoundary, string, error) {
	// First try using the lookup table for a quick approximation
	if len(g.constituencyLookup) > 0 {
		// Find nearest center point
		var bestMatch *ConstituencyBoundaryLookup
		bestDist := float64(1e9)

		for i := range g.constituencyLookup {
			lookup := &g.constituencyLookup[i]
			// Simple Euclidean distance (good enough for nearby points)
			dist := (lat-lookup.CenterLat)*(lat-lookup.CenterLat) +
				(lng-lookup.CenterLng)*(lng-lookup.CenterLng)
			if dist < bestDist {
				bestDist = dist
				bestMatch = lookup
			}
		}

		if bestMatch != nil {
			stateSlug := ToSlug(bestMatch.StateName)
			// Verify the point is actually in this AC
			boundary, err := g.GetBoundaryForAC(stateSlug, bestMatch.ACCode)
			if err == nil && boundary.ContainsPoint(lat, lng) {
				return boundary, stateSlug, nil
			}
		}
	}

	// Fall back to checking all boundaries
	availableStates, err := ListAvailableBoundaries(g.dataDir)
	if err != nil {
		return nil, "", err
	}

	for _, stateName := range availableStates {
		stateSlug := ToSlug(stateName)
		boundary, err := g.FindACAtPoint(stateSlug, lat, lng)
		if err == nil {
			return boundary, stateSlug, nil
		}
	}

	return nil, "", fmt.Errorf("%w: no AC found at (%.6f, %.6f) in any state", ErrACNotFound, lat, lng)
}

// --- Statistics ---

// Stats returns statistics about the loaded data
type IndexStats struct {
	States           int
	Districts        int
	ACs              int
	BoothsLoaded     int
	BoundariesLoaded int
	Parties          int
	StatesWithBooths int
	StatesWithBounds int
}

// GetStats returns statistics about the loaded index
func (g *GeoIndex) GetStats() IndexStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := IndexStats{
		States:    len(g.statesByID),
		Districts: len(g.districtsByID),
		Parties:   len(g.partiesByID),
	}

	// Count ACs
	for _, acs := range g.acsByState {
		stats.ACs += len(acs)
	}

	// Count loaded booths
	for _, booths := range g.boothsByState {
		stats.BoothsLoaded += len(booths)
	}
	stats.StatesWithBooths = len(g.loadedStates)

	// Count loaded boundaries
	for _, bounds := range g.boundariesByState {
		stats.BoundariesLoaded += len(bounds)
	}
	stats.StatesWithBounds = len(g.loadedBounds)

	return stats
}
