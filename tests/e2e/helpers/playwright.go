package helpers

import (
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

// BrowserTest provides a playwright browser and page for UI testing
type BrowserTest struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	Page    playwright.Page
	t       *testing.T
}

// NewBrowserTest launches a browser and creates a page for testing
func NewBrowserTest(t *testing.T) (*BrowserTest, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start playwright: %w", err)
	}

	browser, err := pw.Chromium.Launch()
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("could not launch browser: %w", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("could not create page: %w", err)
	}

	return &BrowserTest{
		pw:      pw,
		browser: browser,
		Page:    page,
		t:       t,
	}, nil
}

// Close cleans up browser resources
func (bt *BrowserTest) Close() error {
	var errs []error
	if bt.Page != nil {
		if err := bt.Page.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close page: %w", err))
		}
	}
	if bt.browser != nil {
		if err := bt.browser.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close browser: %w", err))
		}
	}
	if bt.pw != nil {
		if err := bt.pw.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop playwright: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors during cleanup: %v", errs)
	}
	return nil
}

// BuildTimelineURL constructs a timeline URL with start/end time params
// duration specifies how far back from now to start the time range
func BuildTimelineURL(baseURL string, duration time.Duration) string {
	endTime := time.Now()
	startTime := endTime.Add(-duration)

	u, err := url.Parse(baseURL)
	if err != nil {
		// If baseURL is invalid, just return a constructed string
		return fmt.Sprintf("%s/?start=%s&end=%s",
			baseURL,
			url.QueryEscape(startTime.UTC().Format(time.RFC3339)),
			url.QueryEscape(endTime.UTC().Format(time.RFC3339)))
	}

	u.Path = "/"
	q := u.Query()
	q.Set("start", startTime.UTC().Format(time.RFC3339))
	q.Set("end", endTime.UTC().Format(time.RFC3339))
	u.RawQuery = q.Encode()

	return u.String()
}

// EnsurePlaywrightInstalled checks if Playwright-Go is installed
// This will attempt to install it if not present
func EnsurePlaywrightInstalled(t *testing.T) {
	// Check if playwright is available by trying to run it
	// If it fails, we'll try to install it
	pw, err := playwright.Run()
	if err != nil {
		t.Logf("Playwright not available, attempting to install...")
		// Try to install playwright
		if installErr := playwright.Install(); installErr != nil {
			require.NoError(t, installErr, "Failed to install Playwright. Run: go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps")
		}
		// Try again after installation
		pw, err = playwright.Run()
		require.NoError(t, err, "Failed to start Playwright after installation")
	}
	// If we got here, playwright is working, so clean up
	if pw != nil {
		pw.Stop()
	}
}

// FindFreePort finds an available port on localhost
func FindFreePort() (int, error) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
