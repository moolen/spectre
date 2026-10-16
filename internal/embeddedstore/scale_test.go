//go:build scale

package embeddedstore

// Scale harness for the embedded timeline storage.
//
//	go test -tags scale -run TestTimelineScale -timeout 0 -v ./internal/embeddedstore/ \
//	  SPECTRE_SCALE_PROFILE=medium SPECTRE_SCALE_OUT=/tmp/medium.json
//
// Env knobs:
//
//	SPECTRE_SCALE_PROFILE      small|medium|large|xlarge (default small)
//	SPECTRE_SCALE_CHURN        override ChurnEvents
//	SPECTRE_SCALE_K8S_EVENTS   percent of churn emitted as core/v1 Events (default 0)
//	SPECTRE_SCALE_LIVE_EVENTS  events pushed one-by-one through ProcessEvent after load (default 5000)
//	SPECTRE_SCALE_DIR          data dir (default t.TempDir())
//	SPECTRE_SCALE_OUT          write a JSON report here
//	SPECTRE_SCALE_CKPT_TAIL_EVENTS  override checkpoint tail event budget (bytes budget scales with it)
//	SPECTRE_SCALE_NO_COMPACTION     disable auto-compaction
//	SPECTRE_SCALE_HEAPPROFILE  write a heap profile after load

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

type latencyStats struct {
	Count  int     `json:"count"`
	P50Ms  float64 `json:"p50_ms"`
	P99Ms  float64 `json:"p99_ms"`
	MaxMs  float64 `json:"max_ms"`
	MeanMs float64 `json:"mean_ms"`
}

