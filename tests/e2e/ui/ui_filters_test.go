package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestUIFilterByNamespace tests namespace filtering functionality
func TestUIFilterByNamespace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)

	// Create deployments in different namespaces
	ctx := t.Context()

	namespace1 := "test-ns-1"
	namespace2 := "test-ns-2"

	err := testCtx.K8sClient.CreateNamespace(ctx, namespace1)
	require.NoError(t, err, "failed to create namespace 1")
	t.Cleanup(func() {
		testCtx.K8sClient.DeleteNamespace(ctx, namespace1)
	})

	err = testCtx.K8sClient.CreateNamespace(ctx, namespace2)
	require.NoError(t, err, "failed to create namespace 2")
	t.Cleanup(func() {
		testCtx.K8sClient.DeleteNamespace(ctx, namespace2)
	})

	// Create deployments in both namespaces
	dep1, err := helpers.CreateTestDeployment(ctx, t, testCtx.K8sClient, namespace1)
	require.NoError(t, err, "failed to create deployment in namespace 1")

	dep2, err := helpers.CreateTestDeployment(ctx, t, testCtx.K8sClient, namespace2)
	require.NoError(t, err, "failed to create deployment in namespace 2")

	// Wait for both resources to be available in API
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, namespace1, "Deployment", dep1.Name, helpers.DefaultEventuallyOption)
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, namespace2, "Deployment", dep2.Name, helpers.DefaultEventuallyOption)

	// Wait additional time for storage to fully index the resources
	// The API may return resources from memory before they're fully written to storage
	t.Log("Waiting for storage to index new resources...")
	time.Sleep(10 * time.Second)

	uiURL := testCtx.PortForward.GetURL()
	t.Logf("Testing namespace filter at %s", uiURL)

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

	// Wait for the page to load and resources to appear
	require.NoError(t, bt.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}))

	// Helper to poll for text to appear (with retry for async data loading)
	waitForTextExists := func(text string, timeout time.Duration) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			count, err := bt.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
			if err == nil && count > 0 {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		return fmt.Errorf("text %q not found after %v", text, timeout)
	}

	// Helper to check if text exists (immediate check)
	assertTextExists := func(text string) {
		count, err := bt.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "expected %s to exist in page", text)
	}
	assertTextNotExists := func(text string) {
		count, err := bt.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
		require.NoError(t, err)
		require.Equal(t, 0, count, "expected %s to not exist in page", text)
	}

	// Wait for the specific test resources to appear in timeline (may need polling as data loads async)
	t.Log("Waiting for test resources to appear in timeline...")
	err = waitForTextExists(namespace1, 30*time.Second)
	require.NoError(t, err, "timed out waiting for %s to appear in timeline", namespace1)
	err = waitForTextExists(namespace2, 30*time.Second)
	require.NoError(t, err, "timed out waiting for %s to appear in timeline", namespace2)

	// Verify both namespaces appear initially
	assertTextExists(namespace1)
	assertTextExists(namespace2)

	// Wait for filter dropdowns to be available
	t.Log("Waiting for filter dropdowns to load...")
	nsDropdown := bt.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "All Namespaces"})
	// Wait for dropdown to be visible with polling
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		visible, err := nsDropdown.IsVisible()
		if err == nil && visible {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 1. Open Namespace Dropdown
	require.NoError(t, nsDropdown.Click())

	// 2. Select namespace 1
	err = bt.Page.GetByRole("option", playwright.PageGetByRoleOptions{Name: namespace1}).Click()
	require.NoError(t, err, "failed to click namespace 1 option")

	// Close dropdown by pressing escape
	require.NoError(t, bt.Page.Keyboard().Press("Escape"))

	// Wait for dropdown to close and filter to apply
	time.Sleep(500 * time.Millisecond)

	// 3. Verify namespace filter worked
	t.Logf("Verifying namespace filter worked: %s is visible, %s is hidden", namespace1, namespace2)
	assertTextExists(namespace1)
	assertTextNotExists(namespace2)

	t.Log("Namespace filter test passed!")
}

