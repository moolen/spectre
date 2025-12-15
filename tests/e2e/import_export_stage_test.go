package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ImportExportStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	k8sClient *helpers.K8sClient
	apiClient *helpers.APIClient

	// Test data
	testNamespaces  []string
	deploymentNames map[string][]string
	events          []*models.Event
	baseTime        time.Time

	// Export/Import
	exportPath      string
	exportTimestamp int64
	helmDeployer    *helpers.HelmDeployer

	// Verification
	resourceID       string
	timelineResource *helpers.Resource
	kubernetesEvents []*models.Event
	involvedPodUIDs  []string

	// CLI import on startup
	spectreNamespace string
	testCluster      *helpers.TestCluster
	configMapName    string
}

func NewImportExportStage(t *testing.T) (*ImportExportStage, *ImportExportStage, *ImportExportStage) {
	s := &ImportExportStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *ImportExportStage) and() *ImportExportStage {
	return s
}

func (s *ImportExportStage) a_test_environment() *ImportExportStage {
	s.testCtx = helpers.SetupE2ETest(s.t)
	s.k8sClient = s.testCtx.K8sClient
	s.apiClient = s.testCtx.APIClient
	return s
}

// Test data generation methods

func (s *ImportExportStage) test_data_in_two_namespaces() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Minute)
	defer cancel()

	s.testNamespaces = []string{"import-1", "import-2"}
	s.deploymentNames = make(map[string][]string)

	for _, ns := range s.testNamespaces {
		err := s.k8sClient.CreateNamespace(ctx, ns)
		s.require.NoError(err, "failed to create namespace %s", ns)

		s.deploymentNames[ns] = []string{}

		// Create 25 deployments per namespace (50 total)
		for i := 0; i < 25; i++ {
			deployName := fmt.Sprintf("import-deploy-%d", i)
			deployment := helpers.NewDeploymentBuilder(s.t, deployName, ns).
				WithImage("nginx:latest").
				WithReplicas(1).
				Build()

			_, err := s.k8sClient.CreateDeployment(ctx, ns, deployment)
			s.require.NoError(err, "failed to create deployment %s in namespace %s", deployName, ns)

			s.deploymentNames[ns] = append(s.deploymentNames[ns], deployName)
		}

		s.t.Logf("Created 25 deployments in namespace %s", ns)
	}

	return s
}

func (s *ImportExportStage) generated_test_events_for_multiple_namespaces() *ImportExportStage {
	s.baseTime = time.Now()
	s.testNamespaces = []string{"e2e-import-json-1", "e2e-import-json-2", "e2e-import-json-3", "e2e-import-json-4"}
	s.events = s.generateTestEvents(s.baseTime, s.testNamespaces)
	s.t.Logf("Generated %d test events across %d namespaces", len(s.events), len(s.testNamespaces))
	return s
}

func (s *ImportExportStage) generated_service_with_timeline_events() *ImportExportStage {
	s.baseTime = time.Now()
	s.testNamespaces = []string{"e2e-timeline-test"}
	serviceName := "test-service-timeline"
	serviceUID := uuid.New().String()
	s.events = []*models.Event{}

	// Create initial CREATE event
	createEvent := &models.Event{
		ID:        uuid.New().String(),
		Timestamp: s.baseTime.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Service",
			Namespace: s.testNamespaces[0],
			Name:      serviceName,
			UID:       serviceUID,
		},
		Data: []byte(fmt.Sprintf(
			`{"apiVersion":"v1","kind":"Service","metadata":{"name":%q,"namespace":%q,"uid":%q},"spec":{"ports":[{"port":80,"targetPort":8080}],"selector":{"app":"test"}}}`,
			serviceName, s.testNamespaces[0], serviceUID,
		)),
	}
	createEvent.DataSize = int32(len(createEvent.Data))
	s.events = append(s.events, createEvent)

	// Create 10 UPDATE events with 5-30 seconds between them
	baseInterval := 5 * time.Second
	for i := 0; i < 10; i++ {
		interval := baseInterval + time.Duration(i*2)*time.Second
		timestamp := s.baseTime.Add(interval)

		updateEvent := &models.Event{
			ID:        uuid.New().String(),
			Timestamp: timestamp.UnixNano(),
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Group:     "",
				Version:   "v1",
				Kind:      "Service",
				Namespace: s.testNamespaces[0],
				Name:      serviceName,
				UID:       serviceUID,
			},
			Data: []byte(fmt.Sprintf(
				`{"apiVersion":"v1","kind":"Service","metadata":{"name":%q,"namespace":%q,"uid":%q,"resourceVersion":"%d"},"spec":{"ports":[{"port":80,"targetPort":%d}],"selector":{"app":"test","version":"v%d"}}}`,
				serviceName, s.testNamespaces[0], serviceUID, i+2, 8080+i, i+1,
			)),
		}
		updateEvent.DataSize = int32(len(updateEvent.Data))
		s.events = append(s.events, updateEvent)
	}

	s.resourceID = serviceUID
	s.t.Logf("Generated %d events (1 CREATE + 10 UPDATE) for Service %s/%s", len(s.events), s.testNamespaces[0], serviceName)
	return s
}

