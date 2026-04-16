package daemon

import (
	"errors"
	"testing"
	"time"

	"github.com/rylero/TheFRCSuite/ClaudeScope/session"
)

// mockSession is a do-nothing DataSession for registry tests.
type mockSession struct{ closed bool }

func (m *mockSession) Type() session.SessionType                                         { return session.LogSession }
func (m *mockSession) Fields() ([]session.FieldInfo, error)                              { return nil, nil }
func (m *mockSession) TimeRange() (int64, int64, error)                                  { return 0, 0, nil }
func (m *mockSession) GetValues([]string, int64) (map[string]*session.DataPoint, error)  { return nil, nil }
func (m *mockSession) GetRanges([]string, int64, int64) (map[string][]session.DataPoint, error) {
	return nil, nil
}
func (m *mockSession) FindBoolRanges(string, bool) ([]session.TimeRange, error)             { return nil, nil }
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
