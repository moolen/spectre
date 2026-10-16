//go:build scale

package embeddedstore

// Scale harness for the embedded graph (analysis) store behind /v1/namespace-graph,
// anomaly detection, causal paths and root-cause analysis.
//
//	SPECTRE_GRAPH_PROFILE=medium go test -tags scale -run 'TestGraphScale$' -timeout 0 -v ./internal/embeddedstore/
//
// Env knobs:
//
//	SPECTRE_GRAPH_PROFILE     small|medium|large|xlarge (default small)
//	SPECTRE_GRAPH_CHURN       override ChurnEvents
//	SPECTRE_GRAPH_WINDOW      simulated time span, e.g. 168h (default 24h)
//	SPECTRE_GRAPH_BIG_APPS    apps in the "team-big" namespace
//	SPECTRE_GRAPH_DIR         data dir (default t.TempDir())
//	SPECTRE_GRAPH_OUT         write a JSON report here
//	SPECTRE_GRAPH_QUERIES     only run queries (and the prewarm/interference/breakdown phases) whose name matches this regexp
//	SPECTRE_GRAPH_BUDGET      wall-clock budget per query spec (default 20s)
//	SPECTRE_GRAPH_CAP         skip composite queries estimated to take longer (default 300s)
//	SPECTRE_GRAPH_CPUPROFILE  write a CPU profile of the query phase
//	SPECTRE_GRAPH_HEAPPROFILE write a heap profile after the load

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type graphQueryResult struct {
	Name    string       `json:"name"`
	Latency latencyStats `json:"latency"`
	// Nodes/Edges/Items describe the result of the last run.
	Nodes int `json:"nodes,omitempty"`
	Edges int `json:"edges,omitempty"`
	Items int `json:"items,omitempty"`
	// AllocMB and PayloadReads are per run.
	AllocMB      float64 `json:"alloc_mb"`
	PayloadReads uint64  `json:"payload_reads"`
	Skipped      string  `json:"skipped,omitempty"`
	EstimatedSec float64 `json:"estimated_sec,omitempty"`
}

type graphInterference struct {
	Name             string       `json:"name"`
	IngestLatency    latencyStats `json:"ingest_latency"`
	IngestedEvents   int          `json:"ingested_events"`
	IngestPerSec     float64      `json:"ingest_per_sec"`
	TimelineLatency  latencyStats `json:"timeline_latency"`
	BackgroundQuery  string       `json:"background_query,omitempty"`
	BackgroundRuns   int          `json:"background_runs"`
	BackgroundMeanMs float64      `json:"background_mean_ms"`
}

type graphScaleReport struct {
	Profile           graphProfile        `json:"profile"`
	GOMAXPROCS        int                 `json:"gomaxprocs"`
	TotalEvents       int                 `json:"total_events"`
	DistinctResources int                 `json:"distinct_resources"`
	ActiveResources   int                 `json:"active_resources_now"`
	TeamNamespaceSize int                 `json:"team_namespace_active"`
	BigNamespaceSize  int                 `json:"big_namespace_active"`
	LoadSeconds       float64             `json:"load_seconds"`
	LiveHeapMB        float64             `json:"live_heap_mb"`
	Queries           []graphQueryResult  `json:"queries"`
	Interference      []graphInterference `json:"interference"`
	CachePrewarm      *cachePrewarmResult `json:"cache_prewarm,omitempty"`
}

type cachePrewarmResult struct {
	Namespaces        int     `json:"namespaces"`
	Measured          int     `json:"measured"`
	MeasuredSeconds   float64 `json:"measured_seconds"`
	EstimatedCycleSec float64 `json:"estimated_cycle_seconds"`
}

func graphProfileFromEnv(t *testing.T) graphProfile {
	name := envOr("SPECTRE_GRAPH_PROFILE", "small")
	profile, ok := graphProfiles[name]
	require.True(t, ok, "unknown profile %q", name)
	if v := os.Getenv("SPECTRE_GRAPH_CHURN"); v != "" {
		profile.ChurnEvents = mustAtoi(t, v)
	}
	if v := os.Getenv("SPECTRE_GRAPH_BIG_APPS"); v != "" {
		profile.BigNamespaceApps = mustAtoi(t, v)
	}
	if v := os.Getenv("SPECTRE_GRAPH_WINDOW"); v != "" {
		window, err := time.ParseDuration(v)
		require.NoError(t, err)
		profile.Window = window
	}
	return profile
}

