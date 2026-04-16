# ClaudeScope Full Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the existing REPL prototype with the full ClaudeScope daemon+CLI architecture — a Go binary that Claude invokes as a shell command to inspect live NT4 connections and `.wpilog` files via a local HTTP daemon.

**Architecture:** The binary routes at startup: `--daemon` flag starts an HTTP server on `localhost:5812` managing named `DataSession` objects; CLI mode auto-starts the daemon if needed, sends one HTTP request, prints JSON, and exits. `DataSession` is a shared interface implemented by both `NTSession` (live NT4) and `LogSession` (wpilog file).

**Tech Stack:** Go 1.25, `net/http` + `net/http/httptest`, `encoding/json`, `crypto/rand`, `sort`, `math`, `encoding/binary`, `github.com/levifitzpatrick1/go-nt4 v0.1.1`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `ClaudeScope/session.go` | **Delete** | Replaced by session/ package |
| `ClaudeScope/repl.go` | **Delete** | Replaced by CLI architecture |
| `ClaudeScope/session_test.go` | **Delete** | Replaced by session/ tests |
| `ClaudeScope/repl_test.go` | **Delete** | Replaced by cli/ tests |
| `ClaudeScope/main.go` | **Replace** | Thin entry point: routes --daemon vs CLI |
| `ClaudeScope/session/interface.go` | Create | `DataSession` interface + shared types |
| `ClaudeScope/session/log_session.go` | Create | `.wpilog` binary parser + `DataSession` impl |
| `ClaudeScope/session/log_session_test.go` | Create | Unit tests (no hardware needed) |
| `ClaudeScope/session/nt_session.go` | Create | Live NT4 `DataSession` impl wrapping go-nt4 |
| `ClaudeScope/daemon/registry.go` | Create | Session map, UUID IDs, 10-min expiry sweep |
| `ClaudeScope/daemon/registry_test.go` | Create | Unit tests for registry lifecycle |
| `ClaudeScope/daemon/handlers.go` | Create | HTTP handlers for all commands + JSON types |
| `ClaudeScope/daemon/handlers_test.go` | Create | httptest-based handler tests |
| `ClaudeScope/daemon/server.go` | Create | HTTP server setup + route registration |
| `ClaudeScope/cli/client.go` | Create | HTTP client, daemon auto-start, retry logic |
| `ClaudeScope/cli/client_test.go` | Create | Tests with a mock HTTP server |
| `ClaudeScope/cli/commands.go` | Create | Arg parsing, flag handling, JSON output |
| `ClaudeScope/cli/commands_test.go` | Create | Tests for every subcommand |

---

## Task 1: Delete old files and scaffold package directories

**Files:**
- Delete: `ClaudeScope/session.go`, `ClaudeScope/repl.go`, `ClaudeScope/session_test.go`, `ClaudeScope/repl_test.go`
- Modify: `ClaudeScope/main.go` (temporary stub)

- [ ] **Step 1: Delete the REPL prototype files**

```bash
cd ClaudeScope
rm session.go repl.go session_test.go repl_test.go
```

- [ ] **Step 2: Create package directories**

```bash
mkdir -p session daemon cli
```

- [ ] **Step 3: Replace `ClaudeScope/main.go` with a build stub**

```go
package main

func main() {}
```

- [ ] **Step 4: Verify it builds**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: remove REPL prototype, scaffold new package dirs"
```

---

## Task 2: `session/interface.go` — shared types and `DataSession` interface

**Files:**
- Create: `ClaudeScope/session/interface.go`

- [ ] **Step 1: Create `ClaudeScope/session/interface.go`**

```go
package session

// SessionType identifies whether a session is live NT4 or a loaded log file.
type SessionType int

const (
	LiveSession SessionType = iota
	LogSession
)

// FieldInfo describes one data field in a session.
type FieldInfo struct {
	Key  string `json:"key"`
	Type string `json:"type"` // "double", "boolean", "string", etc.
}

// DataPoint is one timestamped value.
type DataPoint struct {
	Timestamp int64 `json:"timestamp"` // microseconds since log start
	Value     any   `json:"value"`
}

// TimeRange is a closed interval in microseconds.
type TimeRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Stats holds descriptive statistics for a numeric field over a time window.
type Stats struct {
	Mean     float64 `json:"mean"`
	Median   float64 `json:"median"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Q1       float64 `json:"q1"`
	Q3       float64 `json:"q3"`
	AvgDelta float64 `json:"avg_delta"` // average change per second
	MinDelta float64 `json:"min_delta"`
	MaxDelta float64 `json:"max_delta"`
}

// DataSession is the common interface for both live NT4 and wpilog sessions.
type DataSession interface {
	// Type returns LiveSession or LogSession.
	Type() SessionType

	// Fields returns all known fields and their types.
	Fields() ([]FieldInfo, error)

	// TimeRange returns the start and end timestamps in microseconds.
	// For live sessions, start=0 and end=time since connect (relative).
	TimeRange() (start, end int64, err error)

	// GetValues returns the value of each key at or before timestamp t.
	// If t == 0, returns the latest known value.
	GetValues(keys []string, t int64) (map[string]*DataPoint, error)

	// GetRanges returns all data points for each key in [start, end].
	// If start == 0, begins from the first record. If end == 0, goes to the last.
	GetRanges(keys []string, start, end int64) (map[string][]DataPoint, error)

	// FindBoolRanges returns all intervals where key equals value.
	FindBoolRanges(key string, value bool) ([]TimeRange, error)

	// FindThresholdRanges returns all intervals where min <= key <= max.
	FindThresholdRanges(key string, min, max float64) ([]TimeRange, error)

	// Stats computes descriptive statistics for a numeric field over [start, end].
	Stats(key string, start, end int64) (*Stats, error)

	// Set publishes key/value pairs. Returns an error for LogSession.
	Set(pairs map[string]any) error

	// Close releases all resources held by the session.
	Close() error
}
```

- [ ] **Step 2: Verify build**

```bash
cd ClaudeScope && go build ./...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add session/interface.go
git commit -m "feat: add DataSession interface and shared types"
```

---

## Task 3: `session/log_session.go` — `.wpilog` parser

**Files:**
- Create: `ClaudeScope/session/log_session.go`
- Create: `ClaudeScope/session/log_session_test.go`

- [ ] **Step 1: Write the failing tests first**

Create `ClaudeScope/session/log_session_test.go`:

```go
package session

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// makeTestLog builds a minimal valid .wpilog in memory.
// It defines two fields:
//   - "/voltage" (double): values 12.0@t=1000, 11.5@t=2000, 11.0@t=3000
//   - "/enabled" (boolean): true@t=1000, false@t=2500
func makeTestLog() []byte {
	var buf bytes.Buffer

	// Magic + version
	buf.WriteString("WPILOG\x00")
	buf.Write([]byte{0x00, 0x01}) // version 1.0 (LE: minor=0, major=1)

	// Extra header string (empty)
	var extraLen [4]byte
	binary.LittleEndian.PutUint32(extraLen[:], 0)
	buf.Write(extraLen[:])

	writeRecord := func(entryID uint64, timestamp uint64, payload []byte) {
		writeVarintBuf(&buf, entryID)
		writeVarintBuf(&buf, uint64(len(payload)))
		writeVarintBuf(&buf, timestamp)
		buf.Write(payload)
	}

	// Control: Start "/voltage" as double, entry_id=1
	var p bytes.Buffer
	p.WriteByte(0) // Start type
	var eid [4]byte
	binary.LittleEndian.PutUint32(eid[:], 1)
	p.Write(eid[:])
	writeLenString(&p, "/voltage")
	writeLenString(&p, "double")
	writeLenString(&p, "")
	writeRecord(0, 0, p.Bytes())

	// Control: Start "/enabled" as boolean, entry_id=2
	p.Reset()
	p.WriteByte(0)
	binary.LittleEndian.PutUint32(eid[:], 2)
	p.Write(eid[:])
	writeLenString(&p, "/enabled")
	writeLenString(&p, "boolean")
	writeLenString(&p, "")
	writeRecord(0, 0, p.Bytes())

	// Data: /voltage=12.0 @ t=1000
	writeRecord(1, 1000, float64Bytes(12.0))
	// Data: /enabled=true @ t=1000
	writeRecord(2, 1000, []byte{1})
	// Data: /voltage=11.5 @ t=2000
	writeRecord(1, 2000, float64Bytes(11.5))
	// Data: /enabled=false @ t=2500
	writeRecord(2, 2500, []byte{0})
	// Data: /voltage=11.0 @ t=3000
	writeRecord(1, 3000, float64Bytes(11.0))

	return buf.Bytes()
}

func writeLenString(w *bytes.Buffer, s string) {
	var lenBytes [4]byte
	binary.LittleEndian.PutUint32(lenBytes[:], uint32(len(s)))
	w.Write(lenBytes[:])
	w.WriteString(s)
}

func float64Bytes(v float64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(v))
	return b
}

func loadTestLog(t *testing.T) *wpilogSession {
	t.Helper()
	data := makeTestLog()
	s, err := ParseWPILog(data)
	if err != nil {
		t.Fatalf("ParseWPILog failed: %v", err)
	}
	return s
}

func TestLogSession_Type(t *testing.T) {
	s := loadTestLog(t)
	if s.Type() != LogSessionType {
		t.Fatalf("expected LogSession, got %v", s.Type())
	}
}

func TestLogSession_Fields(t *testing.T) {
	s := loadTestLog(t)
	fields, err := s.Fields()
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, f := range fields {
		keys[f.Key] = f.Type
	}
	if keys["/voltage"] != "double" {
		t.Errorf("expected /voltage=double, got %q", keys["/voltage"])
	}
	if keys["/enabled"] != "boolean" {
		t.Errorf("expected /enabled=boolean, got %q", keys["/enabled"])
	}
}

func TestLogSession_TimeRange(t *testing.T) {
	s := loadTestLog(t)
	start, end, err := s.TimeRange()
	if err != nil {
		t.Fatal(err)
	}
	if start != 1000 {
		t.Errorf("expected start=1000, got %d", start)
	}
	if end != 3000 {
		t.Errorf("expected end=3000, got %d", end)
	}
}

func TestLogSession_GetValues_AtTimestamp(t *testing.T) {
	s := loadTestLog(t)
	pts, err := s.GetValues([]string{"/voltage"}, 2500)
	if err != nil {
		t.Fatal(err)
	}
	pt := pts["/voltage"]
	if pt == nil {
		t.Fatal("expected /voltage, got nil")
	}
	if pt.Value.(float64) != 11.5 {
		t.Errorf("expected 11.5, got %v", pt.Value)
	}
}

func TestLogSession_GetValues_Latest(t *testing.T) {
	s := loadTestLog(t)
	// t=0 means latest
	pts, err := s.GetValues([]string{"/voltage"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if pts["/voltage"].Value.(float64) != 11.0 {
		t.Errorf("expected 11.0, got %v", pts["/voltage"].Value)
	}
}

func TestLogSession_GetValues_KeyNotFound(t *testing.T) {
	s := loadTestLog(t)
	_, err := s.GetValues([]string{"/nonexistent"}, 0)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestLogSession_GetRanges(t *testing.T) {
	s := loadTestLog(t)
	ranges, err := s.GetRanges([]string{"/voltage"}, 1000, 2500)
	if err != nil {
		t.Fatal(err)
	}
	pts := ranges["/voltage"]
	if len(pts) != 2 {
		t.Fatalf("expected 2 points in [1000,2500], got %d", len(pts))
	}
	if pts[0].Value.(float64) != 12.0 {
		t.Errorf("expected 12.0, got %v", pts[0].Value)
	}
	if pts[1].Value.(float64) != 11.5 {
		t.Errorf("expected 11.5, got %v", pts[1].Value)
	}
}

func TestLogSession_FindBoolRanges(t *testing.T) {
	s := loadTestLog(t)
	ranges, err := s.FindBoolRanges("/enabled", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 {
		t.Fatalf("expected 1 true range, got %d", len(ranges))
	}
	if ranges[0].Start != 1000 || ranges[0].End != 2500 {
		t.Errorf("expected [1000,2500], got [%d,%d]", ranges[0].Start, ranges[0].End)
	}
}

func TestLogSession_FindThresholdRanges(t *testing.T) {
	s := loadTestLog(t)
	// voltage is in [11.0, 11.5] for t=2000..3000
	ranges, err := s.FindThresholdRanges("/voltage", 11.0, 11.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].Start != 2000 {
		t.Errorf("expected start=2000, got %d", ranges[0].Start)
	}
}

func TestLogSession_Stats(t *testing.T) {
	s := loadTestLog(t)
	stats, err := s.Stats("/voltage", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// values: 12.0, 11.5, 11.0 → mean=11.5, min=11.0, max=12.0
	if stats.Min != 11.0 {
		t.Errorf("expected min=11.0, got %f", stats.Min)
	}
	if stats.Max != 12.0 {
		t.Errorf("expected max=12.0, got %f", stats.Max)
	}
	if stats.Mean != 11.5 {
		t.Errorf("expected mean=11.5, got %f", stats.Mean)
	}
}

func TestLogSession_Set_ReturnsError(t *testing.T) {
	s := loadTestLog(t)
	err := s.Set(map[string]any{"/voltage": 12.0})
	if err == nil {
		t.Fatal("expected error: log sessions are read-only")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail (log_session.go doesn't exist yet)**

```bash
cd ClaudeScope && go test ./session/... -v 2>&1 | head -20
```

Expected: compile error — `ParseWPILog`, `wpilogSession`, `writeVarintBuf`, `LogSessionType` undefined.

- [ ] **Step 3: Create `ClaudeScope/session/log_session.go`**

```go
package session

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
)

