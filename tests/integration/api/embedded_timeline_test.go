package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/apiserver"
	"github.com/moolen/spectre/internal/embedded"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

const embeddedTimelineFixture = `{
  "events": [
    {
      "id": "evt-1",
      "timestamp": 1700000000000000000,
      "type": "CREATE",
      "resource": {
        "group": "",
        "version": "v1",
        "kind": "ConfigMap",
        "namespace": "default",
        "name": "demo-config",
        "uid": "cm-uid"
      },
      "data": {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
          "name": "demo-config",
          "namespace": "default",
          "uid": "cm-uid"
        }
      }
    },
    {
      "id": "evt-2",
      "timestamp": 1700000005000000000,
      "type": "UPDATE",
      "resource": {
        "group": "",
        "version": "v1",
        "kind": "ConfigMap",
        "namespace": "default",
        "name": "demo-config",
        "uid": "cm-uid"
      },
      "data": {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
          "name": "demo-config",
          "namespace": "default",
          "uid": "cm-uid"
        },
        "data": {
          "key": "value"
        }
      }
    },
    {
      "id": "evt-3",
      "timestamp": 1700000007000000000,
      "type": "CREATE",
      "resource": {
        "group": "",
        "version": "v1",
        "kind": "Event",
        "namespace": "default",
        "name": "demo-config.12345",
        "uid": "evt-uid",
        "involvedObjectUid": "cm-uid"
      },
      "data": {
        "reason": "Created",
        "message": "ConfigMap created",
        "type": "Normal"
      }
    }
  ]
}`

func TestEmbeddedTimelineAPI(t *testing.T) {
	events, err := importexport.Import(importexport.FromReader(strings.NewReader(embeddedTimelineFixture)))
	require.NoError(t, err)
	require.NotEmpty(t, events)

	executor, err := embedded.NewQueryExecutor(events)
	require.NoError(t, err)

	server := apiserver.NewWithStorageGraphAndPipeline(
		0,
		executor,
		nil,
		api.TimelineQuerySourceStorage,
		nil,
		nil,
		nil,
		&apiserver.NoOpReadinessChecker{},
		nil,
		time.Minute,
		apiserver.NamespaceGraphCacheConfig{},
		"",
		nil,
		nil,
	)

	start := int64(1700000000)
	end := int64(1700000010)
	req := httptest.NewRequest(http.MethodGet, "/v1/timeline?start=1700000000&end=1700000010", http.NoBody)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response models.SearchResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	require.NotEmpty(t, response.Resources)

	var target *models.Resource
	for i := range response.Resources {
		if response.Resources[i].Name == "demo-config" && response.Resources[i].Kind == "ConfigMap" {
			target = &response.Resources[i]
			break
		}
	}

	require.NotNil(t, target, "expected demo-config resource in timeline response")
	require.NotEmpty(t, target.StatusSegments)
	require.NotEmpty(t, target.Events)
	require.Equal(t, "Created", target.Events[0].Reason)
	require.GreaterOrEqual(t, target.StatusSegments[0].StartTime, start*1e9)
	require.LessOrEqual(t, target.StatusSegments[len(target.StatusSegments)-1].EndTime, end*1e9)
}
