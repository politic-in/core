# Politic Core

Open-source trust and transparency layer for Politic.

[![CI](https://github.com/tsksandeep/politic-core/actions/workflows/ci.yml/badge.svg)](https://github.com/tsksandeep/politic-core/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tsksandeep/politic-core)](https://goreportcard.com/report/github.com/tsksandeep/politic-core)

## What's Here

This repository contains the foundational components that power Politic's trust architecture. We open-source these to:

1. **Prove our privacy claims** — See exactly how anonymization works
2. **Enable community contributions** — Improve booth matching accuracy
3. **Build developer trust** — Transparent schemas and APIs

## Components

| Directory            | Description                                                                    | Status    |
| -------------------- | ------------------------------------------------------------------------------ | --------- |
| `anonymization/`     | Zero-knowledge response architecture with k-anonymity and differential privacy | Complete  |
| `civic-score/`       | Civic Score calculation with decay, streaks, badges, and audit trail           | Complete  |
| `election-blackout/` | Section 126 RP Act compliance with multi-phase election support                | Complete  |
| `booth-matching/`    | Fuzzy matching with phonetic encoding and regional language support            | Complete  |
| `h3-utils/`          | H3 hexagon utilities, batch operations, and geospatial helpers                 | Complete  |
| `data/`              | Geographic data loaders, GeoIndex, and electoral boundary operations           | Complete  |
| `types/`             | Shared types, errors, and validation utilities                                 | Complete  |
| `proto/`             | Protocol Buffer API definitions                                                | Complete  |
| `gen/`               | Generated code (Go, TypeScript, JSON Schema)                                   | Generated |
| `schemas/`           | PostgreSQL migrations and type definitions                                     | Planned   |

## Installation

```bash
go get github.com/politic-in/core
```

## Usage

### Anonymization (Zero-Knowledge Architecture)

```go
import "github.com/politic-in/core/anonymization"

// Generate payout token (links payment without revealing identity)
token, _ := anonymization.GeneratePayoutToken("user123", "poll456", 50.0)

// Aggregate responses with k-anonymity (minimum 10 responses)
aggregator := anonymization.NewAggregator(anonymization.DefaultAggregationConfig())
result, _ := aggregator.AggregateResponses(responses, "poll456")

// Apply differential privacy
noisyCount := anonymization.ApplyDifferentialPrivacy(count, epsilon)
```

### Civic Score

```go
import civicscore "github.com/politic-in/core/civic-score"

// Calculate score from actions
actions := []civicscore.Action{
    {Type: civicscore.ActionKYCCompleted, Count: 1},
    {Type: civicscore.ActionPollCompleted, Count: 5},
}
score := civicscore.Calculate(20, actions) // starting score, actions

// Apply decay for inactive users
decayedScore := civicscore.ApplyDecay(score, 14) // 14 days inactive

// Get badges
badges := civicscore.CalculateBadges(stats)
```

### Booth Matching (Polling Station Challenge)

```go
import boothmatching "github.com/politic-in/core/booth-matching"

// Create matcher with booths
matcher := boothmatching.NewMatcher(booths)

// Fuzzy match user input
result, _ := matcher.Match("govt primary school jayanagar", 176) // AC ID
// Returns: {Booth: {...}, Confidence: 0.87, MatchType: "fuzzy"}

// Phonetic matching handles spelling variations
// "Sarkar Vidyalaya" matches "Government School"
```

### Election Blackout (Section 126 Compliance)

```go
import blackout "github.com/politic-in/core/election-blackout"

// Create checker with elections
checker := blackout.NewChecker(elections)

// Check if action is blocked
blocked, reason := checker.IsActionBlocked("poll", 176, time.Now())
if blocked {
    log.Printf("Action blocked: %s", reason)
}

// Multi-phase election support (e.g., Lok Sabha with 7 phases)
election := blackout.CreateMultiPhaseElection("LS2024", "Lok Sabha 2024", phases)
```

### H3 Hexagon Utilities

```go
import h3utils "github.com/politic-in/core/h3-utils"

// Convert lat/lng to H3 cell (resolution 9 = ~0.1 km²)
cellID := h3utils.LatLngToCell(12.9716, 77.5946)

// Get neighbors
neighbors, _ := h3utils.GetNeighbors(cellID)

// Batch operations
cells := h3utils.BatchLatLngToCell(coords, 9)

// Distance calculations
meters, _ := h3utils.DistanceInMeters(cell1, cell2)
```

### Data Package (Geographic Index)

```go
import "github.com/politic-in/core/data"

// Create and load the geographic index
index := data.NewGeoIndex("./data")
index.LoadAll()

// Lookup state by ID or slug
state, ok := index.GetState("KA")
state, ok := index.GetStateBySlug("karnataka")

// Get districts and ACs for a state
districts := index.GetDistrictsForState("karnataka")
acs := index.GetACsForState("karnataka")

// Lazily load booths for a state (loads on first access)
booths, _ := index.GetBoothsForAC("karnataka", 179)

// Find AC containing a coordinate
boundary, _ := index.FindACAtPoint("karnataka", 12.9716, 77.5946)

// Integrate with booth-matching for fuzzy search
matcher, _ := index.BoothMatcherForAC("karnataka", 179)
result, _ := matcher.Match("govt primary school", 179)

// Integrate with H3 for cell-to-AC mapping
info, _ := index.GetH3CellInfo(cellID)
fmt.Printf("Cell is in AC: %s (%d)\n", info.ACName, info.ACCode)

// Get H3 cells that cover an AC boundary
cells, _ := index.GetH3CellsForAC("karnataka", 179, 9)
```

## API Definitions

The `proto/politic.proto` file defines the complete gRPC API:

- **GeographyService** — States, districts, ACs, hexagons
- **IssueService** — Hyperlocal issue reporting
- **PollService** — Opinion polls with privacy
- **UserService** — Participants, customers, fixers
- **ElectionService** — Blackout compliance

See `gen/README.md` for using the generated client/server code.

## Running Tests

```bash
go test ./...
```

All packages have comprehensive test coverage.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Client Applications                     │
│              (Web, Mobile, Third-party)                     │
└─────────────────────────┬───────────────────────────────────┘
                          │ gRPC / REST
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   Platform (Proprietary)                    │
│         Handlers, Business Logic, Infrastructure            │
└─────────────────────────┬───────────────────────────────────┘
                          │ imports
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                    Core (Open Source)                       │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐    │
│  │anonymization│ │ civic-score │ │ election-blackout   │    │
│  └─────────────┘ └─────────────┘ └─────────────────────┘    │
│  ┌──────────────┐ ┌─────────────┐ ┌────────────────────┐    │
│  │booth-matching│ │  h3-utils   │ │       data         │    │
│  └──────────────┘ └─────────────┘ └────────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                      types                          │    │
│  └─────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              proto/ + gen/ (API Contracts)          │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## License

Apache 2.0 — Use freely, contribute back.

## Contributing

PRs welcome for:

- Booth name corrections (regional languages)
- Geographic data corrections (coordinates, boundaries)
- Civic score formula improvements
- H3 utility enhancements
- Privacy algorithm improvements
- Data loader improvements

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Author

**Sandeep Kumar** ([@tsksandeep](https://github.com/tsksandeep))
- Email: tsksandeep11@gmail.com
- GitHub: https://github.com/tsksandeep