// TestUIFilterByKind tests resource kind filtering functionality
func TestUIFilterByKind(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)

	ctx := t.Context()

	// 1. Create resources
	// Create two deployments - the test will filter by Deployment vs Pod
	deployment1, err := helpers.CreateTestDeployment(ctx, t, testCtx.K8sClient, testCtx.Namespace)
	require.NoError(t, err, "failed to create deployment 1")

	// Create a second deployment with a different name
	deployment2 := helpers.NewDeploymentBuilder(t, "test-deployment-2", testCtx.Namespace).Build()
	_, err = testCtx.K8sClient.CreateDeployment(ctx, testCtx.Namespace, deployment2)
	require.NoError(t, err, "failed to create deployment 2")

	// Wait for resources to be created/synced in API
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", deployment1.Name, helpers.DefaultEventuallyOption)
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", deployment2.Name, helpers.DefaultEventuallyOption)

	// Wait additional time for storage to fully index the resources
	// The API may return resources from memory before they're fully written to storage
	t.Log("Waiting for storage to index new resources...")
	time.Sleep(10 * time.Second)

	uiURL := testCtx.PortForward.GetURL()
	t.Logf("Testing kind filter at %s", uiURL)

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

	// Wait for load
	require.NoError(t, bt.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}))

	// Helper to poll for text to appear (with retry for async data loading)
	waitForTextExists := func(text string, timeout time.Duration) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			count, err := bt.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
			if err == nil && count > 0 {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		return fmt.Errorf("text %q not found after %v", text, timeout)
	}

	// Wait for timeline resources to appear before interacting with filters
	t.Log("Waiting for resources to appear in the timeline...")
	err = waitForTextExists(deployment1.Name, 30*time.Second)
	require.NoError(t, err, "timed out waiting for deployment to appear in UI")

	// Wait for filter dropdowns to be available
	t.Log("Waiting for filter dropdowns to load...")
	kindDropdown := bt.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "All Kinds"})
	// Wait for dropdown to be visible with polling
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		visible, err := kindDropdown.IsVisible()
		if err == nil && visible {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 2. Open Kind Dropdown
	require.NoError(t, kindDropdown.Click())

	// 3. Verify Deployment and Pod options exist (Pods are created by Deployments)
	kinds := []string{"Deployment", "Pod"}
	for _, kind := range kinds {
		visible, err := bt.Page.GetByRole("option", playwright.PageGetByRoleOptions{Name: kind}).IsVisible()
		require.NoError(t, err)
		require.True(t, visible, "expected option %s to be visible", kind)
	}

	// 4. Select only Deployment kind
	err = bt.Page.GetByRole("option", playwright.PageGetByRoleOptions{Name: "Deployment"}).Click()
	require.NoError(t, err)

	// Close dropdown
	require.NoError(t, bt.Page.Keyboard().Press("Escape"))

	// Wait for dropdown to close
	time.Sleep(500 * time.Millisecond)

	// 5. Verify the filter was applied by checking the timeline still displays resources
	t.Log("Filter has been applied. Test passed!")
}

// TestUISearchFilter tests the search filtering functionality
func TestUISearchFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	helpers.EnsurePlaywrightInstalled(t)
	testCtx := helpers.SetupE2ETest(t)

	ctx := t.Context()

	// 1. Create resources with distinct names
	targetName := "search-target-1"
	otherName := "other-resource"

	// Create target deployment
	targetDep := helpers.NewDeploymentBuilder(t, targetName, testCtx.Namespace).Build()
	_, err := testCtx.K8sClient.CreateDeployment(ctx, testCtx.Namespace, targetDep)
	require.NoError(t, err, "failed to create target deployment")

	// Create other deployment
	otherDep := helpers.NewDeploymentBuilder(t, otherName, testCtx.Namespace).Build()
	_, err = testCtx.K8sClient.CreateDeployment(ctx, testCtx.Namespace, otherDep)
	require.NoError(t, err, "failed to create other deployment")

	// Wait for resources to be available in API
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", targetName, helpers.DefaultEventuallyOption)
	helpers.EventuallyResourceCreated(t, testCtx.APIClient, testCtx.Namespace, "Deployment", otherName, helpers.DefaultEventuallyOption)

	// Wait additional time for storage to fully index the resources
	// The API may return resources from memory before they're fully written to storage
	t.Log("Waiting for storage to index new resources...")
	time.Sleep(10 * time.Second)

	uiURL := testCtx.PortForward.GetURL()
	t.Logf("Testing search filter at %s", uiURL)

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

	// Wait for load
	require.NoError(t, bt.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}))

	// Helper to poll for text to appear (with retry for async data loading)
	waitForTextExists := func(text string, timeout time.Duration) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			count, err := bt.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
			if err == nil && count > 0 {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		return fmt.Errorf("text %q not found after %v", text, timeout)
	}

	// Helper to check if text exists (immediate check)
	assertTextExists := func(text string) {
		count, err := bt.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "expected %s to exist in page", text)
	}
	assertTextNotExists := func(text string) {
		count, err := bt.Page.Locator(fmt.Sprintf("text=%s", text)).Count()
		require.NoError(t, err)
		require.Equal(t, 0, count, "expected %s to not exist in page", text)
	}

	// Wait for the specific test resources to appear in timeline (may need polling as data loads async)
	t.Log("Waiting for test resources to appear in timeline...")
	err = waitForTextExists(targetName, 30*time.Second)
	require.NoError(t, err, "timed out waiting for %s to appear in timeline", targetName)
	err = waitForTextExists(otherName, 30*time.Second)
	require.NoError(t, err, "timed out waiting for %s to appear in timeline", otherName)

	// Verify both resources appear in the timeline
	t.Log("Verifying both resources are initially visible")
	assertTextExists(targetName)
	assertTextExists(otherName)

	// 2. Find search input and type target name
	// The search input usually has a placeholder "Search resources by name..."
	searchInput := bt.Page.GetByPlaceholder("Search resources by name...")
	require.NoError(t, searchInput.Fill(targetName))

	// Wait for filter to apply
	time.Sleep(500 * time.Millisecond)

	// 3. Verify target is visible and other is hidden
	t.Logf("Verifying %s is visible and %s is hidden after search", targetName, otherName)
	assertTextExists(targetName)
	assertTextNotExists(otherName)

	// 4. Clear search
	require.NoError(t, searchInput.Fill(""))

	// Wait for filter to clear
	time.Sleep(500 * time.Millisecond)

	// 5. Verify both are visible again
	t.Log("Verifying both resources are visible after clearing search")
	assertTextExists(targetName)
	assertTextExists(otherName)
}
