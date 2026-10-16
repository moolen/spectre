//go:build scale

package embeddedstore

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// scaleProfile describes a synthetic cluster. Object counts follow typical ratios
// (~30 pods per node, ~3 pods per Deployment, 3 retained ReplicaSets per Deployment).
type scaleProfile struct {
	Name            string
	Nodes           int
	Namespaces      int
	DeploymentsPerN int
	Replicas        int
	ConfigMapsPerN  int
	// ChurnEvents is the number of update/replace events generated after the initial list.
	ChurnEvents int
	// Window is the simulated wall-clock span the events are spread over, ending at now.
	Window time.Duration
	// K8sEventPercent is the share of churn emitted as core/v1 Event objects. Events are
	// not in the chart's default watch list, so profiles default to 0.
	K8sEventPercent int
}

var scaleProfiles = map[string]scaleProfile{
	// ~1.5k objects, ~1 day of light churn.
	"small": {Name: "small", Nodes: 10, Namespaces: 10, DeploymentsPerN: 10, Replicas: 3, ConfigMapsPerN: 10, ChurnEvents: 50_000, Window: 24 * time.Hour},
	// ~15k objects, 100 nodes / 3k pods.
	"medium": {Name: "medium", Nodes: 100, Namespaces: 50, DeploymentsPerN: 20, Replicas: 3, ConfigMapsPerN: 20, ChurnEvents: 250_000, Window: 24 * time.Hour},
	// ~75k objects, 500 nodes / 15k pods.
	"large": {Name: "large", Nodes: 500, Namespaces: 200, DeploymentsPerN: 25, Replicas: 3, ConfigMapsPerN: 25, ChurnEvents: 750_000, Window: 24 * time.Hour},
	// ~150k objects, 1000 nodes / 30k pods.
	"xlarge": {Name: "xlarge", Nodes: 1000, Namespaces: 400, DeploymentsPerN: 25, Replicas: 3, ConfigMapsPerN: 25, ChurnEvents: 1_500_000, Window: 24 * time.Hour},
}

type scaleObject struct {
	meta    models.ResourceMetadata
	version int
	owner   string
	node    string
	alive   bool
}

// scaleWorkload deterministically generates watcher-shaped events for a profile.
type scaleWorkload struct {
	profile    scaleProfile
	rng        *rand.Rand
	seq        int
	start      int64
	step       int64
	clock      int64
	pods       []*scaleObject
	nodes      []*scaleObject
	deploys    []*scaleObject
	rsByDeploy map[string]*scaleObject
	configMaps []*scaleObject
	padding    string
}

func newScaleWorkload(profile scaleProfile, now time.Time) *scaleWorkload {
	w := &scaleWorkload{
		profile:    profile,
		rng:        rand.New(rand.NewSource(42)),
		rsByDeploy: make(map[string]*scaleObject),
		padding:    strings.Repeat("x", 4096),
	}
	total := int64(w.initialObjectCount() + profile.ChurnEvents)
	w.start = now.Add(-profile.Window).UnixNano()
	w.step = int64(profile.Window) / max(total, 1)
	w.clock = w.start
	return w
}

func (w *scaleWorkload) initialObjectCount() int {
	p := w.profile
	deploys := p.Namespaces * p.DeploymentsPerN
	return p.Nodes + p.Namespaces + deploys*(1+1+1+p.Replicas) + p.Namespaces*p.ConfigMapsPerN
}

func (w *scaleWorkload) nextMeta(group, version, kind, namespace, name string) models.ResourceMetadata {
	w.seq++
	return models.ResourceMetadata{
		Group:     group,
		Version:   version,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		UID:       fmt.Sprintf("%08x-0000-4000-8000-%012x", w.seq, w.seq),
	}
}

func (w *scaleWorkload) tick() int64 {
	w.clock += w.step
	return w.clock
}

