package scrub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
	"unicode/utf8"
)

func TestScrubEventData_ConfigMapData(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{"kind":"ConfigMap","data":{"JWT_SECRET":"demo_jwt_secret_key","LOG_LEVEL":"info"}}`)

	out, err := s.ScrubEventData("ConfigMap", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got map[string]any
	decoder := json.NewDecoder(bytes.NewReader(out))
	decoder.UseNumber()
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("decoder.Decode() error = %v", err)
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

	raw := got.Metadata.Annotations["kubectl.kubernetes.io/last-applied-configuration"]
	var nested struct {
		Kind string `json:"kind"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &nested); err != nil {
		t.Fatalf("json.Unmarshal() nested error = %v", err)
	}
	if nested.Kind != "ConfigMap" {
		t.Fatalf("expected nested kind to stay ConfigMap, got %q", nested.Kind)
	}
	if nested.Data["JWT_SECRET"] == "demo_jwt_secret_key" {
		t.Fatalf("expected nested JWT_SECRET to be scrubbed")
	}
}

func TestMaskString_PreservesLength(t *testing.T) {
	if got := maskString("demo_jwt_secret_key"); len(got) != len("demo_jwt_secret_key") {
		t.Fatalf("expected preserved length, got %d", len(got))
	}
}

func TestMaskString_SingleByteIsMasked(t *testing.T) {
	if got := maskString("x"); got == "x" {
		t.Fatalf("expected single-byte value to be masked")
	}
}

func TestScrubEventData_PreservesLargeIntegers(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{"kind":"ConfigMap","data":{"JWT_SECRET":"demo_jwt_secret_key"},"observedGeneration":9007199254740993}`)

	out, err := s.ScrubEventData("ConfigMap", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got map[string]any
	decoder := json.NewDecoder(bytes.NewReader(out))
	decoder.UseNumber()
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("decoder.Decode() error = %v", err)
	}

	if got["observedGeneration"] != json.Number("9007199254740993") {
		t.Fatalf("expected large integer preserved, got %#v", got["observedGeneration"])
	}
}

func TestScrubEventData_PodEnvValuesAreScrubbed(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{
		"kind":"Pod",
		"spec":{
			"containers":[{"env":[{"name":"TOKEN","value":"pod_secret"}]}],
			"initContainers":[{"env":[{"name":"INIT_TOKEN","value":"init_secret"}]}],
			"ephemeralContainers":[{"env":[{"name":"EPH_TOKEN","value":"ephemeral_secret"}]}]
		}
	}`)

	out, err := s.ScrubEventData("Pod", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got struct {
		Spec struct {
			Containers []struct {
				Env []struct {
					Value string `json:"value"`
				} `json:"env"`
			} `json:"containers"`
			InitContainers []struct {
				Env []struct {
					Value string `json:"value"`
				} `json:"env"`
			} `json:"initContainers"`
			EphemeralContainers []struct {
				Env []struct {
					Value string `json:"value"`
				} `json:"env"`
			} `json:"ephemeralContainers"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Spec.Containers[0].Env[0].Value == "pod_secret" {
		t.Fatalf("expected pod env value to be scrubbed")
	}
	if got.Spec.InitContainers[0].Env[0].Value == "init_secret" {
		t.Fatalf("expected init container env value to be scrubbed")
	}
	if got.Spec.EphemeralContainers[0].Env[0].Value == "ephemeral_secret" {
		t.Fatalf("expected ephemeral container env value to be scrubbed")
	}
}

func TestScrubEventData_CronJobEnvValuesAreScrubbed(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{
		"kind":"CronJob",
		"spec":{
			"jobTemplate":{
				"spec":{
					"template":{
						"spec":{
							"containers":[{"env":[{"name":"TOKEN","value":"cron_secret"}]}]
						}
					}
				}
			}
		}
	}`)

	out, err := s.ScrubEventData("CronJob", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got struct {
		Spec struct {
			JobTemplate struct {
				Spec struct {
					Template struct {
						Spec struct {
							Containers []struct {
								Env []struct {
									Value string `json:"value"`
								} `json:"env"`
							} `json:"containers"`
						} `json:"spec"`
					} `json:"template"`
				} `json:"spec"`
			} `json:"jobTemplate"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env[0].Value == "cron_secret" {
		t.Fatalf("expected cronjob env value to be scrubbed")
	}
}

func TestScrubEventData_DeploymentTemplateEnvValuesAreScrubbed(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{
		"kind":"Deployment",
		"spec":{
			"template":{
				"spec":{
					"containers":[{"env":[{"name":"TOKEN","value":"deploy_secret"}]}]
				}
			}
		}
	}`)

	out, err := s.ScrubEventData("Deployment", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Env []struct {
							Value string `json:"value"`
						} `json:"env"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Spec.Template.Spec.Containers[0].Env[0].Value == "deploy_secret" {
		t.Fatalf("expected deployment env value to be scrubbed")
	}
}

func TestMaskString_UnicodePreservesValidity(t *testing.T) {
	input := "hëllo"
	got := maskString(input)
	if !utf8.ValidString(got) {
		t.Fatalf("expected masked unicode to stay valid UTF-8")
	}
	if utf8.RuneCountInString(got) != utf8.RuneCountInString(input) {
		t.Fatalf("expected rune length preserved, got %d", utf8.RuneCountInString(got))
	}
}

func TestScrubEventData_SecretBinaryDataMasksBase64String(t *testing.T) {
	s := New(true)
	binary := []byte{0xff, 0xfe, 0xfd, 0xfc}
	encoded := base64.StdEncoding.EncodeToString(binary)
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

	if got.Data["token"] == encoded {
		t.Fatalf("expected encoded secret to be masked")
	}
	decoded, err := base64.StdEncoding.DecodeString(got.Data["token"])
	if err == nil && bytes.Equal(decoded, binary) {
		t.Fatalf("expected binary secret not to round-trip")
	}
}
