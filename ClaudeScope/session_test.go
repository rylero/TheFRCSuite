package main

import (
	"errors"
	"testing"
)

// mockNTClient is an NTClient that records calls without opening a socket.
type mockNTClient struct {
	connectErr       error
	disconnectCalled bool
}

func (m *mockNTClient) Connect() error {
	return m.connectErr
}

func (m *mockNTClient) Disconnect() {
	m.disconnectCalled = true
}

// mockFactory returns a mockNTClient. If connectErr is non-nil, Connect() fails.
func mockFactory(connectErr error) NTClientFactory {
	return func(ip string) (NTClient, error) {
		c := &mockNTClient{connectErr: connectErr}
		if err := c.Connect(); err != nil {
			return nil, err
		}
		return c, nil
	}
}

func TestConnect_Success(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	if err := session.connect("127.0.0.1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !session.connected() {
		t.Fatal("expected session to be connected")
	}
}

func TestConnect_AlreadyConnected(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	_ = session.connect("127.0.0.1")
	err := session.connect("127.0.0.1")
	if err == nil {
		t.Fatal("expected error for double connect, got nil")
	}
	if err.Error() != "already connected; run 'disconnect' first" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestConnect_FactoryError(t *testing.T) {
	factoryErr := errors.New("connection refused")
	session := NewRobotSession(mockFactory(factoryErr))
	err := session.connect("127.0.0.1")
	if err == nil {
		t.Fatal("expected error from failed factory, got nil")
	}
}

func TestDisconnect_Success(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	_ = session.connect("127.0.0.1")
	if err := session.disconnect(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if session.connected() {
		t.Fatal("expected session to be disconnected")
	}
}

func TestDisconnect_NotConnected(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	err := session.disconnect()
	if err == nil {
		t.Fatal("expected error when disconnecting with no active connection")
	}
	if err.Error() != "no active connection" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestConnected_InitiallyFalse(t *testing.T) {
	session := NewRobotSession(mockFactory(nil))
	if session.connected() {
		t.Fatal("new session should not be connected")
	}
}
