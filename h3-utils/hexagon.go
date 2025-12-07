// Package h3utils provides H3 hexagon utilities for Politic.
// Wraps uber/h3-go with Politic-specific helpers.
package h3utils

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/uber/h3-go/v4"
)

// Error definitions
var (
	ErrInvalidCellID      = errors.New("invalid H3 cell ID")
	ErrInvalidCoordinates = errors.New("invalid coordinates")
	ErrInvalidResolution  = errors.New("invalid resolution")
	ErrInvalidPolygon     = errors.New("invalid polygon")
	ErrCellNotFound       = errors.New("cell not found")
	ErrDistanceCalcFailed = errors.New("distance calculation failed")
)

// Resolution constants
const (
	// DefaultResolution is H3 resolution 9 (~0.1 km² per hexagon)
	DefaultResolution = 9

	// MinResolution is the minimum supported resolution
	MinResolution = 0

	// MaxResolution is the maximum supported resolution
	MaxResolution = 15

	// StateResolution is resolution for state-level aggregation
	StateResolution = 4

	// DistrictResolution is resolution for district-level aggregation
	DistrictResolution = 6

	// ACResolution is resolution for AC-level aggregation
	ACResolution = 7

	// NeighborhoodResolution is resolution for neighborhood-level detail
	NeighborhoodResolution = 9
)

// Approximate areas in square kilometers for each resolution (average)
var ResolutionAreasKm2 = map[int]float64{
	0:  4250546.848,
	1:  607220.9782,
	2:  86745.854,
	3:  12392.264,
	4:  1770.324,
	5:  252.903,
	6:  36.129,
	7:  5.161,
	8:  0.737,
	9:  0.105,
	10: 0.015,
	11: 0.0022,
	12: 0.0003,
	13: 0.000044,
	14: 0.0000063,
	15: 0.0000009,
}

// ResolutionAreas is approximate areas in square meters for each resolution
var ResolutionAreas = map[int]float64{
	0:  4250546848000.0,
	1:  607220978200.0,
	2:  86745854000.0,
	3:  12392264000.0,
	4:  1770324000.0,
	5:  252903000.0,
	6:  36129000.0,
	7:  5161000.0,
	8:  737000.0,
	9:  105000.0,
	10: 15000.0,
	11: 2200.0,
	12: 300.0,
	13: 44.0,
	14: 6.3,
	15: 0.9,
}

// LatLng represents a geographic coordinate
type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// HexagonInfo contains metadata about a hexagon
type HexagonInfo struct {
	ID           string  `json:"id"`
	CenterLat    float64 `json:"center_lat"`
	CenterLng    float64 `json:"center_lng"`
	Resolution   int     `json:"resolution"`
	AreaM2       float64 `json:"area_m2"`
	AreaKm2      float64 `json:"area_km2"`
	ParentID     string  `json:"parent_id,omitempty"`
	IsPentagon   bool    `json:"is_pentagon"`
	BoundaryGeoJSON string `json:"boundary_geojson,omitempty"`
}

// cellFromString parses a hex string into an H3 Cell
func cellFromString(cellID string) (h3.Cell, error) {
	var cell h3.Cell
	if err := cell.UnmarshalText([]byte(cellID)); err != nil {
		return 0, err
	}
	if !cell.IsValid() {
		return 0, fmt.Errorf("invalid cell")
	}
	return cell, nil
}

// LatLngToCell converts lat/lng to H3 cell at default resolution
func LatLngToCell(lat, lng float64) string {
	return LatLngToCellAtResolution(lat, lng, DefaultResolution)
}

// LatLngToCellAtResolution converts lat/lng to H3 cell at specified resolution
func LatLngToCellAtResolution(lat, lng float64, resolution int) string {
	if resolution < MinResolution || resolution > MaxResolution {
		return ""
	}
	latLng := h3.NewLatLng(lat, lng)
	cell := h3.LatLngToCell(latLng, resolution)
	return cell.String()
}

// CellToLatLng returns the center of an H3 cell
func CellToLatLng(cellID string) (lat, lng float64, err error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	latLng := cell.LatLng()
	return latLng.Lat, latLng.Lng, nil
}

// GetNeighbors returns the immediate neighbors of a cell (k-ring with k=1)
func GetNeighbors(cellID string) ([]string, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	neighbors := cell.GridDisk(1)
	result := make([]string, 0, len(neighbors)-1)

	for _, n := range neighbors {
		// Exclude the center cell itself
		if n.String() != cellID {
			result = append(result, n.String())
		}
	}

	return result, nil
}