func (w *scaleWorkload) event(eventType models.EventType, obj *scaleObject) models.Event {
	w.seq++
	ts := w.tick()
	obj.version++
	var data []byte
	if eventType != models.EventTypeDelete {
		data = w.render(obj, ts)
	}
	return models.Event{
		ID:        fmt.Sprintf("evt-%012d", w.seq),
		Timestamp: ts,
		Type:      eventType,
		Resource:  obj.meta,
		Data:      data,
		DataSize:  int32(len(data)),
	}
}

// Initial emits the CREATE events a watcher produces on its first list.
func (w *scaleWorkload) Initial(emit func(models.Event)) {
	p := w.profile
	for i := 0; i < p.Nodes; i++ {
		node := &scaleObject{meta: w.nextMeta("", "v1", "Node", "", fmt.Sprintf("node-%04d", i)), alive: true}
		w.nodes = append(w.nodes, node)
		emit(w.event(models.EventTypeCreate, node))
	}
	for n := 0; n < p.Namespaces; n++ {
		ns := fmt.Sprintf("team-%03d", n)
		for c := 0; c < p.ConfigMapsPerN; c++ {
			cm := &scaleObject{meta: w.nextMeta("", "v1", "ConfigMap", ns, fmt.Sprintf("config-%03d", c)), alive: true}
			w.configMaps = append(w.configMaps, cm)
			emit(w.event(models.EventTypeCreate, cm))
		}
		for d := 0; d < p.DeploymentsPerN; d++ {
			name := fmt.Sprintf("svc-%03d", d)
			dep := &scaleObject{meta: w.nextMeta("apps", "v1", "Deployment", ns, name), alive: true}
			w.deploys = append(w.deploys, dep)
			emit(w.event(models.EventTypeCreate, dep))
			svc := &scaleObject{meta: w.nextMeta("", "v1", "Service", ns, name), alive: true}
			emit(w.event(models.EventTypeCreate, svc))
			rs := w.newReplicaSet(dep)
			emit(w.event(models.EventTypeCreate, rs))
			for r := 0; r < p.Replicas; r++ {
				emit(w.event(models.EventTypeCreate, w.newPod(rs)))
			}
		}
	}
}

func (w *scaleWorkload) newReplicaSet(dep *scaleObject) *scaleObject {
	rs := &scaleObject{
		meta:  w.nextMeta("apps", "v1", "ReplicaSet", dep.meta.Namespace, fmt.Sprintf("%s-%06x", dep.meta.Name, w.seq)),
		owner: dep.meta.UID,
		alive: true,
	}
	w.rsByDeploy[dep.meta.UID] = rs
	return rs
}

func (w *scaleWorkload) newPod(rs *scaleObject) *scaleObject {
	pod := &scaleObject{
		meta:  w.nextMeta("", "v1", "Pod", rs.meta.Namespace, fmt.Sprintf("%s-%05x", rs.meta.Name, w.seq&0xfffff)),
		owner: rs.meta.UID,
		node:  w.nodes[w.rng.Intn(len(w.nodes))].meta.Name,
		alive: true,
	}
	w.pods = append(w.pods, pod)
	return pod
}

