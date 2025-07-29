package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	sessionsLoaded bool
	loadingError   error
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gophercon25",
		Title:   "GopherCon 2025 Agenda Server",
		Version: "v1.0.0",
	}, nil)

	// Add tools
	addTools(server)

	// Start session loading in background
	go func() {
		fetcher := newFetcher()
		defer fetcher.Close()

		log.Println("Loading GopherCon agenda sessions...")
		if err := fetcher.loadAllSessions(); err != nil {
			log.Printf("Failed to load sessions: %v", err)
			loadingError = err
			return
		}

		sessionsLoaded = true
		log.Printf("Successfully loaded %d sessions", len(sessionsMap))
	}()

	t := mcp.NewLoggingTransport(mcp.NewStdioTransport(), os.Stderr)
	log.Printf("Starting GopherCon agenda server...")
	if err := server.Run(context.Background(), t); err != nil {
		log.Printf("Server failed: %v", err)
	}
}

func addTools(server *mcp.Server) {
	// Tool 1: Get all sessions
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_all_sessions",
		Description: "Get a list of all GopherCon 2025 sessions with all relevant information.",
	}, func(ctx context.Context, _ *mcp.ServerSession, _ *mcp.CallToolParamsFor[EmptyParams]) (*mcp.CallToolResultFor[any], error) {
		// Check if sessions are still loading
		var result strings.Builder
		if !sessionsLoaded {
			if loadingError != nil {
				return &mcp.CallToolResultFor[any]{
					Content: []mcp.Content{
						&mcp.TextContent{Text: fmt.Sprintf("Error loading sessions: %v. Please try again later.", loadingError)},
					},
				}, nil
			}
			result.WriteString("Sessions are still loading, results may be incomplete.")
		}

		result.WriteString("# GopherCon 2025 Sessions\n\n")
		sessions := sessions()
		for _, session := range sessions {
			result.WriteString(session.String())
		}

		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: result.String()},
			},
		}, nil
	})

	/*
		// Tool 2: Get session details by ID
		mcp.AddTool(server, &mcp.Tool{
			Name:        "get_session_details",
			Description: "Get detailed information about a specific GopherCon session by its ID",
		}, func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[SessionIDParams]) (*mcp.CallToolResultFor[any], error) {
			// Check if sessions are still loading
			var result strings.Builder
			if !sessionsLoaded {
				if loadingError != nil {
					return &mcp.CallToolResultFor[any]{
						Content: []mcp.Content{
							&mcp.TextContent{Text: fmt.Sprintf("Error loading sessions: %v. Please try again later.", loadingError)},
						},
					}, nil
				}
				result.WriteString("Sessions are still loading, results may be incomplete.")
			}

			session, exists := sessionByID(params.Arguments.SessionID)
			if !exists {
				return &mcp.CallToolResultFor[any]{
					Content: []mcp.Content{
						&mcp.TextContent{Text: fmt.Sprintf("Session with ID %s not found. Use get_all_sessions to see available session IDs.", params.Arguments.SessionID)},
					},
				}, nil
			}
			return &mcp.CallToolResultFor[any]{
				Content: []mcp.Content{
					&mcp.TextContent{Text: session.String()},
				},
			}, nil
		})
	*/
}

// EmptyParams represents an empty request struct for tools that don't need input
type EmptyParams struct{}

// SessionIDParams represents parameters for session ID requests
type SessionIDParams struct {
	SessionID string `json:"session_id"`
}
