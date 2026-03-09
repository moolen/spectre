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

package errgrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/containerd/typeurl/v2"

	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errhttp"
	"github.com/containerd/errdefs/pkg/internal/cause"
)

func TestGRPCNilInput(t *testing.T) {
	if err := ToGRPC(nil); err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}
	if err := ToNative(nil); err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}
}

func TestGRPCRoundTrip(t *testing.T) {
	errShouldLeaveAlone := errors.New("unknown to package")

	for _, testcase := range []struct {
		input error
		cause error
		str   string
	}{
		{
			input: errdefs.ErrInvalidArgument,
			cause: errdefs.ErrInvalidArgument,
		},
		{
			input: errdefs.ErrAlreadyExists,
			cause: errdefs.ErrAlreadyExists,
		},
		{
			input: errdefs.ErrNotFound,
			cause: errdefs.ErrNotFound,
		},
		{
			input: errdefs.ErrUnavailable,
			cause: errdefs.ErrUnavailable,
		},
		{
			input: errdefs.ErrNotImplemented,
			cause: errdefs.ErrNotImplemented,
		},
		{
			input: errdefs.ErrUnauthenticated,
			cause: errdefs.ErrUnauthenticated,
		},
		{
			input: errdefs.ErrPermissionDenied,
			cause: errdefs.ErrPermissionDenied,
		},
		{
			input: errdefs.ErrInternal,
			cause: errdefs.ErrInternal,
		},
		{
			input: errdefs.ErrDataLoss,
			cause: errdefs.ErrDataLoss,
		},
		{
			input: errdefs.ErrAborted,
			cause: errdefs.ErrAborted,
		},
		{
			input: errdefs.ErrOutOfRange,
			cause: errdefs.ErrOutOfRange,
		},
		{
			input: errdefs.ErrResourceExhausted,
			cause: errdefs.ErrResourceExhausted,
		},
		{
			input: errdefs.ErrUnknown,
			cause: errdefs.ErrUnknown,
		},
		//nolint:dupword
		{
			input: fmt.Errorf("test test test: %w", errdefs.ErrFailedPrecondition),
			cause: errdefs.ErrFailedPrecondition,
			str:   "test test test: failed precondition",
		},
		{
			// Currently failing
			input: status.Errorf(codes.Unavailable, "should be not available"),
			cause: errdefs.ErrUnavailable,
			str:   "should be not available",
		},
		{
			input: errShouldLeaveAlone,
			cause: errdefs.ErrUnknown,
			str:   errShouldLeaveAlone.Error(),
		},
		{
			input: context.Canceled,
			cause: context.Canceled,
			str:   "context canceled",
		},
		{
			input: fmt.Errorf("this is a test cancel: %w", context.Canceled),
			cause: context.Canceled,
			str:   "this is a test cancel: context canceled",
		},
		{
			input: context.DeadlineExceeded,
			cause: context.DeadlineExceeded,
			str:   "context deadline exceeded",
		},
		{
			input: fmt.Errorf("this is a test deadline exceeded: %w", context.DeadlineExceeded),
			cause: context.DeadlineExceeded,
			str:   "this is a test deadline exceeded: context deadline exceeded",
		},
		{
			input: fmt.Errorf("something conflicted: %w", errdefs.ErrConflict),
			cause: errdefs.ErrConflict,
			str:   "something conflicted: conflict",
		},
		{
			input: fmt.Errorf("everything is the same: %w", errdefs.ErrNotModified),
			cause: errdefs.ErrNotModified,
			str:   "everything is the same: not modified",
		},
		{
			input: fmt.Errorf("odd HTTP response: %w", errhttp.ToNative(418)),
			cause: cause.ErrUnexpectedStatus{Status: 418},
			str:   "odd HTTP response: unexpected status 418",
		},
	} {
		t.Run(testcase.input.Error(), func(t *testing.T) {
			t.Logf("input: %v", testcase.input)
			gerr := ToGRPC(testcase.input)
			t.Logf("grpc: %v", gerr)
			ferr := ToNative(gerr)
			t.Logf("recovered: %v", ferr)

			if !errors.Is(ferr, testcase.cause) {
				t.Fatalf("unexpected cause: !errors.Is(%v, %v)", ferr, testcase.cause)
			}

			expected := testcase.str
			if expected == "" {
				expected = testcase.cause.Error()
			}
			if ferr.Error() != expected {
				t.Fatalf("unexpected string: %q != %q", ferr.Error(), expected)
			}
		})
	}
}

