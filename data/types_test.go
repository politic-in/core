package data

import (
	"testing"
)

func TestToSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Andhra Pradesh", "andhra_pradesh"},
		{"Tamil Nadu", "tamil_nadu"},
		{"Jammu & Kashmir", "jammu_kashmir"},
		{"Dadra and Nagar Haveli", "dadra_and_nagar_haveli"},
		{"Andaman & Nicobar Islands", "andaman_nicobar_islands"},
		{"ST. MARY'S SCHOOL", "st_marys_school"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToSlug(tt.input)
			if result != tt.expected {
				t.Errorf("ToSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFromSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"andhra_pradesh", "andhra_pradesh"},
		{"KARNATAKA", "karnataka"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := FromSlug(tt.input)
			if result != tt.expected {
				t.Errorf("FromSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStateSlug(t *testing.T) {
	state := State{Name: "Andhra Pradesh"}
	if slug := state.Slug(); slug != "andhra_pradesh" {
		t.Errorf("State.Slug() = %q, want %q", slug, "andhra_pradesh")
	}
}

func TestDistrictSlug(t *testing.T) {
	district := District{Name: "Bangalore Urban"}
	if slug := district.Slug(); slug != "bangalore_urban" {
		t.Errorf("District.Slug() = %q, want %q", slug, "bangalore_urban")
	}
}

func TestAssemblyConstituencyReservation(t *testing.T) {
	tests := []struct {
		reserved     string
		isReserved   bool
		isReservedSC bool
		isReservedST bool
	}{
		{"None", false, false, false},
		{"", false, false, false},
		{"SC", true, true, false},
		{"ST", true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.reserved, func(t *testing.T) {
			ac := AssemblyConstituency{Reserved: tt.reserved}
			if ac.IsReserved() != tt.isReserved {
				t.Errorf("IsReserved() = %v, want %v", ac.IsReserved(), tt.isReserved)
			}
			if ac.IsReservedSC() != tt.isReservedSC {
				t.Errorf("IsReservedSC() = %v, want %v", ac.IsReservedSC(), tt.isReservedSC)
			}
			if ac.IsReservedST() != tt.isReservedST {
				t.Errorf("IsReservedST() = %v, want %v", ac.IsReservedST(), tt.isReservedST)
			}
		})
	}
}

func TestPollingBoothFullName(t *testing.T) {
	booth := PollingBooth{
		PartNumber: 1,
		PartName:   "Government Primary School, Jayanagar",
	}

	expected := "1 - Government Primary School, Jayanagar"
	if fullName := booth.FullName(); fullName != expected {
		t.Errorf("FullName() = %q, want %q", fullName, expected)
	}
}

func TestACBoundaryBoundingBox(t *testing.T) {
	boundary := ACBoundary{
		Polygon: [][][]float64{
			{
				{77.0, 12.0}, // [lng, lat]
				{78.0, 12.0},
				{78.0, 13.0},
				{77.0, 13.0},
				{77.0, 12.0}, // closed
			},
		},
	}

	bbox := boundary.BoundingBox()
	expected := [4]float64{77.0, 12.0, 78.0, 13.0}

	if bbox != expected {
		t.Errorf("BoundingBox() = %v, want %v", bbox, expected)
	}
}

func TestACBoundaryBoundingBoxEmpty(t *testing.T) {
	boundary := ACBoundary{}
	bbox := boundary.BoundingBox()
	expected := [4]float64{0, 0, 0, 0}

	if bbox != expected {
		t.Errorf("BoundingBox() for empty = %v, want %v", bbox, expected)
	}
}

func TestACBoundaryExteriorRing(t *testing.T) {
	ring := [][]float64{
		{77.0, 12.0},
		{78.0, 12.0},
		{78.0, 13.0},
		{77.0, 13.0},
	}
	boundary := ACBoundary{
		Polygon: [][][]float64{ring},
	}

	exterior := boundary.GetExteriorRing()
	if len(exterior) != 4 {
		t.Errorf("GetExteriorRing() len = %d, want %d", len(exterior), 4)
	}
}

func TestACBoundaryHoles(t *testing.T) {
	exterior := [][]float64{{77.0, 12.0}, {78.0, 12.0}, {78.0, 13.0}, {77.0, 13.0}}
	hole := [][]float64{{77.2, 12.2}, {77.4, 12.2}, {77.4, 12.4}, {77.2, 12.4}}

	boundary := ACBoundary{
		Polygon: [][][]float64{exterior, hole},
	}

	holes := boundary.GetHoles()
	if len(holes) != 1 {
		t.Errorf("GetHoles() len = %d, want %d", len(holes), 1)
	}
}

func TestACBoundaryContainsPoint(t *testing.T) {
	// Simple square: 77-78 longitude, 12-13 latitude
	boundary := ACBoundary{
		Polygon: [][][]float64{
			{
				{77.0, 12.0},
				{78.0, 12.0},
				{78.0, 13.0},
				{77.0, 13.0},
				{77.0, 12.0},
			},
		},
	}

	tests := []struct {
		name     string
		lat, lng float64
		inside   bool
	}{
		{"center", 12.5, 77.5, true},
		{"corner", 12.0, 77.0, true},
		{"outside_left", 12.5, 76.5, false},
		{"outside_right", 12.5, 78.5, false},
		{"outside_top", 13.5, 77.5, false},
		{"outside_bottom", 11.5, 77.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boundary.ContainsPoint(tt.lat, tt.lng)
			if result != tt.inside {
				t.Errorf("ContainsPoint(%.2f, %.2f) = %v, want %v", tt.lat, tt.lng, result, tt.inside)
			}
		})
	}
}

func TestACBoundaryContainsPointWithHole(t *testing.T) {
	// Square with a hole in the middle
	boundary := ACBoundary{
		Polygon: [][][]float64{
			// Exterior ring
			{
				{77.0, 12.0},
				{78.0, 12.0},
				{78.0, 13.0},
				{77.0, 13.0},
				{77.0, 12.0},
			},
			// Hole (inner ring)
			{
				{77.3, 12.3},
				{77.7, 12.3},
				{77.7, 12.7},
				{77.3, 12.7},
				{77.3, 12.3},
			},
		},
	}

	tests := []struct {
		name     string
		lat, lng float64
		inside   bool
	}{
		{"outside_hole", 12.9, 77.5, true},
		{"in_hole", 12.5, 77.5, false}, // Inside the hole = outside the polygon
		{"outside_polygon", 11.5, 77.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boundary.ContainsPoint(tt.lat, tt.lng)
			if result != tt.inside {
				t.Errorf("ContainsPoint(%.2f, %.2f) = %v, want %v", tt.lat, tt.lng, result, tt.inside)
			}
		})
	}
}

func TestPointInRing(t *testing.T) {
	// Triangle
	ring := [][]float64{
		{0.0, 0.0},
		{2.0, 0.0},
		{1.0, 2.0},
		{0.0, 0.0},
	}

	tests := []struct {
		lat, lng float64
		inside   bool
	}{
		{0.5, 1.0, true},  // inside
		{0.0, 0.0, true},  // vertex
		{5.0, 5.0, false}, // outside
	}

	for _, tt := range tests {
		result := pointInRing(tt.lat, tt.lng, ring)
		if result != tt.inside {
			t.Errorf("pointInRing(%.2f, %.2f) = %v, want %v", tt.lat, tt.lng, result, tt.inside)
		}
	}
}
