/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package stack

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestStack(t *testing.T) {
	s := callers(2)
	if len(s.callers) == 0 {
		t.Fatalf("expected callers, got:\n%v", s)
	}
	tr := s.getDecoded()
	if len(tr.Frames) != len(s.callers) {
		t.Fatalf("expected 1 frame, got %d", len(tr.Frames))
	}
	if name := tr.Frames[0].Name; !strings.HasSuffix(name, "."+t.Name()) {
		t.Fatalf("unexpected frame: %s\n%v", name, s)
	}
}

func TestCollapsed(t *testing.T) {
	checkError := func(err error, expected string) {
		t.Helper()
		if err.Error() != expected {
			t.Fatalf("unexpected error string %q, expected %q", err.Error(), expected)
		}

		if printed := fmt.Sprintf("%v", err); printed != expected {
			t.Fatalf("unexpected error string %q, expected %q", printed, expected)
		}

		if printed := fmt.Sprintf("%+v", err); !strings.HasPrefix(printed, expected) || !strings.Contains(printed, t.Name()) {
			t.Fatalf("unexpected error string %q, expected %q with stack containing %q", printed, expected, t.Name())
		}
	}
	expected := "some error"
	checkError(Join(errors.New(expected)), expected)
	checkError(Join(errors.New(expected), ErrStack()), expected)
	checkError(WithStack(context.Background(), errors.New(expected)), expected)
}

func TestHelpers(t *testing.T) {
	checkError := func(err error, expected string, withHelper bool) {
		t.Helper()
		if err.Error() != expected {
			t.Fatalf("unexpected error string %q, expected %q", err.Error(), expected)
		}

		if printed := fmt.Sprintf("%v", err); printed != expected {
			t.Fatalf("unexpected error string %q, expected %q", printed, expected)
		}

		printed := fmt.Sprintf("%+v", err)
		if !strings.HasPrefix(printed, expected) || !strings.Contains(printed, t.Name()) {
			t.Fatalf("unexpected error string %q, expected %q with stack containing %q", printed, expected, t.Name())
		}
		if withHelper {
			if !strings.Contains(printed, "testHelper") {
				t.Fatalf("unexpected error string, expected stack containing testHelper:\n%s", printed)
			}
		} else if strings.Contains(printed, "testHelper") {
			t.Fatalf("unexpected error string, expected stack with no containing testHelper:\n%s", printed)
		}
	}
	expected := "some error"
	checkError(Join(errors.New(expected)), expected, false)
	checkError(testHelper(expected, false), expected, true)
	checkError(testHelper(expected, true), expected, false)
}

func testHelper(msg string, withHelper bool) error {
	if withHelper {
		return WithStack(WithHelper(context.Background()), errors.New(msg))
	} else {
		return WithStack(context.Background(), errors.New(msg))
	}

}