const LogSessionType = LogSession

// wpilogSession holds all data parsed from a .wpilog file.
type wpilogSession struct {
	fields  map[uint32]FieldInfo      // entry_id → field
	data    map[uint32][]DataPoint    // entry_id → sorted data points
	keyToID map[string]uint32         // field name → entry_id
	start   int64
	end     int64
}

// ParseWPILog parses a .wpilog binary and returns a ready-to-query session.
func ParseWPILog(raw []byte) (*wpilogSession, error) {
	r := bytes.NewReader(raw)

	// Magic
	magic := make([]byte, 7)
	if _, err := r.Read(magic); err != nil || string(magic) != "WPILOG\x00" {
		return nil, errors.New("invalid wpilog: bad magic")
	}

	// Version (2 bytes LE, minor then major — we accept any 1.x)
	var version [2]byte
	if _, err := r.Read(version[:]); err != nil {
		return nil, fmt.Errorf("invalid wpilog: missing version: %w", err)
	}

	// Extra header string
	var extraLen uint32
	if err := binary.Read(r, binary.LittleEndian, &extraLen); err != nil {
		return nil, fmt.Errorf("invalid wpilog: missing extra header length: %w", err)
	}
	if _, err := r.Seek(int64(extraLen), 1); err != nil {
		return nil, fmt.Errorf("invalid wpilog: truncated extra header: %w", err)
	}

	s := &wpilogSession{
		fields:  make(map[uint32]FieldInfo),
		data:    make(map[uint32][]DataPoint),
		keyToID: make(map[string]uint32),
		start:   math.MaxInt64,
		end:     math.MinInt64,
	}

	for r.Len() > 0 {
		entryID, err := readVarint(r)
		if err != nil {
			break
		}
		payloadSize, err := readVarint(r)
		if err != nil {
			return nil, fmt.Errorf("wpilog: truncated record header")
		}
		timestamp, err := readVarint(r)
		if err != nil {
			return nil, fmt.Errorf("wpilog: truncated record header")
		}

		payload := make([]byte, payloadSize)
		if _, err := r.Read(payload); err != nil {
			return nil, fmt.Errorf("wpilog: truncated record payload")
		}

		if entryID == 0 {
			if err := s.applyControl(payload); err != nil {
				return nil, fmt.Errorf("wpilog: control record: %w", err)
			}
		} else {
			id := uint32(entryID)
			fi, ok := s.fields[id]
			if !ok {
				continue // unknown entry — skip
			}
			value, err := decodeValue(fi.Type, payload)
			if err != nil {
				continue // unparseable value — skip
			}
			ts := int64(timestamp)
			s.data[id] = append(s.data[id], DataPoint{Timestamp: ts, Value: value})
			if ts < s.start {
				s.start = ts
			}
			if ts > s.end {
				s.end = ts
			}
		}
	}

	if s.start == math.MaxInt64 {
		s.start = 0
		s.end = 0
	}

	return s, nil
}

func (s *wpilogSession) applyControl(payload []byte) error {
	if len(payload) < 1 {
		return errors.New("empty control payload")
	}
	r := bytes.NewReader(payload[1:])
	switch payload[0] {
	case 0: // Start
		var eid uint32
		if err := binary.Read(r, binary.LittleEndian, &eid); err != nil {
			return err
		}
		name, err := readLenString(r)
		if err != nil {
			return err
		}
		typeStr, err := readLenString(r)
		if err != nil {
			return err
		}
		s.fields[eid] = FieldInfo{Key: name, Type: typeStr}
		s.keyToID[name] = eid
	case 1: // SetMetadata — ignore
	case 2: // Finish — ignore
	}
	return nil
}

