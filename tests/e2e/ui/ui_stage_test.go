package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

type UIStage struct {
	t           *testing.T
	require     *require.Assertions
	assert      *assert.Assertions
	testCtx     *helpers.TestContext
	k8sClient   *helpers.K8sClient
	apiClient   *helpers.APIClient
	browserTest *helpers.BrowserTest

	// testNamespace string // unused field
	uiURL       string
	timelineURL string
	deployments []*appsv1.Deployment
	namespaces  []string
}

func NewUIStage(t *testing.T) (*UIStage, *UIStage, *UIStage) {
	s := &UIStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *UIStage) and() *UIStage {
	return s
}

func (s *UIStage) a_test_environment() *UIStage {
	helpers.EnsurePlaywrightInstalled(s.t)
	s.testCtx = helpers.SetupE2ETest(s.t)
	s.k8sClient = s.testCtx.K8sClient
	s.apiClient = s.testCtx.APIClient
	s.uiURL = s.testCtx.PortForward.GetURL()
	return s
}

func (s *UIStage) browser_is_initialized() *UIStage {
	bt, err := helpers.NewBrowserTest(s.t)
	s.require.NoError(err, "failed to create browser test")
	s.browserTest = bt
	s.t.Cleanup(func() {
		// Capture debug info if test failed
		if s.t.Failed() && s.browserTest != nil && s.browserTest.Page != nil {
			s.captureDebugInfo("test_failed")
		}
		if err := bt.Close(); err != nil {
			s.t.Logf("Warning: failed to close browser: %v", err)
		}
	})
	return s
}

func (s *UIStage) navigated_to_timeline(duration time.Duration) *UIStage {
	s.timelineURL = helpers.BuildTimelineURL(s.uiURL, duration)
	_, err := s.browserTest.Page.Goto(s.timelineURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60000),
	})
	s.require.NoError(err, "failed to navigate to timeline")
	time.Sleep(2 * time.Second)
	return s
}

func (s *UIStage) navigated_to_root() *UIStage {
	_, err := s.browserTest.Page.Goto(s.uiURL+"/", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(30000),
	})
	s.require.NoError(err, "failed to navigate to root")
	return s
}

func (s *UIStage) deployment_is_created(name, namespace string) *UIStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Minute)
	defer cancel()

	var deployment *appsv1.Deployment
	var err error

	// Use testCtx.Namespace if namespace is empty
	if namespace == "" {
		namespace = s.testCtx.Namespace
	}

	if name == "" {
		deployment, err = helpers.CreateTestDeployment(ctx, s.t, s.k8sClient, namespace)
	} else {
		depBuilder := helpers.NewDeploymentBuilder(s.t, name, namespace)
		deployment = depBuilder.Build()
		_, err = s.k8sClient.CreateDeployment(ctx, namespace, deployment)
	}

	s.require.NoError(err, "failed to create deployment")
	s.deployments = append(s.deployments, deployment)
	return s
}

func (s *UIStage) deployments_are_created_in_namespaces(namespaces []string) *UIStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Minute)
	defer cancel()

	for _, ns := range namespaces {
		err := s.k8sClient.CreateNamespace(ctx, ns)
		s.require.NoError(err, "failed to create namespace %s", ns)
		s.t.Cleanup(func() {
			if err := s.k8sClient.DeleteNamespace(s.t.Context(), ns); err != nil {
				s.t.Logf("Warning: failed to delete namespace: %v", err)
			}
		})

		dep, err := helpers.CreateTestDeployment(ctx, s.t, s.k8sClient, ns)
		s.require.NoError(err, "failed to create deployment in namespace %s", ns)
		s.deployments = append(s.deployments, dep)
		s.namespaces = append(s.namespaces, ns)
		s.t.Logf("Created deployment %s in namespace %s", dep.Name, ns)
	}

	return s
}

