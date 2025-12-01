package e2e

import (
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestUIDetailPanelInteraction tests detail panel open/close and resource display
func TestUIDetailPanelInteraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)

	ctx := t.Context()

	// Create a deployment to display in the timeline
	deployment, err := helpers.CreateTestDeployment(ctx, t, testCtx.K8sClient, testCtx.Namespace)
	require.NoError(t, err, "failed to create deployment")

	// Wait for resource to be available
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", deployment.Name, helpers.DefaultEventuallyOption)

	uiURL := testCtx.PortForward.GetURL()
	t.Logf("Testing detail panel interaction at %s", uiURL)

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

	// Wait for resources to be visible
	// We'll click on the resource label which has a direct click handler
	// that opens the detail panel for the last segment (Timeline.tsx:185-188)
	resourceLabelSelector := "g.label"
	resourceLabels := bt.Page.Locator(resourceLabelSelector)
	count, err := resourceLabels.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0, "expected at least one resource label to be visible")

	firstLabel := resourceLabels.First()
	visible, err := firstLabel.IsVisible()
	require.NoError(t, err)
	require.True(t, visible, "first resource label should be visible")

	// Click on the resource label - this has a direct click handler
	// that calls onSegmentClick with the last segment index
	err = firstLabel.Click()
	require.NoError(t, err, "failed to click on resource label")
	t.Log("✓ Successfully clicked on timeline element")

	// Wait for detail panel to appear (with retry)
	// The panel appears after React state updates, so we need to poll
	detailPanelSelector := "[class*=\"DetailPanel\"]"
	var panelCount int
	detailPanels := bt.Page.Locator(detailPanelSelector)
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		panelCount, err = detailPanels.Count()
		require.NoError(t, err)
		if panelCount > 0 {
			break
		}
	}
	require.Greater(t, panelCount, 0, "detail panel should appear after click")
	t.Log("✓ Detail panel appeared after click")

	// Try to find and click close button
	err = bt.Page.Keyboard().Press("Escape")
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	detailPanels = bt.Page.Locator(detailPanelSelector)
	panelCount, err = detailPanels.Count()
	require.NoError(t, err)
	require.Equal(t, 0, panelCount)
	t.Log("✓ Detail panel closed after Escape key press")
}