func TestGraphScale(t *testing.T) {
	// The analyzers log every resource without a recent change at ERROR.
	require.NoError(t, logging.Initialize("fatal"))
	profile := graphProfileFromEnv(t)
	dataDir := os.Getenv("SPECTRE_GRAPH_DIR")
	if dataDir == "" {
		dataDir = t.TempDir()
	} else {
		require.NoError(t, os.RemoveAll(dataDir))
	}
	budget := 20 * time.Second
	if v := os.Getenv("SPECTRE_GRAPH_BUDGET"); v != "" {
		d, err := time.ParseDuration(v)
		require.NoError(t, err)
		budget = d
	}
	capDuration := 300 * time.Second
	if v := os.Getenv("SPECTRE_GRAPH_CAP"); v != "" {
		d, err := time.ParseDuration(v)
		require.NoError(t, err)
		capDuration = d
	}

	report := graphScaleReport{Profile: profile, GOMAXPROCS: runtime.GOMAXPROCS(0)}
	backend, err := Open(Config{
		DataDir:            dataDir,
		CheckpointInterval: 15 * time.Minute,
		MetricsRegisterer:  prometheus.NewRegistry(),
	})
	require.NoError(t, err)
	defer func() { _ = backend.Close() }()
	engine := backend.engine
	ctx := context.Background()
	require.NoError(t, engine.Start(ctx))

	workload := newGraphWorkload(profile, time.Now())
	resources := map[string]struct{}{}
	const loadBatch = 256
	batch := make([]models.Event, 0, loadBatch)
	var ingestTime time.Duration
	flushBatch := func() {
		if len(batch) > 0 {
			start := time.Now()
			require.NoError(t, engine.ProcessBatch(ctx, batch))
			ingestTime += time.Since(start)
			batch = batch[:0]
		}
	}
	// Events stream from the generator into the engine; the newest liveEvents are held
	// back in a ring to push one at a time during the interference runs.
	const liveEvents = 4000
	ring := make([]models.Event, liveEvents)
	held := 0
	load := func(event models.Event) {
		report.TotalEvents++
		resources[event.Resource.UID] = struct{}{}
		batch = append(batch, event)
		if len(batch) == loadBatch {
			flushBatch()
		}
		if report.TotalEvents%250_000 == 0 {
			t.Logf("loaded %d events (%s)", report.TotalEvents, heapString())
		}
	}
	emit := func(event models.Event) {
		slot := held % liveEvents
		if held >= liveEvents {
			load(ring[slot])
		}
		ring[slot] = event
		held++
	}
	workload.Initial(emit)
	workload.Churn(emit)
	flushBatch()
	require.NoError(t, engine.Flush(ctx))
	report.LoadSeconds = ingestTime.Seconds()
	report.DistinctResources = len(resources)
	live := make([]models.Event, 0, min(held, liveEvents))
	for i := max(held-liveEvents, 0); i < held; i++ {
		live = append(live, ring[i%liveEvents])
	}
	ring = nil
	// Let background compaction and checkpoints finish so their buffers are not counted.
	time.Sleep(10 * time.Second)
	runtime.GC()
	runtime.GC()
	report.LiveHeapMB = heapMB()
	if path := os.Getenv("SPECTRE_GRAPH_HEAPPROFILE"); path != "" {
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, pprof.WriteHeapProfile(f))
		require.NoError(t, f.Close())
	}

	store := backend.AnalysisStore()
	nowNs := time.Now().UnixNano()
	store.projection.mu.RLock()
	for _, record := range store.projection.resourcesByUID {
		if record.activeVersionAt(nowNs) != nil {
			report.ActiveResources++
		}
	}
	report.TeamNamespaceSize = len(store.projection.activeResourcesInNamespace("team-001", nowNs))
	report.BigNamespaceSize = len(store.projection.activeResourcesInNamespace("team-big", nowNs))
	store.projection.mu.RUnlock()
	t.Logf("load done: %d events, %d distinct / %d active resources in %.1fs, heap %.0f MB; team-001 has %d, team-big %d",
		report.TotalEvents, report.DistinctResources, report.ActiveResources, report.LoadSeconds, report.LiveHeapMB,
		report.TeamNamespaceSize, report.BigNamespaceSize)

	if path := os.Getenv("SPECTRE_GRAPH_CPUPROFILE"); path != "" {
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, pprof.StartCPUProfile(f))
		defer func() {
			pprof.StopCPUProfile()
			_ = f.Close()
		}()
	}

	report.Queries = runGraphQueries(t, store, workload, budget, capDuration)
	if graphQuerySelected("prewarm") {
		report.CachePrewarm = measureCachePrewarm(t, store, workload, budget)
	}
	if graphQuerySelected("interference") {
		report.Interference = measureGraphInterference(t, engine, store, live)
	}

	out, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	t.Logf("report:\n%s", out)
	if path := os.Getenv("SPECTRE_GRAPH_OUT"); path != "" {
		require.NoError(t, os.WriteFile(path, out, 0o644))
	}
}

