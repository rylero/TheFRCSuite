package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/levifitzpatrick1/go-nt4"
)

// realNTClient wraps go-nt4's client to satisfy the NTClient interface.
type realNTClient struct{ c *nt4.Client }

func (r *realNTClient) Connect() error { return r.c.Connect() }
func (r *realNTClient) Disconnect()    { r.c.Disconnect() }

func realFactory(ip string) (NTClient, error) {
	opts := nt4.DefaultClientOptions(ip)
	opts.Identity = "ClaudeScope"
	c := nt4.NewClient(opts)
	if err := c.Connect(); err != nil {
		return nil, err
	}
	return &realNTClient{c}, nil
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

	session := NewRobotSession(realFactory)
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