func (s *ImportExportStage) generated_test_events_with_kubernetes_events() *ImportExportStage {
	s.baseTime = time.Now()
	s.testNamespaces = []string{"e2e-k8s-events-1", "e2e-k8s-events-2"}
	s.events = []*models.Event{}
	s.kubernetesEvents = []*models.Event{}
	s.involvedPodUIDs = []string{}

	// Generate regular resources
	kinds := []string{"Deployment", "Pod", "Service"}

	for nsIdx, ns := range s.testNamespaces {
		for kindIdx, kind := range kinds {
			resourceUID := fmt.Sprintf("uid-%s-%d-%d", ns, nsIdx, kindIdx)
			resourceName := fmt.Sprintf("test-%s-%d", strings.ToLower(kind), kindIdx)

			// Create a regular resource event
			timestamp := s.baseTime.Add(time.Duration(kindIdx*10) * time.Second)
			event := &models.Event{
				ID:        uuid.New().String(),
				Timestamp: timestamp.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Group:     "apps",
					Version:   "v1",
					Kind:      kind,
					Namespace: ns,
					Name:      resourceName,
					UID:       resourceUID,
				},
				Data: []byte(fmt.Sprintf(
					`{"apiVersion":"apps/v1","kind":%q,"metadata":{"name":%q,"namespace":%q,"uid":%q}}`,
					kind, resourceName, ns, resourceUID,
				)),
			}
			event.DataSize = int32(len(event.Data))
			s.events = append(s.events, event)

			// For Pod resources, create Kubernetes Events that reference them
			if kind == "Pod" {
				s.involvedPodUIDs = append(s.involvedPodUIDs, resourceUID)

				// Create a Kubernetes Event for this Pod
				eventName := fmt.Sprintf("pod-event-%d", kindIdx)
				eventUID := uuid.New().String()
				eventTimestamp := timestamp.Add(5 * time.Second)

				kubeEvent := &models.Event{
					ID:        uuid.New().String(),
					Timestamp: eventTimestamp.UnixNano(),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "Event",
						Namespace: ns,
						Name:      eventName,
						UID:       eventUID,
						// NOTE: InvolvedObjectUID should be populated by enrichEventsWithInvolvedObjectUID
					},
					Data: []byte(fmt.Sprintf(
						`{"apiVersion":"v1","kind":"Event","metadata":{"name":%q,"namespace":%q,"uid":%q},"involvedObject":{"kind":"Pod","name":%q,"namespace":%q,"uid":%q},"reason":"Started","message":"Container started","type":"Normal"}`,
						eventName, ns, eventUID, resourceName, ns, resourceUID,
					)),
				}
				kubeEvent.DataSize = int32(len(kubeEvent.Data))
				s.events = append(s.events, kubeEvent)
				s.kubernetesEvents = append(s.kubernetesEvents, kubeEvent)
			}
		}
	}

	s.t.Logf("Generated %d total events including %d Kubernetes Events across %d namespaces",
		len(s.events), len(s.kubernetesEvents), len(s.testNamespaces))
	return s
}

// Indexing and waiting methods

func (s *ImportExportStage) resources_are_indexed() *ImportExportStage {
	s.t.Log("Waiting for resources to be indexed")
	time.Sleep(10 * time.Second)

	for _, ns := range s.testNamespaces {
		helpers.EventuallyCondition(s.t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
			defer searchCancel()

			now := time.Now().Unix()
			resp, err := s.apiClient.Search(searchCtx, now-300, now+10, ns, "Deployment")
			if err != nil {
				s.t.Logf("Search failed for namespace %s: %v", ns, err)
				return false
			}

			s.t.Logf("Found %d resources in namespace %s", resp.Count, ns)
			return resp.Count > 0
		}, helpers.SlowEventuallyOption)
	}

	s.t.Log("✓ Test data successfully indexed")
	return s
}

func (s *ImportExportStage) wait_for_data_indexing() *ImportExportStage {
	time.Sleep(3 * time.Second)
	return s
}

// Export/Import methods

func (s *ImportExportStage) data_is_exported_to_file() *ImportExportStage {
	s.exportPath = filepath.Join(s.t.TempDir(), "export.tar.gz")

	now := time.Now().Unix()
	s.exportTimestamp = now
	exportURL := fmt.Sprintf("%s/v1/storage/export?from=%d&to=%d&include_open_hour=true&compression=true",
		s.apiClient.BaseURL, now-900, now+60)

	exportResp, err := http.Get(exportURL)
	s.require.NoError(err, "failed to request export")
	s.require.Equal(http.StatusOK, exportResp.StatusCode, "export request failed")

	exportFile, err := os.Create(s.exportPath)
	s.require.NoError(err, "failed to create export file")

	written, err := io.Copy(exportFile, exportResp.Body)
	exportResp.Body.Close()
	exportFile.Close()
	s.require.NoError(err, "failed to write export data")
	s.require.Greater(written, int64(0), "export file is empty")

	s.t.Logf("✓ Exported %d bytes to %s", written, s.exportPath)
	return s
}

