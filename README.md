# Politic Core

The open-source foundation powering [Politic](https://politic.in) — India's hyperlocal anonymous civic engagement platform.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## What is Politic?

Politic is a hyperlocal anonymous civic engagement platform that connects citizens with their local representatives and government. The platform enables:

- **Hyperlocal Issue Reporting** — Report potholes, broken streetlights, water issues at precise locations using H3 hexagons
- **Fixer Network** — Track which representatives and organizations actually fix reported issues
- **Civic Scores** — Build reputation through verified participation and community contributions
- **Opinion Polls** — Participate in paid polls from researchers, journalists, and political analysts
- **Election Insights** — Real-time sentiment from verified, geolocated citizens (with full privacy)

## Why Open Source This?

We open-source the core to:

1. **Prove Privacy Claims** — See exactly how we anonymize responses. No trust required.
2. **Enable Verification** — Community can audit k-anonymity and differential privacy implementations
3. **Improve Accuracy** — Crowdsource booth name corrections across Indian regional languages
4. **Build Trust** — Transparent algorithms for civic scores and election blackout compliance

## Core Packages

```
core/
├── anonymization/       # Zero-knowledge poll response architecture
├── booth-matching/      # Fuzzy matching for polling booth verification
├── civic-score/         # Reputation scoring with decay and badges
├── election-blackout/   # Section 126 RP Act compliance
├── h3-utils/            # H3 hexagon utilities for geolocation
└── data/                # Indian geographic data loaders and types
```

| Package | Purpose | Key Features |
|---------|---------|--------------|
| `anonymization/` | Poll response privacy | k-anonymity, differential privacy, payout tokens, database separation |
| `booth-matching/` | KYC verification | Fuzzy matching, Soundex/Metaphone, regional language support |
| `civic-score/` | User reputation | Points, decay, levels, badges, streak tracking |
| `election-blackout/` | Legal compliance | Section 126 RP Act, multi-phase elections, blackout periods |
| `h3-utils/` | Geolocation | H3 cells, distance calculation, polygon fill |
| `data/` | Geographic data | States, districts, ACs, booths, GeoJSON boundaries |

## Installation

```bash
go get github.com/politic-in/core
```

## Usage Examples

### Anonymization — Poll Response Privacy

```go
import "github.com/politic-in/core/anonymization"

// Generate payout token (links payment without revealing identity)
// Response DB sees: token_hash, amount
// Identity DB sees: token_hash → user_id (encrypted, separate)
token, mapping, _ := anonymization.GeneratePayoutToken("user123", "poll456", 5000) // ₹50

// Aggregate responses with k-anonymity (minimum 10 responses required)
aggregator := anonymization.NewAggregator()
result, _ := aggregator.AggregateResponses(responses, "poll456", nil, nil)
// result.MeetsKAnon = true (only if 10+ responses)

// Apply differential privacy noise to small samples
noisyCount := anonymization.ApplyDifferentialPrivacy(count, 1.0) // epsilon = 1.0
```

### Civic Score — User Reputation

```go
import civicscore "github.com/politic-in/core/civic-score"

// Create calculator
calc := civicscore.NewCalculator()

// Apply action and get new score
newScore, delta := calc.ApplyAction(currentScore, civicscore.PollCompleted, 1)

// Get user level based on score
level := civicscore.GetLevel(newScore) // "active", "top_responder", "power_user"

// Apply decay for inactive users
decayedScore, weeksDecayed := civicscore.ApplyDecay(score, lastActiveAt, now)

// Calculate badges based on activity
badges := civicscore.CalculateBadges(userScore)
```

### Booth Matching — KYC Verification

```go
import boothmatching "github.com/politic-in/core/booth-matching"

// Create matcher with booths for an Assembly Constituency
matcher := boothmatching.NewMatcher(booths)

// User types their polling booth name (with typos, regional spelling)
result, _ := matcher.Match("Sarkar Prathamik Vidyalaya Jayanagar", 176)
// Matches: "Government Primary School Jayanagar"
// result.Confidence = 0.92, result.MatchType = "phonetic"

// Evaluate for Polling Station Challenge
challenge, _ := matcher.EvaluateChallenge("govt school jayanagar", 176)
// challenge.Passed = true (confidence > 0.7)
```

### Election Blackout — Section 126 Compliance

```go
import blackout "github.com/politic-in/core/election-blackout"

// Create checker with active elections
checker := blackout.NewChecker(elections)

// Check if polls are blocked in an AC
if checker.IsBlackoutActive(acID, time.Now()) {
    // Block poll creation/responses for this AC
}

// Get active elections
activeElections := checker.GetActiveElections(time.Now())
```

### H3 Hexagons — Hyperlocal Geolocation

```go
import h3utils "github.com/politic-in/core/h3-utils"

// Convert issue location to H3 cell (resolution 9 = ~0.1 km²)
cellID := h3utils.LatLngToCell(12.9716, 77.5946, 9)

// Calculate distance between two points
distance := h3utils.HaversineDistance(lat1, lng1, lat2, lng2)

// Get cell center coordinates
lat, lng := h3utils.CellToLatLng(cellID)
```

### Data — Indian Electoral Geography

```go
import "github.com/politic-in/core/data"

// Load geographic index from data directory
index := data.NewGeoIndex("./data")
index.LoadAll()

// Lookup state, district, AC
state, _ := index.GetStateBySlug("karnataka")
districts := index.GetDistrictsForState("karnataka")
acs := index.GetACsForState("karnataka")

// Get booths for an AC
booths, _ := index.GetBoothsForAC("karnataka", 176)

// Find AC from coordinates (point-in-polygon)
boundary, _ := index.FindACAtPoint("karnataka", 12.9716, 77.5946)
```

## Data Coverage

| Data | Count |
|------|-------|
| States & UTs | 36 |
| Districts | 738 |
| Assembly Constituencies | 4,140 |
| Polling Booths | 1,057,407 |

## Running Tests

```bash
go test ./...
```

## Contributing

We especially welcome:

- **Booth Name Corrections** — Help us handle regional language variations
- **Geographic Data Fixes** — Correct coordinates, boundaries, spelling
- **Privacy Improvements** — Strengthen anonymization algorithms
- **Test Coverage** — Edge cases for Indian electoral scenarios

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache 2.0 — Use freely, contribute back.

## Author

**Sandeep Kumar** ([@tsksandeep](https://github.com/tsksandeep))
