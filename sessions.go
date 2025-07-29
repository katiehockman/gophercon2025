package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// sessions returns all sessions as a slice
func sessions() []session {
	sessionsMutex.RLock()
	defer sessionsMutex.RUnlock()

	sessions := make([]session, 0, len(sessionsMap))
	for _, session := range sessionsMap {
		sessions = append(sessions, session)
	}
	return sessions
}

// sessionByID returns a session by its ID
func sessionByID(id string) (session, bool) {
	sessionsMutex.RLock()
	defer sessionsMutex.RUnlock()

	session, exists := sessionsMap[id]
	return session, exists
}

func newFetcher() *fetcher {
	var f fetcher
	log.Println("Connecting to browser...")
	// Create a top-level context for the browser instance
	ctx, cancel := chromedp.NewExecAllocator(context.Background(),
		// Disable loading images/fonts for speed
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
	)
	f.cancel = cancel
	// Create a browser context
	f.ctx, f.browserCancel = chromedp.NewContext(ctx)
	log.Println("Connected")
	return &f
}

func (f *fetcher) Close() {
	f.cancel()
	f.browserCancel()
}

type fetcher struct {
	ctx                   context.Context
	cancel, browserCancel context.CancelFunc
}

// session represents a GopherCon session
type session struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Date        string   `json:"date,omitempty"`
	Description string   `json:"description,omitempty"`
	Time        string   `json:"time,omitempty"`
	Location    string   `json:"location,omitempty"`
	Speakers    []string `json:"speakers,omitempty"`
	Duration    string   `json:"duration,omitempty"`
	Speaker     string   `json:"speaker,omitempty"`
	Track       string   `json:"track,omitempty"`
}

// Global session maps loaded at startup
var sessionsMap = make(map[string]session)
var sessionsMutex sync.RWMutex

// loadAllSessions loads all GopherCon sessions at startup using parallelization
func (f *fetcher) loadAllSessions() error {
	log.Printf("Loading %d sessions from GopherCon 2025 using parallel processing...", len(sessionIDs))

	// Configuration for parallel processing
	maxWorkers := runtime.GOMAXPROCS(0) // Use number of available CPU cores
	const maxRetries = 3

	log.Printf("Using %d workers for parallel processing", maxWorkers)

	// Create channels for coordination
	sessionChan := make(chan string, len(sessionIDs))
	resultChan := make(chan sessionResult, len(sessionIDs))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go f.worker(i, sessionChan, resultChan, &wg, maxRetries)
	}

	// Send all session IDs to workers
	go func() {
		defer close(sessionChan)
		for _, sessionID := range sessionIDs {
			sessionChan <- sessionID
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	successCount := 0
	errorCount := 0

	for result := range resultChan {
		if result.err != nil {
			log.Printf("Error loading session %s: %v", result.sessionID, result.err)
			errorCount++
			continue
		}

		// Thread-safe write to sessionsMap
		sessionsMutex.Lock()
		sessionsMap[result.session.ID] = result.session
		sessionsMutex.Unlock()

		log.Printf("Successfully loaded session %s: %s", result.session.ID, result.session.Title)
		successCount++
	}

	log.Printf("Session loading complete: %d successful, %d errors", successCount, errorCount)
	log.Printf("Total sessions loaded: %d", len(sessionsMap))
	return nil
}

// sessionResult represents the result of loading a session
type sessionResult struct {
	sessionID string
	session   session
	err       error
}

// worker processes session loading tasks
func (f *fetcher) worker(id int, sessionChan <-chan string, resultChan chan<- sessionResult, wg *sync.WaitGroup, maxRetries int) {
	defer wg.Done()

	for sessionID := range sessionChan {
		url := fmt.Sprintf("https://www.gophercon.com/agenda/session/%s", sessionID)

		var session session
		var err error

		// Retry logic
		for attempt := 1; attempt <= maxRetries; attempt++ {
			session, err = f.loadSession(sessionID, url)
			if err == nil {
				break
			}

			if attempt < maxRetries {
				log.Printf("Worker %d: Retry %d for session %s: %v", id, attempt, sessionID, err)
				time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
			}
		}

		resultChan <- sessionResult{
			sessionID: sessionID,
			session:   session,
			err:       err,
		}
	}
}

// loadSession loads a single session
func (f *fetcher) loadSession(sessionID, url string) (session, error) {
	htmlContent, err := f.fetchPage(url)
	if err != nil {
		return session{}, fmt.Errorf("failed to fetch session %s: %v", sessionID, err)
	}

	// Parse the HTML to extract session information
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return session{}, fmt.Errorf("failed to parse HTML for session %s: %v", sessionID, err)
	}

	// Extract session information from the HTML using specific selectors from the actual structure
	session := session{
		ID:  sessionID,
		URL: url,
	}

	// Extract title from .session-title
	if title := doc.Find(".session-title").First().Text(); title != "" {
		session.Title = strings.TrimSpace(title)
	}

	// Extract description from .session-description
	if desc := doc.Find(".session-description").First().Text(); desc != "" {
		session.Description = strings.TrimSpace(desc)
	}

	// Extract date from .session-date
	if date := doc.Find(".session-date").First().Text(); date != "" {
		session.Date = strings.TrimSpace(date)
	}

	// Extract time information from .session-dates time element
	if timeText := doc.Find(".session-dates time").First().Text(); timeText != "" {
		session.Time = strings.TrimSpace(timeText)
	}

	// Extract location from .session-location
	if location := doc.Find(".session-location").First().Text(); location != "" {
		session.Location = strings.TrimSpace(location)
	}

	// Extract duration from .session-duration
	if duration := doc.Find(".session-duration").First().Text(); duration != "" {
		session.Duration = strings.TrimSpace(duration)
	}

	// Extract speakers from the speaker container
	// TODO: Fix this as it is not working consistently.
	doc.Find(".speaker-name").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Text())
		if name != "" {
			session.Speakers = append(session.Speakers, name)
		}
	})

	return session, nil
}

