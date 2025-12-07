// Package h3utils provides H3 hexagon utilities for Politic.
package h3utils

import (
	"math"
	"sync"
	"testing"
)

// Test coordinates for India (Delhi area)
const (
	testLat = 28.6139  // Delhi latitude
	testLng = 77.2090  // Delhi longitude
)

func TestLatLngToCell(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	if cell == "" {
		t.Error("expected non-empty cell ID")
	}

	// Verify it's a valid cell
	if !IsValidCell(cell) {
		t.Errorf("expected valid cell, got %s", cell)
	}

	// Verify resolution
	res, err := GetResolution(cell)
	if err != nil {
		t.Errorf("failed to get resolution: %v", err)
	}
	if res != DefaultResolution {
		t.Errorf("expected resolution %d, got %d", DefaultResolution, res)
	}
}

func TestLatLngToCellAtResolution(t *testing.T) {
	tests := []struct {
		name       string
		lat        float64
		lng        float64
		resolution int
		wantEmpty  bool
	}{
		{"valid resolution 0", testLat, testLng, 0, false},
		{"valid resolution 9", testLat, testLng, 9, false},
		{"valid resolution 15", testLat, testLng, 15, false},
		{"invalid resolution -1", testLat, testLng, -1, true},
		{"invalid resolution 16", testLat, testLng, 16, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := LatLngToCellAtResolution(tt.lat, tt.lng, tt.resolution)

			if tt.wantEmpty && cell != "" {
				t.Errorf("expected empty cell for invalid resolution")
			}
			if !tt.wantEmpty && cell == "" {
				t.Error("expected non-empty cell")
			}
		})
	}
}

func TestCellToLatLng(t *testing.T) {
	// Get cell for known coordinates
	cell := LatLngToCell(testLat, testLng)

	// Convert back to lat/lng
	lat, lng, err := CellToLatLng(cell)
	if err != nil {
		t.Errorf("failed to convert cell to lat/lng: %v", err)
	}

	// Verify coordinates are close to original (within cell precision)
	if math.Abs(lat-testLat) > 0.01 {
		t.Errorf("latitude deviation too large: got %f, want ~%f", lat, testLat)
	}
	if math.Abs(lng-testLng) > 0.01 {
		t.Errorf("longitude deviation too large: got %f, want ~%f", lng, testLng)
	}
}

func TestCellToLatLng_InvalidCell(t *testing.T) {
	_, _, err := CellToLatLng("invalid-cell-id")
	if err == nil {
		t.Error("expected error for invalid cell ID")
	}
}

func TestGetNeighbors(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	neighbors, err := GetNeighbors(cell)
	if err != nil {
		t.Errorf("failed to get neighbors: %v", err)
	}

	// Hexagons have 6 neighbors (pentagons have 5)
	if len(neighbors) < 5 || len(neighbors) > 6 {
		t.Errorf("expected 5-6 neighbors, got %d", len(neighbors))
	}

	// Verify center cell is not in neighbors
	for _, n := range neighbors {
		if n == cell {
			t.Error("center cell should not be in neighbors list")
		}
	}
}

func TestGetNeighbors_InvalidCell(t *testing.T) {
	_, err := GetNeighbors("invalid-cell")
	if err == nil {
		t.Error("expected error for invalid cell")
	}
}

func TestGetCellsInRadius(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	tests := []struct {
		name     string
		k        int
		minCells int
		maxCells int
		wantErr  bool
	}{
		{"radius 0", 0, 1, 1, false},
		{"radius 1", 1, 7, 7, false},
		{"radius 2", 2, 19, 19, false},
		{"negative radius", -1, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells, err := GetCellsInRadius(cell, tt.k)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(cells) < tt.minCells || len(cells) > tt.maxCells {
				t.Errorf("expected %d-%d cells, got %d", tt.minCells, tt.maxCells, len(cells))
			}
		})
	}
}

func TestGetRing(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	// Ring at k=0 should just return the center
	ring0, err := GetRing(cell, 0)
	if err != nil {
		t.Errorf("failed to get ring 0: %v", err)
	}
	if len(ring0) != 1 || ring0[0] != cell {
		t.Errorf("ring 0 should return center cell only")
	}

	// Ring at k=1 should return 6 cells (neighbors)
	ring1, err := GetRing(cell, 1)
	if err != nil {
		t.Errorf("failed to get ring 1: %v", err)
	}
	if len(ring1) < 5 || len(ring1) > 6 {
		t.Errorf("expected 5-6 cells in ring 1, got %d", len(ring1))
	}
}

