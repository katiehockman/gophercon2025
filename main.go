package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	flag.Parse()
	if err := loadSessions(); err != nil {
		panic(err)
	}

	// Create the MCP server.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gophercon25",
		Title:   "GopherCon 2025 Agenda Server",
		Version: "v1.0.0",
	}, nil)

	// Add tools.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sessions",
		Description: "Lists all GopherCon 2025 sessions with all relevant information.",
	}, ListSessions)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_session_details",
		Description: "Get detailed information about a specific GopherCon session by ID",
	}, SessionByID)

	// Start the server.
	log.Printf("Starting GopherCon agenda server...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}

// ListSessions is a tool that returns all data about all loaded sessions.
func ListSessions(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, SessionsResult, error) {
	// Block until sessions are ready.
	<-sessionsReady

	// Tool 1: Get all sessions.
	return nil, SessionsResult{Sessions: sessions()}, nil
}

// SessionByID is a tool that returns all data about a specific session by its ID.
func SessionByID(ctx context.Context, _ *mcp.CallToolRequest, params SessionIDParams) (*mcp.CallToolResult, SessionsResult, error) {
	// Block until sessions are ready.
	<-sessionsReady

	// Tool 2: Get session details by ID.
	session, exists := sessionByID(params.SessionID)
	if !exists {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Session with ID %s not found. Use get_all_sessions to see available session IDs.", params.SessionID)},
			},
		}, SessionsResult{}, nil
	}

	return nil, SessionsResult{
		Sessions: []Session{session},
	}, nil
}

// SessionIDParams are the parameters for session ID requests.
type SessionIDParams struct {
	// SessionID is the ID string for a GopherCon session.
	SessionID string `json:"session_id"`
}

// SessionsResult is a list of GopherCon Sessions.
type SessionsResult struct {
	Sessions []Session `json:"sessions"`
}

// Session is a single GopherCon Session.
type Session struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Date        string   `json:"date,omitempty"`
	Description string   `json:"description,omitempty"`
	Time        string   `json:"time,omitempty"`
	Location    string   `json:"location,omitempty"`
	Speakers    []string `json:"speakers,omitempty"`
	Duration    string   `json:"duration,omitempty"`
}

var (
	offlineMode = flag.Bool("offline", false, "Run in offline mode using cached session data")
	dataFile    = flag.String("data-file", "sessions_backup.json", "File to save/load session data")
)

// Channel to signal when sessions are fully loaded.
var sessionsReady = make(chan bool)