func (s *ImportExportStage) data_is_imported_from_binary_file() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Minute)
	defer cancel()

	exportFile, err := os.Open(s.exportPath)
	s.require.NoError(err, "failed to open export file")
	defer exportFile.Close()

	importURL := fmt.Sprintf("%s/v1/storage/import?validate=true&overwrite=true", s.apiClient.BaseURL)
	importReq, err := http.NewRequestWithContext(ctx, "POST", importURL, exportFile)
	s.require.NoError(err, "failed to create import request")
	importReq.Header.Set("Content-Type", "application/vnd.spectre.events.v1+bin")

	importResp, err := http.DefaultClient.Do(importReq)
	s.require.NoError(err, "failed to execute import request")
	defer importResp.Body.Close()

	var importReport map[string]interface{}
	err = json.NewDecoder(importResp.Body).Decode(&importReport)
	s.require.NoError(err, "failed to decode import response")
	s.t.Logf("Import report: %+v", importReport)

	s.require.Equal(http.StatusOK, importResp.StatusCode, "import request failed")

	if failedFiles, ok := importReport["failed_files"].(float64); ok {
		s.assert.Equal(float64(0), failedFiles, "Import should have no failed files")
	}

	if totalEvents, ok := importReport["total_events"].(float64); ok {
		s.assert.Greater(totalEvents, float64(0), "Import should have imported events")
		s.t.Logf("✓ Imported %.0f events", totalEvents)
	}

	return s
}

func (s *ImportExportStage) events_are_imported_via_json() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Minute)
	defer cancel()

	importPayload := map[string]interface{}{
		"events": s.events,
	}

	payloadJSON, err := json.Marshal(importPayload)
	s.require.NoError(err, "failed to marshal import payload")

	importURL := fmt.Sprintf("%s/v1/storage/import?validate=true&overwrite=true", s.apiClient.BaseURL)
	importReq, err := http.NewRequestWithContext(ctx, "POST", importURL, bytes.NewReader(payloadJSON))
	s.require.NoError(err, "failed to create import request")
	importReq.Header.Set("Content-Type", "application/vnd.spectre.events.v1+json")

	importResp, err := http.DefaultClient.Do(importReq)
	s.require.NoError(err, "failed to execute import request")
	defer importResp.Body.Close()

	s.require.Equal(http.StatusOK, importResp.StatusCode, "import request failed with status %d", importResp.StatusCode)

	var importReport map[string]interface{}
	err = json.NewDecoder(importResp.Body).Decode(&importReport)
	s.require.NoError(err, "failed to decode import response")
	s.t.Logf("Import report: %+v", importReport)

	if totalEvents, ok := importReport["total_events"].(float64); ok {
		s.assert.Greater(totalEvents, float64(0), "Import should have imported events")
		s.t.Logf("✓ Imported %.0f events", totalEvents)
	}

	return s
}

// Spectre lifecycle methods

func (s *ImportExportStage) spectre_is_uninstalled() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Minute)
	defer cancel()

	var err error
	s.helmDeployer, err = helpers.NewHelmDeployer(s.t, s.testCtx.Cluster.GetKubeConfig(), s.testCtx.Namespace)
	s.require.NoError(err, "failed to create Helm deployer")

	err = s.helmDeployer.UninstallChart(s.testCtx.ReleaseName)
	s.require.NoError(err, "failed to uninstall Helm release")

	s.t.Log("Waiting for resources to be cleaned up")
	time.Sleep(10 * time.Second)

	// Verify PVC is gone
	pvcName := s.testCtx.ReleaseName + "-spectre"
	_, err = s.k8sClient.Clientset.CoreV1().PersistentVolumeClaims(s.testCtx.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
	s.assert.True(apierrors.IsNotFound(err), "PVC should be deleted after uninstall")

	s.t.Log("✓ Spectre uninstalled and PVC cleaned up")
	return s
}

func (s *ImportExportStage) test_resources_are_deleted() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Minute)
	defer cancel()

	for _, ns := range s.testNamespaces {
		err := s.k8sClient.DeleteNamespace(ctx, ns)
		s.require.NoError(err, "failed to delete namespace %s", ns)
	}

	// Wait for namespaces to be fully deleted
	for _, ns := range s.testNamespaces {
		helpers.EventuallyCondition(s.t, func() bool {
			_, err := s.k8sClient.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, helpers.SlowEventuallyOption)
	}

	s.t.Log("✓ Test namespaces deleted")
	return s
}

func (s *ImportExportStage) spectre_is_redeployed() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Minute)
	defer cancel()

	values, imageRef, err := helpers.LoadHelmValues()
	s.require.NoError(err, "failed to load Helm values")

	err = helpers.BuildAndLoadTestImage(s.t, s.testCtx.Cluster.Name, imageRef)
	s.require.NoError(err, "failed to build/load image")

	chartPath, err := helpers.RepoPath("chart")
	s.require.NoError(err, "failed to get chart path")

	err = s.helmDeployer.InstallOrUpgrade(s.testCtx.ReleaseName, chartPath, values)
	s.require.NoError(err, "failed to reinstall Helm release")

	s.t.Log("Waiting for Spectre to be ready after redeployment")
	err = helpers.WaitForAppReady(ctx, s.k8sClient, s.testCtx.Namespace, s.testCtx.ReleaseName)
	s.require.NoError(err, "Spectre not ready after redeployment")

	err = s.testCtx.ReconnectPortForward()
	s.require.NoError(err, "failed to reconnect port-forward")
	s.apiClient = s.testCtx.APIClient

	s.t.Log("✓ Spectre redeployed and ready")
	return s
}

