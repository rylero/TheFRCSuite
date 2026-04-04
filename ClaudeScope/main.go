package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/levifitzpatrick1/go-nt4"
)

type RobotSession struct {
	ntClient *nt4.Client
	mu       sync.Mutex
}

func (s *RobotSession) connect(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ntClient != nil {
		return fmt.Errorf("already connected; run 'disconnect' first")
	}

	opts := nt4.DefaultClientOptions(ip)
	opts.Identity = "ClaudeScope"
	client := nt4.NewClient(opts)

	if err := client.Connect(); err != nil {
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

func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  connect <ip>   Connect to a robot or simulation (sim: 127.0.0.1, robot: 10.TE.AM.2)")
	fmt.Println("  disconnect     Disconnect from the current robot connection")
	fmt.Println("  status         Show connection status")
	fmt.Println("  help           Show this help message")
	fmt.Println("  exit           Exit ClaudeScope")
}

func main() {
	fmt.Println("ClaudeScope v1.0.0")
	fmt.Println("Type 'help' for available commands.")
	fmt.Println()

	session := &RobotSession{}
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if session.connected() {
			fmt.Print("[connected] > ")
		} else {
			fmt.Print("> ")
		}

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "connect":
			if len(args) != 1 {
				fmt.Println("Usage: connect <ip>")
				continue
			}
			if err := session.connect(args[0]); err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Connected to %s\n", args[0])
			}

		case "disconnect":
			if err := session.disconnect(); err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Disconnected.")
			}

		case "status":
			if session.connected() {
				fmt.Println("Status: connected")
			} else {
				fmt.Println("Status: disconnected")
			}

		case "help":
			printHelp()

		case "exit", "quit":
			if session.connected() {
				_ = session.disconnect()
			}
			fmt.Println("Goodbye.")
			return

		default:
			fmt.Printf("Unknown command: %s (type 'help' for usage)\n", cmd)
		}
	}
}