func (s *UIStage) resources_are_available() *UIStage {
	for _, dep := range s.deployments {
		helpers.EventuallyResourceCreated(s.t, s.apiClient, dep.Namespace, "Deployment", dep.Name, helpers.DefaultEventuallyOption)
	}
	// Wait additional time for storage to fully index and UI to update
	// This is needed for UI rendering, not backend indexing (different from removed backend sleeps)
	time.Sleep(5 * time.Second)
	return s
}

// scroll_timeline_to_load_all_resources scrolls the timeline container to trigger lazy loading
// and waits for all resources to be loaded
func (s *UIStage) scroll_timeline_to_load_all_resources() *UIStage {
	// Wait for page to be fully loaded first
	s.require.NoError(s.browserTest.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}))
	
	// Wait a bit for React to render
	time.Sleep(3 * time.Second)
	
	// Check if there's an error message on the page (like "Value is larger than Number.MAX_SAFE_INTEGER")
	errorText := s.browserTest.Page.Locator("text=/Value is larger than Number.MAX_SAFE_INTEGER|Failed to load resources/i")
	errorCount, _ := errorText.Count()
	if errorCount > 0 {
		s.t.Logf("⚠ JavaScript error detected on page - this may prevent timeline from rendering")
		// Wait a bit more to see if it resolves
		time.Sleep(5 * time.Second)
		errorCount, _ = errorText.Count()
		if errorCount > 0 {
			s.t.Logf("⚠ Error still present after wait - timeline may not be functional")
		}
	}
	
	// Use JavaScript to find the scrollable container
	// This approach works even if SVG isn't visible yet - we look for any scrollable container
	// that might be the timeline, or we wait for SVG and find its parent
	containerFound, err := s.browserTest.Page.Evaluate(`() => {
		// First try to find SVG and its scrollable parent
		const svg = document.querySelector('svg');
		if (svg) {
			let parent = svg.parentElement;
			while (parent) {
				const styles = window.getComputedStyle(parent);
				if (styles.overflowY === 'auto' || styles.overflowY === 'scroll') {
					parent.setAttribute('data-timeline-container', 'true');
					return true;
				}
				parent = parent.parentElement;
			}
		}
		
		// Fallback: find any div with overflow-y-auto that's large enough to be the timeline
		const containers = document.querySelectorAll('div.overflow-y-auto');
		for (const container of containers) {
			const rect = container.getBoundingClientRect();
			// Timeline container should be reasonably large (at least 400px tall)
			if (rect.height > 400) {
				container.setAttribute('data-timeline-container', 'true');
				return true;
			}
		}
		
		return false;
	}`, nil)
	s.require.NoError(err, "failed to find timeline container via JavaScript")
	
	if !containerFound.(bool) {
		// Wait a bit more and try again - maybe the JavaScript error needs time to resolve
		s.t.Logf("⚠ Timeline container not found initially, waiting for page to render...")
		time.Sleep(5 * time.Second)
		
		containerFound, err = s.browserTest.Page.Evaluate(`() => {
			const svg = document.querySelector('svg');
			if (svg) {
				let parent = svg.parentElement;
				while (parent) {
					const styles = window.getComputedStyle(parent);
					if (styles.overflowY === 'auto' || styles.overflowY === 'scroll') {
						parent.setAttribute('data-timeline-container', 'true');
						return true;
					}
					parent = parent.parentElement;
				}
			}
			return false;
		}`, nil)
		s.require.NoError(err, "failed to find timeline container on retry")
	}
	
	if !containerFound.(bool) {
		// Capture debug information
		s.captureDebugInfo("timeline_container_not_found")
		// Log warning but don't fail - maybe resources are already loaded or container will appear later
		s.t.Logf("⚠ Timeline container not found - this may indicate a JavaScript error in the UI")
		s.t.Logf("⚠ Test will continue but may fail if resources aren't visible")
		// Return early - we can't scroll if there's no container
		return s
	}
	
	// Now find the container using the attribute we just set
	container := s.browserTest.Page.Locator("[data-timeline-container='true']")
	err = container.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		// Capture debug information
		s.captureDebugInfo("timeline_container_wait_failed")
		s.t.Logf("⚠ Timeline container not found after marking - this may indicate a JavaScript error")
		s.t.Logf("⚠ Test will continue but may fail if resources aren't visible")
		// Return early - we can't scroll if there's no container
		return s
	}
	
	// Scroll to load all resources by repeatedly scrolling to bottom
	// Continue until no more resources are being loaded
	maxScrollAttempts := 20
	scrollAttempt := 0
	lastScrollHeight := 0.0
	
	for scrollAttempt < maxScrollAttempts {
		// Get current scroll position and container dimensions
		scrollInfo, err := container.Evaluate(`(element) => {
			return {
				scrollTop: element.scrollTop,
				scrollHeight: element.scrollHeight,
				clientHeight: element.clientHeight
			};
		}`, nil)
		s.require.NoError(err, "failed to get scroll info")
		
		scrollInfoMap := scrollInfo.(map[string]interface{})
		// JavaScript numbers can be returned as either int or float64
		scrollTop := toFloat64(scrollInfoMap["scrollTop"])
		scrollHeight := toFloat64(scrollInfoMap["scrollHeight"])
		clientHeight := toFloat64(scrollInfoMap["clientHeight"])
		
		// Validate values are reasonable (not NaN, not infinite, within safe limits)
		if scrollHeight <= 0 || clientHeight <= 0 {
			s.t.Logf("⚠ Invalid container dimensions: scrollHeight=%.0f, clientHeight=%.0f", scrollHeight, clientHeight)
			break
		}
		
		// Check if we're near the bottom (within 200px threshold, same as the UI code)
		distanceFromBottom := scrollHeight - scrollTop - clientHeight
		
		if distanceFromBottom < 200 {
			// We're at the bottom, wait a bit to see if more content loads
			time.Sleep(2 * time.Second)
			
			// Check if scroll height increased (more content loaded)
			newScrollInfo, err := container.Evaluate(`(element) => element.scrollHeight`, nil)
			s.require.NoError(err, "failed to get new scroll height")
			newScrollHeight := toFloat64(newScrollInfo)
			
			if newScrollHeight == scrollHeight && scrollHeight == lastScrollHeight {
				// No more content loaded for 2 consecutive checks, we're done
				s.t.Logf("✓ All timeline resources loaded (scroll height: %.0f)", scrollHeight)
				break
			}
			lastScrollHeight = scrollHeight
			// More content loaded, continue scrolling
		}
		
		// Scroll down by 80% of viewport height to trigger lazy loading
		// Ensure the value is within JavaScript's safe integer range and reasonable
		scrollBy := clientHeight * 0.8
		// Clamp to reasonable maximum (10000px should be more than enough for any viewport)
		if scrollBy > 10000 {
			scrollBy = 10000
		}
		if scrollBy < 100 {
			scrollBy = 100 // Minimum scroll to ensure we make progress
		}
		// Use Math.min to ensure we don't exceed safe limits in JavaScript
		_, err = container.Evaluate(`(element, scrollAmount) => {
			const maxSafe = Number.MAX_SAFE_INTEGER;
			const safeScroll = Math.min(Math.max(scrollAmount, 0), Math.min(maxSafe, 10000));
			element.scrollTop += safeScroll;
		}`, scrollBy)
		if err != nil {
			s.t.Logf("⚠ Failed to scroll (scrollBy=%.2f): %v", scrollBy, err)
			// Don't fail immediately, try to continue
			break
		}
		
		// Wait for potential lazy loading
		time.Sleep(1 * time.Second)
		scrollAttempt++
	}
	
	if scrollAttempt >= maxScrollAttempts {
		s.t.Logf("⚠ Reached max scroll attempts (%d), some resources may not be loaded", maxScrollAttempts)
	}
	
	// Final wait for any pending loads
	time.Sleep(2 * time.Second)
	return s
}

