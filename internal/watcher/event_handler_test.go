package watcher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/scrub"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type recordingIngestor struct {
	event models.Event
}

func (i *recordingIngestor) ProcessEvent(_ context.Context, event models.Event) error {
	i.event = event
	return nil
}

func TestEventCaptureHandlerOnAddScrubsConfigMapDataWhenEnabled(t *testing.T) {
	ingestor := &recordingIngestor{}
	handler := NewEventCaptureHandler(ingestor, scrub.New(true))

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default", UID: "uid-1"},
		Data: map[string]string{
			"JWT_SECRET": "demo_jwt_secret_key",
		},
	}
	obj.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	if err := handler.OnAdd(obj); err != nil {
		t.Fatalf("OnAdd() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(ingestor.event.Data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	data := got["data"].(map[string]any)
	if data["JWT_SECRET"] == "demo_jwt_secret_key" {
		t.Fatalf("expected stored ConfigMap data to be scrubbed")
	}
}

func TestEventCaptureHandlerOnAddLeavesPayloadUntouchedWhenDisabled(t *testing.T) {
	ingestor := &recordingIngestor{}
	handler := NewEventCaptureHandler(ingestor, scrub.New(false))

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default", UID: "uid-1"},
		Data: map[string]string{
			"JWT_SECRET": "demo_jwt_secret_key",
		},
	}
	obj.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	if err := handler.OnAdd(obj); err != nil {
		t.Fatalf("OnAdd() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(ingestor.event.Data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	data := got["data"].(map[string]any)
	if data["JWT_SECRET"] != "demo_jwt_secret_key" {
		t.Fatalf("expected stored ConfigMap data to be untouched")
	}
}
