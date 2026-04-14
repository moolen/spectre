package helpers

import (
	"reflect"
	"testing"
)

func TestDockerBuildArgsPreferLayerCache(t *testing.T) {
	args := dockerBuildArgs("spectre:test-build", "/repo")
	want := []string{"build", "-t", "spectre:test-build", "/repo"}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected docker build args: got %v want %v", args, want)
	}
}
