package watcher

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
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

// scriptedResourceInterface serves a fixed List result and one scripted stream per Watch
// call, recording the options each call was made with.
type scriptedResourceInterface struct {
	dynamic.ResourceInterface

	listVersion string
	streams     [][]watch.Event
	listCalls   int
	watchCalls  []metav1.ListOptions
	exhausted   chan struct{}
}

func (r *scriptedResourceInterface) List(context.Context, metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	r.listCalls++
	list := &unstructured.UnstructuredList{}
	list.SetResourceVersion(r.listVersion)
	return list, nil
}

func (r *scriptedResourceInterface) Watch(_ context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	r.watchCalls = append(r.watchCalls, opts)
	if len(r.streams) == 0 {
		close(r.exhausted)
		return watch.NewFake(), nil
	}
	events := r.streams[0]
	r.streams = r.streams[1:]
	ch := make(chan watch.Event, len(events))
	for _, event := range events {
		ch <- event
	}
	close(ch)
	return watch.NewProxyWatcher(ch), nil
}

func watchTestObject(name, resourceVersion string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": name, "namespace": "default"}}}
	obj.SetResourceVersion(resourceVersion)
	return obj
}

func TestWatcher_ListWatchResumesClosedStreamsWithoutRelisting(t *testing.T) {
	handler := &recordingEventHandler{}
	w := &Watcher{
		logger:       logging.GetLogger("watcher.test"),
		eventHandler: handler,
		stopChan:     make(chan struct{}),
	}
	resources := &scriptedResourceInterface{
		listVersion: "10",
		exhausted:   make(chan struct{}),
		streams: [][]watch.Event{
			// Closed by the server after a change and a bookmark: resume from the bookmark.
			{
				{Type: watch.Modified, Object: watchTestObject("pod-a", "11")},
				{Type: watch.Bookmark, Object: watchTestObject("", "15")},
			},
			// Expired resourceVersion: fall back to a List.
			{
				{Type: watch.Error, Object: &metav1.Status{Code: 410, Reason: metav1.StatusReasonExpired}},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- w.listWatch(ctx, resources, schema.GroupVersionResource{Version: "v1", Resource: "pods"},
			schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, func(string) bool { return true }, 0)
	}()
	<-resources.exhausted
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	require.Equal(t, 2, resources.listCalls)
	require.Len(t, resources.watchCalls, 3)
	require.Equal(t, "10", resources.watchCalls[0].ResourceVersion)
	require.Equal(t, "15", resources.watchCalls[1].ResourceVersion)
	require.Equal(t, "10", resources.watchCalls[2].ResourceVersion)
	for _, call := range resources.watchCalls {
		require.True(t, call.AllowWatchBookmarks)
	}
}