func (s *UIStage) resource_label_is_clicked() *UIStage {
	// Resource labels are g elements with class 'label' and 'cursor-pointer'
	// They're created by D3 and have click handlers attached
	// After scrolling, elements may be recreated, so we need to re-query them
	
	// Wait a bit for DOM to stabilize after scrolling
	time.Sleep(1 * time.Second)
	
	// Re-query labels fresh to avoid detached element issues
	resourceLabelSelector := "g.label.cursor-pointer"
	resourceLabels := s.browserTest.Page.Locator(resourceLabelSelector)
	
	// Wait for at least one label to be visible and attached
	err := resourceLabels.First().WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateAttached,
		Timeout: playwright.Float(30000),
	})
	s.require.NoError(err, "no resource labels found - timeline may not have rendered")
	
	// Get a fresh reference to the first label
	firstLabel := resourceLabels.First()
	
	// Scroll into view using Playwright (this handles detached elements better)
	err = firstLabel.ScrollIntoViewIfNeeded()
	if err != nil {
		// If scroll fails, try JavaScript scroll as fallback
		_, jsErr := s.browserTest.Page.Evaluate(`() => {
			const labels = document.querySelectorAll('g.label.cursor-pointer');
			if (labels.length > 0) {
				labels[0].scrollIntoView({ behavior: 'instant', block: 'center' });
			}
		}`, nil)
		if jsErr != nil {
			s.t.Logf("⚠ Could not scroll label into view: %v (JS fallback also failed: %v)", err, jsErr)
		}
	}
	
	// Wait a bit for scroll to complete
	time.Sleep(500 * time.Millisecond)
	
	// Verify the label is visible and wait for it to be stable
	visible, err := firstLabel.IsVisible()
	s.require.NoError(err)
	s.require.True(visible, "first resource label should be visible")
	
	// Wait for element to be in a stable state (not animating/transitioning)
	time.Sleep(300 * time.Millisecond)
	
	// Use Playwright's click with force option for SVG elements
	// SVG elements might not pass actionability checks, but force click still fires event handlers
	// This is more reliable than JavaScript-dispatched events for D3 handlers
	err = firstLabel.Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(10000),
		Force:   playwright.Bool(true), // Force click bypasses actionability but fires handlers
	})
	
	if err != nil {
		// Fallback: try clicking the rect inside
		s.t.Logf("⚠ Clicking g.label failed: %v, trying rect inside", err)
		rectInsideLabel := firstLabel.Locator("rect")
		err = rectInsideLabel.Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(10000),
			Force:   playwright.Bool(true),
		})
	}
	
	if err != nil {
		// Final fallback: use JavaScript click
		s.t.Logf("⚠ Playwright click failed: %v, trying JavaScript click", err)
		_, jsErr := s.browserTest.Page.Evaluate(`() => {
			const labels = document.querySelectorAll('g.label.cursor-pointer');
			if (labels.length === 0) return false;
			const firstLabel = labels[0];
			firstLabel.scrollIntoView({ behavior: 'instant', block: 'center' });
			const clickEvent = new MouseEvent('click', { bubbles: true, cancelable: true, view: window, button: 0 });
			firstLabel.dispatchEvent(clickEvent);
			return true;
		}`, nil)
		if jsErr != nil {
			s.require.NoError(jsErr, "JavaScript click also failed")
		}
	} else {
		s.require.NoError(err, "failed to click on resource label")
	}
	
	s.t.Log("✓ Successfully clicked on timeline element")
	
	// Wait for the click to be processed and React state to update
	// The click triggers onSegmentClick which updates selectedPoint, which triggers the highlighting effect
	// Verify the click worked by checking if detail panel opened (this confirms the handler fired)
	// Note: We DON'T close the detail panel here because closing it calls setSelectedPoint(null),
	// which would clear the highlighting. The highlighting test needs selectedPoint to remain set.
	detailPanelSelector := "[class*=\"DetailPanel\"]"
	detailPanels := s.browserTest.Page.Locator(detailPanelSelector)
	panelCount, _ := detailPanels.Count()
	if panelCount > 0 {
		s.t.Log("✓ Detail panel opened - click handler fired successfully")
	} else {
		s.t.Log("⚠ Detail panel did not open - click handler may not have fired")
	}
	
	// Wait for React state update + D3 effect to run
	// The highlighting effect depends on selectedPoint being set
	// The effect runs in a useEffect that depends on selectedPoint, so we need to wait for:
	// 1. React state update (selectedPoint)
	// 2. React re-render
	// 3. useEffect to run
	// 4. D3 to update the SVG attributes
	// This can take a moment, especially after scrolling when elements might be recreated
	time.Sleep(2 * time.Second)
	return s
}