func TestDistanceInCells(t *testing.T) {
	cell1 := LatLngToCell(testLat, testLng)

	// Distance to self should be 0
	dist, err := DistanceInCells(cell1, cell1)
	if err != nil {
		t.Errorf("failed to calculate distance: %v", err)
	}
	if dist != 0 {
		t.Errorf("expected distance 0 to self, got %d", dist)
	}

	// Get a neighbor and verify distance is 1
	neighbors, _ := GetNeighbors(cell1)
	if len(neighbors) > 0 {
		dist, err = DistanceInCells(cell1, neighbors[0])
		if err != nil {
			t.Errorf("failed to calculate distance to neighbor: %v", err)
		}
		if dist != 1 {
			t.Errorf("expected distance 1 to neighbor, got %d", dist)
		}
	}
}

func TestDistanceInMeters(t *testing.T) {
	cell1 := LatLngToCell(testLat, testLng)
	cell2 := LatLngToCell(testLat+0.01, testLng+0.01) // ~1.5km away

	dist, err := DistanceInMeters(cell1, cell2)
	if err != nil {
		t.Errorf("failed to calculate distance: %v", err)
	}

	// Expect distance to be roughly 1-2km
	if dist < 1000 || dist > 2000 {
		t.Errorf("expected distance ~1500m, got %f", dist)
	}
}

func TestHaversineDistance(t *testing.T) {
	// Test known distance: Delhi to Mumbai (~1150km)
	delhiLat, delhiLng := 28.6139, 77.2090
	mumbaiLat, mumbaiLng := 19.0760, 72.8777

	dist := HaversineDistance(delhiLat, delhiLng, mumbaiLat, mumbaiLng)

	// Should be approximately 1150km
	if dist < 1100000 || dist > 1200000 {
		t.Errorf("expected ~1150km, got %f meters", dist)
	}

	// Distance to self should be 0
	selfDist := HaversineDistance(delhiLat, delhiLng, delhiLat, delhiLng)
	if selfDist != 0 {
		t.Errorf("expected 0 distance to self, got %f", selfDist)
	}
}

func TestIsValidCell(t *testing.T) {
	validCell := LatLngToCell(testLat, testLng)

	if !IsValidCell(validCell) {
		t.Error("expected valid cell to be valid")
	}

	if IsValidCell("invalid-cell-id") {
		t.Error("expected invalid cell to be invalid")
	}

	if IsValidCell("") {
		t.Error("expected empty string to be invalid")
	}
}

func TestGetResolution(t *testing.T) {
	for res := 0; res <= 15; res++ {
		cell := LatLngToCellAtResolution(testLat, testLng, res)

		gotRes, err := GetResolution(cell)
		if err != nil {
			t.Errorf("failed to get resolution for res %d: %v", res, err)
		}
		if gotRes != res {
			t.Errorf("expected resolution %d, got %d", res, gotRes)
		}
	}
}

func TestGetParent(t *testing.T) {
	cell := LatLngToCellAtResolution(testLat, testLng, 9)

	// Get parent at resolution 7
	parent, err := GetParent(cell, 7)
	if err != nil {
		t.Errorf("failed to get parent: %v", err)
	}

	parentRes, _ := GetResolution(parent)
	if parentRes != 7 {
		t.Errorf("expected parent resolution 7, got %d", parentRes)
	}

	// Parent at same or higher resolution should fail
	_, err = GetParent(cell, 9)
	if err == nil {
		t.Error("expected error for parent at same resolution")
	}

	_, err = GetParent(cell, 10)
	if err == nil {
		t.Error("expected error for parent at higher resolution")
	}
}

func TestGetChildren(t *testing.T) {
	cell := LatLngToCellAtResolution(testLat, testLng, 7)

	// Get children at resolution 8
	children, err := GetChildren(cell, 8)
	if err != nil {
		t.Errorf("failed to get children: %v", err)
	}

	// Should have 7 children (hexagon subdivides into 7)
	if len(children) != 7 {
		t.Errorf("expected 7 children, got %d", len(children))
	}

	// Verify all children are at correct resolution
	for _, child := range children {
		res, _ := GetResolution(child)
		if res != 8 {
			t.Errorf("expected child resolution 8, got %d", res)
		}
	}

	// Children at same or lower resolution should fail
	_, err = GetChildren(cell, 7)
	if err == nil {
		t.Error("expected error for children at same resolution")
	}
}

