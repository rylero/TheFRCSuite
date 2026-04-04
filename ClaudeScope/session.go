package main

import (
	"fmt"
	"sync"
)

// NTClient is the subset of go-nt4's Client that RobotSession needs.
// This interface lets tests inject a mock without opening a real connection.
type NTClient interface {
	Connect() error
	Disconnect()
}

// NTClientFactory creates an NTClient for a given IP address.
type NTClientFactory func(ip string) (NTClient, error)

// RobotSession holds a single live (or mock) NT4 connection.
type RobotSession struct {
	ntClient  NTClient
	mu        sync.Mutex
	newClient NTClientFactory
}

// NewRobotSession creates a session backed by the given factory.
func NewRobotSession(factory NTClientFactory) *RobotSession {
	return &RobotSession{newClient: factory}
}

func (s *RobotSession) connect(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ntClient != nil {
		return fmt.Errorf("already connected; run 'disconnect' first")
	}

	client, err := s.newClient(ip)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	s.ntClient = client
	return nil
}

func (s *RobotSession) disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ntClient == nil {
		return fmt.Errorf("no active connection")
	}

	s.ntClient.Disconnect()
	s.ntClient = nil
	return nil
}

func (s *RobotSession) connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ntClient != nil
}
