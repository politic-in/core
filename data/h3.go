package data

import (
	"fmt"

	h3utils "github.com/politic-in/core/h3-utils"
)

// H3CellInfo contains information about an H3 cell and its electoral mapping
type H3CellInfo struct {
	CellID     string
	Resolution int
	Latitude   float64
	Longitude  float64
	StateSlug  string
	ACBoundary *ACBoundary
	ACName     string
	ACCode     int
}

// GetH3CellInfo returns electoral information for an H3 cell
func (g *GeoIndex) GetH3CellInfo(cellID string) (*H3CellInfo, error) {
	// Get cell center coordinates
	lat, lng, err := h3utils.CellToLatLng(cellID)
	if err != nil {
		return nil, fmt.Errorf("invalid H3 cell: %w", err)
	}

	res, err := h3utils.GetResolution(cellID)
	if err != nil {
		return nil, err
	}

	info := &H3CellInfo{
		CellID:     cellID,
		Resolution: res,
		Latitude:   lat,
		Longitude:  lng,
	}

	// Find AC at this location
	boundary, stateSlug, err := g.FindACAtPointAllStates(lat, lng)
	if err == nil {
		info.StateSlug = stateSlug
		info.ACBoundary = boundary
		info.ACName = boundary.ConsName
		info.ACCode = boundary.ConsCode
	}

	return info, nil
}

// GetACForH3Cell returns the AC containing an H3 cell
func (g *GeoIndex) GetACForH3Cell(cellID string) (*ACBoundary, string, error) {
	lat, lng, err := h3utils.CellToLatLng(cellID)
	if err != nil {
		return nil, "", fmt.Errorf("invalid H3 cell: %w", err)
	}

	return g.FindACAtPointAllStates(lat, lng)
}

// GetH3CellsForAC returns H3 cells that cover an AC boundary
func (g *GeoIndex) GetH3CellsForAC(stateSlug string, consCode, resolution int) ([]string, error) {
	boundary, err := g.GetBoundaryForAC(stateSlug, consCode)
	if err != nil {
		return nil, err
	}

	ring := boundary.GetExteriorRing()
	if len(ring) == 0 {
		return nil, fmt.Errorf("%w: empty polygon for %s/%d", ErrBoundaryNotFound, stateSlug, consCode)
	}

	// Convert ring to lat/lng pairs for polyfill
	coords := make([][2]float64, len(ring))
	for i, pt := range ring {
		coords[i] = [2]float64{pt[1], pt[0]} // [lat, lng] - GeoJSON is [lng, lat]
	}

	// Use h3-utils to get cells covering the polygon
	cells, err := h3utils.PolygonToCells(coords, resolution)
	if err != nil {
		return nil, fmt.Errorf("polyfill error: %w", err)
	}

	return cells, nil
}

// H3CellToACMapping maps H3 cells to their ACs for a state
type H3CellToACMapping struct {
	CellID     string
	StateSlug  string
	ACCode     int
	ACName     string
	Resolution int
}

// BuildH3ToACMapping builds a mapping of H3 cells to ACs for a state
func (g *GeoIndex) BuildH3ToACMapping(stateSlug string, resolution int) ([]H3CellToACMapping, error) {
	boundaries, err := g.GetBoundariesForState(stateSlug)
	if err != nil {
		return nil, err
	}

	var mappings []H3CellToACMapping

	for _, boundary := range boundaries {
		cells, err := g.GetH3CellsForAC(stateSlug, boundary.ConsCode, resolution)
		if err != nil {
			// Skip ACs with errors (e.g., empty polygons)
			continue
		}

		for _, cellID := range cells {
			mappings = append(mappings, H3CellToACMapping{
				CellID:     cellID,
				StateSlug:  stateSlug,
				ACCode:     boundary.ConsCode,
				ACName:     boundary.ConsName,
				Resolution: resolution,
			})
		}
	}

	return mappings, nil
}

// NearbyBooths finds booths near a location using H3 neighbors
func (g *GeoIndex) NearbyBooths(stateSlug string, lat, lng float64, k int) ([]*PollingBooth, error) {
	// Get H3 cell at resolution 9 (~0.1 km^2)
	cellID := h3utils.LatLngToCellAtResolution(lat, lng, 9)

	// Get the AC at this location
	boundary, err := g.FindACAtPoint(stateSlug, lat, lng)
	if err != nil {
		return nil, err
	}

	// Get all booths in the AC
	booths, err := g.GetBoothsForAC(stateSlug, boundary.ConsCode)
	if err != nil {
		return nil, err
	}

	// Get k-ring of cells around the location
	neighbors, err := h3utils.GetCellsInRadius(cellID, k)
	if err != nil {
		return nil, err
	}

	// Create a set of neighbor cells
	cellSet := make(map[string]bool)
	for _, n := range neighbors {
		cellSet[n] = true
	}

	// Filter booths that fall within the cell neighborhood
	// Since booths don't have coordinates, return all booths in the AC
	// In a real implementation, booths would have lat/lng
	return booths, nil
}

// BoothsInH3Cell returns booths whose AC contains an H3 cell
func (g *GeoIndex) BoothsInH3Cell(cellID string) ([]*PollingBooth, error) {
	// Get AC for this cell
	boundary, stateSlug, err := g.GetACForH3Cell(cellID)
	if err != nil {
		return nil, err
	}

	// Return all booths in this AC
	return g.GetBoothsForAC(stateSlug, boundary.ConsCode)
}

// ACStats returns statistics for an AC
type ACStats struct {
	StateSlug   string
	ACCode      int
	ACName      string
	BoothCount  int
	AreaKm2     float64
	BoundingBox [4]float64 // [minLng, minLat, maxLng, maxLat]
	CenterLat   float64
	CenterLng   float64
	H3CellsRes9 int // Number of H3 cells at resolution 9
}

// GetACStats returns detailed statistics for an AC
func (g *GeoIndex) GetACStats(stateSlug string, consCode int) (*ACStats, error) {
	boundary, err := g.GetBoundaryForAC(stateSlug, consCode)
	if err != nil {
		return nil, err
	}

	// Get booths
	booths, _ := g.GetBoothsForAC(stateSlug, consCode)

	stats := &ACStats{
		StateSlug:   stateSlug,
		ACCode:      consCode,
		ACName:      boundary.ConsName,
		BoothCount:  len(booths),
		BoundingBox: boundary.BoundingBox(),
	}

	// Calculate center from bounding box
	stats.CenterLat = (stats.BoundingBox[1] + stats.BoundingBox[3]) / 2
	stats.CenterLng = (stats.BoundingBox[0] + stats.BoundingBox[2]) / 2

	// Calculate area using H3 cells
	cells, err := g.GetH3CellsForAC(stateSlug, consCode, 9)
	if err == nil {
		stats.H3CellsRes9 = len(cells)
		// Each res-9 cell is approximately 0.1052 km^2
		stats.AreaKm2 = float64(len(cells)) * 0.1052
	}

	return stats, nil
}

// LatLngToH3Cell converts coordinates to H3 cell at default resolution
func LatLngToH3Cell(lat, lng float64) string {
	return h3utils.LatLngToCell(lat, lng)
}

// LatLngToH3CellAtResolution converts coordinates to H3 cell at specified resolution
func LatLngToH3CellAtResolution(lat, lng float64, resolution int) string {
	return h3utils.LatLngToCellAtResolution(lat, lng, resolution)
}
