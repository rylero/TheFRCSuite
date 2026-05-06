package session

import (
	"testing"
	"time"
)

// makeNTSession builds an ntSession with pre-populated history for testing,
// bypassing the real NT4 connection.
func makeNTSession(history map[string][]DataPoint) *ntSession {
	meta := make(map[string]string, len(history))
	for k := range history {
		meta[k] = "string"
	}
	return &ntSession{
		history:     history,
		fieldMeta:   meta,
		connectedAt: time.Now(),
	}
}

func TestNTSession_GetValues_Latest(t *testing.T) {
	s := makeNTSession(map[string][]DataPoint{
		"/state": {
			{Timestamp: 1000, Value: "IDLE"},
			{Timestamp: 3000, Value: "SHOOTING"},
			{Timestamp: 5000, Value: "IDLE"},
		},
	})
	pts, err := s.GetValues([]string{"/state"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if pts["/state"].Value.(string) != "IDLE" {
		t.Errorf("expected latest IDLE, got %v", pts["/state"].Value)
	}
}

func TestNTSession_GetValues_AtTimestamp(t *testing.T) {
	// t=3500 should return "SHOOTING" (last change at t=3000), not "IDLE" (t=5000).
	s := makeNTSession(map[string][]DataPoint{
		"/state": {
			{Timestamp: 1000, Value: "IDLE"},
			{Timestamp: 3000, Value: "SHOOTING"},
			{Timestamp: 5000, Value: "IDLE"},
		},
	})
	pts, err := s.GetValues([]string{"/state"}, 3500)
	if err != nil {
		t.Fatal(err)
	}
	if pts["/state"].Value.(string) != "SHOOTING" {
		t.Errorf("expected SHOOTING at t=3500, got %v", pts["/state"].Value)
	}
}

func TestNTSession_GetRanges_CarryOver(t *testing.T) {
	// Window starts at t=4000, after the last change at t=3000 ("SHOOTING").
	// Expect carry-over: SHOOTING returned even though no change in window.
	s := makeNTSession(map[string][]DataPoint{
		"/state": {
			{Timestamp: 1000, Value: "IDLE"},
			{Timestamp: 3000, Value: "SHOOTING"},
		},
	})
	ranges, err := s.GetRanges([]string{"/state"}, 4000, 6000)
	if err != nil {
		t.Fatal(err)
	}
	pts := ranges["/state"]
	if len(pts) != 1 {
		t.Fatalf("expected 1 carry-over point, got %d", len(pts))
	}
	if pts[0].Value.(string) != "SHOOTING" {
		t.Errorf("expected carry-over SHOOTING, got %v", pts[0].Value)
	}
}

func TestNTSession_CheckChooserActive_ErrorOnChooserActive(t *testing.T) {
	s := makeNTSession(map[string][]DataPoint{
		"/SmartDashboard/Auto Choices/.type":  {{Timestamp: 100, Value: "String Chooser"}},
		"/SmartDashboard/Auto Choices/active": {{Timestamp: 200, Value: "Option A"}},
	})
	err := s.checkChooserActive("/SmartDashboard/Auto Choices/active")
	if err == nil {
		t.Fatal("expected error when setting SendableChooser active field, got nil")
	}
}

func TestNTSession_CheckChooserActive_OkForNonChooser(t *testing.T) {
	s := makeNTSession(map[string][]DataPoint{
		"/RealOutputs/Drive/active": {{Timestamp: 100, Value: 42.0}},
	})
	if err := s.checkChooserActive("/RealOutputs/Drive/active"); err != nil {
		t.Errorf("expected no error for non-chooser active field, got %v", err)
	}
}

func TestNTSession_CheckChooserActive_OkForNonActiveField(t *testing.T) {
	s := makeNTSession(map[string][]DataPoint{
		"/SmartDashboard/Auto Choices/.type": {{Timestamp: 100, Value: "String Chooser"}},
	})
	if err := s.checkChooserActive("/SmartDashboard/Auto Choices/options"); err != nil {
		t.Errorf("expected no error for non-active field, got %v", err)
	}
}

func TestNTSession_GetRanges_Normal(t *testing.T) {
	s := makeNTSession(map[string][]DataPoint{
		"/state": {
			{Timestamp: 1000, Value: "IDLE"},
			{Timestamp: 3000, Value: "SHOOTING"},
			{Timestamp: 5000, Value: "IDLE"},
		},
	})
	ranges, err := s.GetRanges([]string{"/state"}, 2000, 4000)
	if err != nil {
		t.Fatal(err)
	}
	pts := ranges["/state"]
	if len(pts) != 1 {
		t.Fatalf("expected 1 point in [2000,4000], got %d", len(pts))
	}
	if pts[0].Value.(string) != "SHOOTING" {
		t.Errorf("expected SHOOTING, got %v", pts[0].Value)
	}
}
