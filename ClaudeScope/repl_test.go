package main

import (
	"bytes"
	"strings"
	"testing"
)

// runREPLWithInput feeds the given commands to RunREPL and returns stdout as a string.
func runREPLWithInput(t *testing.T, session *RobotSession, commands string) string {
	t.Helper()
	in := strings.NewReader(commands)
	var out bytes.Buffer
	RunREPL(in, &out, session)
	return out.String()
}

func TestREPL_Status_Disconnected(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "status\n")
	if !strings.Contains(output, "Status: disconnected") {
		t.Fatalf("expected 'Status: disconnected' in output, got:\n%s", output)
	}
}

func TestREPL_Status_Connected(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "connect 127.0.0.1\nstatus\n")
	if !strings.Contains(output, "Status: connected") {
		t.Fatalf("expected 'Status: connected' in output, got:\n%s", output)
	}
}

func TestREPL_Connect_Success(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "connect 127.0.0.1\n")
	if !strings.Contains(output, "Connected to 127.0.0.1") {
		t.Fatalf("expected connection success message, got:\n%s", output)
	}
}

func TestREPL_Connect_MissingArg(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "connect\n")
	if !strings.Contains(output, "Usage: connect <ip>") {
		t.Fatalf("expected usage error, got:\n%s", output)
	}
}

func TestREPL_Connect_AlreadyConnected(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "connect 127.0.0.1\nconnect 127.0.0.1\n")
	if !strings.Contains(output, "already connected") {
		t.Fatalf("expected 'already connected' error, got:\n%s", output)
	}
}

func TestREPL_Disconnect_Success(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "connect 127.0.0.1\ndisconnect\n")
	if !strings.Contains(output, "Disconnected.") {
		t.Fatalf("expected 'Disconnected.' in output, got:\n%s", output)
	}
}

func TestREPL_Disconnect_NotConnected(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "disconnect\n")
	if !strings.Contains(output, "no active connection") {
		t.Fatalf("expected 'no active connection' error, got:\n%s", output)
	}
}

func TestREPL_Help(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "help\n")
	for _, expected := range []string{"connect", "disconnect", "status", "help", "exit"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("help output missing %q:\n%s", expected, output)
		}
	}
}

func TestREPL_Exit(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "exit\n")
	if !strings.Contains(output, "Goodbye.") {
		t.Fatalf("expected 'Goodbye.' in output, got:\n%s", output)
	}
}

func TestREPL_Quit(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "quit\n")
	if !strings.Contains(output, "Goodbye.") {
		t.Fatalf("expected 'Goodbye.' in output, got:\n%s", output)
	}
}

func TestREPL_Exit_DisconnectsFirst(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	_ = runREPLWithInput(t, session, "connect 127.0.0.1\nexit\n")
	if session.connected() {
		t.Fatal("session should be disconnected after exit")
	}
}

func TestREPL_UnknownCommand(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "frobulate\n")
	if !strings.Contains(output, "Unknown command: frobulate") {
		t.Fatalf("expected unknown command message, got:\n%s", output)
	}
}

func TestREPL_EmptyLinesIgnored(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	// Three blank lines then status — should not crash
	output := runREPLWithInput(t, session, "\n\n\nstatus\n")
	if !strings.Contains(output, "Status: disconnected") {
		t.Fatalf("expected status after blank lines, got:\n%s", output)
	}
}

func TestREPL_Prompt_ShowsConnectedState(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	output := runREPLWithInput(t, session, "connect 127.0.0.1\nstatus\n")
	if !strings.Contains(output, "[connected] >") {
		t.Fatalf("expected '[connected] >' prompt after connecting, got:\n%s", output)
	}
}
