package watcher

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type recordingEventHandler struct {
	added []string
}

func (h *recordingEventHandler) OnAdd(obj runtime.Object) error {
	u := obj.(*unstructured.Unstructured)
	h.added = append(h.added, u.GetName())
	return nil
}

func (h *recordingEventHandler) OnUpdate(runtime.Object, runtime.Object) error {
	return nil
}

func (h *recordingEventHandler) OnDelete(runtime.Object) error {
	return nil
}

func TestWatcher_ProcessListedItemsSkipsInitialReplayWhenConfigured(t *testing.T) {
	handler := &recordingEventHandler{}
	w := &Watcher{
		logger:       logging.GetLogger("watcher.test"),
		eventHandler: handler,
	}
	w.SetSkipInitialListReplay(true)

	items := []unstructured.Unstructured{
		{Object: map[string]any{"metadata": map[string]any{"name": "pod-a", "namespace": "default"}}},
		{Object: map[string]any{"metadata": map[string]any{"name": "pod-b", "namespace": "default"}}},
	}

	err := w.processListedItems(context.Background(), items, schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, "initial", func(string) bool {
		return true
	})
	require.NoError(t, err)
	require.Empty(t, handler.added)
}

func TestWatcher_ProcessListedItemsProcessesInitialReplayByDefault(t *testing.T) {
	handler := &recordingEventHandler{}
	w := &Watcher{
		logger:       logging.GetLogger("watcher.test"),
		eventHandler: handler,
	}

	items := []unstructured.Unstructured{
		{Object: map[string]any{"metadata": map[string]any{"name": "pod-a", "namespace": "default"}}},
	}

	err := w.processListedItems(context.Background(), items, schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, "initial", func(string) bool {
		return true
	})
	require.NoError(t, err)
	require.Equal(t, []string{"pod-a"}, handler.added)
}