func readLenString(r *bytes.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := r.Read(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func decodeValue(typeStr string, payload []byte) (any, error) {
	switch typeStr {
	case "double":
		if len(payload) < 8 {
			return nil, errors.New("short double")
		}
		bits := binary.LittleEndian.Uint64(payload[:8])
		return math.Float64frombits(bits), nil
	case "float":
		if len(payload) < 4 {
			return nil, errors.New("short float")
		}
		bits := binary.LittleEndian.Uint32(payload[:4])
		return float64(math.Float32frombits(bits)), nil
	case "int64":
		if len(payload) < 8 {
			return nil, errors.New("short int64")
		}
		return int64(binary.LittleEndian.Uint64(payload[:8])), nil
	case "boolean":
		if len(payload) < 1 {
			return nil, errors.New("short boolean")
		}
		return payload[0] != 0, nil
	case "string":
		if len(payload) < 4 {
			return nil, errors.New("short string")
		}
		length := binary.LittleEndian.Uint32(payload[:4])
		if int(length) > len(payload)-4 {
			return nil, errors.New("string length overflow")
		}
		return string(payload[4 : 4+length]), nil
	default:
		// Return raw bytes for unknown/array types
		return payload, nil
	}
}

// readVarint decodes the WPILib variable-length integer encoding.
// Low 2 bits of first byte indicate extra byte count: 0→1B, 1→2B, 2→4B, 3→8B.
func readVarint(r *bytes.Reader) (uint64, error) {
	b0, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	switch b0 & 0x03 {
	case 0:
		return uint64(b0) >> 2, nil
	case 1:
		b1, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		return (uint64(b0) | uint64(b1)<<8) >> 2, nil
	case 2:
		b1, _ := r.ReadByte()
		b2, _ := r.ReadByte()
		b3, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		return (uint64(b0) | uint64(b1)<<8 | uint64(b2)<<16 | uint64(b3)<<24) >> 2, nil
	default:
		b1, _ := r.ReadByte()
		b2, _ := r.ReadByte()
		b3, _ := r.ReadByte()
		b4, _ := r.ReadByte()
		b5, _ := r.ReadByte()
		b6, _ := r.ReadByte()
		b7, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		v := uint64(b0) | uint64(b1)<<8 | uint64(b2)<<16 | uint64(b3)<<24 |
			uint64(b4)<<32 | uint64(b5)<<40 | uint64(b6)<<48 | uint64(b7)<<56
		return v >> 2, nil
	}
}

// writeVarintBuf encodes v into the WPILib variable-length integer format and
// appends it to buf. Used only by tests to build fixture .wpilog files.
func writeVarintBuf(buf *bytes.Buffer, v uint64) {
	switch {
	case v <= 0x3F:
		buf.WriteByte(byte(v<<2) | 0x00)
	case v <= 0x3FFF:
		encoded := (v << 2) | 0x01
		buf.WriteByte(byte(encoded))
		buf.WriteByte(byte(encoded >> 8))
	case v <= 0x3FFFFFFF:
		encoded := (v << 2) | 0x02
		buf.WriteByte(byte(encoded))
		buf.WriteByte(byte(encoded >> 8))
		buf.WriteByte(byte(encoded >> 16))
		buf.WriteByte(byte(encoded >> 24))
	default:
		encoded := (v << 2) | 0x03
		for i := 0; i < 8; i++ {
			buf.WriteByte(byte(encoded >> (i * 8)))
		}
	}
}

// --- DataSession implementation ---

func (s *wpilogSession) Type() SessionType { return LogSession }

func (s *wpilogSession) Fields() ([]FieldInfo, error) {
	out := make([]FieldInfo, 0, len(s.fields))
	for _, fi := range s.fields {
		out = append(out, fi)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *wpilogSession) TimeRange() (int64, int64, error) {
	return s.start, s.end, nil
}

func (s *wpilogSession) GetValues(keys []string, t int64) (map[string]*DataPoint, error) {
	result := make(map[string]*DataPoint, len(keys))
	for _, key := range keys {
		id, ok := s.keyToID[key]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		pts := s.data[id]
		if len(pts) == 0 {
			continue
		}
		if t == 0 {
			// Latest
			cp := pts[len(pts)-1]
			result[key] = &cp
			continue
		}
		// Binary search: last point with Timestamp <= t
		idx := sort.Search(len(pts), func(i int) bool { return pts[i].Timestamp > t }) - 1
		if idx >= 0 {
			cp := pts[idx]
			result[key] = &cp
		}
	}
	return result, nil
}

func (s *wpilogSession) GetRanges(keys []string, start, end int64) (map[string][]DataPoint, error) {
	if start < 0 {
		start = s.end + start
	}
	if end < 0 {
		end = s.end + end
	}
	if end == 0 {
		end = s.end
	}

	result := make(map[string][]DataPoint, len(keys))
	for _, key := range keys {
		id, ok := s.keyToID[key]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		pts := s.data[id]
		var out []DataPoint
		for _, p := range pts {
			if p.Timestamp >= start && p.Timestamp <= end {
				out = append(out, p)
			}
		}
		result[key] = out
	}
	return result, nil
}

func (s *wpilogSession) FindBoolRanges(key string, value bool) ([]TimeRange, error) {
	id, ok := s.keyToID[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	pts := s.data[id]
	var ranges []TimeRange
	inRange := false
	var rangeStart int64
	for _, p := range pts {
		v, ok := p.Value.(bool)
		if !ok {
			continue
		}
		if v == value && !inRange {
			inRange = true
			rangeStart = p.Timestamp
		} else if v != value && inRange {
			ranges = append(ranges, TimeRange{Start: rangeStart, End: p.Timestamp})
			inRange = false
		}
	}
	if inRange {
		ranges = append(ranges, TimeRange{Start: rangeStart, End: s.end})
	}
	return ranges, nil
}

func (s *wpilogSession) FindThresholdRanges(key string, min, max float64) ([]TimeRange, error) {
	id, ok := s.keyToID[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	pts := s.data[id]
	var ranges []TimeRange
	inRange := false
	var rangeStart int64
	for _, p := range pts {
		v, err := toFloat64(p.Value)
		if err != nil {
			continue
		}
		inBounds := v >= min && v <= max
		if inBounds && !inRange {
			inRange = true
			rangeStart = p.Timestamp
		} else if !inBounds && inRange {
			ranges = append(ranges, TimeRange{Start: rangeStart, End: p.Timestamp})
			inRange = false
		}
	}
	if inRange {
		ranges = append(ranges, TimeRange{Start: rangeStart, End: s.end})
	}
	return ranges, nil
}

func (s *wpilogSession) Stats(key string, start, end int64) (*Stats, error) {
	if start < 0 {
		start = s.end + start
	}
	if end == 0 {
		end = s.end
	}
	ranges, err := s.GetRanges([]string{key}, start, end)
	if err != nil {
		return nil, err
	}
	pts := ranges[key]
	if len(pts) == 0 {
		return &Stats{}, nil
	}

	var vals []float64
	for _, p := range pts {
		v, err := toFloat64(p.Value)
		if err != nil {
			continue
		}
		vals = append(vals, v)
	}
	if len(vals) == 0 {
		return &Stats{}, fmt.Errorf("field %s has no numeric values", key)
	}

	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}

	stats := &Stats{
		Mean:   sum / float64(len(sorted)),
		Median: percentile(sorted, 0.5),
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Q1:     percentile(sorted, 0.25),
		Q3:     percentile(sorted, 0.75),
	}

	// Deltas: rate of change per second between consecutive points
	if len(pts) >= 2 {
		var deltas []float64
		for i := 1; i < len(pts); i++ {
			v0, e0 := toFloat64(pts[i-1].Value)
			v1, e1 := toFloat64(pts[i].Value)
			if e0 != nil || e1 != nil {
				continue
			}
			dt := float64(pts[i].Timestamp-pts[i-1].Timestamp) / 1e6
			if dt <= 0 {
				continue
			}
			deltas = append(deltas, (v1-v0)/dt)
		}
		if len(deltas) > 0 {
			var dsum float64
			dmin, dmax := deltas[0], deltas[0]
			for _, d := range deltas {
				dsum += d
				if d < dmin {
					dmin = d
				}
				if d > dmax {
					dmax = d
				}
			}
			stats.AvgDelta = dsum / float64(len(deltas))
			stats.MinDelta = dmin
			stats.MaxDelta = dmax
		}
	}

	return stats, nil
}

func (s *wpilogSession) Set(_ map[string]any) error {
	return errors.New("read-only session: cannot set values on a log file")
}

func (s *wpilogSession) Close() error { return nil }

// --- helpers ---

func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 1 {
		return sorted[0]
	}
	idx := p * float64(n-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= n {
		return sorted[n-1]
	}
	return sorted[lo]*(1-(idx-float64(lo))) + sorted[hi]*(idx-float64(lo))
}

func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("not a numeric value: %T", v)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd ClaudeScope && go test ./session/... -v
```

Expected output:
```
--- PASS: TestLogSession_Type (0.00s)
--- PASS: TestLogSession_Fields (0.00s)
--- PASS: TestLogSession_TimeRange (0.00s)
--- PASS: TestLogSession_GetValues_AtTimestamp (0.00s)
--- PASS: TestLogSession_GetValues_Latest (0.00s)
--- PASS: TestLogSession_GetValues_KeyNotFound (0.00s)
--- PASS: TestLogSession_GetRanges (0.00s)
--- PASS: TestLogSession_FindBoolRanges (0.00s)
--- PASS: TestLogSession_FindThresholdRanges (0.00s)
--- PASS: TestLogSession_Stats (0.00s)
--- PASS: TestLogSession_Set_ReturnsError (0.00s)
PASS
```

If any fail, fix `log_session.go` until all pass.

- [ ] **Step 5: Commit**

```bash
git add session/log_session.go session/log_session_test.go
git commit -m "feat: add wpilog parser and LogSession DataSession implementation"
```

---

## Task 4: `session/nt_session.go` — live NT4 `DataSession`

**Files:**
- Create: `ClaudeScope/session/nt_session.go`

> No unit tests for this file — it requires a live NT4 connection. Skipped in CI; test manually with a running simulation.

- [ ] **Step 1: Create `ClaudeScope/session/nt_session.go`**

```go
package session

import (
	"errors"
	"fmt"
	"sync"
	"time"

	nt4 "github.com/levifitzpatrick1/go-nt4"
)

// ntSession wraps a live go-nt4 client and implements DataSession.
// It subscribes to all topics and maintains an in-memory history
// of all received values since the session was created.
type ntSession struct {
	client    *nt4.Client
	sub       *nt4.Subscription
	mu        sync.RWMutex
	history   map[string][]DataPoint // key → sorted data points
	fieldMeta map[string]string      // key → type string
	connectedAt time.Time
}

// NewNTSession connects to an NT4 server at addr and returns a DataSession.
// addr is a hostname or IP (port defaults to NT4 default 5810).
func NewNTSession(addr string) (DataSession, error) {
	opts := nt4.DefaultClientOptions(addr)
	opts.Identity = "ClaudeScope"
	c := nt4.NewClient(opts)
	if err := c.Connect(); err != nil {
		return nil, fmt.Errorf("NT4 connect failed: %w", err)
	}

	s := &ntSession{
		client:      c,
		history:     make(map[string][]DataPoint),
		fieldMeta:   make(map[string]string),
		connectedAt: time.Now(),
	}

	// Subscribe to all topics and buffer updates.
	sub := c.Subscribe([]string{"/"}, nt4.SubscribeOptions{Prefix: true})
	s.sub = sub
	go s.pump(sub)

	return s, nil
}

// pump reads from the subscription channel and appends to history.
func (s *ntSession) pump(sub *nt4.Subscription) {
	for update := range sub.Updates() {
		s.mu.Lock()
		key := update.Topic.Name
		s.fieldMeta[key] = update.Topic.Type
		ts := update.Timestamp
		s.history[key] = append(s.history[key], DataPoint{
			Timestamp: ts,
			Value:     update.Value,
		})
		s.mu.Unlock()
	}
}

func (s *ntSession) Type() SessionType { return LiveSession }

func (s *ntSession) Fields() ([]FieldInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FieldInfo, 0, len(s.fieldMeta))
	for key, typ := range s.fieldMeta {
		out = append(out, FieldInfo{Key: key, Type: typ})
	}
	return out, nil
}

func (s *ntSession) TimeRange() (int64, int64, error) {
	elapsed := time.Since(s.connectedAt).Microseconds()
	return 0, elapsed, nil
}

func (s *ntSession) GetValues(keys []string, _ int64) (map[string]*DataPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*DataPoint, len(keys))
	for _, key := range keys {
		pts := s.history[key]
		if len(pts) == 0 {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		cp := pts[len(pts)-1]
		result[key] = &cp
	}
	return result, nil
}

func (s *ntSession) GetRanges(keys []string, start, end int64) (map[string][]DataPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if end == 0 {
		end = time.Since(s.connectedAt).Microseconds()
	}
	result := make(map[string][]DataPoint, len(keys))
	for _, key := range keys {
		pts := s.history[key]
		var out []DataPoint
		for _, p := range pts {
			if p.Timestamp >= start && p.Timestamp <= end {
				out = append(out, p)
			}
		}
		result[key] = out
	}
	return result, nil
}

func (s *ntSession) FindBoolRanges(key string, value bool) ([]TimeRange, error) {
	s.mu.RLock()
	pts := s.history[key]
	s.mu.RUnlock()
	if pts == nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	// Reuse LogSession logic by building a temporary wpilogSession subset.
	tmp := &wpilogSession{
		keyToID: map[string]uint32{key: 1},
		data:    map[uint32][]DataPoint{1: pts},
		end:     pts[len(pts)-1].Timestamp,
	}
	return tmp.FindBoolRanges(key, value)
}

func (s *ntSession) FindThresholdRanges(key string, min, max float64) ([]TimeRange, error) {
	s.mu.RLock()
	pts := s.history[key]
	s.mu.RUnlock()
	if pts == nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	tmp := &wpilogSession{
		keyToID: map[string]uint32{key: 1},
		data:    map[uint32][]DataPoint{1: pts},
		end:     pts[len(pts)-1].Timestamp,
	}
	return tmp.FindThresholdRanges(key, min, max)
}

func (s *ntSession) Stats(key string, start, end int64) (*Stats, error) {
	s.mu.RLock()
	pts := s.history[key]
	s.mu.RUnlock()
	if pts == nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	if end == 0 {
		end = pts[len(pts)-1].Timestamp
	}
	tmp := &wpilogSession{
		keyToID: map[string]uint32{key: 1},
		data:    map[uint32][]DataPoint{1: pts},
		end:     end,
	}
	return tmp.Stats(key, start, end)
}

func (s *ntSession) Set(pairs map[string]any) error {
	for key, val := range pairs {
		s.mu.RLock()
		typeStr, ok := s.fieldMeta[key]
		s.mu.RUnlock()
		if !ok {
			return fmt.Errorf("key not found: %s (subscribe before setting)", key)
		}
		pub := s.client.Publish(key, typeStr)
		if err := pub.SetValue(val); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	return nil
}

func (s *ntSession) Close() error {
	if s.sub != nil {
		s.client.Unsubscribe(s.sub)
	}
	s.client.Disconnect()
	return nil
}

// Ensure ntSession satisfies DataSession at compile time.
var _ DataSession = (*ntSession)(nil)

// ErrLiveSessionReadOnly is returned when Set is called on a log session.
var ErrLiveSessionReadOnly = errors.New("read-only session")
```

- [ ] **Step 2: Verify build (may need to adjust go-nt4 API calls)**

```bash
cd ClaudeScope && go build ./...
```

If the build fails due to go-nt4 API mismatches, check the library's exported methods:

```bash
grep -n "^func " /root/go/pkg/mod/github.com/levifitzpatrick1/go-nt4@v0.1.1/*.go
```

Adjust `Subscribe`, `Publish`, `SetValue`, `Unsubscribe` calls to match the actual API.

- [ ] **Step 3: Commit**

```bash
git add session/nt_session.go
git commit -m "feat: add NTSession live DataSession wrapping go-nt4"
```

---

## Task 5: `daemon/registry.go` — session map and expiry

**Files:**
- Create: `ClaudeScope/daemon/registry.go`
- Create: `ClaudeScope/daemon/registry_test.go`

- [ ] **Step 1: Write failing tests**

Create `ClaudeScope/daemon/registry_test.go`:

```go
package daemon

import (
	"errors"
	"testing"
	"time"

	"github.com/rylero/TheFRCSuite/ClaudeScope/session"
)

// mockSession is a do-nothing DataSession for registry tests.
type mockSession struct{ closed bool }

func (m *mockSession) Type() session.SessionType                                    { return session.LogSession }
func (m *mockSession) Fields() ([]session.FieldInfo, error)                         { return nil, nil }
func (m *mockSession) TimeRange() (int64, int64, error)                             { return 0, 0, nil }
func (m *mockSession) GetValues([]string, int64) (map[string]*session.DataPoint, error) { return nil, nil }
func (m *mockSession) GetRanges([]string, int64, int64) (map[string][]session.DataPoint, error) {
	return nil, nil
}
func (m *mockSession) FindBoolRanges(string, bool) ([]session.TimeRange, error) { return nil, nil }
func (m *mockSession) FindThresholdRanges(string, float64, float64) ([]session.TimeRange, error) {
	return nil, nil
}
func (m *mockSession) Stats(string, int64, int64) (*session.Stats, error) { return nil, nil }
func (m *mockSession) Set(map[string]any) error {
	return errors.New("read-only")
}
func (m *mockSession) Close() error {
	m.closed = true
	return nil
}

func TestRegistry_AddGet(t *testing.T) {
	r := NewRegistry()
	s := &mockSession{}
	id := r.Add(s)
	if id == "" {
		t.Fatal("Add returned empty ID")
	}
	got, err := r.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != s {
		t.Fatal("Get returned wrong session")
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	s := &mockSession{}
	id := r.Add(s)
	if err := r.Remove(id); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if !s.closed {
		t.Fatal("Remove should call Close on the session")
	}
	_, err := r.Get(id)
	if err == nil {
		t.Fatal("expected error after Remove")
	}
}

func TestRegistry_Remove_NotFound(t *testing.T) {
	r := NewRegistry()
	err := r.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error removing nonexistent session")
	}
}

func TestRegistry_Expiry(t *testing.T) {
	r := NewRegistry()
	s := &mockSession{}
	id := r.Add(s)

	// Backdate the last-used time past the expiry threshold.
	r.mu.Lock()
	r.entries[id].lastUsed = time.Now().Add(-11 * time.Minute)
	r.mu.Unlock()

	r.sweep()

	if !s.closed {
		t.Fatal("expired session should be closed by sweep")
	}
	_, err := r.Get(id)
	if err == nil {
		t.Fatal("expired session should be removed")
	}
}

func TestRegistry_Touch_ResetsExpiry(t *testing.T) {
	r := NewRegistry()
	s := &mockSession{}
	id := r.Add(s)

	r.mu.Lock()
	r.entries[id].lastUsed = time.Now().Add(-11 * time.Minute)
	r.mu.Unlock()

	r.Touch(id)
	r.sweep()

	if s.closed {
		t.Fatal("touched session should not be expired")
	}
}

func TestRegistry_UniqueIDs(t *testing.T) {
	r := NewRegistry()
	ids := map[string]struct{}{}
	for i := 0; i < 100; i++ {
		id := r.Add(&mockSession{})
		if _, exists := ids[id]; exists {
			t.Fatalf("duplicate session ID: %s", id)
		}
		ids[id] = struct{}{}
	}
}
```

- [ ] **Step 2: Run to confirm compile error**

```bash
cd ClaudeScope && go test ./daemon/... -v 2>&1 | head -10
```

Expected: compile error — `NewRegistry`, `Registry`, etc. undefined.

- [ ] **Step 3: Create `ClaudeScope/daemon/registry.go`**

```go
package daemon

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/rylero/TheFRCSuite/ClaudeScope/session"
)

const sessionTTL = 10 * time.Minute
const sweepInterval = 60 * time.Second

type entry struct {
	sess     session.DataSession
	lastUsed time.Time
}

// Registry holds active DataSessions keyed by random session IDs.
type Registry struct {
	mu      sync.Mutex
	entries map[string]*entry
}

// NewRegistry creates a Registry and starts the background expiry goroutine.
func NewRegistry() *Registry {
	r := &Registry{entries: make(map[string]*entry)}
	go r.runSweep()
	return r
}

// Add registers sess and returns its new session ID.
func (r *Registry) Add(sess session.DataSession) string {
	id := newID()
	r.mu.Lock()
	r.entries[id] = &entry{sess: sess, lastUsed: time.Now()}
	r.mu.Unlock()
	return id
}

// Get returns the session for id, refreshing its last-used time.
// Returns an error if the session does not exist.
func (r *Registry) Get(id string) (session.DataSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	e.lastUsed = time.Now()
	return e.sess, nil
}

// Remove closes and deletes the session. Returns an error if not found.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	e, ok := r.entries[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("session not found: %s", id)
	}
	delete(r.entries, id)
	r.mu.Unlock()
	return e.sess.Close()
}

// Touch refreshes the last-used time without returning the session.
// Safe to call with an unknown ID (no-op).
func (r *Registry) Touch(id string) {
	r.mu.Lock()
	if e, ok := r.entries[id]; ok {
		e.lastUsed = time.Now()
	}
	r.mu.Unlock()
}

// sweep closes and removes all sessions idle longer than sessionTTL.
func (r *Registry) sweep() {
	r.mu.Lock()
	expired := []string{}
	for id, e := range r.entries {
		if time.Since(e.lastUsed) > sessionTTL {
			expired = append(expired, id)
		}
	}
	sessions := make([]session.DataSession, 0, len(expired))
	for _, id := range expired {
		sessions = append(sessions, r.entries[id].sess)
		delete(r.entries, id)
	}
	r.mu.Unlock()
	for _, s := range sessions {
		_ = s.Close()
	}
}

func (r *Registry) runSweep() {
	ticker := time.NewTicker(sweepInterval)
	for range ticker.C {
		r.sweep()
	}
}

// newID returns a random hex session ID.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x%x%x%x%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
```

- [ ] **Step 4: Run tests**

```bash
cd ClaudeScope && go test ./daemon/... -run TestRegistry -v
```

Expected:
```
--- PASS: TestRegistry_AddGet (0.00s)
--- PASS: TestRegistry_Get_NotFound (0.00s)
--- PASS: TestRegistry_Remove (0.00s)
--- PASS: TestRegistry_Remove_NotFound (0.00s)
--- PASS: TestRegistry_Expiry (0.00s)
--- PASS: TestRegistry_Touch_ResetsExpiry (0.00s)
--- PASS: TestRegistry_UniqueIDs (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add daemon/registry.go daemon/registry_test.go
git commit -m "feat: add session registry with 10-min expiry sweep"
```

---

## Task 6: `daemon/handlers.go` — HTTP request/response types and handlers

**Files:**
- Create: `ClaudeScope/daemon/handlers.go`
- Create: `ClaudeScope/daemon/handlers_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `ClaudeScope/daemon/handlers_test.go`:

```go
package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rylero/TheFRCSuite/ClaudeScope/session"
)

// testRegistry returns a Registry pre-loaded with a mockSession under a fixed ID.
func testRegistry(t *testing.T) (*Registry, string, *mockSession) {
	t.Helper()
	r := &Registry{entries: make(map[string]*entry)}
	s := &mockSession{}
	id := "test-session-id"
	r.entries[id] = &entry{sess: s}
	return r, id, s
}

func postJSON(t *testing.T, handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func TestHandlePing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	HandlePing(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleConnect_MissingIP(t *testing.T) {
	reg := &Registry{entries: make(map[string]*entry)}
	handler := HandleConnect(reg, func(addr string) (session.DataSession, error) {
		return nil, errors.New("should not be called")
	})
	w := postJSON(t, handler, map[string]string{})
	if w.Code == http.StatusOK {
		t.Fatal("expected error for missing ip")
	}
}

func TestHandleConnect_FactoryError(t *testing.T) {
	reg := &Registry{entries: make(map[string]*entry)}
	handler := HandleConnect(reg, func(addr string) (session.DataSession, error) {
		return nil, errors.New("connection refused")
	})
	w := postJSON(t, handler, map[string]string{"ip": "127.0.0.1"})
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200, got %d", w.Code)
	}
	var errResp errorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "CONNECT_FAILED" {
		t.Errorf("expected CONNECT_FAILED, got %q", errResp.Code)
	}
}

func TestHandleConnect_Success(t *testing.T) {
	reg := &Registry{entries: make(map[string]*entry)}
	handler := HandleConnect(reg, func(addr string) (session.DataSession, error) {
		return &mockSession{}, nil
	})
	w := postJSON(t, handler, map[string]string{"ip": "127.0.0.1"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp connectResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.SessionID == "" {
		t.Fatal("expected session_id in response")
	}
}

func TestHandleLoad_FileNotFound(t *testing.T) {
	reg := &Registry{entries: make(map[string]*entry)}
	handler := HandleLoad(reg)
	w := postJSON(t, handler, map[string]string{"path": "/nonexistent/file.wpilog"})
	if w.Code == http.StatusOK {
		t.Fatal("expected error for missing file")
	}
	var errResp errorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "INVALID_LOG" {
		t.Errorf("expected INVALID_LOG, got %q", errResp.Code)
	}
}

func TestHandleDisconnect_Success(t *testing.T) {
	reg, id, s := testRegistry(t)
	handler := HandleDisconnect(reg)
	w := postJSON(t, handler, map[string]string{"session_id": id})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	if !s.closed {
		t.Fatal("expected session to be closed")
	}
}

func TestHandleDisconnect_NotFound(t *testing.T) {
	reg := &Registry{entries: make(map[string]*entry)}
	handler := HandleDisconnect(reg)
	w := postJSON(t, handler, map[string]string{"session_id": "missing"})
	if w.Code == http.StatusOK {
		t.Fatal("expected error")
	}
	var errResp errorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "SESSION_NOT_FOUND" {
		t.Errorf("expected SESSION_NOT_FOUND, got %q", errResp.Code)
	}
}

func TestHandleInfo_Success(t *testing.T) {
	reg, id, _ := testRegistry(t)
	handler := HandleInfo(reg)
	req := httptest.NewRequest(http.MethodGet, "/info?session="+id, nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
}

func TestHandleSet_OnLogSession_ReturnsError(t *testing.T) {
	reg, id, _ := testRegistry(t)
	handler := HandleSet(reg)
	w := postJSON(t, handler, setRequest{
		SessionID: id,
		Pairs:     map[string]any{"/key": 1.0},
	})
	if w.Code == http.StatusOK {
		t.Fatal("expected error: log sessions are read-only")
	}
	var errResp errorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "READ_ONLY_SESSION" {
		t.Errorf("expected READ_ONLY_SESSION, got %q", errResp.Code)
	}
}
```

- [ ] **Step 2: Run to confirm compile error**

```bash
cd ClaudeScope && go test ./daemon/... -run TestHandle -v 2>&1 | head -10
```

Expected: compile error.

- [ ] **Step 3: Create `ClaudeScope/daemon/handlers.go`**

```go
package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/rylero/TheFRCSuite/ClaudeScope/session"
)

// NTSessionFactory creates a live NT session. Injected so tests can mock it.
type NTSessionFactory func(addr string) (session.DataSession, error)

// --- JSON types ---

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type connectRequest struct {
	IP string `json:"ip"`
}

type connectResponse struct {
	SessionID string `json:"session_id"`
}

type loadRequest struct {
	Path string `json:"path"`
}

type disconnectRequest struct {
	SessionID string `json:"session_id"`
}

type getRequest struct {
	SessionID string   `json:"session_id"`
	Keys      []string `json:"keys"`
	Time      int64    `json:"time"`
}

type rangeRequest struct {
	SessionID string   `json:"session_id"`
	Keys      []string `json:"keys"`
	Start     int64    `json:"start"`
	End       int64    `json:"end"`
}

type findBoolRequest struct {
	SessionID string `json:"session_id"`
	Key       string `json:"key"`
	Value     bool   `json:"value"`
}

type findThresholdRequest struct {
	SessionID string  `json:"session_id"`
	Key       string  `json:"key"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
}

type statsRequest struct {
	SessionID string `json:"session_id"`
	Key       string `json:"key"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
}

type setRequest struct {
	SessionID string         `json:"session_id"`
	Pairs     map[string]any `json:"pairs"`
}

// --- helpers ---

func writeError(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: msg, Code: code})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func decodeBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func sessionNotFound(w http.ResponseWriter, id string) {
	writeError(w, http.StatusNotFound,
		fmt.Sprintf("session not found: %s", id), "SESSION_NOT_FOUND")
}

// --- handlers ---

// HandlePing responds 200 OK so the CLI can detect a running daemon.
func HandlePing(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]bool{"ok": true})
}

// HandleConnect starts a live NT session.
func HandleConnect(reg *Registry, factory NTSessionFactory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req connectRequest
		if err := decodeBody(r, &req); err != nil || req.IP == "" {
			writeError(w, http.StatusBadRequest, "missing ip field", "BAD_REQUEST")
			return
		}
		sess, err := factory(req.IP)
		if err != nil {
			writeError(w, http.StatusBadGateway,
				fmt.Sprintf("connect failed: %v", err), "CONNECT_FAILED")
			return
		}
		id := reg.Add(sess)
		writeJSON(w, connectResponse{SessionID: id})
	}
}

// HandleLoad parses a .wpilog file and creates a log session.
func HandleLoad(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loadRequest
		if err := decodeBody(r, &req); err != nil || req.Path == "" {
			writeError(w, http.StatusBadRequest, "missing path field", "BAD_REQUEST")
			return
		}
		data, err := os.ReadFile(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("cannot read file: %v", err), "INVALID_LOG")
			return
		}
		sess, err := session.ParseWPILog(data)
		if err != nil {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("invalid wpilog: %v", err), "INVALID_LOG")
			return
		}
		id := reg.Add(sess)
		writeJSON(w, connectResponse{SessionID: id})
	}
}

// HandleDisconnect closes and removes a session.
func HandleDisconnect(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req disconnectRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
			return
		}
		if err := reg.Remove(req.SessionID); err != nil {
			sessionNotFound(w, req.SessionID)
			return
		}
		writeJSON(w, struct{}{})
	}
}

// HandleInfo returns field list and time range for a session.
func HandleInfo(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("session")
		sess, err := reg.Get(id)
		if err != nil {
			sessionNotFound(w, id)
			return
		}
		fields, err := sess.Fields()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
			return
		}
		start, end, err := sess.TimeRange()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
			return
		}
		writeJSON(w, map[string]any{
			"fields": fields,
			"start":  start,
			"end":    end,
		})
	}
}

// HandleGet returns values for one or more keys at a timestamp.
func HandleGet(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req getRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
			return
		}
		sess, err := reg.Get(req.SessionID)
		if err != nil {
			sessionNotFound(w, req.SessionID)
			return
		}
		result, err := sess.GetValues(req.Keys, req.Time)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error(), "KEY_NOT_FOUND")
			return
		}
		writeJSON(w, result)
	}
}

// HandleRange returns time-series data for one or more keys.
func HandleRange(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req rangeRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
			return
		}
		sess, err := reg.Get(req.SessionID)
		if err != nil {
			sessionNotFound(w, req.SessionID)
			return
		}
		result, err := sess.GetRanges(req.Keys, req.Start, req.End)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error(), "KEY_NOT_FOUND")
			return
		}
		writeJSON(w, result)
	}
}

// HandleFindBool returns time ranges where a bool field matches a value.
func HandleFindBool(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req findBoolRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
			return
		}
		sess, err := reg.Get(req.SessionID)
		if err != nil {
			sessionNotFound(w, req.SessionID)
			return
		}
		ranges, err := sess.FindBoolRanges(req.Key, req.Value)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error(), "KEY_NOT_FOUND")
			return
		}
		writeJSON(w, ranges)
	}
}

// HandleFindThreshold returns time ranges where a numeric field is within [min, max].
func HandleFindThreshold(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req findThresholdRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
			return
		}
		sess, err := reg.Get(req.SessionID)
		if err != nil {
			sessionNotFound(w, req.SessionID)
			return
		}
		ranges, err := sess.FindThresholdRanges(req.Key, req.Min, req.Max)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error(), "KEY_NOT_FOUND")
			return
		}
		writeJSON(w, ranges)
	}
}

// HandleStats returns descriptive statistics for a numeric field.
func HandleStats(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req statsRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
			return
		}
		sess, err := reg.Get(req.SessionID)
		if err != nil {
			sessionNotFound(w, req.SessionID)
			return
		}
		stats, err := sess.Stats(req.Key, req.Start, req.End)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error(), "KEY_NOT_FOUND")
			return
		}
		writeJSON(w, stats)
	}
}

// HandleSet publishes key/value pairs to a live NT session.
func HandleSet(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req setRequest
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
			return
		}
		sess, err := reg.Get(req.SessionID)
		if err != nil {
			sessionNotFound(w, req.SessionID)
			return
		}
		if err := sess.Set(req.Pairs); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "READ_ONLY_SESSION")
			return
		}
		writeJSON(w, struct{}{})
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd ClaudeScope && go test ./daemon/... -run TestHandle -v
```

Expected:
```
--- PASS: TestHandlePing (0.00s)
--- PASS: TestHandleConnect_MissingIP (0.00s)
--- PASS: TestHandleConnect_FactoryError (0.00s)
--- PASS: TestHandleConnect_Success (0.00s)
--- PASS: TestHandleLoad_FileNotFound (0.00s)
--- PASS: TestHandleDisconnect_Success (0.00s)
--- PASS: TestHandleDisconnect_NotFound (0.00s)
--- PASS: TestHandleInfo_Success (0.00s)
--- PASS: TestHandleSet_OnLogSession_ReturnsError (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add daemon/handlers.go daemon/handlers_test.go
git commit -m "feat: add HTTP handlers for all ClaudeScope commands"
```

---

## Task 7: `daemon/server.go` — HTTP server and route registration

**Files:**
- Create: `ClaudeScope/daemon/server.go`

- [ ] **Step 1: Create `ClaudeScope/daemon/server.go`**

```go
package daemon

import (
	"fmt"
	"net/http"
)

const DaemonAddr = "localhost:5812"

// NewServer builds the HTTP mux with all routes registered.
// factory is called when a connect command arrives.
func NewServer(reg *Registry, factory NTSessionFactory) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", HandlePing)
	mux.HandleFunc("POST /connect", HandleConnect(reg, factory))
	mux.HandleFunc("POST /load", HandleLoad(reg))
	mux.HandleFunc("POST /disconnect", HandleDisconnect(reg))
	mux.HandleFunc("GET /info", HandleInfo(reg))
	mux.HandleFunc("POST /get", HandleGet(reg))
	mux.HandleFunc("POST /range", HandleRange(reg))
	mux.HandleFunc("POST /find-bool", HandleFindBool(reg))
	mux.HandleFunc("POST /find-threshold", HandleFindThreshold(reg))
	mux.HandleFunc("POST /stats", HandleStats(reg))
	mux.HandleFunc("POST /set", HandleSet(reg))
	return mux
}

// Run starts the daemon HTTP server. It blocks until the server exits.
func Run(reg *Registry, factory NTSessionFactory) error {
	mux := NewServer(reg, factory)
	fmt.Printf("ClaudeScope daemon listening on %s\n", DaemonAddr)
	return http.ListenAndServe(DaemonAddr, mux)
}
```

- [ ] **Step 2: Verify build**

```bash
cd ClaudeScope && go build ./...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add daemon/server.go
git commit -m "feat: add daemon HTTP server with route registration"
```

---

## Task 8: `cli/client.go` — HTTP client and daemon auto-start

**Files:**
- Create: `ClaudeScope/cli/client.go`
- Create: `ClaudeScope/cli/client_test.go`

- [ ] **Step 1: Write failing tests**

Create `ClaudeScope/cli/client_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// patchAddr temporarily overrides the daemon address for tests.
func patchAddr(t *testing.T, addr string) {
	t.Helper()
	old := daemonBaseURL
	daemonBaseURL = addr
	t.Cleanup(func() { daemonBaseURL = old })
}

func TestPingDaemon_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer srv.Close()
	patchAddr(t, srv.URL)

	if !PingDaemon() {
		t.Fatal("expected PingDaemon to return true")
	}
}

func TestPingDaemon_Down(t *testing.T) {
	// Point at a port nothing is listening on.
	patchAddr(t, "http://127.0.0.1:19999")
	if PingDaemon() {
		t.Fatal("expected PingDaemon to return false for closed port")
	}
}

func TestDoRequest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"session_id": "abc123"})
	}))
	defer srv.Close()
	patchAddr(t, srv.URL)

	body, err := DoRequest(http.MethodPost, "/connect", map[string]string{"ip": "127.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]string
	json.Unmarshal(body, &resp)
	if resp["session_id"] != "abc123" {
		t.Errorf("expected abc123, got %q", resp["session_id"])
	}
}

func TestDoRequest_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "session not found", "code": "SESSION_NOT_FOUND"})
	}))
	defer srv.Close()
	patchAddr(t, srv.URL)

	_, err := DoRequest(http.MethodPost, "/get", map[string]string{"session_id": "bad"})
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}
```

- [ ] **Step 2: Run to confirm compile error**

```bash
cd ClaudeScope && go test ./cli/... -run TestPing -v 2>&1 | head -10
```

Expected: compile error.

- [ ] **Step 3: Create `ClaudeScope/cli/client.go`**

```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// daemonBaseURL is the base URL for the daemon. Overridable in tests.
var daemonBaseURL = "http://localhost:5812"

var httpClient = &http.Client{Timeout: 10 * time.Second}

// PingDaemon returns true if the daemon is reachable.
func PingDaemon() bool {
	resp, err := httpClient.Get(daemonBaseURL + "/ping")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// EnsureDaemon pings the daemon; if unreachable, spawns it and retries.
// Returns an error if the daemon is still unreachable after retry.
func EnsureDaemon() error {
	if PingDaemon() {
		return nil
	}

	if err := spawnDaemon(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Retry with backoff: 200ms, 400ms, 800ms, 1600ms (total ~3s)
	delay := 200 * time.Millisecond
	for i := 0; i < 4; i++ {
		time.Sleep(delay)
		if PingDaemon() {
			return nil
		}
		delay *= 2
	}
	return fmt.Errorf("daemon unavailable after start attempt")
}

// spawnDaemon launches ClaudeScope --daemon as a detached background process.
func spawnDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	cmd := exec.Command(exe, "--daemon")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	setSysProcAttr(cmd) // platform-specific detach (see spawn_*.go)
	return cmd.Start()
}

// DoRequest sends a JSON request to the daemon and returns the raw response body.
// Returns an error if the HTTP status is not 2xx.
func DoRequest(method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, daemonBaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daemon request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", string(data))
	}
	return data, nil
}
```

- [ ] **Step 4: Create `ClaudeScope/cli/spawn_windows.go`**

```go
//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
```

- [ ] **Step 5: Create `ClaudeScope/cli/spawn_unix.go`**

```go
//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
```

- [ ] **Step 6: Run tests**

```bash
cd ClaudeScope && go test ./cli/... -run "TestPingDaemon|TestDoRequest" -v
```

Expected:
```
--- PASS: TestPingDaemon_Up (0.00s)
--- PASS: TestPingDaemon_Down (0.00s)
--- PASS: TestDoRequest_Success (0.00s)
--- PASS: TestDoRequest_ErrorResponse (0.00s)
PASS
```

- [ ] **Step 7: Commit**

```bash
git add cli/client.go cli/spawn_windows.go cli/spawn_unix.go cli/client_test.go
git commit -m "feat: add CLI HTTP client and daemon auto-start"
```

---

## Task 9: `cli/commands.go` — argument parsing and JSON output

**Files:**
- Create: `ClaudeScope/cli/commands.go`
- Create: `ClaudeScope/cli/commands_test.go`

- [ ] **Step 1: Write failing command tests**

Create `ClaudeScope/cli/commands_test.go`:

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// serveFake starts a fake daemon for command tests.
// responses maps path → JSON body (status 200).
func serveFake(t *testing.T, responses map[string]any) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(v)
	}))
	t.Cleanup(srv.Close)
	patchAddr(t, srv.URL)
}

func TestRunCommand_Connect(t *testing.T) {
	serveFake(t, map[string]any{
		"/connect": map[string]string{"session_id": "abc"},
	})
	out, err := RunCommand([]string{"connect", "10.0.0.2"})
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	json.Unmarshal(out, &resp)
	if resp["session_id"] != "abc" {
		t.Errorf("expected abc, got %q", resp["session_id"])
	}
}

func TestRunCommand_Load(t *testing.T) {
	serveFake(t, map[string]any{
		"/load": map[string]string{"session_id": "xyz"},
	})
	out, err := RunCommand([]string{"load", "/path/to/file.wpilog"})
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	json.Unmarshal(out, &resp)
	if resp["session_id"] != "xyz" {
		t.Errorf("expected xyz, got %q", resp["session_id"])
	}
}

func TestRunCommand_Disconnect(t *testing.T) {
	serveFake(t, map[string]any{"/disconnect": struct{}{}})
	_, err := RunCommand([]string{"disconnect", "--session", "abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCommand_Info(t *testing.T) {
	serveFake(t, map[string]any{
		"/info": map[string]any{
			"fields": []map[string]string{{"key": "/voltage", "type": "double"}},
			"start":  0,
			"end":    3000,
		},
	})
	out, err := RunCommand([]string{"info", "--session", "abc"})
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	json.Unmarshal(out, &resp)
	if resp["end"] == nil {
		t.Error("expected end in info response")
	}
}

func TestRunCommand_Get(t *testing.T) {
	serveFake(t, map[string]any{
		"/get": map[string]any{
			"/voltage": map[string]any{"timestamp": 1000, "value": 12.0},
		},
	})
	out, err := RunCommand([]string{"get", "/voltage", "--session", "abc"})
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	json.Unmarshal(out, &resp)
	if resp["/voltage"] == nil {
		t.Error("expected /voltage in get response")
	}
}

func TestRunCommand_MissingSession(t *testing.T) {
	_, err := RunCommand([]string{"get", "/voltage"})
	if err == nil {
		t.Fatal("expected error for missing --session flag")
	}
}

func TestRunCommand_UnknownSubcommand(t *testing.T) {
	_, err := RunCommand([]string{"frobnicate"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

func TestRunCommand_Stats(t *testing.T) {
	serveFake(t, map[string]any{
		"/stats": map[string]any{"mean": 11.5, "min": 11.0, "max": 12.0},
	})
	out, err := RunCommand([]string{"stats", "/voltage", "--session", "abc"})
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	json.Unmarshal(out, &resp)
	if resp["mean"] == nil {
		t.Error("expected mean in stats response")
	}
}

func TestRunCommand_FindBool(t *testing.T) {
	serveFake(t, map[string]any{
		"/find-bool": []map[string]any{{"start": 1000, "end": 2500}},
	})
	out, err := RunCommand([]string{"find-bool", "/enabled", "true", "--session", "abc"})
	if err != nil {
		t.Fatal(err)
	}
	var resp []any
	json.Unmarshal(out, &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 range, got %d", len(resp))
	}
}

func TestRunCommand_FindThreshold(t *testing.T) {
	serveFake(t, map[string]any{
		"/find-threshold": []map[string]any{{"start": 2000, "end": 3000}},
	})
	out, err := RunCommand([]string{"find-threshold", "/voltage", "--min", "11.0", "--max", "11.5", "--session", "abc"})
	if err != nil {
		t.Fatal(err)
	}
	var resp []any
	json.Unmarshal(out, &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 range, got %d", len(resp))
	}
}

func TestRunCommand_Set(t *testing.T) {
	serveFake(t, map[string]any{"/set": struct{}{}})
	_, err := RunCommand([]string{"set", "/key=1.0", "--session", "abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run to confirm compile error**

```bash
cd ClaudeScope && go test ./cli/... -run TestRunCommand -v 2>&1 | head -10
```

Expected: compile error — `RunCommand` undefined.

- [ ] **Step 3: Create `ClaudeScope/cli/commands.go`**

```go
package cli

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// RunCommand routes the CLI args to the correct daemon endpoint.
// Returns the raw JSON response body, or an error.
func RunCommand(args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no subcommand provided")
	}

	switch args[0] {
	case "connect":
		return runConnect(args[1:])
	case "load":
		return runLoad(args[1:])
	case "disconnect":
		return runDisconnect(args[1:])
	case "info":
		return runInfo(args[1:])
	case "get":
		return runGet(args[1:])
	case "range":
		return runRange(args[1:])
	case "find-bool":
		return runFindBool(args[1:])
	case "find-threshold":
		return runFindThreshold(args[1:])
	case "stats":
		return runStats(args[1:])
	case "set":
		return runSet(args[1:])
	default:
		return nil, fmt.Errorf("unknown subcommand: %s", args[0])
	}
}

// --- flag helpers ---

// parseFlags extracts named flags from args, returning positional args and the flag map.
// Flags have the form --name value. Boolean flags are not supported (all flags take a value).
func parseFlags(args []string) (positional []string, flags map[string]string) {
	flags = make(map[string]string)
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") && i+1 < len(args) {
			key := strings.TrimPrefix(args[i], "--")
			flags[key] = args[i+1]
			i++ // consume value
		} else {
			positional = append(positional, args[i])
		}
	}
	return
}

func requireSession(flags map[string]string) (string, error) {
	id, ok := flags["session"]
	if !ok || id == "" {
		return "", fmt.Errorf("--session <id> is required")
	}
	return id, nil
}

func flagInt64(flags map[string]string, key string, defaultVal int64) int64 {
	s, ok := flags[key]
	if !ok {
		return defaultVal
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func flagFloat64(flags map[string]string, key string) (float64, error) {
	s, ok := flags[key]
	if !ok {
		return 0, fmt.Errorf("missing required flag --%s", key)
	}
	return strconv.ParseFloat(s, 64)
}

// --- subcommand implementations ---

func runConnect(args []string) ([]byte, error) {
	pos, _ := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: connect <ip>")
	}
	return DoRequest(http.MethodPost, "/connect", map[string]string{"ip": pos[0]})
}

func runLoad(args []string) ([]byte, error) {
	pos, _ := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: load <path.wpilog>")
	}
	return DoRequest(http.MethodPost, "/load", map[string]string{"path": pos[0]})
}

func runDisconnect(args []string) ([]byte, error) {
	_, flags := parseFlags(args)
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/disconnect", map[string]string{"session_id": id})
}

func runInfo(args []string) ([]byte, error) {
	_, flags := parseFlags(args)
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodGet, "/info?session="+id, nil)
}

func runGet(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: get <key> [key2 ...] --session <id> [--time <us>]")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	t := flagInt64(flags, "time", 0)
	return DoRequest(http.MethodPost, "/get", map[string]any{
		"session_id": id,
		"keys":       pos,
		"time":       t,
	})
}

func runRange(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: range <key> [key2 ...] --session <id> [--start <us>] [--end <us>]")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	start := flagInt64(flags, "start", 0)
	end := flagInt64(flags, "end", 0)
	return DoRequest(http.MethodPost, "/range", map[string]any{
		"session_id": id,
		"keys":       pos,
		"start":      start,
		"end":        end,
	})
}

func runFindBool(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 2 {
		return nil, fmt.Errorf("usage: find-bool <key> <true|false> --session <id>")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	value := pos[1] == "true"
	return DoRequest(http.MethodPost, "/find-bool", map[string]any{
		"session_id": id,
		"key":        pos[0],
		"value":      value,
	})
}

func runFindThreshold(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: find-threshold <key> --min <n> --max <n> --session <id>")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	minVal, err := flagFloat64(flags, "min")
	if err != nil {
		return nil, err
	}
	maxVal, err := flagFloat64(flags, "max")
	if err != nil {
		return nil, err
	}
	return DoRequest(http.MethodPost, "/find-threshold", map[string]any{
		"session_id": id,
		"key":        pos[0],
		"min":        minVal,
		"max":        maxVal,
	})
}

func runStats(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: stats <key> --session <id> [--start <us>] [--end <us>]")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	start := flagInt64(flags, "start", 0)
	end := flagInt64(flags, "end", 0)
	return DoRequest(http.MethodPost, "/stats", map[string]any{
		"session_id": id,
		"key":        pos[0],
		"start":      start,
		"end":        end,
	})
}

func runSet(args []string) ([]byte, error) {
	pos, flags := parseFlags(args)
	if len(pos) < 1 {
		return nil, fmt.Errorf("usage: set <key>=<val> [key2=val2 ...] --session <id>")
	}
	id, err := requireSession(flags)
	if err != nil {
		return nil, err
	}
	pairs := make(map[string]any, len(pos))
	for _, kv := range pos {
		idx := strings.Index(kv, "=")
		if idx < 0 {
			return nil, fmt.Errorf("invalid key=value pair: %q", kv)
		}
		key := kv[:idx]
		val := kv[idx+1:]
		// Try to parse as float first, fall back to string
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			pairs[key] = f
		} else if b, err := strconv.ParseBool(val); err == nil {
			pairs[key] = b
		} else {
			pairs[key] = val
		}
	}
	return DoRequest(http.MethodPost, "/set", map[string]any{
		"session_id": id,
		"pairs":      pairs,
	})
}
```

- [ ] **Step 4: Run tests**

```bash
cd ClaudeScope && go test ./cli/... -run TestRunCommand -v
```

Expected:
```
--- PASS: TestRunCommand_Connect (0.00s)
--- PASS: TestRunCommand_Load (0.00s)
--- PASS: TestRunCommand_Disconnect (0.00s)
--- PASS: TestRunCommand_Info (0.00s)
--- PASS: TestRunCommand_Get (0.00s)
--- PASS: TestRunCommand_MissingSession (0.00s)
--- PASS: TestRunCommand_UnknownSubcommand (0.00s)
--- PASS: TestRunCommand_Stats (0.00s)
--- PASS: TestRunCommand_FindBool (0.00s)
--- PASS: TestRunCommand_FindThreshold (0.00s)
--- PASS: TestRunCommand_Set (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add cli/commands.go cli/commands_test.go
git commit -m "feat: add CLI command routing for all ClaudeScope subcommands"
```

---

## Task 10: `main.go` — entry point

**Files:**
- Replace: `ClaudeScope/main.go`

- [ ] **Step 1: Replace `ClaudeScope/main.go`**

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	nt4 "github.com/levifitzpatrick1/go-nt4"
	"github.com/rylero/TheFRCSuite/ClaudeScope/cli"
	"github.com/rylero/TheFRCSuite/ClaudeScope/daemon"
	"github.com/rylero/TheFRCSuite/ClaudeScope/session"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 && args[0] == "--daemon" {
		runDaemon()
		return
	}

	if err := cli.EnsureDaemon(); err != nil {
		writeErrorAndExit(err.Error(), "DAEMON_UNAVAILABLE")
	}

	outFile := ""
	filteredArgs := args[:0]
	for i := 0; i < len(args); i++ {
		if args[i] == "--out" && i+1 < len(args) {
			outFile = args[i+1]
			i++
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	result, err := cli.RunCommand(filteredArgs)
	if err != nil {
		writeErrorAndExit(err.Error(), "COMMAND_FAILED")
	}

	if outFile != "" {
		if err := os.WriteFile(outFile, result, 0644); err != nil {
			writeErrorAndExit(fmt.Sprintf("cannot write output file: %v", err), "IO_ERROR")
		}
		return
	}

	os.Stdout.Write(result)
}

func runDaemon() {
	reg := daemon.NewRegistry()
	if err := daemon.Run(reg, ntSessionFactory); err != nil {
		fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
		os.Exit(1)
	}
}

func ntSessionFactory(addr string) (session.DataSession, error) {
	opts := nt4.DefaultClientOptions(addr)
	opts.Identity = "ClaudeScope"
	c := nt4.NewClient(opts)
	if err := c.Connect(); err != nil {
		return nil, err
	}
	return session.NewNTSessionFromClient(c), nil
}

func writeErrorAndExit(msg, code string) {
	json.NewEncoder(os.Stdout).Encode(map[string]string{
		"error": msg,
		"code":  code,
	})
	os.Exit(1)
}
```

> **Note:** `NewNTSessionFromClient` is a constructor you'll need to add to `session/nt_session.go` in step 2.

- [ ] **Step 2: Add `NewNTSessionFromClient` to `session/nt_session.go`**

Append to `session/nt_session.go`:

```go
// NewNTSessionFromClient wraps an already-connected nt4.Client.
// Used by main.go so the factory owns connection setup.
func NewNTSessionFromClient(c *nt4.Client) DataSession {
	s := &ntSession{
		client:      c,
		history:     make(map[string][]DataPoint),
		fieldMeta:   make(map[string]string),
		connectedAt: time.Now(),
	}
	sub := c.Subscribe([]string{"/"}, nt4.SubscribeOptions{Prefix: true})
	s.sub = sub
	go s.pump(sub)
	return s
}
```

- [ ] **Step 3: Build the full binary**

```bash
cd ClaudeScope && go build -o ClaudeScope.exe .
```

If the build fails due to `nt4.SubscribeOptions` or similar API mismatches, check the go-nt4 library:

```bash
grep -n "type.*Options\|func.*Subscribe\|func.*Publish\|func.*Unsubscribe" \
    ~/go/pkg/mod/github.com/levifitzpatrick1/go-nt4@v0.1.1/*.go
```

Adjust the calls to match the actual exported API.

- [ ] **Step 4: Run all tests**

```bash
cd ClaudeScope && go test ./... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Smoke test the daemon**

```bash
# Terminal 1: start daemon manually
./ClaudeScope.exe --daemon &

# Terminal 2: verify ping
./ClaudeScope.exe info --session nonexistent
# Expected: {"error":"session not found: nonexistent","code":"SESSION_NOT_FOUND"}

# Kill the daemon
kill %1
```

- [ ] **Step 6: Commit**

```bash
git add main.go session/nt_session.go
git commit -m "feat: wire up main.go entry point, daemon mode, and --out flag"
```

---

## Task 11: Final build verification and cleanup

- [ ] **Step 1: Run the full test suite**

```bash
cd ClaudeScope && go test ./... -v
```

Expected: all tests PASS, no skips except NT integration tests (which require a running robot/sim — those are not part of the automated suite).

- [ ] **Step 2: Build the Windows binary**

```bash
cd ClaudeScope && GOOS=windows GOARCH=amd64 go build -o ClaudeScope.exe .
```

Expected: `ClaudeScope.exe` produced, no errors.

- [ ] **Step 3: Final commit**

```bash
git add ClaudeScope/
git commit -m "feat: complete ClaudeScope daemon+CLI — all tests passing"
```

---

## Self-Review

### Spec coverage

| Spec requirement | Task |
|---|---|
| `connect <ip>` → `{"session_id": "..."}` | Task 6 (handler), Task 9 (command) |
| `load <path.wpilog>` → `{"session_id": "..."}` | Task 6, Task 9 |
| `disconnect --session <id>` | Task 6, Task 9 |
| `info --session <id>` | Task 6, Task 9 |
| `get <key> [keys] --session --time` | Task 6, Task 9 |
| `range <key> [keys] --session --start --end` | Task 6, Task 9 |
| `find-bool <key> <true\|false> --session` | Task 6, Task 9 |
| `find-threshold <key> --min --max --session` | Task 6, Task 9 |
| `stats <key> --session --start --end` | Task 6, Task 9 |
| `set <key>=<val> --session` | Task 6, Task 9 |
| `--out <file>` flag on all commands | Task 10 |
| Negative `--start`/`--end` offsets | Task 3 (`GetRanges`) |
| Daemon auto-start with retry backoff | Task 8 |
| Session expiry after 10 minutes | Task 5 |
| Expiry sweep every 60s | Task 5 |
| Consistent error JSON `{"error":"...","code":"..."}` | Task 6 |
| All error codes from spec | Task 6 |
| `DataSession` interface | Task 2 |
| `NTSession` wrapping go-nt4 | Task 4 |
| `LogSession` parsing `.wpilog` | Task 3 |
| Stats: mean, median, min, max, Q1, Q3, AvgDelta, MinDelta, MaxDelta | Task 3 |
| Binary format: varint encoding | Task 3 |
| `Set` returns error on log sessions | Task 3 (tested), Task 6 (`READ_ONLY_SESSION`) |
| daemon listens on `localhost:5812` | Task 7 |
| GET /ping for health check | Task 6, Task 8 |

### Placeholder scan

No TBD/TODO/placeholder text. All steps contain complete Go code, exact commands, and expected output.

### Type consistency

All method signatures match `DataSession` interface defined in Task 2:
- `ParseWPILog` → `*wpilogSession` (implements `DataSession`) — Task 3
- `NewNTSession` / `NewNTSessionFromClient` → `DataSession` — Task 4, Task 10
- `Registry.Add(session.DataSession) string` — Task 5, referenced in Task 6
- `HandleConnect(reg, NTSessionFactory)` — Tasks 6 and 7
- `RunCommand([]string) ([]byte, error)` — Tasks 8 and 9
- `DoRequest(method, path, body) ([]byte, error)` — Task 8, referenced in Task 9
- `daemonBaseURL` — exported in Task 8 as package-level var, used in both `client_test.go` and `commands_test.go` via `patchAddr`