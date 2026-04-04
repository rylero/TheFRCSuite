package main

import (
	"fmt"
	"os"

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

func main() {
	fmt.Println("ClaudeScope v1.0.0")
	fmt.Println("Type 'help' for available commands.")
	fmt.Println()

	session := NewRobotSession(realFactory)
	RunREPL(os.Stdin, os.Stdout, session)
}
