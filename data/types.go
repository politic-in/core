// Package data provides loaders and indices for Indian geographic and electoral data.
// It integrates with booth-matching and h3-utils packages for location-based operations.
package data

import (
	"fmt"
	"strings"
)

// State represents an Indian state or union territory
type State struct {
	ID          int          `json:"id"`
	StateID     string       `json:"stateId"` // "AP", "KA", "TN"
	Name        string       `json:"name"`    // "Andhra Pradesh"
	Type        string       `json:"type"`    // "state" or "union_territory"
	Capital     string       `json:"capital"`
	Latitude    float64      `json:"latitude"`
	Longitude   float64      `json:"longitude"`
	Region      string       `json:"region"` // "South India", "North India"
	NewsSources []NewsSource `json:"newsSources,omitempty"`
}

// NewsSource represents a news source configuration for a state
type NewsSource struct {
	Name                 string            `json:"name"`
	BaseURL              string            `json:"baseUrl"`
	Selectors            map[string]string `json:"selectors,omitempty"`
	Priority             int               `json:"priority"`
	ScrapeFrequencyHours int               `json:"scrapeFrequencyHours"`
}

// Slug returns the URL-friendly slug for the state name
func (s State) Slug() string {
	return ToSlug(s.Name)
}

// District represents an Indian district
type District struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	State     string  `json:"state"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Slug returns the URL-friendly slug for the district name
func (d District) Slug() string {
	return ToSlug(d.Name)
}

// AssemblyConstituency represents an Assembly Constituency (AC)
type AssemblyConstituency struct {
	ID        string  `json:"id"`       // "ac_1", "ac_2"
	Name      string  `json:"name"`     // "Ichchapuram"
	Reserved  string  `json:"reserved"` // "None", "SC", "ST"
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`

	// Derived fields (populated during loading)
	StateName string `json:"-"`
	StateSlug string `json:"-"`
	ACNumber  int    `json:"-"` // Parsed from ID
}

// IsReserved returns true if the constituency is reserved (SC/ST)
func (ac AssemblyConstituency) IsReserved() bool {
	return ac.Reserved != "" && ac.Reserved != "None"
}

// IsReservedSC returns true if reserved for Scheduled Castes
func (ac AssemblyConstituency) IsReservedSC() bool {
	return ac.Reserved == "SC"
}

// IsReservedST returns true if reserved for Scheduled Tribes
func (ac AssemblyConstituency) IsReservedST() bool {
	return ac.Reserved == "ST"
}

// StateConstituencies represents constituencies for a state
type StateConstituencies struct {
	Name           string                 `json:"name"`
	TotalSeats     int                    `json:"totalSeats"`
	Constituencies []AssemblyConstituency `json:"constituencies"`
}

// PollingBooth represents a polling station/booth
type PollingBooth struct {
	PartID       int      `json:"partId"`
	StateCode    string   `json:"stateCode"`    // "S10"
	StateName    string   `json:"stateName"`    // "Karnataka"
	DistrictCode string   `json:"districtCode"` // "S1022"
	DistrictName string   `json:"districtName"` // "BANGALORE RURAL"
	ACNumber     int      `json:"acNumber"`     // 179
	ACName       string   `json:"acName"`       // "Devanahalli"
	PartNumber   int      `json:"partNumber"`   // 1
	PartName     string   `json:"partName"`     // "Govt Higher Primary School, Gunjuru"
	Lat          *float64 `json:"lat,omitempty"`
	Lon          *float64 `json:"lon,omitempty"`
}

// FullName returns the full booth name including part number
func (b PollingBooth) FullName() string {
	return fmt.Sprintf("%d - %s", b.PartNumber, b.PartName)
}

// ACBoundary represents a GeoJSON polygon for an Assembly Constituency
type ACBoundary struct {
	ObjectID int           `json:"objectid"`
	UID      string        `json:"uid"`
	StateUT  string        `json:"state_ut"`
	ConsCode int           `json:"cons_code"`
	ConsName string        `json:"cons_name"`
	Polygon  [][][]float64 `json:"-"` // [ring][point][lng,lat]
}

// GetExteriorRing returns the exterior ring of the polygon
func (b ACBoundary) GetExteriorRing() [][]float64 {
	if len(b.Polygon) == 0 {
		return nil
	}
	return b.Polygon[0]
}

// GetHoles returns the holes (interior rings) of the polygon
func (b ACBoundary) GetHoles() [][][]float64 {
	if len(b.Polygon) <= 1 {
		return nil
	}
	return b.Polygon[1:]
}

