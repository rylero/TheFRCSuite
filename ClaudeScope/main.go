package main

import (
	"encoding/json"
	"fmt"
	"os"

	nt4 "github.com/levifitzpatrick1/go-nt4"
	"github.com/rylero/TheFRCSuite/ClaudeScope/cli"
	"github.com/rylero/TheFRCSuite/ClaudeScope/daemon"
	"github.com/rylero/TheFRCSuite/ClaudeScope/session"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 && args[0] == "--daemon" {
		runDaemon()
		return
	}

	if err := cli.EnsureDaemon(); err != nil {
		writeErrorAndExit(err.Error(), "DAEMON_UNAVAILABLE")
	}

	// Strip --out <file> from args before routing
	outFile := ""
	filteredArgs := args[:0]
	for i := 0; i < len(args); i++ {
		if args[i] == "--out" && i+1 < len(args) {
			outFile = args[i+1]
			i++
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	result, err := cli.RunCommand(filteredArgs)
	if err != nil {
		writeErrorAndExit(err.Error(), "COMMAND_FAILED")
	}

	if outFile != "" {
		if err := os.WriteFile(outFile, result, 0644); err != nil {
			writeErrorAndExit(fmt.Sprintf("cannot write output file: %v", err), "IO_ERROR")
		}
		return
	}

	os.Stdout.Write(result)
}

func runDaemon() {
	reg := daemon.NewRegistry()
	if err := daemon.Run(reg, ntSessionFactory); err != nil {
		fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
		os.Exit(1)
	}
}

func ntSessionFactory(addr string) (session.DataSession, error) {
	opts := nt4.DefaultClientOptions(addr)
	opts.Identity = "ClaudeScope"
	c := nt4.NewClient(opts)
	if err := c.Connect(); err != nil {
		return nil, err
	}
	return session.NewNTSessionFromClient(c), nil
}

func writeErrorAndExit(msg, code string) {
	json.NewEncoder(os.Stdout).Encode(map[string]string{
		"error": msg,
		"code":  code,
	})
	os.Exit(1)
}
