package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  connect <ip>   Connect to a robot or simulation (sim: 127.0.0.1, robot: 10.TE.AM.2)")
	fmt.Fprintln(out, "  disconnect     Disconnect from the current robot connection")
	fmt.Fprintln(out, "  status         Show connection status")
	fmt.Fprintln(out, "  help           Show this help message")
	fmt.Fprintln(out, "  exit           Exit ClaudeScope")
}

// RunREPL reads commands from in, writes responses to out.
// Returns when input is exhausted or an exit/quit command is received.
func RunREPL(in io.Reader, out io.Writer, session *RobotSession) {
	scanner := bufio.NewScanner(in)

	for {
		if session.connected() {
			fmt.Fprint(out, "[connected] > ")
		} else {
			fmt.Fprint(out, "> ")
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
				fmt.Fprintln(out, "Usage: connect <ip>")
				continue
			}
			if err := session.connect(args[0]); err != nil {
				fmt.Fprintf(out, "Error: %v\n", err)
			} else {
				fmt.Fprintf(out, "Connected to %s\n", args[0])
			}

		case "disconnect":
			if err := session.disconnect(); err != nil {
				fmt.Fprintf(out, "Error: %v\n", err)
			} else {
				fmt.Fprintln(out, "Disconnected.")
			}

		case "status":
			if session.connected() {
				fmt.Fprintln(out, "Status: connected")
			} else {
				fmt.Fprintln(out, "Status: disconnected")
			}

		case "help":
			printHelp(out)

		case "exit", "quit":
			if session.connected() {
				_ = session.disconnect()
			}
			fmt.Fprintln(out, "Goodbye.")
			return

		default:
			fmt.Fprintf(out, "Unknown command: %s (type 'help' for usage)\n", cmd)
		}
	}
}
