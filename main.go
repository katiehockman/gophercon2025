package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gophercon25",
		Title:   "GopherCon 2025 Agenda Server",
		Version: "v1.0.0",
	}, nil)

	// Add tools.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_all_sessions",
		Description: "Get a list of all GopherCon 2025 sessions with all relevant information.",
	}, AllSessions)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_session_details",
		Description: "Get detailed information about a specific GopherCon session by its ID",
	}, SessionByID)

	// Start session loading in background.
	go func() {
		fetcher := newFetcher()
		defer fetcher.Close()

		log.Println("Loading GopherCon agenda sessions...")
		fetcher.loadAllSessions()
	}()

	t := mcp.NewLoggingTransport(mcp.NewStdioTransport(), os.Stderr)
	log.Printf("Starting GopherCon agenda server...")
	if err := server.Run(context.Background(), t); err != nil {
		log.Printf("Server failed: %v", err)
	}
}

func AllSessions(ctx context.Context, _ *mcp.ServerSession, _ *mcp.CallToolParamsFor[struct{}]) (*mcp.CallToolResultFor[SessionsResult], error) {
	// Tool 1: Get all sessions.

	// Block until sessions are ready.
	<-sessionsReady
	
	var res mcp.CallToolResultFor[SessionsResult]
	res.StructuredContent = SessionsResult{
		Sessions: sessions(),
	}

	return &res, nil
}

func SessionByID(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[SessionIDParams]) (*mcp.CallToolResultFor[SessionsResult], error) {
	// Tool 2: Get session details by ID.

	// Block until sessions are ready.
	<-sessionsReady

	var res mcp.CallToolResultFor[SessionsResult]
	session, exists := sessionByID(params.Arguments.SessionID)
	if !exists {
		res.Content = []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Session with ID %s not found. Use get_all_sessions to see available session IDs.", params.Arguments.SessionID)},
		}
		return &res, nil
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Successfully found session details."},
	}
	res.StructuredContent = SessionsResult{
		Sessions: []Session{session},
	}
	return &res, nil
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