// String returns a formatted string representation of the session
func (s session) String() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("**%s** (ID: %s)\n", s.Title, s.ID))
	result.WriteString(fmt.Sprintf("URL: %s\n", s.URL))
	result.WriteString(fmt.Sprintf("Date: %s\n", s.Date))
	result.WriteString(fmt.Sprintf("Time: %s\n", s.Time))
	result.WriteString(fmt.Sprintf("Location: %s\n", s.Location))
	if len(s.Speakers) > 0 {
		result.WriteString(fmt.Sprintf("Speakers: %s\n", strings.Join(s.Speakers, ", ")))
	}

	result.WriteString(fmt.Sprintf("Description: %s\n", s.Description))
	return result.String()
}

// fetchPage fetches HTML content using chromedp
func (f *fetcher) fetchPage(url string) (string, error) {
	tabCtx, cancel := chromedp.NewContext(f.ctx)
	defer cancel()

	// Set a timeout per request
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancelTimeout()

	log.Printf("Fetching %s", url)
	var htmlContent string
	if err := chromedp.Run(tabCtx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.session-title`, chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent),
	); err != nil {
		return "", fmt.Errorf("chromedp failed: %v", err)
	}
	return htmlContent, nil
}

// Session IDs for GopherCon 2025
var sessionIDs = []string{
	"1545653", "1557197", "1590663", "1545640", "1590103", "1594224", "1545643", "1545641", "1557237", "1557206",
	"1545646", "1557199", "1557216", "1545650", "1545651", "1565804", "1557235", "1545655", "1545656", "1545657",
	"1545658", "1545682", "1572365", "1545661", "1545662", "1545663", "1545664", "1557386", "1557394", "1545667",
	"1557388", "1557392", "1557390", "1557391", "1545671", "1557387", "1557389", "1557348", "1647415", "1557393",
	"1557391", "1545679", "1545681", "1557342", "1572366", "1557343", "1545685", "1545686", "1545687", "1557395",
	"1557396", "1557397", "1557345", "1557398", "1557399", "1557400", "1557347", "1557344", "1557402", "1557403",
	"1545674", "1557401", "1557404", "1557405", "1557195",
}