func summarize(samples []time.Duration) latencyStats {
	if len(samples) == 0 {
		return latencyStats{}
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total time.Duration
	for _, s := range sorted {
		total += s
	}
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	return latencyStats{
		Count:  len(sorted),
		P50Ms:  ms(sorted[len(sorted)/2]),
		P99Ms:  ms(sorted[min(len(sorted)-1, len(sorted)*99/100)]),
		MaxMs:  ms(sorted[len(sorted)-1]),
		MeanMs: ms(total / time.Duration(len(sorted))),
	}
}

type queryResult struct {
	Name      string       `json:"name"`
	Latency   latencyStats `json:"latency"`
	Resources int          `json:"resources"`
	Events    int          `json:"events"`
}

type scaleReport struct {
	Profile             scaleProfile     `json:"profile"`
	GOMAXPROCS          int              `json:"gomaxprocs"`
	TotalEvents         int              `json:"total_events"`
	DistinctResources   int              `json:"distinct_resources"`
	PayloadBytes        int64            `json:"payload_bytes"`
	LoadSeconds         float64          `json:"load_seconds"`
	LoadEventsPerSec    float64          `json:"load_events_per_sec"`
	LoadBatchLatency    latencyStats     `json:"load_batch_latency"`
	LiveEventLatency    latencyStats     `json:"live_event_latency"`
	LiveEventsPerSec    float64          `json:"live_events_per_sec"`
	Checkpoints         int              `json:"checkpoints"`
	CheckpointMeanMs    float64          `json:"checkpoint_mean_ms"`
	Flushes             int              `json:"flushes"`
	FlushMeanMs         float64          `json:"flush_mean_ms"`
	Compactions         int              `json:"compactions"`
	CompactionMeanMs    float64          `json:"compaction_mean_ms"`
	PeakHeapMB          float64          `json:"peak_heap_mb"`
	LiveHeapMB          float64          `json:"live_heap_mb"`
	HeapBytesPerEvent   float64          `json:"heap_bytes_per_event"`
	DiskMB              map[string]int64 `json:"disk_mb"`
	ActiveSegments      int              `json:"active_segments"`
	Queries             []queryResult    `json:"queries"`
	CloseSeconds        float64          `json:"close_seconds"`
	ReopenSeconds       float64          `json:"reopen_seconds"`
	ReopenHeapMB        float64          `json:"reopen_heap_mb"`
	FirstQueryAfterOpen float64          `json:"first_query_after_open_ms"`
	ReopenStartupMode   string           `json:"reopen_startup_mode"`
}

func TestTimelineScale(t *testing.T) {
	profileName := envOr("SPECTRE_SCALE_PROFILE", "small")
	profile, ok := scaleProfiles[profileName]
	require.True(t, ok, "unknown profile %q", profileName)
	if v := os.Getenv("SPECTRE_SCALE_CHURN"); v != "" {
		profile.ChurnEvents = mustAtoi(t, v)
	}
	if v := os.Getenv("SPECTRE_SCALE_K8S_EVENTS"); v != "" {
		profile.K8sEventPercent = mustAtoi(t, v)
	}
	liveEvents := 5000
	if v := os.Getenv("SPECTRE_SCALE_LIVE_EVENTS"); v != "" {
		liveEvents = mustAtoi(t, v)
	}
	dataDir := os.Getenv("SPECTRE_SCALE_DIR")
	if dataDir == "" {
		dataDir = t.TempDir()
	} else {
		require.NoError(t, os.RemoveAll(dataDir))
	}

	report := scaleReport{Profile: profile, GOMAXPROCS: runtime.GOMAXPROCS(0), DiskMB: map[string]int64{}}
	peak := startHeapSampler()

	// Production defaults: chart values (15m checkpoint interval, retentionDays 0) on top
	// of the backend defaults for everything else.
	reg := prometheus.NewRegistry()
	cfg := Config{
		DataDir:            dataDir,
		CheckpointInterval: 15 * time.Minute,
		MetricsRegisterer:  reg,
	}
	if v := os.Getenv("SPECTRE_SCALE_CKPT_TAIL_EVENTS"); v != "" {
		cfg.CheckpointMaxTailEvents = mustAtoi(t, v)
		cfg.CheckpointMaxTailBytes = int64(cfg.CheckpointMaxTailEvents) * 16 << 10
	}
	cfg.DisableAutoCompaction = os.Getenv("SPECTRE_SCALE_NO_COMPACTION") != ""
	backend, err := Open(cfg)
	require.NoError(t, err)
	engine := backend.engine
	require.NoError(t, engine.Start(context.Background()))

	ctx := context.Background()
	workload := newScaleWorkload(profile, time.Now())
	resources := map[string]struct{}{}

	const loadBatch = 256
	batch := make([]models.Event, 0, loadBatch)
	var batchLatencies []time.Duration
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		start := time.Now()
		require.NoError(t, engine.ProcessBatch(ctx, batch))
		batchLatencies = append(batchLatencies, time.Since(start))
		batch = batch[:0]
	}
	emit := func(event models.Event) {
		report.TotalEvents++
		report.PayloadBytes += int64(len(event.Data))
		resources[event.Resource.UID] = struct{}{}
		batch = append(batch, event)
		if len(batch) == loadBatch {
			flushBatch()
		}
		if report.TotalEvents%100_000 == 0 {
			t.Logf("loaded %d events (%s)", report.TotalEvents, heapString())
		}
	}

	loadStart := time.Now()
	workload.Initial(emit)
	// Hold back the tail of the churn so it can be pushed one event at a time, the way
	// the watcher does, on top of the fully loaded store.
	liveBuffer := make([]models.Event, 0, liveEvents)
	bulkChurn := max(profile.ChurnEvents-liveEvents, 0)
	churned := 0
	workload.Churn(func(event models.Event) {
		if churned < bulkChurn {
			emit(event)
		} else {
			liveBuffer = append(liveBuffer, event)
		}
		churned++
	})
	flushBatch()
	report.LoadSeconds = time.Since(loadStart).Seconds()
	report.LoadEventsPerSec = float64(report.TotalEvents) / report.LoadSeconds
	report.LoadBatchLatency = summarize(batchLatencies)

	var liveLatencies []time.Duration
	liveStart := time.Now()
	for _, event := range liveBuffer {
		start := time.Now()
		require.NoError(t, engine.ProcessEvent(ctx, event))
		liveLatencies = append(liveLatencies, time.Since(start))
		report.TotalEvents++
		report.PayloadBytes += int64(len(event.Data))
		resources[event.Resource.UID] = struct{}{}
	}
	if len(liveBuffer) > 0 {
		report.LiveEventsPerSec = float64(len(liveBuffer)) / time.Since(liveStart).Seconds()
	}
	report.LiveEventLatency = summarize(liveLatencies)
	report.DistinctResources = len(resources)
	t.Logf("load done: %d events, %d resources in %.1fs (%.0f ev/s), live %.0f ev/s",
		report.TotalEvents, report.DistinctResources, report.LoadSeconds, report.LoadEventsPerSec, report.LiveEventsPerSec)

	report.Checkpoints, report.CheckpointMeanMs = histogramStats(t, reg, "spectre_embedded_checkpoint_duration_seconds")
	report.Flushes, report.FlushMeanMs = histogramStats(t, reg, "spectre_embedded_flush_duration_seconds")
	report.Compactions, report.CompactionMeanMs = histogramStats(t, reg, "spectre_embedded_compaction_duration_seconds")

	runtime.GC()
	report.LiveHeapMB = heapMB()
	if path := os.Getenv("SPECTRE_SCALE_HEAPPROFILE"); path != "" {
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, pprof.WriteHeapProfile(f))
		require.NoError(t, f.Close())
	}
	report.HeapBytesPerEvent = report.LiveHeapMB * (1 << 20) / float64(report.TotalEvents)
	engine.mu.Lock()
	report.ActiveSegments = len(engine.segmentReaders)
	engine.mu.Unlock()
	report.DiskMB = diskUsageMB(t, embeddedRootDir(dataDir))

	// Let background cache seeding/index warmup settle so queries measure steady state.
	time.Sleep(2 * time.Second)
	report.Queries = runTimelineQueries(t, engine.QueryExecutor(), profile)

	closeStart := time.Now()
	require.NoError(t, backend.Close())
	report.CloseSeconds = time.Since(closeStart).Seconds()
	report.PeakHeapMB = peak.stop()
	engine = nil
	backend = nil
	runtime.GC()

	reopenStart := time.Now()
	reg2 := prometheus.NewRegistry()
	reopened, err := Open(Config{DataDir: dataDir, CheckpointInterval: 15 * time.Minute, MetricsRegisterer: reg2})
	require.NoError(t, err)
	report.ReopenSeconds = time.Since(reopenStart).Seconds()
	report.ReopenStartupMode = startupModeFromMetrics(reg2)
	nowSec := time.Now().Unix()
	qStart := time.Now()
	_, _, err = reopened.engine.QueryExecutor().ExecutePaginated(ctx, &models.QueryRequest{
		StartTimestamp: nowSec - 3600, EndTimestamp: nowSec,
	}, &models.PaginationRequest{PageSize: 100})
	require.NoError(t, err)
	report.FirstQueryAfterOpen = float64(time.Since(qStart).Microseconds()) / 1000
	runtime.GC()
	report.ReopenHeapMB = heapMB()
	require.NoError(t, reopened.Close())

	out, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	t.Logf("report:\n%s", out)
	if path := os.Getenv("SPECTRE_SCALE_OUT"); path != "" {
		require.NoError(t, os.WriteFile(path, out, 0o644))
	}
}

