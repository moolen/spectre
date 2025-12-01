package e2e

import (
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestTimelineBarHighlighting tests that timeline bars show visual highlighting (outline) when selected
// via clicking and arrow key navigation
func TestTimelineBarHighlighting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)

	ctx := t.Context()

	// Create a deployment to have data to interact with
	deployment, err := helpers.CreateTestDeployment(ctx, t, testCtx.K8sClient, testCtx.Namespace)
	require.NoError(t, err, "failed to create deployment")

	// Wait for resource to be available in the API
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)

	uiURL := testCtx.PortForward.GetURL()
	t.Logf("Testing timeline highlighting at %s with deployment %s", uiURL, deployment.Name)

	bt, err := helpers.NewBrowserTest(t)
	require.NoError(t, err, "failed to create browser test")
	defer bt.Close()

	// Navigate to timeline with time range (last 10 minutes)
	timelineURL := helpers.BuildTimelineURL(uiURL, 10*time.Minute)
	_, err = bt.Page.Goto(timelineURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60000),
	})
	require.NoError(t, err, "failed to navigate to timeline")

	time.Sleep(2 * time.Second)

	// Find resource labels (clicking on a label selects the last segment of that resource)
	resourceLabelSelector := "g.label"
	resourceLabels := bt.Page.Locator(resourceLabelSelector)
	labelCount, err := resourceLabels.Count()
	require.NoError(t, err)
	require.Greater(t, labelCount, 0, "expected at least one resource label to be visible")
	t.Logf("✓ Found %d resource labels", labelCount)

	// Click on first resource label to select its last segment
	firstLabel := resourceLabels.First()
	err = firstLabel.Click()
	require.NoError(t, err, "failed to click on first resource label")
	t.Log("✓ Clicked on first resource label")

	time.Sleep(1 * time.Second)

	// Find all segments and verify one has a stroke (is highlighted)
	segmentSelector := "rect.segment"
	segments := bt.Page.Locator(segmentSelector)
	segmentCount, err := segments.Count()
	require.NoError(t, err)
	require.Greater(t, segmentCount, 0, "expected at least one segment to be visible")
	t.Logf("✓ Found %d timeline segments", segmentCount)

	// Check that at least one segment has a non-'none' stroke (indicating selection)
	var highlightedSegmentFound bool
	for i := 0; i < segmentCount; i++ {
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		require.NoError(t, err, "failed to get stroke attribute for segment %d", i)

		if strokeAttr != "" && strokeAttr != "none" {
			highlightedSegmentFound = true
			t.Logf("✓ Segment %d has visible stroke: %s", i, strokeAttr)
			break
		}
	}
	require.True(t, highlightedSegmentFound, "at least one segment should have a visible stroke after clicking")

	// Test arrow key navigation - this requires the detail panel to be open
	// Let the detail panel be open from the click above

	// Get the first segment with a stroke to track it
	var firstHighlightedIndex int
	for i := 0; i < segmentCount; i++ {
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		require.NoError(t, err)
		if strokeAttr != "" && strokeAttr != "none" {
			firstHighlightedIndex = i
			break
		}
	}
	t.Logf("✓ Initial highlighted segment index: %d", firstHighlightedIndex)

	// Press ArrowRight to navigate to next segment
	err = bt.Page.Keyboard().Press("ArrowRight")
	require.NoError(t, err, "failed to press ArrowRight")
	t.Log("✓ Pressed ArrowRight to navigate to next segment")

	time.Sleep(800 * time.Millisecond)

	// Verify that navigation works (at least one segment is still highlighted)
	var currentHighlightedFound bool
	for i := 0; i < segmentCount; i++ {
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		require.NoError(t, err)

		if strokeAttr != "" && strokeAttr != "none" {
			// Found a highlighted segment after navigation
			if i != firstHighlightedIndex {
				// It's different from the original
				t.Logf("✓ After ArrowRight, segment %d is now highlighted (moved from %d)", i, firstHighlightedIndex)
			}
			currentHighlightedFound = true
			break
		}
	}
	require.True(t, currentHighlightedFound, "at least one segment should still be highlighted after ArrowRight")
	t.Log("✓ Arrow key navigation works")

	// Press ArrowLeft to navigate back
	err = bt.Page.Keyboard().Press("ArrowLeft")
	require.NoError(t, err, "failed to press ArrowLeft")
	t.Log("✓ Pressed ArrowLeft to navigate back")

	time.Sleep(800 * time.Millisecond)

	// Verify highlighting still exists after navigation
	highlightedAfterNavigation := false
	for i := 0; i < segmentCount; i++ {
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		require.NoError(t, err)
		if strokeAttr != "" && strokeAttr != "none" {
			highlightedAfterNavigation = true
			break
		}
	}
	require.True(t, highlightedAfterNavigation, "a segment should still be highlighted after back navigation")
	t.Log("✓ Highlighting persists after navigation")

	t.Log("✅ All timeline highlighting tests passed")
}

