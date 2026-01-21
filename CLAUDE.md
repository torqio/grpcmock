# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gRPCMock is a protobuf plugin that generates mock gRPC server implementations from `.proto` files. It generates type-safe mock servers with fluent configuration API, supporting all RPC types (unary, server/client/bidirectional streaming).

## Build & Test Commands

```bash
# Full test pipeline: builds plugin, generates protos, runs tests
make test

# Install the protoc plugin
go install github.com/torqio/grpcmock/protoc-gen-grpcmock

# Generate mocks from protos (in tests/ dir)
cd tests && buf generate
```

**Required tools:** buf, protoc, gotestsum, Go 1.22+

## Architecture

### Core Components

**pkg/mocker/** - Mock orchestration engine
- `mocker.go` - Thread-safe call registration, matching, and tracking
- `call.go` - `SingleExpectedCall` struct with DoAndReturn caching
- `matchers.go` - Request matchers (`Any()`, `Eq()` with proto.Equal support)

**protoc-gen-grpcmock/** - Code generation plugin
- `main.go` - Plugin entry, template orchestration
- `server_tmpl/mock_server.tmpl` - Main template generating `*_grpcmock.pb.go` files

### Code Generation Flow

```
.proto → buf/protoc → protoc-gen-grpcmock → Go template → *_grpcmock.pb.go
```

Generated code per service:
- `NewServiceMockServer()` constructor
- Service implementation delegating to `Mocker.CallV2()`
- Fluent configurers for `.On()`, `.Return()`, `.DoAndReturn()`

### Mock Call Matching

Mocker tries expected calls in registration order, checking `Matcher.Matches()` for each arg. First match wins → else DefaultCall → else `ErrNoMatchingCalls`.

### API Versions

- V1/V2: Basic `On().Return()`
- V3: Adds `DoAndReturn`/`DefaultDoAndReturn` for dynamic responses

## Key Patterns

- **Thread safety:** All Mocker ops protected by RWMutex
- **Call caching:** DoAndReturn results cached per-call for idempotency
- **Streaming context:** Stream methods receive message + ServerStream for metadata access
- **Parallel tests:** Integration tests use atomic counters, 50 concurrent subtests per scenario