func (s *ImportExportStage) old_data_is_not_present() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Minute)
	defer cancel()

	time.Sleep(5 * time.Second)

	metadata, err := s.apiClient.GetMetadata(ctx, nil, nil)
	s.require.NoError(err, "failed to get metadata")

	for _, ns := range s.testNamespaces {
		s.assert.NotContains(metadata.Namespaces, ns, "Namespace %s should not be in metadata before import", ns)
	}

	for _, ns := range s.testNamespaces {
		searchResp, err := s.apiClient.Search(ctx, s.exportTimestamp-900, s.exportTimestamp+60, ns, "Deployment")
		s.require.NoError(err, "search failed for namespace %s", ns)
		s.assert.Equal(0, searchResp.Count, "Should find no resources in namespace %s before import", ns)
	}

	s.t.Log("✓ Confirmed old data is not present")
	return s
}

// Verification methods

func (s *ImportExportStage) namespaces_appear_in_metadata() *ImportExportStage {
	helpers.EventuallyCondition(s.t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer metadataCancel()

		metadata, err := s.apiClient.GetMetadata(metadataCtx, nil, nil)
		if err != nil {
			s.t.Logf("GetMetadata failed: %v", err)
			return false
		}

		for _, ns := range s.testNamespaces {
			found := false
			for _, metaNs := range metadata.Namespaces {
				if metaNs == ns {
					found = true
					break
				}
			}
			if !found {
				s.t.Logf("Namespace %s not yet in metadata", ns)
				return false
			}
		}

		return true
	}, helpers.SlowEventuallyOption)

	s.t.Log("✓ All test namespaces appear in metadata")
	return s
}

func (s *ImportExportStage) expected_resource_kinds_are_present() *ImportExportStage {
	helpers.EventuallyCondition(s.t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer metadataCancel()

		metadata, err := s.apiClient.GetMetadata(metadataCtx, nil, nil)
		if err != nil {
			s.t.Logf("GetMetadata failed: %v", err)
			return false
		}

		expectedKinds := map[string]bool{
			"Deployment": false,
			"Pod":        false,
			"Service":    false,
			"ConfigMap":  false,
		}

		for _, kind := range metadata.Kinds {
			if _, exists := expectedKinds[kind]; exists {
				expectedKinds[kind] = true
			}
		}

		allFound := true
		for kind, found := range expectedKinds {
			if !found {
				s.t.Logf("Kind %s not yet in metadata", kind)
				allFound = false
			}
		}

		return allFound
	}, helpers.SlowEventuallyOption)

	s.t.Log("✓ All expected resource kinds appear in metadata")
	return s
}

func (s *ImportExportStage) service_kind_is_present() *ImportExportStage {
	helpers.EventuallyCondition(s.t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer metadataCancel()

		metadata, err := s.apiClient.GetMetadata(metadataCtx, nil, nil)
		if err != nil {
			s.t.Logf("GetMetadata failed: %v", err)
			return false
		}

		for _, kind := range metadata.Kinds {
			if kind == "Service" {
				s.t.Logf("✓ Service kind found in metadata")
				return true
			}
		}

		s.t.Logf("Service kind not yet in metadata")
		return false
	}, helpers.SlowEventuallyOption)

	return s
}

func (s *ImportExportStage) all_resources_are_queryable() *ImportExportStage {
	resourceKinds := []string{"Deployment", "Pod", "Service", "ConfigMap"}
	startTime := s.baseTime.Unix() - 300
	endTime := s.baseTime.Unix() + 300

	for _, ns := range s.testNamespaces {
		for _, kind := range resourceKinds {
			helpers.EventuallyCondition(s.t, func() bool {
				searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
				defer searchCancel()

				resp, err := s.apiClient.Search(searchCtx, startTime, endTime, ns, kind)
				if err != nil {
					s.t.Logf("Search failed for namespace %s, kind %s: %v", ns, kind, err)
					return false
				}

				if resp.Count > 0 {
					s.t.Logf("Found %d %s resources in namespace %s", resp.Count, kind, ns)
					return true
				}

				return false
			}, helpers.SlowEventuallyOption)
		}
	}

	s.t.Log("✓ All resource kinds queryable in all test namespaces")
	return s
}

func (s *ImportExportStage) deployments_can_be_queried() *ImportExportStage {
	for _, ns := range s.testNamespaces {
		helpers.EventuallyCondition(s.t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
			defer searchCancel()

			resp, err := s.apiClient.Search(searchCtx, s.exportTimestamp-900, s.exportTimestamp+60, ns, "Deployment")
			if err != nil {
				s.t.Logf("Search failed for namespace %s: %v", ns, err)
				return false
			}

			s.t.Logf("Found %d resources in namespace %s after import", resp.Count, ns)
			return resp.Count > 0
		}, helpers.SlowEventuallyOption)
	}

	return s
}

