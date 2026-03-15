# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**network-doctor** is a network reachability diagnosis CLI tool written in Go. It detects connectivity from the local machine to a target address, identifies the specific cause of unreachability, and outputs Chinese diagnostic results.

## Build & Test Commands

```bash
go build -o network-doctor .     # Build binary
go test ./...                     # Run all tests
go test ./pkg/probe/ -run TestDNS -v  # Run specific test
go vet ./...                      # Static analysis
```

## Usage

```bash
./network-doctor https://example.com           # Single target
./network-doctor -f targets.txt                 # Batch from file
./network-doctor https://example.com --json     # JSON output
./network-doctor https://example.com --verbose  # Detailed info
./network-doctor example.com:3306               # Auto-infer MySQL
```

## Architecture

Sequential probe pipeline with dependency-based skip logic:

```
SystemProbe → DNSProbe → ConnProbe → TLSProbe → ProtocolProbe → Diagnosis
```

Each probe receives results from previous probes via `prev map[string]*ProbeResult`. If a probe fails, dependent probes skip with a reason message.

## Code Structure

- `cmd/root.go` — CLI entry, pipeline orchestration, flag handling
- `pkg/target/parser.go` — Target parsing (URI, host:port, IP, batch file)
- `pkg/probe/probe.go` — Core types: `Probe` interface, `ProbeResult`, `Target`, detail structs
- `pkg/probe/system.go` + `system_{darwin,linux,windows}.go` — Proxy/TUN/route detection (build tags)
- `pkg/probe/dns.go` — DNS resolution + consistency check vs 8.8.8.8
- `pkg/probe/conn.go` — TCP connect + error classification
- `pkg/probe/tls.go` — TLS handshake, SNI, MITM detection (14 enterprise CA list)
- `pkg/probe/protocol.go` — Protocol handshake (HTTP/MySQL/Redis/PostgreSQL/SSH/generic TCP)
- `pkg/diagnosis/engine.go` — Rule-based diagnosis from probe results
- `pkg/output/text.go` — Colored terminal output (Chinese)
- `pkg/output/json.go` — JSON output with `JSONOutput` struct

## Key Patterns

- Platform-specific code uses Go build tags (`_darwin.go`, `_linux.go`, `_windows.go`)
- `ProbeResult.FinalizeStatus()` must be called before JSON serialization to sync `StatusStr`
- Port inference: 3306→MySQL, 6379→Redis, 5432→PostgreSQL, 22→SSH, 80→HTTP, 443→HTTPS
- Unknown ports default to generic TCP probe
- Exit codes: 0=reachable, 1=unreachable, 2=argument/internal error

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/fatih/color` — Terminal color (supports `NO_COLOR` env var)