func TestCellArea(t *testing.T) {
	// Higher resolution = smaller area
	cell9 := LatLngToCellAtResolution(testLat, testLng, 9)
	cell7 := LatLngToCellAtResolution(testLat, testLng, 7)

	area9, err := CellArea(cell9)
	if err != nil {
		t.Errorf("failed to get area: %v", err)
	}

	area7, err := CellArea(cell7)
	if err != nil {
		t.Errorf("failed to get area: %v", err)
	}

	if area9 >= area7 {
		t.Errorf("resolution 9 should be smaller than resolution 7")
	}

	// Resolution 9 should be approximately 0.1 km² = 100,000 m²
	if area9 < 80000 || area9 > 150000 {
		t.Errorf("expected area ~100,000 m², got %f", area9)
	}
}

func TestGetHexagonInfo(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	info, err := GetHexagonInfo(cell)
	if err != nil {
		t.Errorf("failed to get hexagon info: %v", err)
	}

	if info.ID != cell {
		t.Errorf("expected ID %s, got %s", cell, info.ID)
	}

	if info.Resolution != DefaultResolution {
		t.Errorf("expected resolution %d, got %d", DefaultResolution, info.Resolution)
	}

	if info.AreaM2 <= 0 {
		t.Error("expected positive area")
	}

	if info.AreaKm2 <= 0 {
		t.Error("expected positive area in km²")
	}

	if info.ParentID == "" {
		t.Error("expected non-empty parent ID for non-zero resolution")
	}
}

func TestPolygonToCells(t *testing.T) {
	// Define a small polygon around Delhi
	polygon := [][2]float64{
		{28.60, 77.20},
		{28.60, 77.22},
		{28.62, 77.22},
		{28.62, 77.20},
		{28.60, 77.20}, // close polygon
	}

	cells, err := PolygonToCells(polygon, 9)
	if err != nil {
		t.Errorf("failed to get cells in polygon: %v", err)
	}

	if len(cells) == 0 {
		t.Error("expected cells in polygon")
	}

	// Invalid polygon (< 3 points)
	_, err = PolygonToCells([][2]float64{{28.60, 77.20}, {28.62, 77.22}}, 9)
	if err != ErrInvalidPolygon {
		t.Error("expected ErrInvalidPolygon for < 3 points")
	}

	// Invalid resolution
	_, err = PolygonToCells(polygon, 20)
	if err != ErrInvalidResolution {
		t.Error("expected ErrInvalidResolution")
	}
}

func TestCellsToMultiPolygon(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)
	cells, _ := GetCellsInRadius(cell, 1)

	multiPoly, err := CellsToMultiPolygon(cells)
	if err != nil {
		t.Errorf("failed to convert cells to multi polygon: %v", err)
	}

	if len(multiPoly) == 0 {
		t.Error("expected non-empty multi polygon")
	}
}

func TestGetCellBoundary(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	boundary, err := GetCellBoundary(cell)
	if err != nil {
		t.Errorf("failed to get cell boundary: %v", err)
	}

	// Hexagons have 6 vertices
	if len(boundary) < 5 || len(boundary) > 6 {
		t.Errorf("expected 5-6 boundary vertices, got %d", len(boundary))
	}

	// All vertices should be close to the cell center
	for _, vertex := range boundary {
		if math.Abs(vertex.Lat-testLat) > 0.1 {
			t.Errorf("vertex latitude too far from center")
		}
	}
}

func TestCompactCells(t *testing.T) {
	// Get children of a cell and compact them back
	parent := LatLngToCellAtResolution(testLat, testLng, 7)
	children, _ := GetChildren(parent, 8)

	compacted, err := CompactCells(children)
	if err != nil {
		t.Errorf("failed to compact cells: %v", err)
	}

	// Should compact to single parent
	if len(compacted) != 1 {
		t.Errorf("expected 1 compacted cell, got %d", len(compacted))
	}

	if compacted[0] != parent {
		t.Errorf("expected parent %s, got %s", parent, compacted[0])
	}

	// Empty slice should return empty
	empty, _ := CompactCells([]string{})
	if len(empty) != 0 {
		t.Error("expected empty result for empty input")
	}
}

