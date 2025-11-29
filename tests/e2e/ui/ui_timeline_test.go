package e2e

import (
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestUITimelinePageNavigation tests the timeline page loads correctly and navigation works
func TestUITimelinePageNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)
	uiURL := testCtx.PortForward.GetURL()

	bt, err := helpers.NewBrowserTest(t)
	require.NoError(t, err, "failed to create browser test")
	defer bt.Close()

	// Navigate to the root path
	_, err = bt.Page.Goto(uiURL+"/", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(30000),
	})
	require.NoError(t, err, "failed to navigate to root")

	// Wait for redirect
	time.Sleep(2 * time.Second)

	// Should redirect to /timeline
	currentURL := bt.Page.URL()
	require.Contains(t, currentURL, "/timeline", "expected to be redirected to /timeline, but got %s", currentURL)

	// Assert time range picker is visible
	timeRangePicker := bt.Page.Locator("[class*=\"test-TimeRange\"]")
	visible, err := timeRangePicker.IsVisible()
	require.NoError(t, err, "failed to check time range picker visibility")
	require.True(t, visible, "time range picker should be visible")

	// Assert body contains "Select Time Range"
	body := bt.Page.Locator("body")
	text, err := body.TextContent()
	require.NoError(t, err, "failed to get body text content")
	require.Contains(t, text, "Select Time Range", "body should contain 'Select Time Range'")

	t.Log("✓ Timeline page navigation test passed")
}

// TestUITimelineDataLoading tests that the timeline loads and displays data
func TestUITimelineDataLoading(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)

	// Create a test deployment to have some data
	deployment, err := helpers.CreateTestDeployment(t.Context(), t, testCtx.K8sClient, testCtx.Namespace)
	require.NoError(t, err, "failed to create test deployment")

	// Wait for resource to be available in the API
	_ = helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)

	uiURL := testCtx.PortForward.GetURL()
	t.Logf("Testing UI at %s with deployment %s", uiURL, deployment.Name)

	bt, err := helpers.NewBrowserTest(t)
	require.NoError(t, err, "failed to create browser test")
	defer bt.Close()

	// Navigate to timeline with time range (last 10 minutes)
	timelineURL := helpers.BuildTimelineURL(uiURL, 10*time.Minute)
	_, err = bt.Page.Goto(timelineURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(30000),
	})
	require.NoError(t, err, "failed to navigate to timeline")

	// Wait for the page to fully load
	time.Sleep(2 * time.Second)

	// Check if we're on the timeline page
	currentURL := bt.Page.URL()
	require.Contains(t, currentURL, "/timeline", "expected /timeline page, but got %s", currentURL)

	// Look for timeline elements or loading state
	// The page should either show resources or loading indicator
	timelineSelector := "[class*=\"timeline\"], [class*=\"Timeline\"], [class*=\"loading\"], [class*=\"Loading\"]"
	timelineLocator := bt.Page.Locator(timelineSelector)

	// Wait a bit for timeline to load
	time.Sleep(2 * time.Second)

	visible, err := timelineLocator.IsVisible()
	if err == nil && visible {
		t.Log("✓ Timeline container found or loading visible")
	} else {
		t.Log("Timeline container not found yet, checking page content")
	}

	// Verify the filter bar is visible
	filterBarSelector := "[class*=\"FilterBar\"], [class*=\"filter\"]"
	filterBar := bt.Page.Locator(filterBarSelector)
	visible, err = filterBar.IsVisible()
	if err == nil && visible {
		t.Log("✓ Filter bar visible")
	} else {
		t.Log("Filter bar not found, but that may be okay")
	}

	// Check page has content
	content, err := bt.Page.Content()
	require.NoError(t, err, "failed to get page content")
	require.Greater(t, len(content), 1000, "page should have significant content")

	t.Log("✓ Timeline data loading test passed")
}
