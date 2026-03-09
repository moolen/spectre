package cat

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// sample args
var (
	strArgs = []string{"test", "123", "true", "4.56"}
	anyArgs = []any{"test", 123, true, 4.56}
)

// ---------- Existing / package APIs ----------

func BenchmarkJoin_MixedAny(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Join(strArgs...)
	}
}

func BenchmarkConcat_MixedAny(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Concat(anyArgs...)
	}
}

func BenchmarkConcat_StringsAsAny(b *testing.B) {
	b.ReportAllocs()
	anyStrs := make([]any, len(strArgs))
	for i, s := range strArgs {
		anyStrs[i] = s
	}
	for i := 0; i < b.N; i++ {
		_ = Concat(anyStrs...)
	}
}

func BenchmarkWith_FastPath(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = With(" ", "GET", "/v1/resource", 200, 1234)
	}
}

// ---------- Baselines / stdlib ----------

func BenchmarkFmtSprint(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprint("GET", " ", "/v1/resource", " ", 200, " ", 1234)
	}
}

func BenchmarkStringsJoin(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = strings.Join(strArgs, " ")
	}
}

// ---------- strings.Builder variants (manual) ----------

func BenchmarkBuilder_Manual_NoGrow(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		sb.WriteString("GET")
		sb.WriteString(" ")
		sb.WriteString("/v1/resource")
		sb.WriteString(" ")
		sb.WriteString(strconv.Itoa(200))
		sb.WriteString(" ")
		sb.WriteString(strconv.Itoa(1234))
		_ = sb.String()
	}
}

func BenchmarkBuilder_Manual_Grow(b *testing.B) {
	b.ReportAllocs()
	// Rough size: len("GET /v1/resource 200 1234") = ~26
	size := 26
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		sb.Grow(size)
		sb.WriteString("GET")
		sb.WriteString(" ")
		sb.WriteString("/v1/resource")
		sb.WriteString(" ")
		sb.WriteString(strconv.Itoa(200))
		sb.WriteString(" ")
		sb.WriteString(strconv.Itoa(1234))
		_ = sb.String()
	}
}

func BenchmarkBuilder_FmtFprint(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		fmt.Fprint(&sb, "GET", " ", "/v1/resource", " ", 200, " ", 1234)
		_ = sb.String()
	}
}

// ---------- Specialized fast paths in package ----------

func BenchmarkStrs_Specialized(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = JoinWith(" ", strArgs...)
	}
}

func BenchmarkPair_2Args(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = PairWith("-", "foo", "bar")
	}
}

func BenchmarkTrio_3Args(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = TrioWith(":", "a", "b", "c")
	}
}

// ---------- Builder pooling & unsafe bytes ----------

func BenchmarkBuilder_NoPool(b *testing.B) {
	Pool(false)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := New(" ")
		buf.Add("GET", "/v1/resource", 200, 1234)
		_ = buf.String()
		buf.Release()
	}
}

func BenchmarkBuilder_Pool(b *testing.B) {
	Pool(true)
	defer Pool(false)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := New(" ")
		buf.Add("GET", "/v1/resource", 200, 1234)
		_ = buf.String()
		buf.Release()
	}
}

func BenchmarkWith_BytesUnsafe(b *testing.B) {
	SetUnsafeBytes(true)
	defer SetUnsafeBytes(false)
	m := []byte("GET")
	p := []byte("/v1/resource")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = With(" ", m, p, 200, 1234)
	}
}