func TestUncompactCells(t *testing.T) {
	parent := LatLngToCellAtResolution(testLat, testLng, 7)

	uncompacted, err := UncompactCells([]string{parent}, 8)
	if err != nil {
		t.Errorf("failed to uncompact cells: %v", err)
	}

	// Should expand to 7 children
	if len(uncompacted) != 7 {
		t.Errorf("expected 7 uncompacted cells, got %d", len(uncompacted))
	}

	// Invalid resolution
	_, err = UncompactCells([]string{parent}, 20)
	if err != ErrInvalidResolution {
		t.Error("expected ErrInvalidResolution")
	}
}

func TestBatchLatLngToCell(t *testing.T) {
	coords := []LatLng{
		{Lat: 28.6139, Lng: 77.2090}, // Delhi
		{Lat: 19.0760, Lng: 72.8777}, // Mumbai
		{Lat: 13.0827, Lng: 80.2707}, // Chennai
	}

	cells := BatchLatLngToCell(coords, DefaultResolution)

	if len(cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(cells))
	}

	for i, cell := range cells {
		if cell == "" {
			t.Errorf("expected non-empty cell at index %d", i)
		}
		if !IsValidCell(cell) {
			t.Errorf("expected valid cell at index %d", i)
		}
	}

	// All cells should be different
	if cells[0] == cells[1] || cells[1] == cells[2] {
		t.Error("expected different cells for different locations")
	}
}

func TestBatchCellToLatLng(t *testing.T) {
	coords := []LatLng{
		{Lat: 28.6139, Lng: 77.2090},
		{Lat: 19.0760, Lng: 72.8777},
	}
	cells := BatchLatLngToCell(coords, DefaultResolution)

	// Add an invalid cell
	cells = append(cells, "invalid-cell")

	results, errs := BatchCellToLatLng(cells)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// First two should succeed
	if errs[0] != nil {
		t.Errorf("expected no error for first cell: %v", errs[0])
	}
	if errs[1] != nil {
		t.Errorf("expected no error for second cell: %v", errs[1])
	}

	// Last should fail
	if errs[2] == nil {
		t.Error("expected error for invalid cell")
	}
}

func TestParallelLatLngToCell(t *testing.T) {
	// Generate many coordinates
	coords := make([]LatLng, 1000)
	for i := 0; i < 1000; i++ {
		coords[i] = LatLng{
			Lat: 28.0 + float64(i)*0.001,
			Lng: 77.0 + float64(i)*0.001,
		}
	}

	// Test with different worker counts
	results4 := ParallelLatLngToCell(coords, DefaultResolution, 4)
	results8 := ParallelLatLngToCell(coords, DefaultResolution, 8)
	results0 := ParallelLatLngToCell(coords, DefaultResolution, 0) // Should default to 4

	if len(results4) != 1000 || len(results8) != 1000 || len(results0) != 1000 {
		t.Error("expected 1000 results from parallel conversion")
	}

	// All results should be the same
	for i := 0; i < 1000; i++ {
		if results4[i] != results8[i] || results4[i] != results0[i] {
			t.Errorf("parallel results differ at index %d", i)
		}
	}
}

func TestGetUniqueCells(t *testing.T) {
	cells := []string{"a", "b", "a", "c", "b", "a"}

	unique := GetUniqueCells(cells)

	if len(unique) != 3 {
		t.Errorf("expected 3 unique cells, got %d", len(unique))
	}

	// Check all unique values are present
	seen := make(map[string]bool)
	for _, c := range unique {
		seen[c] = true
	}

	if !seen["a"] || !seen["b"] || !seen["c"] {
		t.Error("missing expected unique values")
	}
}

func TestSortCells(t *testing.T) {
	cells := []string{"c", "a", "b"}

	sorted := SortCells(cells)

	if sorted[0] != "a" || sorted[1] != "b" || sorted[2] != "c" {
		t.Error("cells not sorted correctly")
	}

	// Original should be unchanged
	if cells[0] != "c" {
		t.Error("original slice was modified")
	}
}

func TestCellContains(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	// Cell should contain its center
	contains, err := CellContains(cell, testLat, testLng)
	if err != nil {
		t.Errorf("failed to check contains: %v", err)
	}
	if !contains {
		t.Error("cell should contain its center point")
	}

	// Cell should not contain a far away point
	contains, err = CellContains(cell, 0, 0)
	if err != nil {
		t.Errorf("failed to check contains: %v", err)
	}
	if contains {
		t.Error("cell should not contain distant point")
	}
}

