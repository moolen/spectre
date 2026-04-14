package scrub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type Scrubber struct {
	enabled bool
}

func New(enabled bool) *Scrubber {
	return &Scrubber{enabled: enabled}
}

func (s *Scrubber) Enabled() bool {
	return s != nil && s.enabled
}

func (s *Scrubber) ScrubEventData(kind string, data json.RawMessage) (json.RawMessage, error) {
	if !s.Enabled() || len(data) == 0 {
		return data, nil
	}

	var obj map[string]any
	if err := decodeJSONNumber(data, &obj); err != nil {
		return nil, fmt.Errorf("parse scrub target: %w", err)
	}

	switch kind {
	case "Secret":
		s.scrubStringMap(obj, "stringData")
		s.scrubBase64Map(obj, "data")
	case "ConfigMap":
		s.scrubStringMap(obj, "data")
		s.scrubBase64Map(obj, "binaryData")
	}

	s.scrubWorkloadEnv(obj)
	s.scrubLastAppliedConfiguration(obj)

	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal scrub target: %w", err)
	}
	return json.RawMessage(out), nil
}

func maskString(value string) string {
	if value == "" {
		return value
	}

	runes := []rune(value)
	n := len(runes)
	if n == 1 {
		return "*"
	}
	if n <= 4 {
		return string(runes[:1]) + repeatMask(n-1)
	}
	if n <= 8 {
		return string(runes[:1]) + repeatMask(n-2) + string(runes[n-1:])
	}
	return string(runes[:3]) + repeatMask(n-5) + string(runes[n-2:])
}

func repeatMask(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = '*'
	}
	return string(buf)
}

func (s *Scrubber) scrubStringMap(obj map[string]any, field string) {
	values, ok := obj[field].(map[string]any)
	if !ok {
		return
	}
	for key, raw := range values {
		if text, ok := raw.(string); ok {
			values[key] = maskString(text)
		}
	}
}

func (s *Scrubber) scrubBase64Map(obj map[string]any, field string) {
	values, ok := obj[field].(map[string]any)
	if !ok {
		return
	}
	for key, raw := range values {
		text, ok := raw.(string)
		if !ok {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			values[key] = maskString(text)
			continue
		}
		if !utf8.Valid(decoded) {
			values[key] = maskBase64(text)
			continue
		}
		values[key] = base64.StdEncoding.EncodeToString([]byte(maskString(string(decoded))))
	}
}

func (s *Scrubber) scrubWorkloadEnv(obj map[string]any) {
	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return
	}

	s.scrubEnvInSpec(spec)
	s.scrubTemplateSpec(spec)

	jobTemplate, ok := spec["jobTemplate"].(map[string]any)
	if ok {
		jobSpec, ok := jobTemplate["spec"].(map[string]any)
		if ok {
			s.scrubTemplateSpec(jobSpec)
		}
	}
}

func (s *Scrubber) scrubTemplateSpec(spec map[string]any) {
	template, ok := spec["template"].(map[string]any)
	if !ok {
		return
	}
	templateSpec, ok := template["spec"].(map[string]any)
	if !ok {
		return
	}
	s.scrubEnvInSpec(templateSpec)
}

func (s *Scrubber) scrubEnvInSpec(spec map[string]any) {
	for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
		items, ok := spec[field].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			container, ok := item.(map[string]any)
			if !ok {
				continue
			}
			envItems, ok := container["env"].([]any)
			if !ok {
				continue
			}
			for _, envItem := range envItems {
				envMap, ok := envItem.(map[string]any)
				if !ok {
					continue
				}
				value, ok := envMap["value"].(string)
				if !ok {
					continue
				}
				envMap["value"] = maskString(value)
			}
		}
	}
}

func (s *Scrubber) scrubLastAppliedConfiguration(obj map[string]any) {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}
	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok {
		return
	}
	raw, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"].(string)
	if !ok || raw == "" {
		return
	}

	var nested map[string]any
	if err := decodeJSONNumber([]byte(raw), &nested); err != nil {
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = maskString(raw)
		return
	}

	nestedKind, _ := nested["kind"].(string)
	nestedJSON, err := json.Marshal(nested)
	if err != nil {
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = maskString(raw)
		return
	}

	scrubbed, err := s.ScrubEventData(nestedKind, nestedJSON)
	if err != nil {
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = maskString(raw)
		return
	}
	annotations["kubectl.kubernetes.io/last-applied-configuration"] = string(scrubbed)
}

func decodeJSONNumber(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("unexpected trailing JSON data")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON data")
		}
		return err
	}
	return nil
}

func maskBase64(value string) string {
	if value == "" {
		return value
	}

	trimmed := strings.TrimRight(value, "=")
	padding := value[len(trimmed):]
	if trimmed == "" {
		return value
	}

	runes := []rune(trimmed)
	n := len(runes)
	if n <= 4 {
		return strings.Repeat("A", n) + padding
	}

	head := string(runes[:2])
	tail := string(runes[n-2:])
	mid := strings.Repeat("A", n-4)
	return head + mid + tail + padding
}