// Churn emits steady-state update traffic: pod status flaps, pod replacement,
// rollouts, node heartbeats, config edits.
func (w *scaleWorkload) Churn(emit func(models.Event)) {
	for emitted := 0; emitted < w.profile.ChurnEvents; {
		if w.profile.K8sEventPercent > 0 && w.rng.Intn(100) < w.profile.K8sEventPercent {
			emit(w.k8sEvent(w.livePod()))
			emitted++
			continue
		}
		roll := w.rng.Intn(100)
		switch {
		case roll < 55: // pod status update
			pod := w.livePod()
			emit(w.event(models.EventTypeUpdate, pod))
			emitted++
		case roll < 70: // pod replaced (crash/evict/scale): delete + create + 2 status updates
			old := w.livePod()
			old.alive = false
			emit(w.event(models.EventTypeDelete, old))
			rs := w.rsForPod(old)
			fresh := w.newPod(rs)
			emit(w.event(models.EventTypeCreate, fresh))
			emit(w.event(models.EventTypeUpdate, fresh))
			emit(w.event(models.EventTypeUpdate, fresh))
			emitted += 4
		case roll < 73: // rollout: deployment update, new RS, RS update
			dep := w.deploys[w.rng.Intn(len(w.deploys))]
			emit(w.event(models.EventTypeUpdate, dep))
			rs := w.newReplicaSet(dep)
			emit(w.event(models.EventTypeCreate, rs))
			emit(w.event(models.EventTypeUpdate, rs))
			emitted += 3
		case roll < 93: // node status update
			node := w.nodes[w.rng.Intn(len(w.nodes))]
			emit(w.event(models.EventTypeUpdate, node))
			emitted++
		default: // config map edit
			cm := w.configMaps[w.rng.Intn(len(w.configMaps))]
			emit(w.event(models.EventTypeUpdate, cm))
			emitted++
		}
	}
}

func (w *scaleWorkload) k8sEvent(pod *scaleObject) models.Event {
	meta := w.nextMeta("", "v1", "Event", pod.meta.Namespace, fmt.Sprintf("%s.%x", pod.meta.Name, w.seq))
	meta.InvolvedObjectUID = pod.meta.UID
	reasons := []string{"Pulled", "Created", "Started", "Unhealthy", "BackOff", "Killing"}
	reason := reasons[w.rng.Intn(len(reasons))]
	w.seq++
	ts := w.tick()
	stamp := time.Unix(0, ts).UTC().Format(time.RFC3339)
	data := fmt.Sprintf(`{"apiVersion":"v1","kind":"Event","metadata":{"name":%q,"namespace":%q,"uid":%q,"creationTimestamp":%q},"involvedObject":{"kind":"Pod","namespace":%q,"name":%q,"uid":%q},"reason":%q,"message":"Readiness probe failed: HTTP probe failed with statuscode: 503 %s","source":{"component":"kubelet","host":%q},"firstTimestamp":%q,"lastTimestamp":%q,"count":1,"type":"Warning"}`,
		meta.Name, meta.Namespace, meta.UID, stamp, pod.meta.Namespace, pod.meta.Name, pod.meta.UID, reason, w.padding[:300], pod.node, stamp, stamp)
	return models.Event{
		ID:        fmt.Sprintf("evt-%012d", w.seq),
		Timestamp: ts,
		Type:      models.EventTypeCreate,
		Resource:  meta,
		Data:      []byte(data),
		DataSize:  int32(len(data)),
	}
}

func (w *scaleWorkload) livePod() *scaleObject {
	for {
		idx := w.rng.Intn(len(w.pods))
		pod := w.pods[idx]
		if pod.alive {
			return pod
		}
		// Compact dead pods out lazily so lookups stay O(1) amortised.
		w.pods[idx] = w.pods[len(w.pods)-1]
		w.pods = w.pods[:len(w.pods)-1]
	}
}

func (w *scaleWorkload) rsForPod(pod *scaleObject) *scaleObject {
	return &scaleObject{meta: models.ResourceMetadata{
		Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: pod.meta.Namespace,
		Name: pod.meta.Name[:strings.LastIndex(pod.meta.Name, "-")], UID: pod.owner,
	}}
}