func (s *ImportExportStage) specific_deployment_is_present() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Minute)
	defer cancel()

	searchResp, err := s.apiClient.Search(ctx, s.exportTimestamp-900, s.exportTimestamp+60, "import-1", "Deployment")
	s.require.NoError(err, "search failed")
	s.require.Greater(searchResp.Count, 0, "should find deployments in import-1")

	foundSpecificDeploy := false
	for _, r := range searchResp.Resources {
		if r.Name == "import-deploy-0" {
			foundSpecificDeploy = true
			s.t.Logf("✓ Found specific deployment: %s/%s", r.Namespace, r.Name)
			break
		}
	}
	s.assert.True(foundSpecificDeploy, "Should find import-deploy-0 after import")

	s.t.Log("✓ Import/Export round-trip test completed successfully!")
	return s
}

func (s *ImportExportStage) specific_resources_are_present_by_name() *ImportExportStage {
	startTime := s.baseTime.Unix() - 300
	endTime := s.baseTime.Unix() + 300

	expectedResources := []struct {
		namespace string
		name      string
		kind      string
	}{
		{"e2e-import-json-1", "test-deployment-0", "Deployment"},
		{"e2e-import-json-1", "test-pod-1", "Pod"},
		{"e2e-import-json-2", "test-service-2", "Service"},
		{"e2e-import-json-2", "test-configmap-3", "ConfigMap"},
	}

	for _, expected := range expectedResources {
		helpers.EventuallyCondition(s.t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
			defer searchCancel()

			resp, err := s.apiClient.Search(searchCtx, startTime, endTime, expected.namespace, expected.kind)
			if err != nil {
				s.t.Logf("Search failed for %s/%s: %v", expected.namespace, expected.name, err)
				return false
			}

			for _, r := range resp.Resources {
				if r.Name == expected.name && r.Kind == expected.kind {
					s.t.Logf("✓ Found expected resource: %s/%s (%s)", r.Namespace, r.Name, r.Kind)
					return true
				}
			}

			s.t.Logf("Resource %s/%s not yet found", expected.namespace, expected.name)
			return false
		}, helpers.SlowEventuallyOption)
	}

	s.t.Log("✓ JSON event batch import test completed successfully!")
	return s
}

func (s *ImportExportStage) service_is_found_via_search() *ImportExportStage {
	startTime := s.baseTime.Unix() - 300
	endTime := s.baseTime.Unix() + 300

	helpers.EventuallyCondition(s.t, func() bool {
		searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer searchCancel()

		resp, err := s.apiClient.Search(searchCtx, startTime, endTime, s.testNamespaces[0], "Service")
		if err != nil {
			s.t.Logf("Search failed: %v", err)
			return false
		}

		for _, r := range resp.Resources {
			if r.ID == s.resourceID {
				s.t.Logf("✓ Found Service with ID %s", s.resourceID)
				return true
			}
		}

		s.t.Logf("Service not yet found in search results")
		return false
	}, helpers.SlowEventuallyOption)

	return s
}

func (s *ImportExportStage) timeline_shows_status_segments() *ImportExportStage {
	startTime := s.baseTime.Unix() - 300
	endTime := s.baseTime.Unix() + 300

	helpers.EventuallyCondition(s.t, func() bool {
		timelineCtx, timelineCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer timelineCancel()

		timelineURL := fmt.Sprintf("%s/v1/timeline?start=%d&end=%d&namespace=%s&kind=Service",
			s.apiClient.BaseURL, startTime, endTime, s.testNamespaces[0])

		req, err := http.NewRequestWithContext(timelineCtx, "GET", timelineURL, http.NoBody)
		if err != nil {
			s.t.Logf("Failed to create timeline request: %v", err)
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			s.t.Logf("Timeline request failed: %v", err)
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			s.t.Logf("Timeline request returned status %d", resp.StatusCode)
			return false
		}

		var timelineResp helpers.SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&timelineResp); err != nil {
			s.t.Logf("Failed to decode timeline response: %v", err)
			return false
		}

		for i := range timelineResp.Resources {
			if timelineResp.Resources[i].ID == s.resourceID {
				s.timelineResource = &timelineResp.Resources[i]
				break
			}
		}

		if s.timelineResource == nil {
			s.t.Logf("Resource %s not found in timeline response", s.resourceID)
			return false
		}

		if len(s.timelineResource.StatusSegments) < 1 {
			s.t.Logf("Expected status segments in timeline, got %d", len(s.timelineResource.StatusSegments))
			return false
		}

		s.t.Logf("✓ Timeline contains %d status segments for resource", len(s.timelineResource.StatusSegments))
		return true
	}, helpers.SlowEventuallyOption)

	s.require.NotNil(s.timelineResource, "Timeline resource should be found")
	return s
}

func (s *ImportExportStage) status_segments_are_ordered() *ImportExportStage {
	s.assert.Greater(len(s.timelineResource.StatusSegments), 0, "Should have status segments")
	for i := 1; i < len(s.timelineResource.StatusSegments); i++ {
		s.assert.LessOrEqual(s.timelineResource.StatusSegments[i-1].StartTime, s.timelineResource.StatusSegments[i].StartTime,
			"Status segments should be ordered by start time")
	}

	s.t.Log("✓ Batch import with resource timeline test completed successfully!")
	return s
}

