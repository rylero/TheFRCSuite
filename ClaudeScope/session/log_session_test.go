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
	if s.Type() != LogSession {
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