// GetCellsInRadius returns all cells within k hexagons of the center
func GetCellsInRadius(cellID string, k int) ([]string, error) {
	if k < 0 {
		return nil, fmt.Errorf("radius must be non-negative")
	}

	cell, err := cellFromString(cellID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	cells := cell.GridDisk(k)
	result := make([]string, len(cells))

	for i, c := range cells {
		result[i] = c.String()
	}

	return result, nil
}

// GetRing returns cells at exactly distance k from center (hollow ring)
func GetRing(cellID string, k int) ([]string, error) {
	if k < 0 {
		return nil, fmt.Errorf("ring distance must be non-negative")
	}
	if k == 0 {
		return []string{cellID}, nil
	}

	cell, err := cellFromString(cellID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	// Get all cells in disk of radius k and k-1, then subtract
	diskK := cell.GridDisk(k)
	diskKMinus1 := cell.GridDisk(k - 1)

	// Create set of inner disk
	innerSet := make(map[h3.Cell]bool)
	for _, c := range diskKMinus1 {
		innerSet[c] = true
	}

	// Ring = outer disk - inner disk
	var ring []string
	for _, c := range diskK {
		if !innerSet[c] {
			ring = append(ring, c.String())
		}
	}

	return ring, nil
}

// DistanceInCells returns the grid distance between two cells
func DistanceInCells(cellID1, cellID2 string) (int, error) {
	cell1, err := cellFromString(cellID1)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID1)
	}

	cell2, err := cellFromString(cellID2)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID2)
	}

	dist := h3.GridDistance(cell1, cell2)
	return dist, nil
}

// DistanceInMeters calculates approximate distance between cell centers in meters
func DistanceInMeters(cellID1, cellID2 string) (float64, error) {
	lat1, lng1, err := CellToLatLng(cellID1)
	if err != nil {
		return 0, err
	}

	lat2, lng2, err := CellToLatLng(cellID2)
	if err != nil {
		return 0, err
	}

	return HaversineDistance(lat1, lng1, lat2, lng2), nil
}

// HaversineDistance calculates the distance between two points in meters
func HaversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371000 // meters

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// IsValidCell checks if a cell ID is valid
func IsValidCell(cellID string) bool {
	cell, err := cellFromString(cellID)
	if err != nil {
		return false
	}
	return cell.IsValid()
}

// GetResolution returns the resolution of a cell
func GetResolution(cellID string) (int, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}
	return cell.Resolution(), nil
}

// GetParent returns the parent cell at the specified resolution
func GetParent(cellID string, parentResolution int) (string, error) {
	if parentResolution < MinResolution || parentResolution > MaxResolution {
		return "", ErrInvalidResolution
	}

	cell, err := cellFromString(cellID)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	if parentResolution >= cell.Resolution() {
		return "", fmt.Errorf("parent resolution must be less than cell resolution")
	}

	parent := cell.Parent(parentResolution)
	return parent.String(), nil
}

// GetChildren returns the child cells at the specified resolution
func GetChildren(cellID string, childResolution int) ([]string, error) {
	if childResolution < MinResolution || childResolution > MaxResolution {
		return nil, ErrInvalidResolution
	}

	cell, err := cellFromString(cellID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	if childResolution <= cell.Resolution() {
		return nil, fmt.Errorf("child resolution must be greater than cell resolution")
	}

	children := cell.Children(childResolution)
	result := make([]string, len(children))

	for i, c := range children {
		result[i] = c.String()
	}

	return result, nil
}

// CellArea returns the area of a cell in square meters
func CellArea(cellID string) (float64, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	return h3.CellAreaM2(cell), nil
}

// GetHexagonInfo returns detailed info about a hexagon
func GetHexagonInfo(cellID string) (*HexagonInfo, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	latLng := cell.LatLng()
	area := h3.CellAreaM2(cell)

	info := &HexagonInfo{
		ID:         cellID,
		CenterLat:  latLng.Lat,
		CenterLng:  latLng.Lng,
		Resolution: cell.Resolution(),
		AreaM2:     area,
		AreaKm2:    area / 1000000,
		IsPentagon: cell.IsPentagon(),
	}

	// Get parent at one level up if not at resolution 0
	if cell.Resolution() > 0 {
		parent := cell.Parent(cell.Resolution() - 1)
		info.ParentID = parent.String()
	}

	return info, nil
}

// PolygonToCells fills a polygon with H3 cells
// Polygon is defined as a slice of lat/lng pairs
func PolygonToCells(polygon [][2]float64, resolution int) ([]string, error) {
	if len(polygon) < 3 {
		return nil, ErrInvalidPolygon
	}

	if resolution < MinResolution || resolution > MaxResolution {
		return nil, ErrInvalidResolution
	}

	// Convert to h3 LatLng
	geoLoop := make([]h3.LatLng, len(polygon))
	for i, coord := range polygon {
		geoLoop[i] = h3.NewLatLng(coord[0], coord[1])
	}

	cells := h3.PolygonToCells(h3.GeoPolygon{
		GeoLoop: geoLoop,
	}, resolution)

	result := make([]string, len(cells))
	for i, c := range cells {
		result[i] = c.String()
	}

	return result, nil
}

// CellsToMultiPolygon converts a set of cells to multi-polygon GeoJSON coordinates
func CellsToMultiPolygon(cellIDs []string) ([][][]float64, error) {
	cells := make([]h3.Cell, len(cellIDs))
	for i, id := range cellIDs {
		cell, err := cellFromString(id)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, id)
		}
		cells[i] = cell
	}

	multiPoly := h3.CellsToMultiPolygon(cells)

	result := make([][][]float64, len(multiPoly))
	for i, poly := range multiPoly {
		coords := make([][]float64, len(poly.GeoLoop))
		for j, ll := range poly.GeoLoop {
			coords[j] = []float64{ll.Lng, ll.Lat} // GeoJSON is [lng, lat]
		}
		result[i] = coords
	}

	return result, nil
}