func graphQuerySelected(name string) bool {
	filter := os.Getenv("SPECTRE_GRAPH_QUERIES")
	return filter == "" || regexp.MustCompile(filter).MatchString(name)
}

type graphQuerySpec struct {
	name string
	// estimate, when set, returns the expected duration of one run; runs above the cap
	// are skipped and reported with the estimate.
	estimate func() time.Duration
	run      func() (nodes, edges, items int)
}

// measureGraphQuery runs spec until the budget is spent (at least once, at most 10 times).
func measureGraphQuery(t *testing.T, spec graphQuerySpec, budget, capDuration time.Duration) graphQueryResult {
	result := graphQueryResult{Name: spec.name}
	if spec.estimate != nil {
		if estimate := spec.estimate(); estimate > capDuration {
			result.Skipped = "estimated above cap"
			result.EstimatedSec = estimate.Seconds()
			t.Logf("%-55s skipped, estimated %.0fs", spec.name, estimate.Seconds())
			return result
		}
	}
	var samples []time.Duration
	var allocTotal uint64
	var readsTotal uint64
	start := time.Now()
	for len(samples) < 10 && (len(samples) == 0 || time.Since(start) < budget) {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		reads := versionDataReads.Load()
		began := time.Now()
		result.Nodes, result.Edges, result.Items = spec.run()
		samples = append(samples, time.Since(began))
		readsTotal += versionDataReads.Load() - reads
		runtime.ReadMemStats(&after)
		allocTotal += after.TotalAlloc - before.TotalAlloc
	}
	result.Latency = summarize(samples)
	result.AllocMB = float64(allocTotal) / float64(len(samples)) / (1 << 20)
	result.PayloadReads = readsTotal / uint64(len(samples))
	t.Logf("%-55s p50 %9.1f ms  max %9.1f ms  nodes %5d edges %6d items %5d  alloc %8.1f MB  reads %7d",
		spec.name, result.Latency.P50Ms, result.Latency.MaxMs, result.Nodes, result.Edges, result.Items, result.AllocMB, result.PayloadReads)
	return result
}

// pickPod returns the most recently changed pod in the namespace and the most recently
// changed crash-looping one (or the former when none crash-loops). Root-cause analysis
// only runs for resources that changed near the failure timestamp.
func pickPod(t *testing.T, store *Store, namespace string, nowNs int64) (string, string, int64) {
	store.projection.mu.RLock()
	defer store.projection.mu.RUnlock()
	var newest, crashing *resourceVersion
	for _, version := range store.projection.activeResourcesInNamespace(namespace, nowNs) {
		if version.meta.Kind != "Pod" {
			continue
		}
		if newest == nil || version.timestamp > newest.timestamp {
			newest = version
		}
		if (crashing == nil || version.timestamp > crashing.timestamp) && strings.Contains(string(versionPayload(version)), "CrashLoopBackOff") {
			crashing = version
		}
	}
	require.NotNil(t, newest, "no pod in %s", namespace)
	if crashing == nil {
		crashing = newest
	}
	return newest.meta.UID, crashing.meta.UID, crashing.timestamp
}

