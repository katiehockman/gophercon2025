package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

var sessionsMap = make(map[string]Session)
var sessionsMutex sync.RWMutex

// sessions returns a copy of all sessions.
func sessions() []Session {
	sessionsMutex.RLock()
	defer sessionsMutex.RUnlock()

	sessions := make([]Session, 0, len(sessionsMap))
	for _, session := range sessionsMap {
		sessions = append(sessions, session)
	}
	return sessions
}

// sessionByID returns a session by its ID
func sessionByID(id string) (Session, bool) {
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
	log.Println("Connected.")
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

// fetch fetches all GopherCon sessions in parallel
func (f *fetcher) fetch() {
	log.Printf("Loading %d sessions from GopherCon 2025 using parallel processing...", len(sessionIDs))

	// Configuration for parallel processing
	maxWorkers := runtime.GOMAXPROCS(0) // Use number of available CPU cores
	const maxRetries = 3

	log.Printf("Using %d workers for parallel processing.", maxWorkers)

	// Create channels for coordination
	sessionChan := make(chan string, len(sessionIDs))
	resultChan := make(chan sessionResult, len(sessionIDs))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := range maxWorkers {
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

	for result := range resultChan {
		if result.err != nil {
			log.Printf("Error loading session %s: %v.", result.sessionID, result.err)
			continue
		}

		// Thread-safe write to sessionsMap.
		sessionsMutex.Lock()
		sessionsMap[result.session.ID] = result.session
		sessionsMutex.Unlock()

		log.Printf("Successfully loaded session %s: %s.", result.session.ID, result.session.Title)
	}

	log.Printf("Total sessions loaded: %d", len(sessionsMap))
	close(sessionsReady)
}

// sessionResult is the result of loading a session.
type sessionResult struct {
	sessionID string
	session   Session
	err       error
}

func (f *fetcher) worker(id int, sessionChan <-chan string, resultChan chan<- sessionResult, wg *sync.WaitGroup, maxRetries int) {
	defer wg.Done()

	for sessionID := range sessionChan {
		url := fmt.Sprintf("https://www.gophercon.com/agenda/session/%s", sessionID)

		var session Session
		var err error

		// Retry logic
		for attempt := 1; attempt <= maxRetries; attempt++ {
			session, err = f.parseSession(sessionID, url)
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

// parseSession parses a single session from the HTML.
func (f *fetcher) parseSession(sessionID, url string) (Session, error) {
	htmlContent, err := f.fetchPage(url)
	if err != nil {
		return Session{}, fmt.Errorf("failed to fetch session %s: %v", sessionID, err)
	}

	// Parse the HTML to extract session information.
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return Session{}, fmt.Errorf("failed to parse HTML for session %s: %v", sessionID, err)
	}

	// Extract session information from the HTML using specific selectors from the actual structure.
	session := Session{
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

// fetchPage fetches HTML content using chromedp.
func (f *fetcher) fetchPage(url string) (string, error) {
	tabCtx, cancel := chromedp.NewContext(f.ctx)
	defer cancel()

	// Set a timeout per request.
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, 30*time.Second)
	defer cancelTimeout()

	log.Printf("Fetching %q.", url)
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

func loadSessions() error {
	if *offlineMode {
		log.Printf("Running in offline mode, loading sessions from %s", *dataFile)
		if err := loadSessionsFromFile(*dataFile); err != nil {
			return fmt.Errorf("Failed to load sessions from file: %w", err)
		}
		close(sessionsReady)
		return nil
	}
	// Load sessions in the background.
	go func() {
		log.Println("Loading GopherCon agenda sessions...")
		fetcher := newFetcher()
		defer fetcher.Close()
		fetcher.fetch()
		close(sessionsReady)
	}()
	return nil
}

// loadSessionsFromFile loads sessions from a JSON file
func loadSessionsFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	var sessionsData []Session
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&sessionsData); err != nil {
		return fmt.Errorf("failed to decode sessions from JSON: %w", err)
	}

	log.Printf("Loaded %d sessions from file", len(sessionsData))

	// Load sessions into the global map
	sessionsMutex.Lock()
	defer sessionsMutex.Unlock()
	for _, session := range sessionsData {
		sessionsMap[session.ID] = session
		log.Printf("Added session %s: %s", session.ID, session.Title)
	}

	log.Printf("Total sessions in map: %d", len(sessionsMap))
	return nil
}

// sessionIDs are hard-coded session IDs for GopherCon 2025.
var sessionIDs = []string{
	"1545653", "1557197", "1590663", "1545640", "1590103", "1594224", "1545643", "1545641", "1557237", "1557206",
	"1545646", "1557199", "1557216", "1545650", "1545651", "1565804", "1557235", "1545655", "1545656", "1545657",
	"1545658", "1545682", "1572365", "1545661", "1545662", "1545663", "1545664", "1557386", "1557394", "1545667",
	"1557388", "1557392", "1557390", "1557391", "1545671", "1557387", "1557389", "1557348", "1647415", "1557393",
	"1557391", "1545679", "1545681", "1557342", "1572366", "1557343", "1545685", "1545686", "1545687", "1557395",
	"1557396", "1557397", "1557345", "1557398", "1557399", "1557400", "1557347", "1557344", "1557402", "1557403",
	"1545674", "1557401", "1557404", "1557405", "1557195",
}
