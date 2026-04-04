# ClaudeScope Design

**Date:** 2026-04-03
**Status:** Approved

## Overview

ClaudeScope is a Go CLI tool that gives Claude structured access to FRC robot data — both live NetworkTables (NT4) connections and `.wpilog` log files — for debugging and tuning workflows. Claude invokes it as a shell command; it outputs JSON.

---

## Architecture & Process Model

A single `ClaudeScope.exe` binary operates in two modes:

- **CLI mode** (default): Parses args, checks if the daemon is running on `localhost:5812` via `GET /ping`, auto-starts the daemon as a detached background process (`ClaudeScope.exe --daemon`) if not reachable, then sends an HTTP request and prints the JSON response to stdout (and optionally writes to `--out <file>`).
- **Daemon mode** (`--daemon` flag, internal): Runs an HTTP server on `localhost:5812`, holds all active sessions in memory, and runs a background sweep every 60s to expire sessions idle for more than 10 minutes.

**Daemon auto-start flow:**
1. CLI sends `GET /ping` to `localhost:5812`
2. If connection refused → spawn detached `ClaudeScope.exe --daemon`, retry with backoff (up to ~2s)
3. If still unreachable → return error JSON and exit non-zero

---

## Session Interface

All data sources implement a single `DataSession` interface, ensuring live NT and log file sessions are interchangeable:

```go
type SessionType int
const (
    LiveSession SessionType = iota
    LogSession
)

type FieldInfo struct {
    Key  string
    Type string // "double", "boolean", "string", etc.
}

type DataPoint struct {
    Timestamp int64  // microseconds
    Value     any
}

type TimeRange struct {
    Start int64 // microseconds
    End   int64
}

type Stats struct {
    Mean      float64
    Median    float64
    Min       float64
    Max       float64
    Q1        float64
    Q3        float64
    AvgDelta  float64 // average change per second
    MinDelta  float64
    MaxDelta  float64
}

type DataSession interface {
    Type() SessionType
    Fields() ([]FieldInfo, error)
    TimeRange() (start, end int64, error)   // microseconds
    GetValues(keys []string, t int64) (map[string]*DataPoint, error)
    GetRanges(keys []string, start, end int64) (map[string][]DataPoint, error)
    FindBoolRanges(key string, value bool) ([]TimeRange, error)
    FindThresholdRanges(key string, min, max float64) ([]TimeRange, error)
    Stats(key string, start, end int64) (*Stats, error)
    Set(pairs map[string]any) error  // returns error for log sessions
    Close() error
}
```

Two implementations:
- `NTSession` — wraps `go-nt4`, live NT4 over WebSocket
- `LogSession` — parses `.wpilog` binary format, fully in-memory after load

---

## CLI Commands

All commands output JSON to stdout and exit 0 on success, non-zero on error. All accept `--out <file>` to write JSON to a file instead.

| Command | Description |
|---|---|
| `claudescope connect <ip>` | Start live NT session → `{"session_id": "..."}` |
| `claudescope load <path.wpilog>` | Load log file session → `{"session_id": "..."}` |
| `claudescope disconnect --session <id>` | Close and remove session |
| `claudescope info --session <id>` | All fields + types, time range, FMS info |
| `claudescope get <key> [key2 ...] --session <id> [--time <us>]` | Values for one or more keys at a timestamp (default: now/end) |
| `claudescope range <key> [key2 ...] --session <id> [--start <us>] [--end <us>]` | Time-series data for one or more keys |
| `claudescope find-bool <key> <true\|false> --session <id>` | All time ranges where the bool field matches |
| `claudescope find-threshold <key> --min <n> --max <n> --session <id>` | All time ranges where the value is within [min, max] |
| `claudescope stats <key> --session <id> [--start <us>] [--end <us>]` | Descriptive statistics for a numeric field |
| `claudescope set <key>=<val> [key2=val2 ...] --session <id>` | Set one or more live NT values (live sessions only) |

**Time offsets:** Negative `--start`/`--end` values are offsets from the end of the log (e.g., `--start -30000000` = last 30 seconds).

**Multi-key output:** JSON object keyed by field name, e.g.:
```json
{
  "/SmartDashboard/speed": {"timestamp": 1234567, "value": 1.23},
  "/SmartDashboard/voltage": {"timestamp": 1234567, "value": 12.1}
}
```

---

## File Structure

```
ClaudeScope/
├── main.go              # Entry point: routes to CLI or --daemon mode
├── daemon/
│   ├── server.go        # HTTP server setup, route registration
│   ├── registry.go      # Session map, 10-min expiry sweep
│   └── handlers.go      # HTTP handler for each command
├── session/
│   ├── interface.go     # DataSession interface + shared types
│   ├── nt_session.go    # Live NT4 implementation (wraps go-nt4)
│   └── log_session.go   # .wpilog binary parser + DataSession impl
└── cli/
    ├── client.go        # HTTP client, daemon auto-start logic
    └── commands.go      # Arg parsing, JSON output, --out file handling
```

---

## Error Handling

All errors return a consistent JSON shape:
```json
{"error": "session not found", "code": "SESSION_NOT_FOUND"}
```

CLI exits with non-zero status on any error so Claude can detect failures.

Key error codes:
| Code | Meaning |
|---|---|
| `SESSION_NOT_FOUND` | Session ID doesn't exist or has expired |
| `READ_ONLY_SESSION` | `set` called on a log session |
| `DAEMON_UNAVAILABLE` | Daemon failed to start or unreachable |
| `CONNECT_FAILED` | NT4 connection to robot/sim failed |
| `INVALID_LOG` | `.wpilog` file not found or corrupt |
| `KEY_NOT_FOUND` | Requested NT key doesn't exist in session |

---

## Testing Strategy

- `DataSession` interface enables unit testing both session types without real hardware
- Daemon HTTP handlers tested with `httptest.NewRecorder`
- `.wpilog` parser tested against fixture files (sample logs checked into `testdata/`)
- Integration tests for live NT require a running simulation — skipped in CI, run manually
- Existing `NTClient` interface + `realFactory` pattern (already in codebase) extended to the session layer