func runGraphQueries(t *testing.T, store *Store, workload *graphWorkload, budget, capDuration time.Duration) []graphQueryResult {
	ctx := context.Background()
	nowNs := time.Now().UnixNano()
	lookback := int64(30 * time.Minute)
	podUID, crashingPodUID, crashedAtNs := pickPod(t, store, "team-001", nowNs)
	bigPodUID, _, _ := pickPod(t, store, "team-big", nowNs)
	rootCause := analysis.NewRootCauseAnalyzer(store)
	detector := anomaly.NewDetector(store)
	nsAnalyzer := namespacegraph.NewAnalyzer(store)

	nsGraph := func(namespace string, ts int64, depth, limit int) func() (int, int, int) {
		return func() (int, int, int) {
			data, err := store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
				Namespace: namespace, TimestampNs: ts, LookbackNs: lookback, MaxDepth: depth, Limit: limit,
			})
			require.NoError(t, err)
			return len(data.Graph.Nodes), len(data.Graph.Edges), 0
		}
	}
	window := analysisstore.ResourceWindow{FailureTimestampNs: nowNs, LookbackNs: lookback}
	rootCauseRun := func(uid string) func() (int, int, int) {
		return func() (int, int, int) {
			result, err := rootCause.Analyze(ctx, analysis.AnalyzeInput{
				ResourceUID: uid, FailureTimestamp: crashedAtNs, LookbackNs: lookback, MaxDepth: 5, MinConfidence: 0.5, Format: analysis.FormatDiff,
			})
			require.NoError(t, err)
			return len(result.Incident.Graph.Nodes), len(result.Incident.Graph.Edges), 0
		}
	}
	detectRun := func(uid string) func() (int, int, int) {
		return func() (int, int, int) {
			result, err := detector.Detect(ctx, anomaly.DetectInput{
				ResourceUID: uid, Start: (crashedAtNs - lookback) / 1e9, End: crashedAtNs / 1e9,
			})
			require.NoError(t, err)
			return result.Metadata.NodesAnalyzed, 0, len(result.Anomalies)
		}
	}
	nsAnalyze := func(namespace string, limit int, anomalies, causal bool) func() (int, int, int) {
		return func() (int, int, int) {
			result, err := nsAnalyzer.Analyze(ctx, namespacegraph.AnalyzeInput{
				Namespace: namespace, Timestamp: nowNs, Limit: limit, IncludeAnomalies: anomalies, IncludeCausalPaths: causal,
				Lookback: time.Duration(lookback),
			})
			require.NoError(t, err)
			return len(result.Graph.Nodes), len(result.Graph.Edges), len(result.Anomalies)
		}
	}
	// The UI loads every page of a namespace with anomalies and causal paths enabled.
	nsAnalyzeAllPages := func(namespace string) func() (int, int, int) {
		return func() (int, int, int) {
			cursor := ""
			nodes, edges, anomalies := 0, 0, 0
			for {
				result, err := nsAnalyzer.Analyze(ctx, namespacegraph.AnalyzeInput{
					Namespace: namespace, Timestamp: nowNs, Limit: 50, IncludeAnomalies: true, IncludeCausalPaths: true,
					Lookback: time.Duration(lookback), Cursor: cursor,
				})
				require.NoError(t, err)
				nodes += len(result.Graph.Nodes)
				edges += len(result.Graph.Edges)
				anomalies += len(result.Anomalies)
				if !result.Metadata.HasMore || result.Metadata.NextCursor == "" {
					return nodes, edges, anomalies
				}
				cursor = result.Metadata.NextCursor
			}
		}
	}

	// The per-page graph cost and the per-candidate cost of anomaly detection, measured
	// once, size the estimates for the UI paths. Only candidates that changed within
	// ±5 minutes of the timestamp get a full analysis; the rest return early.
	var detectCost, pageCost time.Duration
	estimateUI := func(namespace string, pages int) func() time.Duration {
		return func() time.Duration {
			if detectCost == 0 {
				start := time.Now()
				detectRun(crashingPodUID)()
				detectCost = time.Since(start)
				start = time.Now()
				nsGraph(namespace, nowNs, 1, 50)()
				pageCost = time.Since(start)
			}
			store.projection.mu.RLock()
			resources := store.projection.activeResourcesInNamespace(namespace, nowNs)
			store.projection.mu.RUnlock()
			if pages > 0 {
				resources = resources[:min(len(resources), pages*50)]
			}
			candidates := 0
			for _, version := range resources {
				if nowNs-version.timestamp > int64(5*time.Minute) {
					continue
				}
				switch version.meta.Kind {
				case "Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob", "Service", "Ingress", "Node":
					candidates++
				}
			}
			pageCount := (len(resources) + 49) / 50
			return time.Duration(pageCount)*pageCost + time.Duration(candidates)*detectCost
		}
	}

	historical := nowNs - int64(12*time.Hour)
	specs := []graphQuerySpec{
		{name: "store GetResource", run: func() (int, int, int) {
			_, err := store.GetResource(ctx, podUID)
			require.NoError(t, err)
			return 1, 0, 0
		}},
		{name: "store GetOwnershipChain pod", run: func() (int, int, int) {
			chain, err := store.GetOwnershipChain(ctx, podUID, nowNs, 5)
			require.NoError(t, err)
			return len(chain), 0, 0
		}},
		{name: "store GetManagers pod", run: func() (int, int, int) {
			managers, err := store.GetManagers(ctx, []string{podUID}, 0.5)
			require.NoError(t, err)
			return len(managers), 0, 0
		}},
		{name: "store GetChangeEvents pod 30m", run: func() (int, int, int) {
			events, err := store.GetChangeEvents(ctx, []string{podUID}, window)
			require.NoError(t, err)
			return 0, 0, len(events[podUID])
		}},
		{name: "store GetK8sEvents pod 30m", run: func() (int, int, int) {
			events, err := store.GetK8sEvents(ctx, []string{podUID}, window)
			require.NoError(t, err)
			return 0, 0, len(events[podUID])
		}},
		{name: "store GetRelatedResources pod", run: func() (int, int, int) {
			related, err := store.GetRelatedResources(ctx, []string{podUID}, window)
			require.NoError(t, err)
			return 0, 0, len(related[podUID])
		}},
		{name: "store GetRelatedResources pod team-big", run: func() (int, int, int) {
			related, err := store.GetRelatedResources(ctx, []string{bigPodUID}, window)
			require.NoError(t, err)
			return 0, 0, len(related[bigPodUID])
		}},
		{name: "store NamespaceGraph team-001 limit=50 depth=1", run: nsGraph("team-001", nowNs, 1, 50)},
		{name: "store NamespaceGraph team-001 limit=500 depth=1", run: nsGraph("team-001", nowNs, 1, 500)},
		{name: "store NamespaceGraph team-001 limit=50 depth=3", run: nsGraph("team-001", nowNs, 3, 50)},
		{name: "store NamespaceGraph team-001 limit=50 12h ago", run: nsGraph("team-001", historical, 1, 50)},
		{name: "store NamespaceGraph team-big limit=50 depth=1", run: nsGraph("team-big", nowNs, 1, 50)},
		{name: "store NamespaceGraph team-big limit=500 depth=1", run: nsGraph("team-big", nowNs, 1, 500)},
		{name: "analysis RootCause pod", run: rootCauseRun(crashingPodUID)},
		{name: "analysis anomaly Detect pod", run: detectRun(crashingPodUID)},
		{name: "nsgraph Analyze team-001 page=50 plain", run: nsAnalyze("team-001", 50, false, false)},
		{name: "nsgraph Analyze team-001 page=500 plain (cache)", run: nsAnalyze("team-001", 500, false, false)},
		{name: "nsgraph Analyze team-001 page=50 anomalies+paths (UI)", estimate: estimateUI("team-001", 1), run: nsAnalyze("team-001", 50, true, true)},
		{name: "nsgraph Analyze team-001 all pages (UI)", estimate: estimateUI("team-001", 0), run: nsAnalyzeAllPages("team-001")},
		{name: "nsgraph Analyze team-big page=50 anomalies+paths (UI)", estimate: estimateUI("team-big", 1), run: nsAnalyze("team-big", 50, true, true)},
		{name: "nsgraph Analyze team-big all pages (UI)", estimate: estimateUI("team-big", 0), run: nsAnalyzeAllPages("team-big")},
	}
	_ = workload

	var results []graphQueryResult
	for _, spec := range specs {
		if !graphQuerySelected(spec.name) {
			continue
		}
		results = append(results, measureGraphQuery(t, spec, budget, capDuration))
	}
	if graphQuerySelected("breakdown") {
		results = append(results, namespaceGraphBreakdown(t, store, "team-001", nowNs, lookback)...)
	}
	return results
}