// BoundingBox returns [minLng, minLat, maxLng, maxLat]
func (b ACBoundary) BoundingBox() [4]float64 {
	ring := b.GetExteriorRing()
	if len(ring) == 0 {
		return [4]float64{}
	}

	minLng, minLat := ring[0][0], ring[0][1]
	maxLng, maxLat := ring[0][0], ring[0][1]

	for _, pt := range ring {
		if pt[0] < minLng {
			minLng = pt[0]
		}
		if pt[0] > maxLng {
			maxLng = pt[0]
		}
		if pt[1] < minLat {
			minLat = pt[1]
		}
		if pt[1] > maxLat {
			maxLat = pt[1]
		}
	}

	return [4]float64{minLng, minLat, maxLng, maxLat}
}

// ContainsPoint checks if a point is inside the boundary using ray casting
func (b ACBoundary) ContainsPoint(lat, lng float64) bool {
	ring := b.GetExteriorRing()
	if len(ring) == 0 {
		return false
	}

	// Check bounding box first (fast rejection)
	bbox := b.BoundingBox()
	if lng < bbox[0] || lng > bbox[2] || lat < bbox[1] || lat > bbox[3] {
		return false
	}

	// Ray casting algorithm
	inside := pointInRing(lat, lng, ring)

	// Check holes - if point is in a hole, it's outside the polygon
	for _, hole := range b.GetHoles() {
		if pointInRing(lat, lng, hole) {
			inside = !inside
		}
	}

	return inside
}

// pointInRing uses ray casting to check if point is inside ring
func pointInRing(lat, lng float64, ring [][]float64) bool {
	n := len(ring)
	inside := false

	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := ring[i][0], ring[i][1] // lng, lat
		xj, yj := ring[j][0], ring[j][1]

		if ((yi > lat) != (yj > lat)) &&
			(lng < (xj-xi)*(lat-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}

	return inside
}

// Party represents a political party
type Party struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	ShortName    string   `json:"shortName"`
	Symbol       string   `json:"symbol,omitempty"`
	Founded      int      `json:"founded,omitempty"`
	Headquarters string   `json:"headquarters,omitempty"`
	Ideology     []string `json:"ideology,omitempty"`
	Alliance     string   `json:"alliance,omitempty"`
	Website      string   `json:"website,omitempty"`
}

// ConstituencyBoundaryLookup represents a lookup entry for coordinate-to-AC mapping
type ConstituencyBoundaryLookup struct {
	StateName string  `json:"state_name"`
	ACCode    int     `json:"ac_code"`
	ACName    string  `json:"ac_name"`
	CenterLat float64 `json:"center_lat"`
	CenterLng float64 `json:"center_lng"`
}

// ToSlug converts a name to a URL-friendly slug
func ToSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "_")
	slug = strings.ReplaceAll(slug, "&", "_")
	slug = strings.ReplaceAll(slug, "-", "_")
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, ".", "")
	slug = strings.ReplaceAll(slug, ",", "")
	// Replace multiple underscores with single
	for strings.Contains(slug, "__") {
		slug = strings.ReplaceAll(slug, "__", "_")
	}
	return slug
}

// FromSlug converts a slug back to a likely directory name
func FromSlug(slug string) string {
	return strings.ToLower(slug)
}

// boothDirToStateSlug maps booth directory names to proper state slugs
// This handles cases where directory names don't match ToSlug(stateName)
var boothDirToStateSlug = map[string]string{
	"nct_of_delhi":                       "delhi",
	"andaman__nicobar_islands":           "andaman_and_nicobar_islands",
	"dadra__nagar_haveli_and_daman__diu": "dadra_and_nagar_haveli_and_daman_and_diu",
}

// NormalizeBoothDirToStateSlug converts a booth directory name to a proper state slug
// that can be matched against ToSlug(stateName)
func NormalizeBoothDirToStateSlug(dirName string) string {
	if slug, ok := boothDirToStateSlug[dirName]; ok {
		return slug
	}
	return dirName
}

// File names
const (
	StatesFile                     = "states.json"
	DistrictsFile                  = "districts.json"
	AssemblyConstituenciesFile     = "assembly_constituency.json"
	PartiesFile                    = "parties.json"
	ConstituencyBoundaryLookupFile = "constituency_boundary_lookup.json"
	BoothsDir                      = "booths"
	BoundariesDir                  = "boundaries"
)
