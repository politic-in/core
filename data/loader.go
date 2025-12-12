package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Common errors
var (
	ErrDataDirNotFound  = errors.New("data directory not found")
	ErrFileNotFound     = errors.New("file not found")
	ErrInvalidJSON      = errors.New("invalid JSON format")
	ErrInvalidGeoJSON   = errors.New("invalid GeoJSON format")
	ErrStateNotFound    = errors.New("state not found")
	ErrDistrictNotFound = errors.New("district not found")
	ErrACNotFound       = errors.New("assembly constituency not found")
	ErrBoothNotFound    = errors.New("booth not found")
	ErrBoundaryNotFound = errors.New("boundary not found")
)

// statesFile is the JSON structure for states.json
type statesFile struct {
	Metadata struct {
		Version       string `json:"version"`
		LastUpdated   string `json:"lastUpdated"`
		Source        string `json:"source"`
		TotalEntities int    `json:"totalEntities"`
	} `json:"metadata"`
	States           []State `json:"states"`
	UnionTerritories []State `json:"unionTerritories"`
}

// districtsFile is the JSON structure for districts.json
type districtsFile struct {
	Metadata struct {
		Version        string `json:"version"`
		LastUpdated    string `json:"lastUpdated"`
		TotalDistricts int    `json:"totalDistricts"`
	} `json:"metadata"`
	Districts []District `json:"districts"`
}

// acFile is the JSON structure for assembly_constituency.json
type acFile struct {
	States []StateConstituencies `json:"states"`
}

// partiesFile is the JSON structure for parties.json
type partiesFile struct {
	Metadata struct {
		Version     string `json:"version"`
		LastUpdated string `json:"lastUpdated"`
	} `json:"metadata"`
	Parties []Party `json:"parties"`
}

// geoJSONFile represents a GeoJSON FeatureCollection
type geoJSONFile struct {
	Type      string           `json:"type"`
	StateName string           `json:"state_name"`
	Features  []geoJSONFeature `json:"features"`
}

// geoJSONFeature represents a GeoJSON Feature
type geoJSONFeature struct {
	Type       string `json:"type"`
	Properties struct {
		ObjectID int    `json:"objectid"`
		UID      string `json:"uid"`
		StateUT  string `json:"state_ut"`
		ConsCode int    `json:"cons_code"`
		ConsName string `json:"cons_name"`
	} `json:"properties"`
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

// LoadStates loads all states and union territories from states.json
func LoadStates(dataDir string) ([]State, error) {
	filePath := filepath.Join(dataDir, StatesFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
		}
		return nil, err
	}

	var file statesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	// Combine states and union territories
	all := make([]State, 0, len(file.States)+len(file.UnionTerritories))
	all = append(all, file.States...)
	all = append(all, file.UnionTerritories...)

	return all, nil
}

// LoadDistricts loads all districts from districts.json
func LoadDistricts(dataDir string) ([]District, error) {
	filePath := filepath.Join(dataDir, DistrictsFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
		}
		return nil, err
	}

	var file districtsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	return file.Districts, nil
}

// LoadConstituencies loads all assembly constituencies
// Returns a map of state name -> constituencies
func LoadConstituencies(dataDir string) (map[string][]AssemblyConstituency, error) {
	filePath := filepath.Join(dataDir, AssemblyConstituenciesFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
		}
		return nil, err
	}

	var file acFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	result := make(map[string][]AssemblyConstituency)
	for _, state := range file.States {
		// Enrich each constituency with state info
		for i := range state.Constituencies {
			state.Constituencies[i].StateName = state.Name
			state.Constituencies[i].StateSlug = ToSlug(state.Name)
			// Parse AC number from ID (e.g., "ac_1" -> 1)
			if strings.HasPrefix(state.Constituencies[i].ID, "ac_") {
				numStr := strings.TrimPrefix(state.Constituencies[i].ID, "ac_")
				if num, err := strconv.Atoi(numStr); err == nil {
					state.Constituencies[i].ACNumber = num
				}
			}
		}
		result[state.Name] = state.Constituencies
	}

	return result, nil
}

// LoadConstituenciesForState loads constituencies for a specific state
func LoadConstituenciesForState(dataDir, stateName string) ([]AssemblyConstituency, error) {
	all, err := LoadConstituencies(dataDir)
	if err != nil {
		return nil, err
	}

	// Try exact match first
	if acs, ok := all[stateName]; ok {
		return acs, nil
	}

	// Try case-insensitive match
	stateNameLower := strings.ToLower(stateName)
	for name, acs := range all {
		if strings.ToLower(name) == stateNameLower {
			return acs, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrStateNotFound, stateName)
}

// LoadParties loads all political parties from parties.json
func LoadParties(dataDir string) ([]Party, error) {
	filePath := filepath.Join(dataDir, PartiesFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
		}
		return nil, err
	}

	var file partiesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	return file.Parties, nil
}

