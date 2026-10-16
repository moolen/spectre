//go:build scale

package embeddedstore

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// graphProfile describes a synthetic cluster for the graph scale harness. Unlike the
// timeline workload, every object carries the fields the analysis store derives edges
// from: owner references, selectors that match pod labels, volume and envFrom
// references, service accounts bound by RoleBindings, Ingress backends and Flux
// manager labels.
type graphProfile struct {
	Name       string
	Nodes      int
	Namespaces int
	// AppsPerNamespace is the number of applications (Deployment, ReplicaSet, Pods,
	// Service, ServiceAccount, Role, RoleBinding, ConfigMap, Secret, every 4th an
	// Ingress) in each namespace.
	AppsPerNamespace int
	Replicas         int
	// BigNamespaceApps adds one namespace ("team-big") with this many apps, the
	// "everything in one namespace" shape some clusters have.
	BigNamespaceApps int
	ChurnEvents      int
	Window           time.Duration
	K8sEventPercent  int
}

// Object counts are close to the timeline profiles: ~30 pods per node.
var graphProfiles = map[string]graphProfile{
	"small":  {Name: "small", Nodes: 10, Namespaces: 10, AppsPerNamespace: 10, Replicas: 3, BigNamespaceApps: 50, ChurnEvents: 50_000, Window: 24 * time.Hour, K8sEventPercent: 5},
	"medium": {Name: "medium", Nodes: 100, Namespaces: 50, AppsPerNamespace: 20, Replicas: 3, BigNamespaceApps: 200, ChurnEvents: 250_000, Window: 24 * time.Hour, K8sEventPercent: 5},
	"large":  {Name: "large", Nodes: 500, Namespaces: 200, AppsPerNamespace: 25, Replicas: 3, BigNamespaceApps: 500, ChurnEvents: 750_000, Window: 24 * time.Hour, K8sEventPercent: 5},
	"xlarge": {Name: "xlarge", Nodes: 1000, Namespaces: 400, AppsPerNamespace: 25, Replicas: 3, BigNamespaceApps: 1000, ChurnEvents: 1_500_000, Window: 24 * time.Hour, K8sEventPercent: 5},
}

type graphObject struct {
	meta    models.ResourceMetadata
	version int
	alive   bool
	app     string
	// owner is the controller's UID and ownerKind its kind.
	owner     string
	ownerKind string
	node      string
	hash      string
	crashing  bool
	release   string
	ingressTo string
}

type graphApp struct {
	namespace string
	name      string
	deploy    *graphObject
	rs        *graphObject
	pods      []*graphObject
	configMap *graphObject
}

type graphWorkload struct {
	profile    graphProfile
	rng        *rand.Rand
	seq        int
	clock      int64
	step       int64
	nodes      []*graphObject
	apps       []*graphApp
	namespaces []string
	padding    string
}

func newGraphWorkload(profile graphProfile, now time.Time) *graphWorkload {
	w := &graphWorkload{
		profile: profile,
		rng:     rand.New(rand.NewSource(7)),
		padding: strings.Repeat("x", 4096),
	}
	total := int64(w.initialObjectCount() + profile.ChurnEvents)
	start := now.Add(-profile.Window).UnixNano()
	w.step = int64(profile.Window) / max(total, 1)
	w.clock = start
	return w
}

func (w *graphWorkload) initialObjectCount() int {
	p := w.profile
	apps := p.Namespaces*p.AppsPerNamespace + p.BigNamespaceApps
	return p.Nodes + p.Namespaces + 1 + apps*(9+p.Replicas)
}

func (w *graphWorkload) uid() string {
	w.seq++
	return fmt.Sprintf("%08x-0000-4000-8000-%012x", w.seq, w.seq)
}

func (w *graphWorkload) newObject(group, version, kind, namespace, name string) *graphObject {
	return &graphObject{
		meta: models.ResourceMetadata{
			Group: group, Version: version, Kind: kind, Namespace: namespace, Name: name, UID: w.uid(),
		},
		alive: true,
	}
}

