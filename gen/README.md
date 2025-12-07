# Generated Code

This directory contains code generated from `proto/politic.proto`.

**Do not edit these files directly.** They are auto-generated.

## Contents

| Directory     | Purpose                               | Consumers                             |
| ------------- | ------------------------------------- | ------------------------------------- |
| `go/`         | Go types and gRPC client/server stubs | Backend services implementing the API |
| `ts/`         | TypeScript types and client           | Web/React Native frontends            |
| `jsonschema/` | JSON Schema definitions               | API documentation, validation         |

## Regenerating

If you modify `proto/politic.proto`, regenerate with:

```bash
./scripts/generate-sdks.sh
```

### Prerequisites

```bash
# Protocol Buffer Compiler
brew install protobuf  # macOS

# Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# TypeScript plugin
npm install -g @protobuf-ts/plugin

# JSON Schema plugin
go install github.com/chrusty/protoc-gen-jsonschema@latest
```

## Usage

### Go Server Implementation

```go
package server

import (
    pb "github.com/politic-in/core/gen/go"
    "context"
)

type PoliticServer struct {
    pb.UnimplementedPoliticServiceServer
}

func (s *PoliticServer) CreateIssue(ctx context.Context, req *pb.CreateIssueRequest) (*pb.CreateIssueResponse, error) {
    // Your implementation here
    return &pb.CreateIssueResponse{}, nil
}
```

### TypeScript Client

```typescript
import { PoliticServiceClient } from "@politic-in/core/gen/ts/politic.client";
import { GrpcWebFetchTransport } from "@protobuf-ts/grpcweb-transport";

const transport = new GrpcWebFetchTransport({
  baseUrl: "https://api.politic.in",
});

const client = new PoliticServiceClient(transport);

const { response } = await client.getIssue({ id: "123" });
```

## API Overview

The proto defines these main services:

- **GeographyService** - States, districts, ACs, hexagons
- **IssueService** - Hyperlocal issue reporting and tracking
- **PollService** - Opinion polls with privacy guarantees
- **UserService** - Participants, customers, fixers
- **ElectionService** - Blackout compliance checking

See `proto/politic.proto` for the complete API definition.