// namespaceGraphBreakdown times the stages of GetNamespaceGraph for one page of 50.
func namespaceGraphBreakdown(t *testing.T, store *Store, namespace string, nowNs, lookback int64) []graphQueryResult {
	p := store.projection
	p.mu.RLock()
	defer p.mu.RUnlock()
	stage := func(name string, fn func() int) graphQueryResult {
		reads := versionDataReads.Load()
		start := time.Now()
		items := fn()
		res := graphQueryResult{Name: "breakdown " + name, Items: items, PayloadReads: versionDataReads.Load() - reads}
		res.Latency = summarize([]time.Duration{time.Since(start)})
		t.Logf("%-55s %9.1f ms  items %6d  reads %7d", res.Name, res.Latency.P50Ms, items, res.PayloadReads)
		return res
	}
	var page []*resourceVersion
	var results []graphQueryResult
	results = append(results, stage("activeResourcesInNamespace", func() int {
		page = p.activeResourcesInNamespace(namespace, nowNs)
		n := len(page)
		page = page[:min(50, len(page))]
		return n
	}))
	results = append(results, stage("clusterScopedReachableFrom", func() int {
		return len(store.clusterScopedReachableFrom(page, nowNs, lookback, 1))
	}))
	selected := map[string]*resourceVersion{}
	for _, version := range page {
		selected[version.meta.UID] = version
	}
	results = append(results, stage("namespaceEdgesForSet (page)", func() int {
		return len(store.namespaceEdgesForSet(selected, nowNs, lookback))
	}))
	results = append(results, stage("spec diffs (page)", func() int {
		for _, version := range page {
			if earliest, latest, ok := store.specDiffVersions(version.meta.UID, nowNs, lookback); ok {
				specDiff(earliest, latest)
			}
		}
		return len(page)
	}))
	return results
}