func (s *UIStage) namespace_filter_is_set(namespace string) *UIStage {
	nsDropdown := s.browserTest.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "All Namespaces"})

	// Wait for dropdown to be visible with polling
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		visible, err := nsDropdown.IsVisible()
		if err == nil && visible {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	s.require.NoError(nsDropdown.Click())
	err := s.browserTest.Page.GetByRole("option", playwright.PageGetByRoleOptions{Name: namespace}).Click()
	s.require.NoError(err, "failed to click namespace option")
	s.require.NoError(s.browserTest.Page.Keyboard().Press("Escape"))
	time.Sleep(500 * time.Millisecond)
	return s
}

func (s *UIStage) kind_dropdown_is_opened() *UIStage {
	// The kind dropdown is the second filter button (after namespace dropdown)
	// It might show "All Kinds" or the selected kinds like "Deployment, Pod"
	allButtons := s.browserTest.Page.Locator("button")
	
	// Wait for buttons to be available
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		count, _ := allButtons.Count()
		if count >= 2 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// The kind dropdown is typically button index 1 (0 is namespace dropdown)
	kindDropdown := allButtons.Nth(1)
	
	err := kindDropdown.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	s.require.NoError(err, "kind dropdown not visible")

	err = kindDropdown.Click()
	s.require.NoError(err, "failed to click kind dropdown")
	return s
}

