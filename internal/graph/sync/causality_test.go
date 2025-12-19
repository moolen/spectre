package sync

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCausalityEngine_InferCausality(t *testing.T) {
	engine := NewCausalityEngine(5*time.Minute, 0.5)
	ctx := context.Background()

	t.Run("Deployment rollout triggers Pod changes", func(t *testing.T) {
		now := time.Now()

		events := []models.Event{
			{
				ID:        "deploy-update",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					Kind:      "Deployment",
					Namespace: "default",
					Name:      "frontend",
				},
			},
			{
				ID:        "pod-delete",
				Timestamp: now.Add(10 * time.Second).UnixNano(),
				Type:      models.EventTypeDelete,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "frontend-abc",
				},
			},
			{
				ID:        "pod-create",
				Timestamp: now.Add(15 * time.Second).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "frontend-xyz",
				},
			},
		}

		links, err := engine.InferCausality(ctx, events)
		require.NoError(t, err)

		// Should find causality: Deployment update → Pod delete
		assert.GreaterOrEqual(t, len(links), 1)

		// Check link properties
		found := false
		for _, link := range links {
			if link.CauseEventID == "deploy-update" && link.EffectEventID == "pod-delete" {
				found = true
				assert.Greater(t, link.Confidence, 0.5)
				assert.Equal(t, int64(10000), link.LagMs)
				assert.Contains(t, link.HeuristicUsed, "deployment-rollout")
				break
			}
		}
		assert.True(t, found, "Should find causality link between Deployment update and Pod delete")
	})

	t.Run("Same resource state transitions", func(t *testing.T) {
		now := time.Now()

		events := []models.Event{
			{
				ID:        "pod-update-1",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID:  "pod-123",
					Kind: "Pod",
					Name: "test-pod",
				},
			},
			{
				ID:        "pod-update-2",
				Timestamp: now.Add(5 * time.Second).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID:  "pod-123",
					Kind: "Pod",
					Name: "test-pod",
				},
			},
		}

		links, err := engine.InferCausality(ctx, events)
		require.NoError(t, err)

		// Should find causality: same resource transitions
		assert.Len(t, links, 1)
		assert.Equal(t, "pod-update-1", links[0].CauseEventID)
		assert.Equal(t, "pod-update-2", links[0].EffectEventID)
		assert.Greater(t, links[0].Confidence, 0.9) // High confidence for same resource
	})

	t.Run("Events too far apart", func(t *testing.T) {
		now := time.Now()

		events := []models.Event{
			{
				ID:        "event-1",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					Kind: "Deployment",
				},
			},
			{
				ID:        "event-2",
				Timestamp: now.Add(10 * time.Minute).UnixNano(), // 10 minutes later
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind: "Pod",
				},
			},
		}

		links, err := engine.InferCausality(ctx, events)
		require.NoError(t, err)

		// Should not find causality (too far apart)
		assert.Len(t, links, 0)
	})

	t.Run("No causality in single event", func(t *testing.T) {
		events := []models.Event{
			{
				ID:        "event-1",
				Timestamp: time.Now().UnixNano(),
				Type:      models.EventTypeCreate,
			},
		}

		links, err := engine.InferCausality(ctx, events)
		require.NoError(t, err)
		assert.Len(t, links, 0)
	})
}

func TestCausalityEngine_AnalyzePair(t *testing.T) {
	engine := NewCausalityEngine(5*time.Minute, 0.5)
	ctx := context.Background()

	t.Run("ConfigMap update triggers Pod restart", func(t *testing.T) {
		now := time.Now()

		cause := models.Event{
			ID:        "cm-update",
			Timestamp: now.UnixNano(),
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Kind:      "ConfigMap",
				Namespace: "default",
			},
		}

		effect := models.Event{
			ID:        "pod-restart",
			Timestamp: now.Add(30 * time.Second).UnixNano(),
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
			},
		}

		link, err := engine.AnalyzePair(ctx, cause, effect)
		require.NoError(t, err)
		require.NotNil(t, link)

		assert.Equal(t, "cm-update", link.CauseEventID)
		assert.Equal(t, "pod-restart", link.EffectEventID)
		assert.Greater(t, link.Confidence, 0.5)
		assert.Equal(t, int64(30000), link.LagMs)
	})

	t.Run("Effect before cause - no link", func(t *testing.T) {
		now := time.Now()

		cause := models.Event{
			ID:        "event-1",
			Timestamp: now.Add(1 * time.Minute).UnixNano(),
			Type:      models.EventTypeUpdate,
		}

		effect := models.Event{
			ID:        "event-2",
			Timestamp: now.UnixNano(), // Before cause
			Type:      models.EventTypeCreate,
		}

		link, err := engine.AnalyzePair(ctx, cause, effect)
		require.NoError(t, err)
		assert.Nil(t, link)
	})
}

func TestCausalityEngine_Heuristics(t *testing.T) {
	engine := NewCausalityEngine(5*time.Minute, 0.5)

	heuristics := engine.GetHeuristics()

	// Should have default heuristics registered
	assert.Greater(t, len(heuristics), 0)

	// Check some expected heuristics
	heuristicNames := make(map[string]bool)
	for _, h := range heuristics {
		heuristicNames[h.Name] = true
	}

	assert.True(t, heuristicNames["deployment-rollout"])
	assert.True(t, heuristicNames["same-resource-transition"])
	assert.True(t, heuristicNames["config-change-restart"])
}
