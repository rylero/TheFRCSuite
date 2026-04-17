package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