type TestError struct {
	Value string `json:"value"`
}

func (*TestError) Error() string {
	return "test error"
}

func TestGRPCCustomDetails(t *testing.T) {
	typeurl.Register(&TestError{}, t.Name())
	expected := &TestError{
		Value: "test 1",
	}

	err := errors.Join(errdefs.ErrInternal, expected)
	gerr := ToGRPC(err)

	s, ok := status.FromError(gerr)
	if !ok {
		t.Fatalf("Not GRPC error: %v", gerr)
	}
	if s.Code() != codes.Internal {
		t.Fatalf("Unexpectd GRPC code %v, expected %v", s.Code(), codes.Internal)
	}

	nerr := ToNative(gerr)
	if !errors.Is(nerr, errdefs.ErrInternal) {
		t.Fatalf("Expected internal error type, got %v", nerr)
	}
	if !errdefs.IsInternal(err) {
		t.Fatalf("Expected internal error type, got %v", nerr)
	}
	terr := &TestError{}
	if !errors.As(nerr, &terr) {
		t.Fatalf("TestError not preserved, got %v", nerr)
	} else if terr.Value != expected.Value {
		t.Fatalf("Value not preserved, got %v", terr.Value)
	}
}

func TestGRPCMultiError(t *testing.T) {
	err := errors.Join(errdefs.ErrPermissionDenied, errdefs.ErrDataLoss, errdefs.ErrConflict, fmt.Errorf("Was not changed at all!: %w", errdefs.ErrNotModified))

	checkError := func(err error) {
		t.Helper()
		if !errors.Is(err, errdefs.ErrPermissionDenied) {
			t.Fatal("Not permission denied")
		}
		if !errors.Is(err, errdefs.ErrDataLoss) {
			t.Fatal("Not data loss")
		}
		if !errors.Is(err, errdefs.ErrConflict) {
			t.Fatal("Not conflict")
		}
		if !errors.Is(err, errdefs.ErrNotModified) {
			t.Fatal("Not not modified")
		}
		if errors.Is(err, errdefs.ErrFailedPrecondition) {
			t.Fatal("Should not be failed precondition")
		}
		if !strings.Contains(err.Error(), "Was not changed at all!") {
			t.Fatalf("Not modified error message missing from:\n%v", err)
		}
	}
	checkError(err)

	terr := ToNative(ToGRPC(err))

	checkError(terr)

	// Try again with decoded error
	checkError(ToNative(ToGRPC(terr)))
}

func TestGRPCNestedError(t *testing.T) {
	multiErr := errors.Join(fmt.Errorf("First error: %w", errdefs.ErrNotFound), fmt.Errorf("Second error: %w", errdefs.ErrResourceExhausted))

	checkError := func(err error) {
		t.Helper()
		if !errors.Is(err, errdefs.ErrNotFound) {
			t.Fatal("Not not found")
		}
		if !errors.Is(err, errdefs.ErrResourceExhausted) {
			t.Fatal("Not resource exhausted")
		}
		if errors.Is(err, errdefs.ErrFailedPrecondition) {
			t.Fatal("Should not be failed precondition")
		}
	}
	checkError(multiErr)

	werr := fmt.Errorf("Wrapping the error: %w", multiErr)

	checkError(werr)

	checkError(ToNative(ToGRPC(werr)))
}
