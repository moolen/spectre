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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/containerd/typeurl/v2"

	"github.com/containerd/errdefs/pkg/internal/types"
)

func init() {
	typeurl.Register((*stack)(nil), "github.com/containerd/errdefs", "stack+json")
}

var (
	// Version is version of running process
	Version string = "dev"

	// Revision is the specific revision of the running process
	Revision string = "dirty"
)

type stack struct {
	decoded *Trace

	callers []uintptr
	helpers []uintptr
}

// Trace is a stack trace along with process information about the source
type Trace struct {
	Version  string   `json:"version,omitempty"`
	Revision string   `json:"revision,omitempty"`
	Cmdline  []string `json:"cmdline,omitempty"`
	Frames   []Frame  `json:"frames,omitempty"`
	Pid      int32    `json:"pid,omitempty"`
}

// Frame is a single frame of the trace representing a line of code
type Frame struct {
	Name string `json:"Name,omitempty"`
	File string `json:"File,omitempty"`
	Line int32  `json:"Line,omitempty"`
}

func (f Frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case s.Flag('+'):
			fmt.Fprintf(s, "%s\n\t%s:%d\n", f.Name, f.File, f.Line)
		default:
			fmt.Fprint(s, f.Name)
		}
	case 's':
		fmt.Fprint(s, path.Base(f.Name))
	case 'q':
		fmt.Fprintf(s, "%q", path.Base(f.Name))
	}
}

// callers returns the current stack, skipping over the number of frames mentioned
// Frames with skip=0:
//
//	frame[0] runtime.Callers
//	frame[1] <this function> github.com/containerd/errdefs/stack.callers
//	frame[2] <caller> (Use skip=2 to have this be first frame)
func callers(skip int) *stack {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	return &stack{
		callers: pcs[0:n],
	}
}

func (s *stack) getDecoded() *Trace {
	if s.decoded == nil {
		var unsafeDecoded = (*unsafe.Pointer)(unsafe.Pointer(&s.decoded))

		var helpers map[string]struct{}
		if len(s.helpers) > 0 {
			helpers = make(map[string]struct{})
			frames := runtime.CallersFrames(s.helpers)
			for {
				frame, more := frames.Next()
				helpers[frame.Function] = struct{}{}
				if !more {
					break
				}
			}
		}

		f := make([]Frame, 0, len(s.callers))
		if len(s.callers) > 0 {
			frames := runtime.CallersFrames(s.callers)
			for {
				frame, more := frames.Next()
				if _, ok := helpers[frame.Function]; !ok {
					f = append(f, Frame{
						Name: frame.Function,
						File: frame.File,
						Line: int32(frame.Line),
					})
				}
				if !more {
					break
				}
			}
		}

		t := Trace{
			Version:  Version,
			Revision: Revision,
			Cmdline:  os.Args,
			Frames:   f,
			Pid:      int32(os.Getpid()),
		}

		atomic.StorePointer(unsafeDecoded, unsafe.Pointer(&t))
	}

	return s.decoded
}

func (s *stack) Error() string {
	return fmt.Sprintf("%+v", s.getDecoded())
}

func (s *stack) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.getDecoded())
}

func (s *stack) UnmarshalJSON(b []byte) error {
	var unsafeDecoded = (*unsafe.Pointer)(unsafe.Pointer(&s.decoded))
	var t Trace

	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}

	atomic.StorePointer(unsafeDecoded, unsafe.Pointer(&t))

	return nil
}

func (s *stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		if st.Flag('+') {
			t := s.getDecoded()
			fmt.Fprintf(st, "%d %s %s\n", t.Pid, t.Version, strings.Join(t.Cmdline, " "))
			for _, f := range t.Frames {
				f.Format(st, verb)
			}
			fmt.Fprintln(st)
			return
		}
	}
}

func (s *stack) StackTrace() Trace {
	return *s.getDecoded()
}

func (s *stack) CollapseError() {}

// ErrStack returns a new error for the callers stack,
// this can be wrapped or joined into an existing error.
// NOTE: When joined with errors.Join, the stack
// will show up in the error string output.
// Use with `stack.Join` to force addition of the
// error stack.
func ErrStack() error {
	return callers(3)
}

// Join adds a stack if there is no stack included to the errors
// and returns a joined error with the stack hidden from the error
// output. The stack error shows up when Unwrapped or formatted
// with `%+v`.
func Join(errs ...error) error {
	return joinErrors(nil, errs)
}

// WithStack will check if the error already has a stack otherwise
// return a new error with the error joined with a stack error
// Any helpers will be skipped.
func WithStack(ctx context.Context, errs ...error) error {
	return joinErrors(ctx.Value(helperKey{}), errs)
}

func joinErrors(helperVal any, errs []error) error {
	var filtered []error
	var collapsible []error
	var hasStack bool
	for _, err := range errs {
		if err != nil {
			if !hasStack && hasLocalStackTrace(err) {
				hasStack = true
			}
			if _, ok := err.(types.CollapsibleError); ok {
				collapsible = append(collapsible, err)
			} else {
				filtered = append(filtered, err)
			}

		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if !hasStack {
		s := callers(4)
		if helpers, ok := helperVal.([]uintptr); ok {
			s.helpers = helpers
		}
		collapsible = append(collapsible, s)
	}
	var err error
	if len(filtered) > 1 {
		err = errors.Join(filtered...)
	} else {
		err = filtered[0]
	}
	if len(collapsible) == 0 {
		return err
	}

	return types.CollapsedError(err, collapsible...)
}

func hasLocalStackTrace(err error) bool {
	switch e := err.(type) {
	case *stack:
		return true
	case interface{ Unwrap() error }:
		if hasLocalStackTrace(e.Unwrap()) {
			return true
		}
	case interface{ Unwrap() []error }:
		for _, ue := range e.Unwrap() {
			if hasLocalStackTrace(ue) {
				return true
			}
		}
	}

	// TODO: Consider if pkg/errors compatibility is needed
	// NOTE: This was implemented before the standard error package
	// so it may unwrap and have this interface.
	//if _, ok := err.(interface{ StackTrace() pkgerrors.StackTrace }); ok {
	//	return true
	//}

	return false
}

type helperKey struct{}

// WithHelper marks the context as from a helper function
// This will add an additional skip to the error stack trace
func WithHelper(ctx context.Context) context.Context {
	helpers, _ := ctx.Value(helperKey{}).([]uintptr)
	var pcs [1]uintptr
	n := runtime.Callers(2, pcs[:])
	if n == 1 {
		ctx = context.WithValue(ctx, helperKey{}, append(helpers, pcs[0]))
	}
	return ctx
}