// measureCachePrewarm runs the namespace graph cache's pre-warm computation (one
// limit=500 graph per namespace) over as many namespaces as fit in the budget.
func measureCachePrewarm(t *testing.T, store *Store, workload *graphWorkload, budget time.Duration) *cachePrewarmResult {
	analyzer := namespacegraph.NewAnalyzer(store)
	namespaces := append([]string(nil), workload.namespaces...)
	sort.Strings(namespaces)
	result := &cachePrewarmResult{Namespaces: len(namespaces)}
	start := time.Now()
	for _, ns := range namespaces {
		_, err := analyzer.Analyze(context.Background(), namespacegraph.AnalyzeInput{
			Namespace: ns, Timestamp: time.Now().UnixNano(), MaxDepth: namespacegraph.DefaultMaxDepth, Limit: namespacegraph.MaxLimit,
		})
		require.NoError(t, err)
		result.Measured++
		if time.Since(start) > budget {
			break
		}
	}
	result.MeasuredSeconds = time.Since(start).Seconds()
	result.EstimatedCycleSec = result.MeasuredSeconds / float64(result.Measured) * float64(result.Namespaces)
	t.Logf("cache pre-warm: %d of %d namespaces in %.1fs, full cycle ~%.0fs", result.Measured, result.Namespaces, result.MeasuredSeconds, result.EstimatedCycleSec)
	return result
}