func (s *ImportExportStage) kubernetes_event_kind_is_present() *ImportExportStage {
	startTime := s.baseTime.Unix() - 300
	endTime := s.baseTime.Unix() + 300

	helpers.EventuallyCondition(s.t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer metadataCancel()

		metadata, err := s.apiClient.GetMetadata(metadataCtx, &startTime, &endTime)
		if err != nil {
			s.t.Logf("GetMetadata failed: %v", err)
			return false
		}

		for _, kind := range metadata.Kinds {
			if kind == "Event" {
				s.t.Logf("✓ Event kind found in metadata")
				// Also check resource count
				if count, ok := metadata.ResourceCounts["Event"]; ok && count > 0 {
					s.t.Logf("✓ Event resource count: %d", count)
				}
				return true
			}
		}

		s.t.Logf("Event kind not yet in metadata, found kinds: %v", metadata.Kinds)
		return false
	}, helpers.SlowEventuallyOption)

	return s
}

func (s *ImportExportStage) kubernetes_events_can_be_queried() *ImportExportStage {
	startTime := s.baseTime.Unix() - 300
	endTime := s.baseTime.Unix() + 300

	// Events are not returned as standalone resources, they are attached to their involved objects
	// So we query for Pod resources and verify they have events attached
	for _, ns := range s.testNamespaces {
		helpers.EventuallyCondition(s.t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
			defer searchCancel()

			resp, err := s.apiClient.Search(searchCtx, startTime, endTime, ns, "Pod")
			if err != nil {
				s.t.Logf("Search failed for namespace %s, kind Pod: %v", ns, err)
				return false
			}

			if resp.Count > 0 {
				s.t.Logf("Found %d Pod resources in namespace %s (which should have Events attached)", resp.Count, ns)
				return true
			}

			s.t.Logf("No Pod resources found yet in namespace %s", ns)
			return false
		}, helpers.SlowEventuallyOption)
	}

	s.t.Log("✓ Pod resources with attached Kubernetes Events are queryable in all test namespaces")
	return s
}

func (s *ImportExportStage) specific_kubernetes_event_is_present() *ImportExportStage {
	startTime := s.baseTime.Unix() - 300
	endTime := s.baseTime.Unix() + 300

	// Verify that Kubernetes Events are attached to their involved Pod resources
	// Events are not standalone resources but are attached via InvolvedObjectUID
	for _, ns := range s.testNamespaces {
		helpers.EventuallyCondition(s.t, func() bool {
			timelineCtx, timelineCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
			defer timelineCancel()

			// Use timeline API to get resources with their attached events
			timelineURL := fmt.Sprintf("%s/v1/timeline?start=%d&end=%d&namespace=%s&kind=Pod",
				s.apiClient.BaseURL, startTime, endTime, ns)

			req, err := http.NewRequestWithContext(timelineCtx, "GET", timelineURL, http.NoBody)
			if err != nil {
				s.t.Logf("Failed to create timeline request: %v", err)
				return false
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				s.t.Logf("Timeline request failed: %v", err)
				return false
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				s.t.Logf("Timeline request returned status %d", resp.StatusCode)
				return false
			}

			var timelineResp helpers.SearchResponse
			if err := json.NewDecoder(resp.Body).Decode(&timelineResp); err != nil {
				s.t.Logf("Failed to decode timeline response: %v", err)
				return false
			}

			if timelineResp.Count == 0 {
				s.t.Logf("No Pod resources found yet in namespace %s", ns)
				return false
			}

			// Check if Pods have Events attached
			foundPodWithEvents := false
			totalEventsFound := 0
			for _, r := range timelineResp.Resources {
				if r.Kind == "Pod" && len(r.Events) > 0 {
					foundPodWithEvents = true
					totalEventsFound += len(r.Events)
					s.t.Logf("✓ Found Pod %s/%s with %d attached Kubernetes Events", r.Namespace, r.Name, len(r.Events))

					// Verify that the events have expected properties
					for _, evt := range r.Events {
						if evt.Reason == "" || evt.Message == "" {
							s.t.Logf("Warning: Event %s has empty reason or message", evt.ID)
						}
					}
				}
			}

			if foundPodWithEvents && totalEventsFound > 0 {
				s.t.Logf("✓ Found %d total Kubernetes Events attached to Pods in namespace %s", totalEventsFound, ns)
				return true
			}

			s.t.Logf("No Pods with attached Events found yet in namespace %s", ns)
			return false
		}, helpers.SlowEventuallyOption)
	}

	s.t.Log("✓ JSON event batch import with Kubernetes Events test completed successfully!")
	return s
}

// CLI import on startup methods

func (s *ImportExportStage) a_test_cluster() *ImportExportStage {
	var err error
	clusterName := fmt.Sprintf("cli-test-%d", time.Now().Unix()%1000000)
	s.testCluster, err = helpers.CreateKindCluster(s.t, clusterName)
	s.require.NoError(err, "Should create test cluster")
	s.t.Cleanup(func() {
		if err := s.testCluster.Delete(); err != nil {
			s.t.Logf("Warning: failed to delete Kind cluster: %v", err)
		}
	})
	s.k8sClient, err = helpers.NewK8sClient(s.t, s.testCluster.KubeConfig)
	s.require.NoError(err, "Should create Kubernetes client")

	return s
}