// LoadBoothsForState loads all booths for a specific state
func LoadBoothsForState(dataDir, stateSlug string) ([]PollingBooth, error) {
	boothsDir := filepath.Join(dataDir, BoothsDir, FromSlug(stateSlug))

	// Check if directory exists
	info, err := os.Stat(boothsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrStateNotFound, stateSlug)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrDataDirNotFound, boothsDir)
	}

	// Read all JSON files in the directory
	entries, err := os.ReadDir(boothsDir)
	if err != nil {
		return nil, err
	}

	var allBooths []PollingBooth
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(boothsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		var booths []PollingBooth
		if err := json.Unmarshal(data, &booths); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalidJSON, entry.Name(), err)
		}

		allBooths = append(allBooths, booths...)
	}

	return allBooths, nil
}

// LoadBoothsForDistrict loads booths for a specific district within a state
func LoadBoothsForDistrict(dataDir, stateSlug, districtSlug string) ([]PollingBooth, error) {
	filePath := filepath.Join(dataDir, BoothsDir, FromSlug(stateSlug), FromSlug(districtSlug)+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s/%s", ErrDistrictNotFound, stateSlug, districtSlug)
		}
		return nil, err
	}

	var booths []PollingBooth
	if err := json.Unmarshal(data, &booths); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	return booths, nil
}

// LoadBoundariesForState loads AC boundaries (GeoJSON) for a state
func LoadBoundariesForState(dataDir, stateSlug string) ([]ACBoundary, error) {
	filePath := filepath.Join(dataDir, BoundariesDir, FromSlug(stateSlug)+".geojson")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrStateNotFound, stateSlug)
		}
		return nil, err
	}

	var geoJSON geoJSONFile
	if err := json.Unmarshal(data, &geoJSON); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidGeoJSON, err)
	}

	boundaries := make([]ACBoundary, 0, len(geoJSON.Features))
	for _, feature := range geoJSON.Features {
		boundary := ACBoundary{
			ObjectID: feature.Properties.ObjectID,
			UID:      feature.Properties.UID,
			StateUT:  feature.Properties.StateUT,
			ConsCode: feature.Properties.ConsCode,
			ConsName: feature.Properties.ConsName,
		}

		// Parse coordinates based on geometry type
		switch feature.Geometry.Type {
		case "Polygon":
			var coords [][][]float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &coords); err != nil {
				return nil, fmt.Errorf("%w: polygon coordinates: %v", ErrInvalidGeoJSON, err)
			}
			boundary.Polygon = coords

		case "MultiPolygon":
			var multiCoords [][][][]float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &multiCoords); err != nil {
				return nil, fmt.Errorf("%w: multipolygon coordinates: %v", ErrInvalidGeoJSON, err)
			}
			// Use the first polygon (largest)
			if len(multiCoords) > 0 {
				boundary.Polygon = multiCoords[0]
			}
		}

		boundaries = append(boundaries, boundary)
	}

	return boundaries, nil
}

// LoadBoundaryForAC loads a specific AC boundary
func LoadBoundaryForAC(dataDir, stateSlug string, consCode int) (*ACBoundary, error) {
	boundaries, err := LoadBoundariesForState(dataDir, stateSlug)
	if err != nil {
		return nil, err
	}

	for i := range boundaries {
		if boundaries[i].ConsCode == consCode {
			return &boundaries[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %s/%d", ErrBoundaryNotFound, stateSlug, consCode)
}

// LoadConstituencyLookup loads the constituency boundary lookup table
func LoadConstituencyLookup(dataDir string) ([]ConstituencyBoundaryLookup, error) {
	filePath := filepath.Join(dataDir, ConstituencyBoundaryLookupFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
		}
		return nil, err
	}

	var lookup []ConstituencyBoundaryLookup
	if err := json.Unmarshal(data, &lookup); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	return lookup, nil
}

// ListAvailableStates returns the list of states that have booth data
func ListAvailableStates(dataDir string) ([]string, error) {
	boothsDir := filepath.Join(dataDir, BoothsDir)

	entries, err := os.ReadDir(boothsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrDataDirNotFound, boothsDir)
		}
		return nil, err
	}

	var states []string
	for _, entry := range entries {
		if entry.IsDir() {
			states = append(states, entry.Name())
		}
	}

	return states, nil
}

// ListAvailableBoundaries returns the list of states that have boundary data
func ListAvailableBoundaries(dataDir string) ([]string, error) {
	boundariesDir := filepath.Join(dataDir, BoundariesDir)

	entries, err := os.ReadDir(boundariesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrDataDirNotFound, boundariesDir)
		}
		return nil, err
	}

	var states []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".geojson") {
			name := strings.TrimSuffix(entry.Name(), ".geojson")
			states = append(states, name)
		}
	}

	return states, nil
}

// GetBoothCount returns the total number of booths for a state
func GetBoothCount(dataDir, stateSlug string) (int, error) {
	booths, err := LoadBoothsForState(dataDir, stateSlug)
	if err != nil {
		return 0, err
	}
	return len(booths), nil
}

// GetACCount returns the number of ACs for a state
func GetACCount(dataDir, stateName string) (int, error) {
	acs, err := LoadConstituenciesForState(dataDir, stateName)
	if err != nil {
		return 0, err
	}
	return len(acs), nil
}
