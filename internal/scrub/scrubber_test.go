package scrub

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestScrubEventData_ConfigMapData(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{"kind":"ConfigMap","data":{"JWT_SECRET":"demo_jwt_secret_key","LOG_LEVEL":"info"}}`)

	out, err := s.ScrubEventData("ConfigMap", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	data := got["data"].(map[string]any)
	if data["JWT_SECRET"] == "demo_jwt_secret_key" {
		t.Fatalf("expected JWT_SECRET to be scrubbed")
	}
	if data["LOG_LEVEL"] == "info" {
		t.Fatalf("expected ConfigMap values to be scrubbed")
	}
}

func TestScrubEventData_SecretDataReencodesBase64(t *testing.T) {
	s := New(true)
	encoded := base64.StdEncoding.EncodeToString([]byte("sk_test_fake_key_payment"))
	input := json.RawMessage(`{"kind":"Secret","data":{"token":"` + encoded + `"}}`)

	out, err := s.ScrubEventData("Secret", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(got.Data["token"])
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if string(decoded) == "sk_test_fake_key_payment" {
		t.Fatalf("expected decoded secret to be scrubbed")
	}
}

func TestScrubEventData_LastAppliedConfigurationRecurses(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{
		"metadata": {
			"annotations": {
				"kubectl.kubernetes.io/last-applied-configuration": "{\"kind\":\"ConfigMap\",\"data\":{\"JWT_SECRET\":\"demo_jwt_secret_key\"}}"
			}
		}
	}`)

	out, err := s.ScrubEventData("Deployment", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Metadata.Annotations["kubectl.kubernetes.io/last-applied-configuration"] == "{\"kind\":\"ConfigMap\",\"data\":{\"JWT_SECRET\":\"demo_jwt_secret_key\"}}" {
		t.Fatalf("expected annotation payload to be scrubbed")
	}
}

func TestMaskString_PreservesLength(t *testing.T) {
	if got := maskString("demo_jwt_secret_key"); len(got) != len("demo_jwt_secret_key") {
		t.Fatalf("expected preserved length, got %d", len(got))
	}
}