func TestFindCellsInBoundingBox(t *testing.T) {
	// Small bounding box around Delhi
	cells, err := FindCellsInBoundingBox(28.60, 77.20, 28.62, 77.22, 9)
	if err != nil {
		t.Errorf("failed to find cells in bounding box: %v", err)
	}

	if len(cells) == 0 {
		t.Error("expected cells in bounding box")
	}

	// Invalid resolution
	_, err = FindCellsInBoundingBox(28.60, 77.20, 28.62, 77.22, 20)
	if err != ErrInvalidResolution {
		t.Error("expected ErrInvalidResolution")
	}
}

func TestGetCellsAlongLine(t *testing.T) {
	// Line from one point to nearby point
	cells, err := GetCellsAlongLine(28.60, 77.20, 28.61, 77.21, 9)
	if err != nil {
		t.Errorf("failed to get cells along line: %v", err)
	}

	if len(cells) < 2 {
		t.Error("expected at least 2 cells along line")
	}

	// Invalid resolution
	_, err = GetCellsAlongLine(28.60, 77.20, 28.61, 77.21, 20)
	if err != ErrInvalidResolution {
		t.Error("expected ErrInvalidResolution")
	}
}

func TestIsPentagon(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	isPent, err := IsPentagon(cell)
	if err != nil {
		t.Errorf("failed to check pentagon: %v", err)
	}

	// Most cells are not pentagons
	if isPent {
		t.Log("cell is a pentagon (rare)")
	}

	_, err = IsPentagon("invalid-cell")
	if err == nil {
		t.Error("expected error for invalid cell")
	}
}

func TestGetBaseCellNumber(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)

	baseNum, err := GetBaseCellNumber(cell)
	if err != nil {
		t.Errorf("failed to get base cell number: %v", err)
	}

	// Base cell numbers are 0-121
	if baseNum < 0 || baseNum > 121 {
		t.Errorf("expected base cell number 0-121, got %d", baseNum)
	}
}

func TestEstimateAreaKm2(t *testing.T) {
	cell := LatLngToCell(testLat, testLng)
	cells, _ := GetCellsInRadius(cell, 1)

	area, err := EstimateAreaKm2(cells)
	if err != nil {
		t.Errorf("failed to estimate area: %v", err)
	}

	// 7 cells at resolution 9 (~0.1 km² each) = ~0.7 km²
	if area < 0.5 || area > 1.0 {
		t.Errorf("expected area ~0.7 km², got %f", area)
	}
}

func TestGetResolutionForArea(t *testing.T) {
	// Want approximately 1 km²
	res := GetResolutionForArea(1000000) // 1 km² = 1,000,000 m²

	// Should be around resolution 8 or 9
	if res < 7 || res > 10 {
		t.Errorf("expected resolution 7-10 for 1 km², got %d", res)
	}

	// Very large area should give low resolution
	largeRes := GetResolutionForArea(1000000000000)
	if largeRes > 2 {
		t.Errorf("expected low resolution for very large area")
	}
}

func TestGroupCellsByParent(t *testing.T) {
	// Get children of two different parents
	parent1 := LatLngToCellAtResolution(testLat, testLng, 7)
	parent2 := LatLngToCellAtResolution(testLat+0.5, testLng+0.5, 7) // Different parent

	children1, _ := GetChildren(parent1, 8)
	children2, _ := GetChildren(parent2, 8)

	allChildren := append(children1, children2...)

	groups, err := GroupCellsByParent(allChildren, 7)
	if err != nil {
		t.Errorf("failed to group cells: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	if len(groups[parent1]) != 7 {
		t.Errorf("expected 7 children in parent1 group, got %d", len(groups[parent1]))
	}
}

func TestFilterCellsByResolution(t *testing.T) {
	cells := []string{
		LatLngToCellAtResolution(testLat, testLng, 7),
		LatLngToCellAtResolution(testLat, testLng, 8),
		LatLngToCellAtResolution(testLat, testLng, 9),
		LatLngToCellAtResolution(testLat, testLng, 9),
	}

	filtered := FilterCellsByResolution(cells, 9)

	if len(filtered) != 2 {
		t.Errorf("expected 2 cells at resolution 9, got %d", len(filtered))
	}

	for _, c := range filtered {
		res, _ := GetResolution(c)
		if res != 9 {
			t.Errorf("expected resolution 9, got %d", res)
		}
	}
}

func TestCellSetIntersection(t *testing.T) {
	set1 := []string{"a", "b", "c", "d"}
	set2 := []string{"c", "d", "e", "f"}

	intersection := CellSetIntersection(set1, set2)

	if len(intersection) != 2 {
		t.Errorf("expected 2 cells in intersection, got %d", len(intersection))
	}

	seen := make(map[string]bool)
	for _, c := range intersection {
		seen[c] = true
	}

	if !seen["c"] || !seen["d"] {
		t.Error("expected c and d in intersection")
	}
}

func TestCellSetUnion(t *testing.T) {
	set1 := []string{"a", "b", "c"}
	set2 := []string{"c", "d", "e"}

	union := CellSetUnion(set1, set2)

	if len(union) != 5 {
		t.Errorf("expected 5 cells in union, got %d", len(union))
	}

	seen := make(map[string]bool)
	for _, c := range union {
		seen[c] = true
	}

	for _, expected := range []string{"a", "b", "c", "d", "e"} {
		if !seen[expected] {
			t.Errorf("expected %s in union", expected)
		}
	}
}

func TestCellSetDifference(t *testing.T) {
	set1 := []string{"a", "b", "c", "d"}
	set2 := []string{"c", "d", "e"}

	diff := CellSetDifference(set1, set2)

	if len(diff) != 2 {
		t.Errorf("expected 2 cells in difference, got %d", len(diff))
	}

	seen := make(map[string]bool)
	for _, c := range diff {
		seen[c] = true
	}

	if !seen["a"] || !seen["b"] {
		t.Error("expected a and b in difference")
	}
	if seen["c"] || seen["d"] {
		t.Error("c and d should not be in difference")
	}
}

// Benchmarks

func BenchmarkLatLngToCell(b *testing.B) {
	for i := 0; i < b.N; i++ {
		LatLngToCell(testLat, testLng)
	}
}

func BenchmarkGetNeighbors(b *testing.B) {
	cell := LatLngToCell(testLat, testLng)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		GetNeighbors(cell)
	}
}