func (s *ImportExportStage) generated_test_events_stored_in_configmap() *ImportExportStage {
	s.testNamespaces = []string{"cli-import-1", "cli-import-2"}
	s.baseTime = time.Now()
	s.events = s.generateTestEvents(s.baseTime, s.testNamespaces)
	s.require.Greater(len(s.events), 0, "Should generate test events")
	s.t.Logf("Generated %d test events", len(s.events))

	// Convert events to JSON
	importPayload := map[string]interface{}{
		"events": s.events,
	}
	payloadJSON, err := json.MarshalIndent(importPayload, "", "  ")
	s.require.NoError(err, "failed to marshal events to JSON")

	s.t.Logf("JSON payload size: %d bytes", len(payloadJSON))

	s.spectreNamespace = "monitoring"
	err = s.k8sClient.CreateNamespace(s.t.Context(), s.spectreNamespace)
	s.require.NoError(err, "failed to create namespace")

	s.t.Logf("Creating ConfigMap with JSON event data")
	s.configMapName = "import-events"
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.configMapName,
			Namespace: s.spectreNamespace,
		},
		Data: map[string]string{
			"events.json": string(payloadJSON),
		},
	}

	_, err = s.k8sClient.Clientset.CoreV1().ConfigMaps(s.spectreNamespace).Create(s.t.Context(), configMap, metav1.CreateOptions{})
	s.require.NoError(err, "failed to create ConfigMap")
	s.t.Logf("ConfigMap created: %s/%s", s.spectreNamespace, s.configMapName)
	return s
}

func (s *ImportExportStage) spectre_is_deployed_with_import_on_startup() *ImportExportStage {
	s.t.Logf("Preparing Helm deployment with import configuration")

	values, imageRef, err := helpers.LoadHelmValues()
	s.require.NoError(err, "failed to load Helm values")

	err = helpers.BuildAndLoadTestImage(s.t, s.testCluster.Name, imageRef)
	s.require.NoError(err, "failed to build/load image")

	importMountPath := "/import-data"

	values["extraVolumes"] = []map[string]interface{}{
		{
			"name": "import-data",
			"configMap": map[string]string{
				"name": s.configMapName,
			},
		},
	}

	values["extraVolumeMounts"] = []map[string]interface{}{
		{
			"name":      "import-data",
			"mountPath": importMountPath,
			"readOnly":  true,
		},
	}

	values["extraArgs"] = []string{
		fmt.Sprintf("--import=%s", importMountPath),
	}

	helmDeployer, err := helpers.NewHelmDeployer(s.t, s.testCluster.GetKubeConfig(), s.spectreNamespace)
	s.require.NoError(err, "failed to create Helm deployer")

	chartPath, err := helpers.RepoPath("chart")
	s.require.NoError(err, "failed to get chart path")

	err = helmDeployer.InstallOrUpgrade(s.testCluster.Name, chartPath, values)
	s.require.NoError(err, "failed to install Helm release")

	s.t.Logf("Spectre deployed with import configuration")
	return s
}

func (s *ImportExportStage) wait_for_spectre_to_become_ready() *ImportExportStage {
	err := helpers.WaitForAppReady(s.t.Context(), s.k8sClient, s.spectreNamespace, s.testCluster.Name)
	s.require.NoError(err, "Spectre not ready")
	return s
}

func (s *ImportExportStage) port_forward_to_spectre() *ImportExportStage {
	portForwarder, err := helpers.NewPortForwarder(s.t, s.testCluster.GetKubeConfig(), s.spectreNamespace, s.testCluster.Name, 8080)
	s.require.NoError(err, "failed to create port-forwarder")
	s.t.Cleanup(func() {
		if err := portForwarder.Stop(); err != nil {
			s.t.Logf("Warning: failed to stop port-forwarder: %v", err)
		}
	})
	err = portForwarder.WaitForReady(30 * time.Second)
	s.require.NoError(err, "service not reachable via port-forward")
	s.apiClient = helpers.NewAPIClient(s.t, portForwarder.GetURL())
	return s
}

func (s *ImportExportStage) verify_imported_data_is_present_via_metadata_api() *ImportExportStage {
	startTime := time.Now().Unix() - 300
	endTime := time.Now().Unix() + 300

	helpers.EventuallyCondition(s.t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer metadataCancel()

		metadata, err := s.apiClient.GetMetadata(metadataCtx, &startTime, &endTime)
		if err != nil {
			s.t.Logf("GetMetadata failed: %v", err)
			return false
		}

		// Check if our test namespaces are present
		foundNamespaces := make(map[string]bool)
		for _, ns := range metadata.Namespaces {
			for _, testNs := range s.testNamespaces {
				if ns == testNs {
					foundNamespaces[testNs] = true
				}
			}
		}

		if len(foundNamespaces) != len(s.testNamespaces) {
			s.t.Logf("Not all namespaces found in metadata yet. Found: %v, all namespaces: %v", foundNamespaces, metadata.Namespaces)
			return false
		}

		return true
	}, helpers.DefaultEventuallyOption)

	s.t.Log("✓ All test namespaces appear in metadata")
	return s
}