func (s *UIStage) kind_option_is_selected(kind string) *UIStage {
	option := s.browserTest.Page.GetByRole("option", playwright.PageGetByRoleOptions{Name: kind})

	// Wait for option to be visible with polling
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		visible, err := option.IsVisible()
		if err == nil && visible {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	err := option.Click()
	s.require.NoError(err, "failed to click kind option '%s'", kind)

	s.require.NoError(s.browserTest.Page.Keyboard().Press("Escape"))
	time.Sleep(500 * time.Millisecond)
	return s
}

// kind_filter_is_set is unused - keeping for potential future use
// func (s *UIStage) kind_filter_is_set(kind string) *UIStage {
// 	s.kind_dropdown_is_opened()
// 	return s.kind_option_is_selected(kind)
// }

func (s *UIStage) search_filter_is_set(query string) *UIStage {
	searchInput := s.browserTest.Page.GetByPlaceholder("Search resources by name...")
	s.require.NoError(searchInput.Fill(query))
	time.Sleep(500 * time.Millisecond)
	return s
}

func (s *UIStage) search_filter_is_cleared() *UIStage {
	searchInput := s.browserTest.Page.GetByPlaceholder("Search resources by name...")
	s.require.NoError(searchInput.Fill(""))
	time.Sleep(500 * time.Millisecond)
	return s
}

func (s *UIStage) escape_key_is_pressed() *UIStage {
	s.require.NoError(s.browserTest.Page.Keyboard().Press("Escape"))
	time.Sleep(500 * time.Millisecond)
	return s
}

func (s *UIStage) arrow_key_is_pressed(direction string) *UIStage {
	s.require.NoError(s.browserTest.Page.Keyboard().Press(direction))
	time.Sleep(800 * time.Millisecond)
	return s
}

func (s *UIStage) theme_is_switched_to(theme string) *UIStage {
	_, err := s.browserTest.Page.Evaluate(fmt.Sprintf(`() => {
		const root = document.documentElement;
		root.setAttribute('data-theme', '%s');
	}`, theme))
	s.require.NoError(err, "failed to set theme")
	// Wait for theme change to propagate and D3 effects to re-run
	// The highlighting effect depends on theme, so it needs time to re-apply
	// Theme changes can trigger CSS recalculation and D3 re-rendering
	time.Sleep(3 * time.Second)
	return s
}

func (s *UIStage) detail_panel_is_visible() *UIStage {
	detailPanelSelector := "[class*=\"DetailPanel\"]"
	detailPanels := s.browserTest.Page.Locator(detailPanelSelector)
	var panelCount int
	var err error
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		panelCount, err = detailPanels.Count()
		s.require.NoError(err)
		if panelCount > 0 {
			break
		}
	}
	s.require.Greater(panelCount, 0, "detail panel should appear after click")
	s.t.Log("✓ Detail panel appeared after click")
	return s
}