// TestTimelineHighlightingWithThemeSwitch tests highlighting works when switching themes
func TestTimelineHighlightingWithThemeSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)

	ctx := t.Context()

	// Create a deployment to have data to interact with
	deployment, err := helpers.CreateTestDeployment(ctx, t, testCtx.K8sClient, testCtx.Namespace)
	require.NoError(t, err, "failed to create deployment")

	// Wait for resource to be available in the API
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)

	uiURL := testCtx.PortForward.GetURL()
	t.Logf("Testing timeline highlighting with theme switching at %s", uiURL)

	bt, err := helpers.NewBrowserTest(t)
	require.NoError(t, err, "failed to create browser test")
	defer bt.Close()

	// Navigate to timeline with time range (last 10 minutes)
	timelineURL := helpers.BuildTimelineURL(uiURL, 10*time.Minute)
	_, err = bt.Page.Goto(timelineURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60000),
	})
	require.NoError(t, err, "failed to navigate to timeline")

	time.Sleep(2 * time.Second)

	// Find resource labels
	resourceLabelSelector := "g.label"
	resourceLabels := bt.Page.Locator(resourceLabelSelector)
	labelCount, err := resourceLabels.Count()
	require.NoError(t, err)
	require.Greater(t, labelCount, 0, "expected at least one resource label")

	// Click on first resource label to select a segment
	firstLabel := resourceLabels.First()
	err = firstLabel.Click()
	require.NoError(t, err, "failed to click on resource label")
	t.Log("✓ Clicked on resource label")

	time.Sleep(1 * time.Second)

	// Verify segment is highlighted in dark theme (default)
	segmentSelector := "rect.segment"
	segments := bt.Page.Locator(segmentSelector)
	var highlightedInDark bool
	for i := 0; i < 50 && i < 100; i++ { // Limit iterations
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		if err != nil {
			break
		}
		if strokeAttr != "" && strokeAttr != "none" {
			highlightedInDark = true
			t.Logf("✓ Dark theme: segment has stroke: %s", strokeAttr)
			break
		}
	}
	require.True(t, highlightedInDark, "segment should be highlighted in dark theme")

	// Switch to light theme
	_, err = bt.Page.Evaluate(`() => {
		const root = document.documentElement;
		root.setAttribute('data-theme', 'light');
	}`)
	require.NoError(t, err, "failed to set light theme")

	time.Sleep(1 * time.Second)

	// Verify segment is still highlighted in light theme
	var highlightedInLight bool
	for i := 0; i < 50 && i < 100; i++ { // Limit iterations
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		if err != nil {
			break
		}
		if strokeAttr != "" && strokeAttr != "none" {
			highlightedInLight = true
			t.Logf("✓ Light theme: segment has stroke: %s", strokeAttr)
			break
		}
	}
	require.True(t, highlightedInLight, "segment should be highlighted in light theme")

	// Switch back to dark theme
	_, err = bt.Page.Evaluate(`() => {
		const root = document.documentElement;
		root.setAttribute('data-theme', 'dark');
	}`)
	require.NoError(t, err, "failed to set dark theme")

	time.Sleep(1 * time.Second)

	// Verify highlighting still works
	var highlightedAfterSwitch bool
	for i := 0; i < 50 && i < 100; i++ { // Limit iterations
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		if err != nil {
			break
		}
		if strokeAttr != "" && strokeAttr != "none" {
			highlightedAfterSwitch = true
			break
		}
	}
	require.True(t, highlightedAfterSwitch, "segment should be highlighted after switching back to dark theme")

	t.Log("✅ Theme switching highlighting test passed")
}