// GetCellBoundary returns the boundary vertices of a cell
func GetCellBoundary(cellID string) ([]LatLng, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	boundary := cell.Boundary()
	result := make([]LatLng, len(boundary))

	for i, ll := range boundary {
		result[i] = LatLng{Lat: ll.Lat, Lng: ll.Lng}
	}

	return result, nil
}

// CompactCells compacts a set of cells to their common ancestors
func CompactCells(cellIDs []string) ([]string, error) {
	if len(cellIDs) == 0 {
		return []string{}, nil
	}

	cells := make([]h3.Cell, len(cellIDs))
	for i, id := range cellIDs {
		cell, err := cellFromString(id)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, id)
		}
		cells[i] = cell
	}

	compacted := h3.CompactCells(cells)
	result := make([]string, len(compacted))

	for i, c := range compacted {
		result[i] = c.String()
	}

	return result, nil
}

// UncompactCells expands compacted cells to a target resolution
func UncompactCells(cellIDs []string, resolution int) ([]string, error) {
	if len(cellIDs) == 0 {
		return []string{}, nil
	}

	if resolution < MinResolution || resolution > MaxResolution {
		return nil, ErrInvalidResolution
	}

	cells := make([]h3.Cell, len(cellIDs))
	for i, id := range cellIDs {
		cell, err := cellFromString(id)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidCellID, id)
		}
		cells[i] = cell
	}

	uncompacted := h3.UncompactCells(cells, resolution)
	result := make([]string, len(uncompacted))

	for i, c := range uncompacted {
		result[i] = c.String()
	}

	return result, nil
}

// Batch operations

// BatchLatLngToCell converts multiple coordinates to cells
func BatchLatLngToCell(coords []LatLng, resolution int) []string {
	result := make([]string, len(coords))
	for i, coord := range coords {
		result[i] = LatLngToCellAtResolution(coord.Lat, coord.Lng, resolution)
	}
	return result
}

// BatchCellToLatLng converts multiple cells to coordinates
func BatchCellToLatLng(cellIDs []string) ([]LatLng, []error) {
	results := make([]LatLng, len(cellIDs))
	errs := make([]error, len(cellIDs))

	for i, id := range cellIDs {
		lat, lng, err := CellToLatLng(id)
		if err != nil {
			errs[i] = err
		} else {
			results[i] = LatLng{Lat: lat, Lng: lng}
		}
	}

	return results, errs
}

// ParallelLatLngToCell converts coordinates to cells in parallel
func ParallelLatLngToCell(coords []LatLng, resolution int, workers int) []string {
	if workers <= 0 {
		workers = 4
	}

	result := make([]string, len(coords))
	var wg sync.WaitGroup
	chunkSize := (len(coords) + workers - 1) / workers

	for w := 0; w < workers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(coords) {
			end = len(coords)
		}
		if start >= end {
			break
		}

		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				result[i] = LatLngToCellAtResolution(coords[i].Lat, coords[i].Lng, resolution)
			}
		}(start, end)
	}

	wg.Wait()
	return result
}

// GetUniqueCells returns unique cells from a list
func GetUniqueCells(cellIDs []string) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, id := range cellIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	return unique
}

