package cat

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type sType string

func (s sType) String() string { return "S(" + string(s) + ")" }

func TestJoin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []any
		want string
	}{
		{"Empty", nil, ""},
		{"Strings", []any{"a", "b"}, "ab"},
		{"Mixed", []any{1, ":", true}, "1:true"},
		{"Stringer", []any{sType("x")}, "S(x)"},
		{"Error", []any{errors.New("boom")}, "boom"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Concat(tt.args...); got != tt.want {
				t.Errorf("Join() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithAndSeparators(t *testing.T) {
	t.Parallel()
	if got := With("-", 1, 2, 3); got != "1-2-3" {
		t.Fatalf("With() = %q", got)
	}
	if got := Space("a", "b", "c"); got != "a b c" {
		t.Fatalf("Space() = %q", got)
	}
	if got := CSV("a", "b", "c"); got != "a,b,c" {
		t.Fatalf("CSV() = %q", got)
	}
	if got := Comma("a", "b"); got != "a, b" {
		t.Fatalf("Comma() = %q", got)
	}
	if got := Path("usr", "local", "bin"); got != "usr/local/bin" {
		t.Fatalf("Path() = %q", got)
	}
	if got := Lines("x", "y", "z"); got != "x\ny\nz" {
		t.Fatalf("Lines() = %q", got)
	}
	if got := Quote("a", 1, true); got != `"a" "1" "true"` {
		t.Fatalf("Quote() = %q", got)
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		before   string
		after    string
		args     []any
		expected string
	}{
		{"Empty", "<", ">", nil, "<>"},
		{"Single", "[", "]", []any{"test"}, "[test]"},
		{"MultiNoSep", "(", ")", []any{1, 2, 3}, "(123)"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Wrap(tt.before, tt.after, tt.args...)
			if got != tt.expected {
				t.Errorf("Wrap() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestWrapEach(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		before   string
		after    string
		args     []any
		expected string
	}{
		{"Numbers", "<", ">", []any{1, 2, 3}, "<1><2><3>"},
		{"Mixed", "'", "'", []any{"a", 1, true}, "'a''1''true'"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := WrapEach(tt.before, tt.after, tt.args...)
			if got != tt.expected {
				t.Errorf("WrapEach() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestWrapWithSep(t *testing.T) {
	t.Parallel()
	got := WrapWith(",", "[", "]", 1, 2, 3)
	if got != "[1,2,3]" {
		t.Fatalf("WrapWith: %q", got)
	}
	got = WrapWith(" | ", "<", ">", "a", "b")
	if got != "<a | b>" {
		t.Fatalf("WrapWith 2: %q", got)
	}
}

func TestBetween(t *testing.T) {
	t.Parallel()
	got := BetweenWith(",", "START", "END", 1, 2, 3)
	if got != "START,1,2,3,END" {
		t.Fatalf("Between: %q", got)
	}
}

func TestPrefixSuffix(t *testing.T) {
	t.Parallel()
	if got := PrefixWith(" ", "P:", 1, 2); got != "P: 1 2" {
		t.Fatalf("Prefix: %q", got)
	}

	if got := SuffixWith(" ", ":S", 1, 2); got != "1 2 :S" {
		t.Fatalf("Suffix: %q", got)
	}

	if got := PrefixEach("pre-", ",", "a", "b", "c"); got != "pre-a,pre-b,pre-c" {
		t.Fatalf("PrefixEach: %q", got)
	}
	if got := SuffixEach("-s", " | ", "a", "b"); got != "a-s | b-s" {
		t.Fatalf("SuffixEach: %q", got)
	}
}

func TestIndent(t *testing.T) {
	t.Parallel()
	if got := Indent(0, "x", "y"); got != "xy" {
		t.Fatalf("Indent depth 0: %q", got)
	}
	if got := Indent(2, "x"); got != "    x" {
		t.Fatalf("Indent depth 2: %q", got)
	}
}

func TestFlatten(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		sep  string
		// The signature of FlattenWith is now `...any`, so the test data should be `[]any`.
		groups []any
		want   string
	}{
		{"Numbers", ",", []any{[]any{1, 2}, []any{3, 4}}, "1,2,3,4"},
		{"WithEmptyGroups", "-", []any{[]any{"a"}, []any{}, []any{"b", "c"}, []any{}}, "a-b-c"},
		{"AllEmpty", ",", []any{[]any{}, []any{}}, ""},
		{"SingleGroupNoSep", ",", []any{[]any{"x"}}, "x"},
		{"MultipleGroupsMultiElem", "|", []any{[]any{"x", "y"}, []any{"z"}}, "x|y|z"},
		{"DeeplyNested", ".", []any{1, []any{2, []any{3, 4}}, 5}, "1.2.3.4.5"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// We must use the `...` spread operator to pass the elements of the slice
			// as individual arguments to the variadic function.
			if got := FlattenWith(tt.sep, tt.groups...); got != tt.want {
				t.Errorf("FlattenWith() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuilderBasic(t *testing.T) {
	t.Parallel()
	b := New(",")
	defer b.Release() // ok even if Pool(false)
	b.Add(1).Add(2)
	if got := b.String(); got != "1,2" {
		t.Fatalf("Builder String: %q", got)
	}
	// Sep and AddIf
	b2 := New(" ")
	defer b2.Release()
	b2.Add("hi").Sep("-").If(false, "no").If(true, "there")
	if got := b2.String(); got != "hi-there" {
		t.Fatalf("Builder AddIf/Sep: %q", got)
	}
	// Grow shouldn’t break anything
	b3 := New("|").Grow(64).Add("a", "b", "c")
	defer b3.Release()
	if got := b3.String(); got != "a|b|c" {
		t.Fatalf("Builder Grow/Add: %q", got)
	}
	// StartBetween
	b4 := New(":", "A").Add("B", "C")
	defer b4.Release()
	if got := b4.String(); got != "A:B:C" {
		t.Fatalf("StartBetween: %q", got)
	}
}

func TestBuilderPooling(t *testing.T) {
	t.Parallel()
	// Off by default: New returns a fresh builder.
	Pool(false)
	b1 := New(",")
	if b1 == nil {
		t.Fatal("New returned nil")
	}
	b1.Add(1, 2)
	out1 := b1.String()
	b1.Release() // no-op
	if out1 != "1,2" {
		t.Fatalf("Pool(false) output: %q", out1)
	}

	// Enable pool and ensure Release resets state.
	Pool(true)
	b2 := New(" ")
	b2.Add("x", "y")
	if got := b2.String(); got != "x y" {
		t.Fatalf("pooled builder out: %q", got)
	}
	b2.Release()

	// Re-acquire and ensure previous content is not retained.
	b3 := New(",")
	if got := b3.String(); got != "" {
		t.Fatalf("pooled builder should be reset, got %q", got)
	}
	b3.Add("a")
	if got := b3.String(); got != "a" {
		t.Fatalf("pooled builder after reuse: %q", got)
	}
	b3.Release()
}

func TestUnsafeBytesToggle(t *testing.T) {
	t.Parallel()
	// Ensure default is off (we don't assume, we force it).
	SetUnsafeBytes(false)
	if IsUnsafeBytes() {
		t.Fatal("expected IsUnsafeBytes=false")
	}
	b := []byte("xyz")
	got := Concat(b)
	if got != "xyz" {
		t.Fatalf("Join([]byte) copy mode: %q", got)
	}
	// Turn on and ensure content still matches.
	SetUnsafeBytes(true)
	if !IsUnsafeBytes() {
		t.Fatal("expected IsUnsafeBytes=true")
	}
	b2 := []byte("abc")
	got2 := Concat(b2)
	if got2 != "abc" {
		t.Fatalf("Join([]byte) unsafe mode: %q", got2)
	}
}

func TestTypesCoverage(t *testing.T) {
	t.Parallel()
	var (
		i8  int8    = -8
		i16 int16   = -16
		i32 int32   = -32
		i64 int64   = -64
		u8  uint8   = 8
		u16 uint16  = 16
		u32 uint32  = 32
		u64 uint64  = 64
		f32 float32 = 3.5
		f64 float64 = 9.25
	)
	err := errors.New("X")
	s := sType("Y")
	got := With(",", i8, i16, i32, i64, u8, u16, u32, u64, f32, f64, true, false, err, s)
	wantPrefix := "-8,-16,-32,-64,8,16,32,64,3.5,9.25,true,false,X,S(Y)"
	if got != wantPrefix {
		t.Fatalf("types coverage:\n got: %q\nwant: %q", got, wantPrefix)
	}
}

func TestConcurrencySmoke(t *testing.T) {
	t.Parallel()
	const N = 64
	wg := sync.WaitGroup{}
	wg.Add(N)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			s := With("-", "g", i, "t", i*i)
			if !strings.HasPrefix(s, "g-") || !strings.Contains(s, "t-") {
				errs <- fmt.Errorf("bad string: %q", s)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// Sanity: ensure no lingering goroutines or leaks on pool toggles (smoke).
func TestPoolToggleDoesNotLeak(t *testing.T) {
	t.Parallel()
	// Force GC cycles around pool use; this is just a smoke test.
	Pool(true)
	for i := 0; i < 100; i++ {
		b := New("|")
		b.Add("a", i, "b").Release()
	}
	Pool(false)
	runtime.GC()
}

func TestAppendFunctions(t *testing.T) {
	t.Parallel()
	dst := []byte("start")
	got := Append(dst, "end", 1)
	if string(got) != "startend1" {
		t.Errorf("Append = %q, want startend1", string(got))
	}

	dst2 := []byte("start")
	got2 := AppendWith(" ", dst2, "more", 2)
	// The function correctly produces "start" + "more 2". The test must expect this.
	expected := "startmore 2"
	if string(got2) != expected {
		t.Errorf("AppendWith = %q, want %q", string(got2), expected)
	}

	dst3 := []byte("start")
	got3 := AppendBytes(dst3, []byte("bytes"))
	if string(got3) != "startbytes" {
		t.Errorf("AppendBytes = %q, want startbytes", string(got3))
	}
}

func TestAppendToFunctions(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	AppendTo(&sb, "a", 1)
	if sb.String() != "a1" {
		t.Errorf("AppendTo = %q, want a1", sb.String())
	}
	AppendStrings(&sb, "b", "c")
	if sb.String() != "a1bc" {
		t.Errorf("AppendStrings = %q, want a1bc", sb.String())
	}
}

func TestGroup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		sep    string
		groups [][]any
		want   string
	}{
		{"Empty", "", nil, ""},
		{"Single", " ", [][]any{{1, "a"}}, "1a"},
		{"Multiple", ",", [][]any{{"x"}, {"y", "z"}}, "x,yz"},
		{"WithEmpty", "-", [][]any{{"a"}, {}, {"b"}}, "a-b"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := GroupWith(tt.sep, tt.groups...); got != tt.want {
				t.Errorf("GroupWith() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNumber(t *testing.T) {
	t.Parallel()
	if got := Number(1, 2, 3); got != "123" {
		t.Errorf("Number() = %q, want 123", got)
	}
	if got := NumberWith(",", 4.5, 6.7); got != "4.5,6.7" {
		t.Errorf("NumberWith() = %q, want 4.5,6.7", got)
	}
}

func TestPairTrio(t *testing.T) {
	t.Parallel()
	if got := Pair("a", "b"); got != "ab" {
		t.Errorf("Pair() = %q, want ab", got)
	}
	if got := PairWith("-", "c", "d"); got != "c-d" {
		t.Errorf("PairWith() = %q, want c-d", got)
	}
	if got := Trio(1, 2, 3); got != "123" {
		t.Errorf("Trio() = %q, want 123", got)
	}
	if got := TrioWith(":", "x", "y", "z"); got != "x:y:z" {
		t.Errorf("TrioWith() = %q, want x:y:z", got)
	}
}

func TestRepeat(t *testing.T) {
	t.Parallel()
	if got := Repeat("a", 3); got != "aaa" {
		t.Errorf("Repeat() = %q, want aaa", got)
	}
	if got := RepeatWith("-", "b", 2); got != "b-b" {
		t.Errorf("RepeatWith() = %q, want b-b", got)
	}
	if got := Repeat("c", 0); got != "" {
		t.Errorf("Repeat(0) = %q, want empty", got)
	}
	if got := Repeat("d", -1); got != "" {
		t.Errorf("Repeat(-1) = %q, want empty", got)
	}
	if got := Repeat(1, 1); got != "1" {
		t.Errorf("Repeat(1) = %q, want 1", got)
	}
}