// render produces JSON sized like real objects with managedFields stripped:
// Pod ~4KB, Node ~9KB, Deployment/ReplicaSet ~3KB, ConfigMap ~2KB, Service ~1KB.
func (w *scaleWorkload) render(obj *scaleObject, ts int64) []byte {
	m := obj.meta
	stamp := time.Unix(0, ts).UTC().Format(time.RFC3339)
	apiVersion := m.Version
	if m.Group != "" {
		apiVersion = m.Group + "/" + m.Version
	}
	var b strings.Builder
	fmt.Fprintf(&b, `{"apiVersion":%q,"kind":%q,"metadata":{"name":%q,"namespace":%q,"uid":%q,"resourceVersion":"%d","generation":%d,"creationTimestamp":%q,"labels":{"app":%q,"team":%q,"pod-template-hash":"5d8f7c9b4"},"annotations":{"deployment.kubernetes.io/revision":"%d","kubectl.kubernetes.io/last-applied-configuration":%q}`,
		apiVersion, m.Kind, m.Name, m.Namespace, m.UID, w.seq, obj.version, stamp, m.Name, m.Namespace, obj.version, w.padding[:w.paddingFor(m.Kind)])
	if obj.owner != "" {
		ownerKind := "ReplicaSet"
		if m.Kind == "ReplicaSet" {
			ownerKind = "Deployment"
		}
		fmt.Fprintf(&b, `,"ownerReferences":[{"apiVersion":"apps/v1","kind":%q,"name":"owner","uid":%q,"controller":true}]`, ownerKind, obj.owner)
	}
	b.WriteString(`}`)
	ready := "True"
	if obj.version%3 == 0 {
		ready = "False"
	}
	switch m.Kind {
	case "Pod":
		fmt.Fprintf(&b, `,"spec":{"nodeName":%q,"serviceAccountName":"default","containers":[{"name":"app","image":"registry.example.com/%s:v%d","ports":[{"containerPort":8080}],"resources":{"requests":{"cpu":"100m","memory":"128Mi"}},"env":[{"name":"A","value":"1"},{"name":"B","value":"2"}],"readinessProbe":{"httpGet":{"path":"/ready","port":8080}}},{"name":"sidecar","image":"registry.example.com/proxy:1.2.3"}]},"status":{"phase":"Running","podIP":"10.0.%d.%d","conditions":[{"type":"Ready","status":%q,"lastTransitionTime":%q},{"type":"ContainersReady","status":%q,"lastTransitionTime":%q}],"containerStatuses":[{"name":"app","ready":true,"restartCount":%d,"state":{"running":{"startedAt":%q}}}]}}`,
			obj.node, m.Name, obj.version/5, w.seq%255, obj.version%255, ready, stamp, ready, stamp, obj.version, stamp)
	case "Node":
		fmt.Fprintf(&b, `,"spec":{"podCIDR":"10.0.0.0/24"},"status":{"conditions":[{"type":"Ready","status":"True","lastHeartbeatTime":%q},{"type":"MemoryPressure","status":"False","lastHeartbeatTime":%q}],"allocatable":{"cpu":"16","memory":"64Gi","pods":"110"}}}`, stamp, stamp)
	case "Deployment", "ReplicaSet":
		fmt.Fprintf(&b, `,"spec":{"replicas":%d,"selector":{"matchLabels":{"app":%q}},"template":{"metadata":{"labels":{"app":%q}},"spec":{"containers":[{"name":"app","image":"registry.example.com/%s:v%d"}]}}},"status":{"replicas":%d,"readyReplicas":%d,"observedGeneration":%d}}`,
			w.profile.Replicas, m.Name, m.Name, m.Name, obj.version, w.profile.Replicas, w.profile.Replicas, obj.version)
	case "ConfigMap":
		fmt.Fprintf(&b, `,"data":{"config.yaml":"revision: %d"}}`, obj.version)
	default:
		fmt.Fprintf(&b, `,"spec":{"type":"ClusterIP","ports":[{"port":80,"targetPort":8080}],"selector":{"app":%q}}}`, m.Name)
	}
	return []byte(b.String())
}

func (w *scaleWorkload) paddingFor(kind string) int {
	switch kind {
	case "Pod":
		return 2800
	case "Node":
		return 4000 // stands in for the images list
	case "Deployment", "ReplicaSet":
		return 2200
	case "ConfigMap":
		return 1600
	default:
		return 600
	}
}