// measureGraphInterference pushes live events one at a time and runs timeline queries,
// first alone and then while namespace graph queries run back to back in the background
// (what the cache refresher and a few UI users do).
func measureGraphInterference(t *testing.T, engine *Engine, store *Store, live []models.Event) []graphInterference {
	ctx := context.Background()
	exec := engine.QueryExecutor()
	half := len(live) / 2
	// Each phase stops after phaseDuration, so a stalled ingest path cannot run for hours.
	const phaseDuration = 30 * time.Second
	runForeground := func(events []models.Event) ([]time.Duration, []time.Duration) {
		var ingest, timeline []time.Duration
		phaseStart := time.Now()
		for i, event := range events {
			if time.Since(phaseStart) > phaseDuration {
				break
			}
			start := time.Now()
			require.NoError(t, engine.ProcessEvent(ctx, event))
			ingest = append(ingest, time.Since(start))
			if i%100 == 0 {
				nowSec := time.Now().Unix()
				start := time.Now()
				_, _, err := exec.ExecutePaginated(ctx, &models.QueryRequest{StartTimestamp: nowSec - 3600, EndTimestamp: nowSec},
					&models.PaginationRequest{PageSize: 100})
				require.NoError(t, err)
				timeline = append(timeline, time.Since(start))
			}
			// Watch traffic arrives spread out, not in a tight loop.
			time.Sleep(time.Millisecond)
		}
		return ingest, timeline
	}

	var results []graphInterference
	phaseStart := time.Now()
	ingest, timeline := runForeground(live[:half])
	results = append(results, graphInterference{
		Name: "baseline", IngestLatency: summarize(ingest), TimelineLatency: summarize(timeline),
		IngestedEvents: len(ingest), IngestPerSec: float64(len(ingest)) / time.Since(phaseStart).Seconds(),
	})

	var stop atomic.Bool
	var runs atomic.Int64
	var busy atomic.Int64
	var wg sync.WaitGroup
	background := "NamespaceGraph limit=500 team-big + team-001 loop"
	wg.Add(1)
	go func() {
		defer wg.Done()
		namespaces := []string{"team-big", "team-001"}
		for i := 0; !stop.Load(); i++ {
			start := time.Now()
			_, err := store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
				Namespace: namespaces[i%2], TimestampNs: time.Now().UnixNano(), MaxDepth: 1, Limit: 500,
			})
			require.NoError(t, err)
			runs.Add(1)
			busy.Add(int64(time.Since(start)))
		}
	}()
	phaseStart = time.Now()
	ingest, timeline = runForeground(live[half:])
	elapsed := time.Since(phaseStart)
	stop.Store(true)
	wg.Wait()
	bg := graphInterference{
		Name: "with background graph queries", IngestLatency: summarize(ingest), TimelineLatency: summarize(timeline),
		BackgroundQuery: background, BackgroundRuns: int(runs.Load()),
		IngestedEvents: len(ingest), IngestPerSec: float64(len(ingest)) / elapsed.Seconds(),
	}
	if runs.Load() > 0 {
		bg.BackgroundMeanMs = float64(busy.Load()) / float64(runs.Load()) / 1e6
	}
	results = append(results, bg)
	for _, r := range results {
		t.Logf("interference %-32s ingest %d events (%.0f/s) p50 %.2f p99 %.1f max %.1f ms | timeline p50 %.1f max %.1f ms | bg runs %d mean %.0f ms",
			r.Name, r.IngestedEvents, r.IngestPerSec, r.IngestLatency.P50Ms, r.IngestLatency.P99Ms, r.IngestLatency.MaxMs, r.TimelineLatency.P50Ms, r.TimelineLatency.MaxMs,
			r.BackgroundRuns, r.BackgroundMeanMs)
	}
	return results
}