func BenchmarkHaversineDistance(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HaversineDistance(28.6139, 77.2090, 19.0760, 72.8777)
	}
}

func BenchmarkBatchLatLngToCell(b *testing.B) {
	coords := make([]LatLng, 100)
	for i := 0; i < 100; i++ {
		coords[i] = LatLng{Lat: 28.0 + float64(i)*0.01, Lng: 77.0 + float64(i)*0.01}
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		BatchLatLngToCell(coords, DefaultResolution)
	}
}

func BenchmarkParallelLatLngToCell(b *testing.B) {
	coords := make([]LatLng, 1000)
	for i := 0; i < 1000; i++ {
		coords[i] = LatLng{Lat: 28.0 + float64(i)*0.001, Lng: 77.0 + float64(i)*0.001}
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ParallelLatLngToCell(coords, DefaultResolution, 4)
	}
}

func BenchmarkPolygonToCells(b *testing.B) {
	polygon := [][2]float64{
		{28.60, 77.20},
		{28.60, 77.25},
		{28.65, 77.25},
		{28.65, 77.20},
		{28.60, 77.20},
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		PolygonToCells(polygon, 9)
	}
}

// Concurrent access test
func TestConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			lat := 28.0 + float64(i)*0.01
			lng := 77.0 + float64(i)*0.01

			cell := LatLngToCell(lat, lng)
			GetNeighbors(cell)
			GetCellsInRadius(cell, 2)
			GetHexagonInfo(cell)
		}(i)
	}

	wg.Wait()
}

// Resolution constants test
func TestResolutionConstants(t *testing.T) {
	if DefaultResolution != 9 {
		t.Errorf("expected DefaultResolution 9, got %d", DefaultResolution)
	}

	if MinResolution != 0 {
		t.Errorf("expected MinResolution 0, got %d", MinResolution)
	}

	if MaxResolution != 15 {
		t.Errorf("expected MaxResolution 15, got %d", MaxResolution)
	}

	// Verify semantic resolutions
	if StateResolution > DistrictResolution {
		t.Error("state resolution should be lower than district")
	}

	if DistrictResolution > ACResolution {
		t.Error("district resolution should be lower than AC")
	}

	if ACResolution > NeighborhoodResolution {
		t.Error("AC resolution should be lower than neighborhood")
	}
}

// ResolutionAreas map test
func TestResolutionAreas(t *testing.T) {
	// Areas should decrease as resolution increases
	var prevArea float64 = math.MaxFloat64

	for res := 0; res <= 15; res++ {
		area, ok := ResolutionAreas[res]
		if !ok {
			t.Errorf("missing area for resolution %d", res)
			continue
		}

		if area >= prevArea {
			t.Errorf("area at res %d should be smaller than res %d", res, res-1)
		}
		prevArea = area
	}
}