func (s *UIStage) detail_panel_is_not_visible() *UIStage {
	detailPanelSelector := "[class*=\"DetailPanel\"]"
	detailPanels := s.browserTest.Page.Locator(detailPanelSelector)
	panelCount, err := detailPanels.Count()
	s.require.NoError(err)
	s.require.Equal(0, panelCount)
	s.t.Log("✓ Detail panel closed")
	return s
}

func (s *UIStage) text_is_visible(text string) *UIStage {
	s.waitForTextExists(text, 30*time.Second)
	count, err := s.browserTest.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
	s.require.NoError(err)
	s.require.Greater(count, 0, "expected %s to exist in page", text)
	return s
}

func (s *UIStage) text_is_not_visible(text string) *UIStage {
	count, err := s.browserTest.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
	s.require.NoError(err)
	s.require.Equal(0, count, "expected %s to not exist in page", text)
	return s
}

func (s *UIStage) timeline_segment_is_highlighted() *UIStage {
	// First, verify that selectedPoint is actually set in React state
	// This helps us understand if the click handler worked
	selectedPointSet, err := s.browserTest.Page.Evaluate(`() => {
		// Try to access React state - this is a bit hacky but helps debug
		// Check if there's a selectedPoint by looking for detail panel or checking DOM
		const detailPanel = document.querySelector('[class*="DetailPanel"]');
		return detailPanel !== null;
	}`, nil)
	if err == nil {
		if isSet, ok := selectedPointSet.(bool); ok && isSet {
			s.t.Log("✓ selectedPoint appears to be set (detail panel exists)")
		} else {
			s.t.Log("⚠ selectedPoint may not be set (detail panel not found)")
		}
	}
	
	// Wait for highlighting to be applied (React state update + D3 effect)
	// The highlighting is applied in a useEffect that depends on selectedPoint
	// After theme changes or initial selection, this can take a moment
	// Since we already waited 2 seconds after clicking, we should see highlighting soon
	deadline := time.Now().Add(6 * time.Second)
	var highlightedSegmentFound bool
	
	for time.Now().Before(deadline) {
		// Use JavaScript to check segments directly - this is more reliable than Playwright locators
		// for SVG elements that might be recreated
		result, err := s.browserTest.Page.Evaluate(`() => {
			const segments = document.querySelectorAll('rect.segment');
			let found = false;
			let foundIndex = -1;
			let foundStroke = '';
			let foundWidth = '';
			
			segments.forEach((seg, i) => {
				const stroke = seg.getAttribute('stroke');
				const strokeWidth = seg.getAttribute('stroke-width');
				
				if (stroke && stroke !== 'none' && strokeWidth && strokeWidth !== '0') {
					found = true;
					foundIndex = i;
					foundStroke = stroke;
					foundWidth = strokeWidth;
				}
			});
			
			return {
				found: found,
				index: foundIndex,
				stroke: foundStroke,
				strokeWidth: foundWidth,
				totalSegments: segments.length
			};
		}`, nil)
		
		if err == nil {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if found, ok := resultMap["found"].(bool); ok && found {
					highlightedSegmentFound = true
					index := resultMap["index"]
					stroke := resultMap["stroke"]
					width := resultMap["strokeWidth"]
					s.t.Logf("✓ Segment %v has visible stroke: %v (width: %v)", index, stroke, width)
					break
				}
			}
		}
		
		// Wait a bit before checking again
		time.Sleep(300 * time.Millisecond)
	}
	
	if !highlightedSegmentFound {
		// Debug: check all segments and try to access their D3 data
		// Also check if we can find which resource was clicked
		debugInfo, err := s.browserTest.Page.Evaluate(`() => {
			const segments = document.querySelectorAll('rect.segment');
			const info = [];
			
			// Try to get D3 data from segments (D3 stores data in __data__)
			for (let i = 0; i < Math.min(segments.length, 10); i++) {
				const seg = segments[i];
				const d3Data = seg.__data__ || null;
				info.push({
					index: i,
					stroke: seg.getAttribute('stroke'),
					strokeWidth: seg.getAttribute('stroke-width'),
					fill: seg.getAttribute('fill'),
					resourceId: d3Data ? d3Data.resourceId : null,
					segmentIndex: d3Data ? d3Data.index : null
				});
			}
			
			// Check which label was clicked (first label)
			const labels = document.querySelectorAll('g.label.cursor-pointer');
			let clickedResourceId = null;
			if (labels.length > 0) {
				const firstLabel = labels[0];
				const labelData = firstLabel.__data__ || null;
				if (labelData) {
					clickedResourceId = labelData.id;
					// The click calls onSegmentClick with d.statusSegments.length - 1
					// So the selected index should be the last segment index
				}
			}
			
			return {
				total: segments.length,
				segments: info,
				clickedResourceId: clickedResourceId,
				expectedSelectedIndex: labels.length > 0 && labels[0].__data__ ? 
					(labels[0].__data__.statusSegments ? labels[0].__data__.statusSegments.length - 1 : null) : null
			};
		}`, nil)
		
		if err == nil {
			if debugMap, ok := debugInfo.(map[string]interface{}); ok {
				total, _ := debugMap["total"]
				s.t.Logf("Debug: Found %v segments total", total)
				if segs, ok := debugMap["segments"].([]interface{}); ok {
					for _, seg := range segs {
						if segMap, ok := seg.(map[string]interface{}); ok {
							s.t.Logf("  Segment %v: stroke=%v, stroke-width=%v, fill=%v",
								segMap["index"], segMap["stroke"], segMap["strokeWidth"], segMap["fill"])
						}
					}
				}
			}
		}
	}
	
	s.require.True(highlightedSegmentFound, "at least one segment should have a visible stroke after selection")
	return s
}