func (w *graphWorkload) event(eventType models.EventType, obj *graphObject) models.Event {
	w.seq++
	w.clock += w.step
	obj.version++
	var data []byte
	if eventType != models.EventTypeDelete {
		data = w.render(obj, w.clock)
	} else {
		obj.alive = false
	}
	return models.Event{
		ID:        fmt.Sprintf("%08x-0000-4000-8000-%012x", w.seq>>16, w.seq),
		Timestamp: w.clock,
		Type:      eventType,
		Resource:  obj.meta,
		Data:      data,
		DataSize:  int32(len(data)),
	}
}

// Initial emits the CREATE events of the first list.
func (w *graphWorkload) Initial(emit func(models.Event)) {
	p := w.profile
	for i := 0; i < p.Nodes; i++ {
		node := w.newObject("", "v1", "Node", "", fmt.Sprintf("node-%04d", i))
		w.nodes = append(w.nodes, node)
		emit(w.event(models.EventTypeCreate, node))
	}
	gitRepo := w.newObject("source.toolkit.fluxcd.io", "v1", "GitRepository", "flux-system", "platform")
	emit(w.event(models.EventTypeCreate, gitRepo))

	for n := 0; n < p.Namespaces; n++ {
		w.emitNamespace(fmt.Sprintf("team-%03d", n), p.AppsPerNamespace, emit)
	}
	if p.BigNamespaceApps > 0 {
		w.emitNamespace("team-big", p.BigNamespaceApps, emit)
	}
}

func (w *graphWorkload) emitNamespace(ns string, apps int, emit func(models.Event)) {
	w.namespaces = append(w.namespaces, ns)
	emit(w.event(models.EventTypeCreate, w.newObject("", "v1", "Namespace", "", ns)))
	release := w.newObject("helm.toolkit.fluxcd.io", "v2", "HelmRelease", ns, "apps")
	emit(w.event(models.EventTypeCreate, release))

	for a := 0; a < apps; a++ {
		name := fmt.Sprintf("svc-%03d", a)
		app := &graphApp{namespace: ns, name: name}
		cm := w.newObject("", "v1", "ConfigMap", ns, name+"-config")
		app.configMap = cm
		emit(w.event(models.EventTypeCreate, cm))
		emit(w.event(models.EventTypeCreate, w.newObject("", "v1", "Secret", ns, name+"-secret")))
		sa := w.newObject("", "v1", "ServiceAccount", ns, name)
		emit(w.event(models.EventTypeCreate, sa))
		emit(w.event(models.EventTypeCreate, w.newObject("rbac.authorization.k8s.io", "v1", "Role", ns, name)))
		rb := w.newObject("rbac.authorization.k8s.io", "v1", "RoleBinding", ns, name)
		rb.app = name
		emit(w.event(models.EventTypeCreate, rb))

		dep := w.newObject("apps", "v1", "Deployment", ns, name)
		dep.app, dep.release = name, "apps"
		app.deploy = dep
		emit(w.event(models.EventTypeCreate, dep))
		svc := w.newObject("", "v1", "Service", ns, name)
		svc.app = name
		emit(w.event(models.EventTypeCreate, svc))
		if a%4 == 0 {
			ing := w.newObject("networking.k8s.io", "v1", "Ingress", ns, name)
			ing.ingressTo = name
			emit(w.event(models.EventTypeCreate, ing))
		}
		rs := w.newReplicaSet(app)
		emit(w.event(models.EventTypeCreate, rs))
		for r := 0; r < w.profile.Replicas; r++ {
			emit(w.event(models.EventTypeCreate, w.newPod(app)))
		}
		w.apps = append(w.apps, app)
	}
}

func (w *graphWorkload) newReplicaSet(app *graphApp) *graphObject {
	hash := fmt.Sprintf("%08x", w.rng.Uint32())
	rs := w.newObject("apps", "v1", "ReplicaSet", app.namespace, app.name+"-"+hash)
	rs.app, rs.hash, rs.owner, rs.ownerKind, rs.release = app.name, hash, app.deploy.meta.UID, "Deployment", "apps"
	app.rs = rs
	return rs
}

