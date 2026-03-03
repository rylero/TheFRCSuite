package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/levifitzpatrick1/go-nt4"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// --- Session Implementation ---

type RobotSession struct {
	id            string
	notifChannel  chan mcp.JSONRPCNotification
	isInitialized bool

	ntClient *nt4.Client
	mu       sync.Mutex
}

func (s *RobotSession) SessionID() string                                   { return s.id }
func (s *RobotSession) NotificationChannel() chan<- mcp.JSONRPCNotification { return s.notifChannel }
func (s *RobotSession) Initialize()                                         { s.isInitialized = true }
func (s *RobotSession) Initialized() bool                                   { return s.isInitialized }

// --- Server Setup ---
func main() {
	s := server.NewMCPServer(
		"ClaudeScope",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	connectTool := mcp.NewTool("connect_nt",
		mcp.WithDescription("Connect to a specific FRC Robot/Simulation via IP. Robot ip adress will be 10.TE.AM.38 For simulation connect to 127.0.0.1"),
		mcp.WithString("ip", mcp.Required(), mcp.Description("RoboRIO or Sim IP")),
	)

	disconnectTool := mcp.NewTool("disconnect",
		mcp.WithDescription("Disconnect from the current log session"),
	)

	s.AddTool(connectTool, handleConnect)
	s.AddTool(disconnectTool, handleDisconnect)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// --- Handlers ---
func handleConnect(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return mcp.NewToolResultError("No active MCP session found"), nil
	}

	rs, ok := session.(*RobotSession)
	if !ok {
		return mcp.NewToolResultError("Session is not a valid RobotSession"), nil
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.ntClient != nil {
		return mcp.NewToolResultText("This session is already connected to a robot."), nil
	}

	args, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		return mcp.NewToolResultError("Missing or invalid arguments map"), nil
	}

	ip, ok := args["ip"].(string)
	if !ok || ip == "" {
		return mcp.NewToolResultError("A valid 'ip' string is required"), nil
	}

	opts := nt4.DefaultClientOptions(ip)
	opts.Identity = "ClaudeScope"
	rs.ntClient = nt4.NewClient(opts)

	if err := rs.ntClient.Connect(); err != nil {
		rs.ntClient = nil
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Session %s connected to %s", rs.id, ip)), nil
}

func handleDisconnect(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session := server.ClientSessionFromContext(ctx)
	rs, ok := session.(*RobotSession)
	if !ok || rs.ntClient == nil {
		return mcp.NewToolResultText("No active robot connection to disconnect."), nil
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.ntClient.Disconnect()
	rs.ntClient = nil

	return mcp.NewToolResultText("Disconnected successfully."), nil
}