// SortCells sorts cells by their ID
func SortCells(cellIDs []string) []string {
	sorted := make([]string, len(cellIDs))
	copy(sorted, cellIDs)
	sort.Strings(sorted)
	return sorted
}

// CellContains checks if a point is inside a cell
func CellContains(cellID string, lat, lng float64) (bool, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}

	// Get the cell that contains the point
	pointCell := h3.LatLngToCell(h3.NewLatLng(lat, lng), cell.Resolution())

	return cell == pointCell, nil
}

// FindCellsInBoundingBox returns all cells that intersect with a bounding box
func FindCellsInBoundingBox(minLat, minLng, maxLat, maxLng float64, resolution int) ([]string, error) {
	if resolution < MinResolution || resolution > MaxResolution {
		return nil, ErrInvalidResolution
	}

	// Create polygon from bounding box
	polygon := [][2]float64{
		{minLat, minLng},
		{minLat, maxLng},
		{maxLat, maxLng},
		{maxLat, minLng},
		{minLat, minLng}, // Close the polygon
	}

	return PolygonToCells(polygon, resolution)
}

// GetCellsAlongLine returns cells along a line between two points
func GetCellsAlongLine(startLat, startLng, endLat, endLng float64, resolution int) ([]string, error) {
	if resolution < MinResolution || resolution > MaxResolution {
		return nil, ErrInvalidResolution
	}

	startCell, err := cellFromString(LatLngToCellAtResolution(startLat, startLng, resolution))
	if err != nil {
		return nil, err
	}

	endCell, err := cellFromString(LatLngToCellAtResolution(endLat, endLng, resolution))
	if err != nil {
		return nil, err
	}

	line := startCell.GridPath(endCell)
	result := make([]string, len(line))

	for i, c := range line {
		result[i] = c.String()
	}

	return result, nil
}

// IsPentagon checks if a cell is a pentagon (12 pentagons per resolution)
func IsPentagon(cellID string) (bool, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}
	return cell.IsPentagon(), nil
}

// GetBaseCellNumber returns the base cell number (0-121)
func GetBaseCellNumber(cellID string) (int, error) {
	cell, err := cellFromString(cellID)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidCellID, cellID)
	}
	return cell.BaseCellNumber(), nil
}

// EstimateAreaKm2 estimates the area covered by cells in square kilometers
func EstimateAreaKm2(cellIDs []string) (float64, error) {
	var totalArea float64
	for _, id := range cellIDs {
		area, err := CellArea(id)
		if err != nil {
			return 0, err
		}
		totalArea += area
	}
	return totalArea / 1000000, nil // Convert m² to km²
}

// GetResolutionForArea returns the appropriate resolution for a target area
func GetResolutionForArea(targetAreaM2 float64) int {
	for res := MaxResolution; res >= MinResolution; res-- {
		if ResolutionAreas[res] >= targetAreaM2 {
			return res
		}
	}
	return MinResolution
}

// GroupCellsByParent groups cells by their parent at a given resolution
func GroupCellsByParent(cellIDs []string, parentResolution int) (map[string][]string, error) {
	groups := make(map[string][]string)

	for _, id := range cellIDs {
		parent, err := GetParent(id, parentResolution)
		if err != nil {
			return nil, err
		}
		groups[parent] = append(groups[parent], id)
	}

	return groups, nil
}

// FilterCellsByResolution filters cells to only include those at the specified resolution
func FilterCellsByResolution(cellIDs []string, resolution int) []string {
	var filtered []string
	for _, id := range cellIDs {
		res, err := GetResolution(id)
		if err == nil && res == resolution {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

// CellSetIntersection returns cells that exist in both sets
func CellSetIntersection(set1, set2 []string) []string {
	setMap := make(map[string]bool)
	for _, id := range set1 {
		setMap[id] = true
	}

	var intersection []string
	for _, id := range set2 {
		if setMap[id] {
			intersection = append(intersection, id)
		}
	}

	return intersection
}

// CellSetUnion returns all unique cells from both sets
func CellSetUnion(set1, set2 []string) []string {
	setMap := make(map[string]bool)

	for _, id := range set1 {
		setMap[id] = true
	}
	for _, id := range set2 {
		setMap[id] = true
	}

	union := make([]string, 0, len(setMap))
	for id := range setMap {
		union = append(union, id)
	}

	return union
}

// CellSetDifference returns cells in set1 that are not in set2
func CellSetDifference(set1, set2 []string) []string {
	set2Map := make(map[string]bool)
	for _, id := range set2 {
		set2Map[id] = true
	}

	var diff []string
	for _, id := range set1 {
		if !set2Map[id] {
			diff = append(diff, id)
		}
	}

	return diff
}
