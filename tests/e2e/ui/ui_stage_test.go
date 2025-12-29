package e2e

import (
	"context"
	"fmt"
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

func (s *UIStage) resource_label_is_clicked() *UIStage {
	resourceLabelSelector := "g.label"
	resourceLabels := s.browserTest.Page.Locator(resourceLabelSelector)
	count, err := resourceLabels.Count()
	s.require.NoError(err)
	s.require.Greater(count, 0, "expected at least one resource label to be visible")

	firstLabel := resourceLabels.First()
	visible, err := firstLabel.IsVisible()
	s.require.NoError(err)
	s.require.True(visible, "first resource label should be visible")

	err = firstLabel.Click()
	s.require.NoError(err, "failed to click on resource label")
	s.t.Log("✓ Successfully clicked on timeline element")
	time.Sleep(1 * time.Second)
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
	time.Sleep(1 * time.Second)
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
	segmentSelector := "rect.segment"
	segments := s.browserTest.Page.Locator(segmentSelector)
	segmentCount, err := segments.Count()
	s.require.NoError(err)
	s.require.Greater(segmentCount, 0, "expected at least one segment to be visible")

	var highlightedSegmentFound bool
	for i := 0; i < segmentCount; i++ {
		segment := segments.Nth(i)
		strokeAttr, err := segment.GetAttribute("stroke")
		s.require.NoError(err, "failed to get stroke attribute for segment %d", i)

		if strokeAttr != "" && strokeAttr != "none" {
			highlightedSegmentFound = true
			s.t.Logf("✓ Segment %d has visible stroke: %s", i, strokeAttr)
			break
		}
	}
	s.require.True(highlightedSegmentFound, "at least one segment should have a visible stroke")
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
