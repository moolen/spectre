package api

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/watcher"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedRuntimeLiveWatcherWritesAreServed(t *testing.T) {
	backend, err := embeddedstore.Open(embeddedstore.Config{DataDir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend.Close()
	})

	server := newEmbeddedRuntimeServer(t, backend)
	handler := watcher.NewEventCaptureHandler(backend)

	err = handler.OnAdd(&corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "live-pod",
			Namespace: "default",
			UID:       types.UID("live-pod-uid"),
		},
	})
	require.NoError(t, err)

	response := queryEmbeddedTimeline(t, server, 0, 4102444800)
	resource := findResource(response.Resources, "Pod", "live-pod")
	require.NotNil(t, resource)

	identity, err := backend.AnalysisStore().GetResource(context.Background(), "live-pod-uid")
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Equal(t, "live-pod-uid", identity.UID)
}