func (w *graphWorkload) newPod(app *graphApp) *graphObject {
	pod := w.newObject("", "v1", "Pod", app.namespace, fmt.Sprintf("%s-%05x", app.rs.meta.Name, w.seq&0xfffff))
	pod.app, pod.hash, pod.owner, pod.ownerKind = app.name, app.rs.hash, app.rs.meta.UID, "ReplicaSet"
	pod.node = w.nodes[w.rng.Intn(len(w.nodes))].meta.Name
	app.pods = append(app.pods, pod)
	return pod
}

// Churn emits steady-state traffic with the timeline workload's mix: pod status
// updates (a few crash-looping), pod replacements, rollouts, node heartbeats and
// ConfigMap edits, plus core/v1 Events about pods.
func (w *graphWorkload) Churn(emit func(models.Event)) {
	for emitted := 0; emitted < w.profile.ChurnEvents; {
		if w.profile.K8sEventPercent > 0 && w.rng.Intn(100) < w.profile.K8sEventPercent {
			emit(w.k8sEvent(w.livePod()))
			emitted++
			continue
		}
		roll := w.rng.Intn(100)
		switch {
		case roll < 55:
			_, pod := w.livePodWithApp()
			pod.crashing = w.rng.Intn(100) < 5
			emit(w.event(models.EventTypeUpdate, pod))
			emitted++
		case roll < 70:
			app, old := w.livePodWithApp()
			emit(w.event(models.EventTypeDelete, old))
			app.dropPod(old)
			fresh := w.newPod(app)
			emit(w.event(models.EventTypeCreate, fresh))
			emit(w.event(models.EventTypeUpdate, fresh))
			emit(w.event(models.EventTypeUpdate, fresh))
			emitted += 4
		case roll < 73:
			emitted += w.rollout(w.apps[w.rng.Intn(len(w.apps))], emit)
		case roll < 93:
			emit(w.event(models.EventTypeUpdate, w.nodes[w.rng.Intn(len(w.nodes))]))
			emitted++
		default:
			emit(w.event(models.EventTypeUpdate, w.apps[w.rng.Intn(len(w.apps))].configMap))
			emitted++
		}
	}
}

// rollout updates the Deployment, creates a new ReplicaSet, scales the old one down
// and replaces every pod.
func (w *graphWorkload) rollout(app *graphApp, emit func(models.Event)) int {
	emit(w.event(models.EventTypeUpdate, app.deploy))
	old := app.rs
	rs := w.newReplicaSet(app)
	emit(w.event(models.EventTypeCreate, rs))
	old.alive = false // scaled to zero: rendered with replicas 0
	emit(w.event(models.EventTypeUpdate, old))
	count := 3
	pods := app.pods
	app.pods = nil
	for _, pod := range pods {
		emit(w.event(models.EventTypeDelete, pod))
		fresh := w.newPod(app)
		emit(w.event(models.EventTypeCreate, fresh))
		emit(w.event(models.EventTypeUpdate, fresh))
		count += 3
	}
	return count
}

func (a *graphApp) dropPod(pod *graphObject) {
	for i, p := range a.pods {
		if p == pod {
			a.pods = append(a.pods[:i], a.pods[i+1:]...)
			return
		}
	}
}

func (w *graphWorkload) livePodWithApp() (*graphApp, *graphObject) {
	for {
		app := w.apps[w.rng.Intn(len(w.apps))]
		if len(app.pods) > 0 {
			return app, app.pods[w.rng.Intn(len(app.pods))]
		}
	}
}

func (w *graphWorkload) livePod() *graphObject {
	_, pod := w.livePodWithApp()
	return pod
}