func runTimelineQueries(t *testing.T, exec *QueryExecutor, profile scaleProfile) []queryResult {
	ctx := context.Background()
	now := time.Now().Unix()
	windowSec := int64(profile.Window / time.Second)
	type spec struct {
		name    string
		window  int64
		filters models.QueryFilters
		events  bool
	}
	specs := []spec{
		{name: "resources 15m all", window: 900},
		{name: "resources 1h all", window: 3600},
		{name: "resources 6h all", window: 6 * 3600},
		{name: "resources full all", window: windowSec},
		{name: "resources 1h ns", window: 3600, filters: models.QueryFilters{Namespaces: []string{"team-001"}}},
		{name: "resources full ns", window: windowSec, filters: models.QueryFilters{Namespaces: []string{"team-001"}}},
		{name: "resources 1h kind=Deployment", window: 3600, filters: models.QueryFilters{Kinds: []string{"Deployment"}}},
		{name: "resources full kind=Node", window: windowSec, filters: models.QueryFilters{Kinds: []string{"Node"}}},
		{name: "events 1h all", window: 3600, events: true},
		{name: "events 6h all", window: 6 * 3600, events: true},
		{name: "events full all", window: windowSec, events: true},
		{name: "events full ns", window: windowSec, events: true, filters: models.QueryFilters{Namespaces: []string{"team-001"}}},
	}

	var results []queryResult
	for _, s := range specs {
		req := &models.QueryRequest{StartTimestamp: now - s.window, EndTimestamp: now, Filters: s.filters}
		var pagination *models.PaginationRequest
		if s.events {
			req.Filters.Kinds = []string{"Event"}
			req.Filters.Version = "v1"
		} else {
			pagination = &models.PaginationRequest{PageSize: models.DefaultPageSize}
		}
		var samples []time.Duration
		var res queryResult
		for i := 0; i < 5; i++ {
			start := time.Now()
			result, _, err := exec.ExecutePaginated(ctx, req, pagination)
			samples = append(samples, time.Since(start))
			require.NoError(t, err, s.name)
			res.Events = len(result.Events)
			uids := map[string]struct{}{}
			for _, e := range result.Events {
				uids[e.Resource.UID] = struct{}{}
			}
			res.Resources = len(uids)
		}
		res.Name = s.name
		res.Latency = summarize(samples)
		results = append(results, res)
		t.Logf("%-32s p50=%8.1fms max=%8.1fms resources=%d events=%d", s.name, res.Latency.P50Ms, res.Latency.MaxMs, res.Resources, res.Events)
	}

	// Walking every page of the 1h window, as a client scrolling the whole timeline would.
	var pages int
	var walkResources int
	start := time.Now()
	pagination := &models.PaginationRequest{PageSize: models.MaxPageSize}
	for {
		result, page, err := exec.ExecutePaginated(ctx, &models.QueryRequest{StartTimestamp: now - 3600, EndTimestamp: now}, pagination)
		require.NoError(t, err)
		pages++
		walkResources += countUIDs(result.Events)
		if page == nil || !page.HasMore || page.NextCursor == "" || pages > 10_000 {
			break
		}
		pagination = &models.PaginationRequest{PageSize: models.MaxPageSize, Cursor: page.NextCursor}
	}
	walk := time.Since(start)
	results = append(results, queryResult{
		Name:      fmt.Sprintf("resources 1h all, walk %d pages of 500", pages),
		Latency:   latencyStats{Count: 1, P50Ms: float64(walk.Milliseconds()), MaxMs: float64(walk.Milliseconds())},
		Resources: walkResources,
	})
	t.Logf("walked %d pages (%d resources) in %s", pages, walkResources, walk)

	// Concurrency: 8 concurrent timeline requests (resource + event query each).
	var wg sync.WaitGroup
	var mu sync.Mutex
	var concurrent []time.Duration
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				start := time.Now()
				_, _, err := exec.ExecutePaginated(ctx, &models.QueryRequest{StartTimestamp: now - 3600, EndTimestamp: now}, &models.PaginationRequest{PageSize: 100})
				require.NoError(t, err)
				_, _, err = exec.ExecutePaginated(ctx, &models.QueryRequest{StartTimestamp: now - 3600, EndTimestamp: now, Filters: models.QueryFilters{Kinds: []string{"Event"}, Version: "v1"}}, nil)
				require.NoError(t, err)
				mu.Lock()
				concurrent = append(concurrent, time.Since(start))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	results = append(results, queryResult{Name: "8 concurrent timeline requests 1h", Latency: summarize(concurrent)})
	return results
}