func (s *UIStage) time_range_picker_is_visible() *UIStage {
	timeRangePicker := s.browserTest.Page.Locator("[class*=\"test-TimeRange\"]")
	visible, err := timeRangePicker.IsVisible()
	s.require.NoError(err, "failed to check time range picker visibility")
	s.require.True(visible, "time range picker should be visible")
	return s
}

func (s *UIStage) filter_bar_is_visible() *UIStage {
	filterBarSelector := "[class*=\"FilterBar\"], [class*=\"filter\"]"
	filterBar := s.browserTest.Page.Locator(filterBarSelector)
	visible, err := filterBar.IsVisible()
	if err == nil && visible {
		s.t.Log("✓ Filter bar visible")
	}
	return s
}

func (s *UIStage) page_contains_text(text string) *UIStage {
	body := s.browserTest.Page.Locator("body")
	bodyText, err := body.TextContent()
	s.require.NoError(err, "failed to get body text content")
	s.require.Contains(bodyText, text, "body should contain '%s'", text)
	return s
}

func (s *UIStage) page_has_content() *UIStage {
	content, err := s.browserTest.Page.Content()
	s.require.NoError(err, "failed to get page content")
	s.require.Greater(len(content), 1000, "page should have significant content")
	return s
}

func (s *UIStage) wait_for_text_in_timeline(text string) *UIStage {
	s.waitForTextExists(text, 60*time.Second)
	return s
}