func (s *ImportExportStage) verify_resources_can_be_queried_via_search_api() *ImportExportStage {
	startTime := time.Now().Unix() - 300
	endTime := time.Now().Unix() + 300
	resourceKinds := []string{"Deployment", "Pod", "Service", "ConfigMap"}

	for _, ns := range s.testNamespaces {
		for _, kind := range resourceKinds {
			helpers.EventuallyCondition(s.t, func() bool {
				searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
				defer searchCancel()

				resp, err := s.apiClient.Search(searchCtx, startTime, endTime, ns, kind)
				if err != nil {
					s.t.Logf("Search failed for %s/%s: %v", ns, kind, err)
					return false
				}

				if resp.Count > 0 {
					s.t.Logf("Found %d %s resources in namespace %s", resp.Count, kind, ns)
					return true
				}

				s.t.Logf("No %s resources found yet in namespace %s", kind, ns)
				return false
			}, helpers.SlowEventuallyOption)
		}
	}

	s.t.Log("✓ All resource kinds queryable in all test namespaces")
	return s
}

func (s *ImportExportStage) specific_resources_are_present_by_name_for_cli_import() *ImportExportStage {
	startTime := time.Now().Unix() - 300
	endTime := time.Now().Unix() + 300
	expectedResources := []struct {
		namespace string
		name      string
		kind      string
	}{
		{"cli-import-1", "test-deployment-0", "Deployment"},
		{"cli-import-1", "test-pod-1", "Pod"},
		{"cli-import-2", "test-service-2", "Service"},
		{"cli-import-2", "test-configmap-3", "ConfigMap"},
	}

	for _, expected := range expectedResources {
		helpers.EventuallyCondition(s.t, func() bool {
			searchCtx, searchCancel := context.WithTimeout(s.t.Context(), 5*time.Second)
			defer searchCancel()

			resp, err := s.apiClient.Search(searchCtx, startTime, endTime, expected.namespace, expected.kind)
			if err != nil {
				s.t.Logf("Search failed for %s/%s: %v", expected.namespace, expected.name, err)
				return false
			}

			for _, r := range resp.Resources {
				if r.Name == expected.name && r.Kind == expected.kind {
					s.t.Logf("✓ Found expected resource: %s/%s (%s)", r.Namespace, r.Name, r.Kind)
					return true
				}
			}

			s.t.Logf("Resource %s/%s not yet found", expected.namespace, expected.name)
			return false
		}, helpers.SlowEventuallyOption)
	}
	return s
}

func (s *ImportExportStage) verify_import_report_in_logs() *ImportExportStage {
	pods, err := s.k8sClient.ListPods(s.t.Context(), s.spectreNamespace, fmt.Sprintf("app.kubernetes.io/instance=%s", s.testCluster.Name))
	s.require.NoError(err, "failed to list pods")
	s.require.Greater(len(pods.Items), 0, "no pods found")

	podName := pods.Items[0].Name
	logs, err := s.k8sClient.Clientset.CoreV1().Pods(s.spectreNamespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "spectre",
	}).DoRaw(s.t.Context())
	s.require.NoError(err, "failed to get pod logs")

	logsStr := string(logs)

	// Check for import-related log messages
	s.require.True(strings.Contains(logsStr, "Importing events from") || strings.Contains(logsStr, "Import"),
		"Pod logs should contain import-related messages")
	s.require.True(strings.Contains(logsStr, "Import Summary") || strings.Contains(logsStr, "Import completed"),
		"Pod logs should contain import summary or completion message")

	s.t.Logf("✓ Pod logs confirm import execution")
	s.t.Log("✓ CLI import on startup test completed successfully!")
	return s
}

// Helper methods

func (s *ImportExportStage) generateTestEvents(baseTime time.Time, namespaces []string) []*models.Event {
	var events []*models.Event

	kinds := []string{"Deployment", "Pod", "Service", "ConfigMap"}
	eventTypes := []models.EventType{models.EventTypeCreate, models.EventTypeUpdate, models.EventTypeDelete}

	for nsIdx, ns := range namespaces {
		for kindIdx, kind := range kinds {
			resourceUID := fmt.Sprintf("uid-%s-%d-%d", ns, nsIdx, kindIdx)
			resourceName := fmt.Sprintf("test-%s-%d", strings.ToLower(kind), kindIdx)

			for eventIdx := 0; eventIdx < 3; eventIdx++ {
				timestamp := baseTime.Add(time.Duration(eventIdx*10) * time.Second)
				eventType := eventTypes[eventIdx%len(eventTypes)]

				event := &models.Event{
					ID:        uuid.New().String(),
					Timestamp: timestamp.UnixNano(),
					Type:      eventType,
					Resource: models.ResourceMetadata{
						Group:     "apps",
						Version:   "v1",
						Kind:      kind,
						Namespace: ns,
						Name:      resourceName,
						UID:       resourceUID,
					},
					Data: []byte(fmt.Sprintf(
						`{"apiVersion":"apps/v1","kind":%q,"metadata":{"name":%q,"namespace":%q}}`,
						kind, resourceName, ns,
					)),
				}
				event.DataSize = int32(len(event.Data))
				events = append(events, event)
			}
		}
	}

	return events
}