func countUIDs(events []models.Event) int {
	uids := map[string]struct{}{}
	for _, e := range events {
		uids[e.Resource.UID] = struct{}{}
	}
	return len(uids)
}

type heapSampler struct {
	stopCh chan struct{}
	done   chan struct{}
	peak   atomic.Uint64
}

func startHeapSampler() *heapSampler {
	s := &heapSampler{stopCh: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			if ms.HeapInuse > s.peak.Load() {
				s.peak.Store(ms.HeapInuse)
			}
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
			}
		}
	}()
	return s
}

func (s *heapSampler) stop() float64 {
	close(s.stopCh)
	<-s.done
	return float64(s.peak.Load()) / (1 << 20)
}

func heapMB() float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.HeapInuse) / (1 << 20)
}

func heapString() string {
	return fmt.Sprintf("heap=%.0fMB", heapMB())
}

func histogramStats(t *testing.T, reg *prometheus.Registry, name string) (int, float64) {
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		var count uint64
		var sum float64
		for _, metric := range family.GetMetric() {
			h := metric.GetHistogram()
			count += h.GetSampleCount()
			sum += h.GetSampleSum()
		}
		if count == 0 {
			return 0, 0
		}
		return int(count), sum / float64(count) * 1000
	}
	return 0, 0
}

func startupModeFromMetrics(reg *prometheus.Registry) string {
	families, err := reg.Gather()
	if err != nil {
		return ""
	}
	for _, family := range families {
		if family.GetName() != "spectre_embedded_startup_mode" {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metricValue(metric) > 0 {
				for _, label := range metric.GetLabel() {
					return label.GetValue()
				}
			}
		}
	}
	return ""
}

func metricValue(metric *dto.Metric) float64 {
	if g := metric.GetGauge(); g != nil {
		return g.GetValue()
	}
	if c := metric.GetCounter(); c != nil {
		return c.GetValue()
	}
	return 0
}

func diskUsageMB(t *testing.T, root string) map[string]int64 {
	usage := map[string]int64{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		top := rel
		if idx := indexOfSep(rel); idx >= 0 {
			top = rel[:idx]
		}
		usage[top] += info.Size()
		usage["total"] += info.Size()
		return nil
	})
	require.NoError(t, err)
	for k, v := range usage {
		usage[k] = v >> 20
	}
	return usage
}

func indexOfSep(path string) int {
	for i := range path {
		if path[i] == filepath.Separator {
			return i
		}
	}
	return -1
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustAtoi(t *testing.T, value string) int {
	n, err := strconv.Atoi(value)
	require.NoError(t, err)
	return n
}