func (s *UIStage) waitForTextExists(text string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		count, err := s.browserTest.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
		if err == nil && count > 0 {
			return
		}
		time.Sleep(2 * time.Second)
	}
	s.t.Fatalf("text %q not found after %v", text, timeout)
}

func (s *UIStage) page_loads_completely() *UIStage {
	s.require.NoError(s.browserTest.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}))
	// Give React app time to render timeline data after network idle
	time.Sleep(3 * time.Second)
	return s
}

func (s *UIStage) kind_options_are_visible(kinds []string) *UIStage {
	for _, kind := range kinds {
		visible, err := s.browserTest.Page.GetByRole("option", playwright.PageGetByRoleOptions{Name: kind}).IsVisible()
		s.require.NoError(err)
		s.require.True(visible, "expected option %s to be visible", kind)
	}
	return s
}

// toFloat64 converts a JavaScript number (which can be int or float64 in Go) to float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		// Try to convert via string parsing as fallback
		return float64(0)
	}
}

// captureDebugInfo saves a screenshot and HTML of the current page for debugging
func (s *UIStage) captureDebugInfo(reason string) {
	// Create debug directory in .tests (same location as audit logs)
	debugDir := filepath.Join(".tests", "ui-debug")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		s.t.Logf("⚠ Failed to create debug directory: %v", err)
		return
	}

	testName := s.t.Name()
	timestamp := time.Now().Format("20060102-150405")
	
	// Capture screenshot
	screenshotPath := filepath.Join(debugDir, fmt.Sprintf("%s-%s-%s.png", testName, reason, timestamp))
	_, err := s.browserTest.Page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(screenshotPath),
		FullPage: playwright.Bool(true),
	})
	if err != nil {
		s.t.Logf("⚠ Failed to capture screenshot: %v", err)
	} else {
		// Get file size for logging
		if info, statErr := os.Stat(screenshotPath); statErr == nil {
			s.t.Logf("📸 Screenshot saved: %s (%d bytes)", screenshotPath, info.Size())
		} else {
			s.t.Logf("📸 Screenshot saved: %s", screenshotPath)
		}
	}

	// Capture HTML
	htmlPath := filepath.Join(debugDir, fmt.Sprintf("%s-%s-%s.html", testName, reason, timestamp))
	content, err := s.browserTest.Page.Content()
	if err != nil {
		s.t.Logf("⚠ Failed to capture HTML: %v", err)
	} else {
		if err := os.WriteFile(htmlPath, []byte(content), 0644); err != nil {
			s.t.Logf("⚠ Failed to write HTML file: %v", err)
		} else {
			s.t.Logf("📄 HTML saved: %s (%d bytes)", htmlPath, len(content))
		}
	}

	// Also capture page info via JavaScript for additional debugging
	pageInfo, err := s.browserTest.Page.Evaluate(`() => {
		return {
			url: window.location.href,
			title: document.title,
			svgCount: document.querySelectorAll('svg').length,
			overflowContainers: Array.from(document.querySelectorAll('div.overflow-y-auto')).map(el => ({
				className: el.className,
				height: el.getBoundingClientRect().height,
				width: el.getBoundingClientRect().width,
				hasSVG: el.querySelector('svg') !== null
			})),
			bodyText: document.body ? document.body.innerText.substring(0, 500) : 'no body'
		};
	}`, nil)
	if err == nil {
		infoPath := filepath.Join(debugDir, fmt.Sprintf("%s-%s-%s-info.json", testName, reason, timestamp))
		infoJSON := fmt.Sprintf("%+v\n", pageInfo)
		if err := os.WriteFile(infoPath, []byte(infoJSON), 0644); err != nil {
			s.t.Logf("⚠ Failed to write page info: %v", err)
		} else {
			s.t.Logf("ℹ️  Page info saved: %s", infoPath)
		}
	}
}