func (w *graphWorkload) k8sEvent(pod *graphObject) models.Event {
	w.seq++
	meta := models.ResourceMetadata{
		Version: "v1", Kind: "Event", Namespace: pod.meta.Namespace,
		Name: fmt.Sprintf("%s.%x", pod.meta.Name, w.seq), UID: w.uid(),
		InvolvedObjectUID: pod.meta.UID,
	}
	reasons := []string{"Pulled", "Created", "Started", "Unhealthy", "BackOff", "Killing"}
	reason := reasons[w.rng.Intn(len(reasons))]
	eventType := "Normal"
	if reason == "Unhealthy" || reason == "BackOff" {
		eventType = "Warning"
	}
	w.clock += w.step
	stamp := time.Unix(0, w.clock).UTC().Format(time.RFC3339)
	data := mustJSON(map[string]any{
		"apiVersion": "v1", "kind": "Event",
		"metadata":       map[string]any{"name": meta.Name, "namespace": meta.Namespace, "uid": meta.UID, "creationTimestamp": stamp},
		"involvedObject": map[string]any{"kind": "Pod", "namespace": pod.meta.Namespace, "name": pod.meta.Name, "uid": pod.meta.UID},
		"reason":         reason, "message": "Readiness probe failed: HTTP probe failed with statuscode: 503 " + w.padding[:300],
		"source":         map[string]any{"component": "kubelet", "host": pod.node},
		"firstTimestamp": stamp, "lastTimestamp": stamp, "count": 1, "type": eventType,
	})
	return models.Event{
		ID:        fmt.Sprintf("%08x-0000-4000-8000-%012x", w.seq>>16, w.seq),
		Timestamp: w.clock,
		Type:      models.EventTypeCreate,
		Resource:  meta,
		Data:      data,
		DataSize:  int32(len(data)),
	}
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

// render produces objects sized like real ones with managedFields stripped (Pod ~4 KB,
// Node ~5 KB, Deployment/ReplicaSet ~3 KB) carrying every field the store reads edges
// from.
func (w *graphWorkload) render(obj *graphObject, ts int64) []byte {
	m := obj.meta
	stamp := time.Unix(0, ts).UTC().Format(time.RFC3339)
	apiVersion := m.Version
	if m.Group != "" {
		apiVersion = m.Group + "/" + m.Version
	}
	labels := map[string]any{}
	if obj.app != "" {
		labels["app"] = obj.app
	}
	if obj.hash != "" {
		labels["pod-template-hash"] = obj.hash
	}
	if obj.release != "" {
		labels["helm.toolkit.fluxcd.io/name"] = obj.release
		labels["helm.toolkit.fluxcd.io/namespace"] = m.Namespace
	}
	metadata := map[string]any{
		"name": m.Name, "uid": m.UID, "resourceVersion": fmt.Sprint(w.seq), "generation": obj.version,
		"creationTimestamp": stamp, "labels": labels,
		"annotations": map[string]any{"kubectl.kubernetes.io/last-applied-configuration": w.padding[:w.paddingFor(m.Kind)]},
	}
	if m.Namespace != "" {
		metadata["namespace"] = m.Namespace
	}
	if obj.owner != "" {
		metadata["ownerReferences"] = []any{map[string]any{
			"apiVersion": "apps/v1", "kind": obj.ownerKind, "name": "owner", "uid": obj.owner, "controller": true,
		}}
	}
	object := map[string]any{"apiVersion": apiVersion, "kind": m.Kind, "metadata": metadata}
	switch m.Kind {
	case "Pod":
		object["spec"] = map[string]any{
			"nodeName": obj.node, "serviceAccountName": obj.app,
			"volumes": []any{
				map[string]any{"name": "config", "configMap": map[string]any{"name": obj.app + "-config"}},
				map[string]any{"name": "secret", "secret": map[string]any{"secretName": obj.app + "-secret"}},
			},
			"containers": []any{
				map[string]any{
					"name": "app", "image": fmt.Sprintf("registry.example.com/%s:v1", obj.app),
					"envFrom":        []any{map[string]any{"configMapRef": map[string]any{"name": obj.app + "-config"}}},
					"resources":      map[string]any{"requests": map[string]any{"cpu": "100m", "memory": "128Mi"}},
					"readinessProbe": map[string]any{"httpGet": map[string]any{"path": "/ready", "port": 8080}},
				},
				map[string]any{"name": "sidecar", "image": "registry.example.com/proxy:1.2.3"},
			},
		}
		ready := "True"
		state := map[string]any{"running": map[string]any{"startedAt": stamp}}
		if obj.crashing {
			ready = "False"
			state = map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff", "message": "back-off 5m0s restarting failed container"}}
		}
		object["status"] = map[string]any{
			"phase": "Running", "podIP": fmt.Sprintf("10.0.%d.%d", w.seq%255, obj.version%255),
			"conditions": []any{
				map[string]any{"type": "Ready", "status": ready, "lastTransitionTime": stamp},
				map[string]any{"type": "ContainersReady", "status": ready, "lastTransitionTime": stamp},
			},
			"containerStatuses": []any{map[string]any{"name": "app", "ready": ready == "True", "restartCount": obj.version, "state": state}},
		}
	case "Node":
		object["spec"] = map[string]any{"podCIDR": "10.0.0.0/24"}
		object["status"] = map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "lastHeartbeatTime": stamp},
				map[string]any{"type": "MemoryPressure", "status": "False", "lastHeartbeatTime": stamp},
			},
			"allocatable": map[string]any{"cpu": "16", "memory": "64Gi", "pods": "110"},
		}
	case "Deployment", "ReplicaSet":
		replicas := w.profile.Replicas
		if m.Kind == "ReplicaSet" && !obj.alive {
			replicas = 0
		}
		selector := map[string]any{"app": obj.app}
		templateLabels := map[string]any{"app": obj.app}
		if obj.hash != "" {
			selector["pod-template-hash"] = obj.hash
			templateLabels["pod-template-hash"] = obj.hash
		}
		object["spec"] = map[string]any{
			"replicas": replicas,
			"selector": map[string]any{"matchLabels": selector},
			"template": map[string]any{
				"metadata": map[string]any{"labels": templateLabels},
				"spec":     map[string]any{"containers": []any{map[string]any{"name": "app", "image": fmt.Sprintf("registry.example.com/%s:v%d", obj.app, obj.version)}}},
			},
		}
		object["status"] = map[string]any{"replicas": replicas, "readyReplicas": replicas, "availableReplicas": replicas, "observedGeneration": obj.version}
	case "Service":
		object["spec"] = map[string]any{"type": "ClusterIP", "selector": map[string]any{"app": obj.app}, "ports": []any{map[string]any{"port": 80, "targetPort": 8080}}}
	case "Ingress":
		object["spec"] = map[string]any{"rules": []any{map[string]any{
			"host": obj.ingressTo + ".example.com",
			"http": map[string]any{"paths": []any{map[string]any{
				"path": "/", "pathType": "Prefix",
				"backend": map[string]any{"service": map[string]any{"name": obj.ingressTo, "port": map[string]any{"number": 80}}},
			}}},
		}}}
	case "RoleBinding":
		object["roleRef"] = map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": obj.app}
		object["subjects"] = []any{map[string]any{"kind": "ServiceAccount", "name": obj.app, "namespace": m.Namespace}}
	case "Role":
		object["rules"] = []any{map[string]any{"apiGroups": []any{""}, "resources": []any{"configmaps"}, "verbs": []any{"get", "list"}}}
	case "HelmRelease":
		object["spec"] = map[string]any{"chart": map[string]any{"spec": map[string]any{
			"chart": "apps", "sourceRef": map[string]any{"kind": "GitRepository", "name": "platform", "namespace": "flux-system"},
		}}}
		object["status"] = map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}}
	case "ConfigMap":
		object["data"] = map[string]any{"config.yaml": fmt.Sprintf("revision: %d", obj.version)}
	case "Secret":
		object["type"] = "Opaque"
	case "GitRepository":
		object["spec"] = map[string]any{"url": "https://git.example.com/platform", "interval": "1m"}
	}
	return mustJSON(object)
}

func (w *graphWorkload) paddingFor(kind string) int {
	switch kind {
	case "Pod":
		return 2600
	case "Node":
		return 4000
	case "Deployment", "ReplicaSet":
		return 2000
	case "ConfigMap":
		return 1600
	default:
		return 400
	}
}
