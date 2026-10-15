// Mock API responses used to render the UI for README screenshots.
// Times are generated relative to "now" so the default "last 30 minutes" window is populated.

type Status = 'Ready' | 'Warning' | 'Error';

interface RowSpec {
  kind: string;
  apiVersion: string;
  ns: string;
  name: string;
  // [startMinute, endMinute, status] within a 40-minute virtual window
  segs: Array<[number, number, Status]>;
  events?: Array<[number, string, string, string]>; // [minute, type, reason, message]
  burst?: { from: number; to: number; count: number; reason: string; message: string };
  data?: (index: number) => Record<string, unknown>;
}

const WINDOW_MINUTES = 40;
const TARGET_WINDOW_MINUTES = 30;

const deploymentData = (index: number) => ({
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: { name: 'external-secrets', namespace: 'external-secrets', generation: 12 + index },
  spec: {
    replicas: 1,
    template: {
      spec: {
        containers: [
          {
            name: 'external-secrets',
            image: `ghcr.io/external-secrets/external-secrets:${index >= 1 ? 'v0.10.5' : 'v0.10.4'}`,
            resources: { limits: { memory: index >= 3 ? '256Mi' : '128Mi' } },
          },
        ],
      },
    },
  },
  status: { readyReplicas: index % 2 === 0 ? 1 : 0, replicas: 1 },
});

const ROWS: RowSpec[] = [
  {
    kind: 'HelmRelease', apiVersion: 'helm.toolkit.fluxcd.io/v2', ns: 'external-secrets', name: 'external-secrets',
    segs: [[0, 40, 'Warning']],
    events: [
      [13.6, 'Warning', 'UpgradeFailed', 'Helm upgrade failed: timed out waiting for the condition'],
      [14.4, 'Normal', 'InstallSucceeded', 'Helm install succeeded for release external-secrets/external-secrets.v14'],
      [26.4, 'Warning', 'UpgradeFailed', 'Helm upgrade failed: context deadline exceeded'],
      [30.8, 'Normal', 'Progressing', 'Running upgrade action'],
      [32.6, 'Warning', 'ReconcileFailed', 'Reconciliation failed after 5m0s, next try in 10m0s'],
    ],
  },
  { kind: 'HelmRelease', apiVersion: 'helm.toolkit.fluxcd.io/v2', ns: 'n8n', name: 'n8n', segs: [[0, 40, 'Error']] },
  {
    kind: 'Kustomization', apiVersion: 'kustomize.toolkit.fluxcd.io/v1', ns: 'cert-manager', name: 'cert-manager-crds',
    segs: [[0, 12.4, 'Error'], [12.4, 40, 'Error']],
    events: [1.5, 11.5, 21.5, 27.5, 35.5].map(m => [m, 'Warning', 'ReconciliationFailed', 'CustomResourceDefinition/certificates.cert-manager.io dry-run failed'] as [number, string, string, string]),
  },
  {
    kind: 'Deployment', apiVersion: 'apps/v1', ns: 'external-secrets', name: 'external-secrets',
    segs: [[0, 9, 'Ready'], [9, 14.4, 'Warning'], [14.4, 26.4, 'Ready'], [26.4, 33.2, 'Warning'], [33.2, 40, 'Ready']],
    events: [
      [9, 'Normal', 'ScalingReplicaSet', 'Scaled up replica set external-secrets-68cfff45c5 to 1'],
      [14.4, 'Normal', 'ScalingReplicaSet', 'Scaled down replica set external-secrets-7d9b8c6f4 to 0 from 1'],
      [26.4, 'Normal', 'ScalingReplicaSet', 'Scaled up replica set external-secrets-68cfff45c5 to 1'],
      [28.6, 'Warning', 'ProgressDeadlineExceeded', 'ReplicaSet "external-secrets-68cfff45c5" has timed out progressing.'],
      [33.2, 'Normal', 'ScalingReplicaSet', 'Scaled down replica set external-secrets-7d9b8c6f4 to 0 from 1'],
    ],
    data: deploymentData,
  },
  {
    kind: 'Deployment', apiVersion: 'apps/v1', ns: 'monitoring', name: 'spectre',
    segs: [[0, 4, 'Warning'], [4, 40, 'Ready']],
    events: [[4, 'Normal', 'ScalingReplicaSet', 'Scaled up replica set spectre-694b684cd8 to 1']],
  },
  {
    kind: 'ReplicaSet', apiVersion: 'apps/v1', ns: 'external-secrets', name: 'external-secrets-68cfff45c5',
    segs: [[0, 9, 'Warning'], [9, 14.4, 'Warning'], [14.4, 26.4, 'Ready'], [26.4, 33.2, 'Warning'], [33.2, 40, 'Ready']],
    events: [7.2, 9, 13.6, 14.4, 26.4, 28.6, 30.6, 32.2].map(m => [m, 'Normal', 'SuccessfulCreate', 'Created pod: external-secrets-68cfff45c5-7s2t4'] as [number, string, string, string]),
  },
  {
    kind: 'ReplicaSet', apiVersion: 'apps/v1', ns: 'monitoring', name: 'spectre-694b684cd8',
    segs: [[0, 34, 'Warning'], [34, 40, 'Ready']],
    events: [[34, 'Normal', 'SuccessfulCreate', 'Created pod: spectre-694b684cd8-djpzr']],
  },
  {
    kind: 'Pod', apiVersion: 'v1', ns: 'external-secrets', name: 'external-secrets-68cfff45c5-7s2t4',
    segs: [[25.6, 33.4, 'Error']],
    burst: { from: 25.6, to: 33.4, count: 24, reason: 'BackOff', message: 'Back-off restarting failed container external-secrets' },
  },
  {
    kind: 'Pod', apiVersion: 'v1', ns: 'external-secrets', name: 'external-secrets-68cfff45c5-88dm4',
    segs: [[9, 14.6, 'Error']],
    burst: { from: 9, to: 14.6, count: 19, reason: 'BackOff', message: 'Back-off restarting failed container external-secrets' },
  },
  ...['cert-manager-c567f7bdd-rpx4w', 'cert-manager-cainjector-7f5bd9c869-x2kq9'].map(name => ({
    kind: 'Pod', apiVersion: 'v1', ns: 'cert-manager', name,
    segs: [[0, 23, 'Warning'], [23, 40, 'Ready']] as Array<[number, number, Status]>,
    events: [[23, 'Normal', 'Started', 'Started container'] as [number, string, string, string]],
  })),
  ...['alertmanager-kube-prometheus-stack-0', 'kube-prometheus-stack-grafana-c8f6d9b4-tq7lm', 'prometheus-kube-prometheus-stack-prometheus-0', 'spectre-694b684cd8-djpzr'].map(name => ({
    kind: 'Pod', apiVersion: 'v1', ns: 'monitoring', name,
    segs: [[0, 23, 'Warning'], [23, 40, 'Ready']] as Array<[number, number, Status]>,
    events: [[23, 'Normal', 'Started', 'Started container'] as [number, string, string, string]],
  })),
  { kind: 'Pod', apiVersion: 'v1', ns: 'n8n', name: 'n8n-6764f65cff-jbvwd', segs: [[0, 40, 'Error']] },
];

const encoder = new TextEncoder();

export function buildTimelineResources(nowSeconds: number) {
  const start = nowSeconds - TARGET_WINDOW_MINUTES * 60;
  const t = (minute: number) => Math.round(start + (minute / WINDOW_MINUTES) * TARGET_WINDOW_MINUTES * 60);

  return ROWS.map((row, rowIndex) => {
    const id = `${row.kind}/${row.ns}/${row.name}`;
    const events = (row.events ?? []).map(([minute, type, reason, message], i) => ({
      uid: `${id}/evt-${i}`,
      type,
      reason,
      message,
      timestamp: t(minute),
      involvedObjectUid: id,
    }));
    if (row.burst) {
      const { from, to, count, reason, message } = row.burst;
      for (let i = 0; i < count; i++) {
        events.push({
          uid: `${id}/burst-${i}`,
          type: 'Warning',
          reason,
          message,
          timestamp: t(from + ((to - from) * i) / (count - 1)),
          involvedObjectUid: id,
        });
      }
    }
    return {
      id,
      kind: row.kind,
      apiVersion: row.apiVersion,
      namespace: row.ns,
      name: row.name,
      createdAt: start - 3600,
      deletedAt: 0,
      labels: {},
      preExisting: rowIndex % 3 === 0,
      statusSegments: row.segs.map(([a, b, status], i) => ({
        id: `${id}/seg-${i}`,
        resourceId: id,
        status,
        reason: '',
        message: status === 'Ready' ? 'All replicas available' : status === 'Warning' ? 'Progressing: waiting for rollout to finish' : 'CrashLoopBackOff: back-off restarting failed container',
        startTime: t(a),
        endTime: t(b),
        inferred: false,
        resourceData: row.data ? encoder.encode(JSON.stringify(row.data(i))) : new Uint8Array(),
      })),
      events,
    };
  });
}

export const NAMESPACES = ['cert-manager', 'external-secrets', 'flux-system', 'kube-system', 'monitoring', 'n8n'];
export const KINDS = ['ConfigMap', 'DaemonSet', 'Deployment', 'ExternalSecret', 'HelmRelease', 'Kustomization', 'Node', 'Pod', 'ReplicaSet', 'Secret', 'Service', 'ServiceAccount', 'StatefulSet'];

// ---------------------------------------------------------------------------
// Namespace graph
// ---------------------------------------------------------------------------

interface NodeSpec {
  uid: string;
  kind: string;
  name: string;
  status: Status;
  ns?: string;
}

const GRAPH_NODES: NodeSpec[] = [
  { uid: 'hr', kind: 'HelmRelease', name: 'external-secrets', status: 'Error' },
  { uid: 'dep', kind: 'Deployment', name: 'external-secrets', status: 'Warning' },
  { uid: 'rs', kind: 'ReplicaSet', name: 'external-secrets-68cfff45c5', status: 'Warning' },
  { uid: 'pod', kind: 'Pod', name: 'external-secrets-68cfff45c5-7s2t4', status: 'Error' },
  { uid: 'rs2', kind: 'ReplicaSet', name: 'external-secrets-7d9b8c6f4', status: 'Ready' },
  { uid: 'dep2', kind: 'Deployment', name: 'external-secrets-webhook', status: 'Ready' },
  { uid: 'rs4', kind: 'ReplicaSet', name: 'external-secrets-webhook-6b9f8', status: 'Ready' },
  { uid: 'pod2', kind: 'Pod', name: 'external-secrets-webhook-6b9f8-k2x7p', status: 'Ready' },
  { uid: 'dep3', kind: 'Deployment', name: 'external-secrets-cert-controller', status: 'Ready' },
  { uid: 'rs5', kind: 'ReplicaSet', name: 'external-secrets-cert-controller-8c4d', status: 'Ready' },
  { uid: 'pod3', kind: 'Pod', name: 'external-secrets-cert-controller-8c4d-9xq', status: 'Ready' },
  { uid: 'pod4', kind: 'Pod', name: 'external-secrets-68cfff45c5-88dm4', status: 'Ready' },
  { uid: 'sa', kind: 'ServiceAccount', name: 'external-secrets', status: 'Ready' },
  { uid: 'svc', kind: 'Service', name: 'external-secrets-webhook', status: 'Ready' },
  { uid: 'es', kind: 'ExternalSecret', name: 'db-credentials', status: 'Ready' },
  { uid: 'cm', kind: 'ConfigMap', name: 'external-secrets-config', status: 'Ready' },
  { uid: 'n1', kind: 'Node', name: 'z440', status: 'Ready', ns: '' },
  { uid: 'n2', kind: 'Node', name: 't440', status: 'Ready', ns: '' },
];

const GRAPH_EDGES: Array<[string, string, string]> = [
  ['hr', 'dep', 'OWNS'], ['dep', 'rs', 'OWNS'], ['rs', 'pod', 'OWNS'],
  ['dep', 'rs2', 'OWNS'], ['hr', 'dep2', 'OWNS'], ['dep2', 'rs4', 'OWNS'], ['rs4', 'pod2', 'OWNS'],
  ['hr', 'dep3', 'OWNS'], ['dep3', 'rs5', 'OWNS'], ['rs5', 'pod3', 'OWNS'], ['rs', 'pod4', 'OWNS'],
  ['pod', 'n1', 'SCHEDULED_ON'], ['pod4', 'n1', 'SCHEDULED_ON'], ['pod2', 'n2', 'SCHEDULED_ON'], ['pod3', 'n2', 'SCHEDULED_ON'],
  ['dep2', 'svc', 'SELECTS'], ['hr', 'es', 'MANAGES'], ['pod', 'cm', 'REFERENCES'], ['dep', 'sa', 'USES'],
];

const resourceOf = (uid: string) => {
  const n = GRAPH_NODES.find(node => node.uid === uid)!;
  return { uid: n.uid, kind: n.kind, namespace: n.ns ?? 'external-secrets', name: n.name };
};

const anomaly = (uid: string, type: string, severity: string, summary: string, iso: string) => ({
  node: resourceOf(uid),
  category: 'state',
  type,
  severity,
  timestamp: iso,
  summary,
  details: {},
});

export function buildNamespaceGraph(nowSeconds: number) {
  const iso = new Date((nowSeconds - 16 * 60) * 1000).toISOString();
  const anomalies = [
    ...Array.from({ length: 12 }, () => anomaly('hr', 'ReconcileFailed', 'critical', 'HelmRelease failed to reconcile', iso)),
    ...Array.from({ length: 11 }, () => anomaly('dep', 'ProgressDeadlineExceeded', 'high', 'Deployment rollout stalled', iso)),
    anomaly('rs', 'ReplicasUnavailable', 'high', 'ReplicaSet has no ready replicas', iso),
    anomaly('rs', 'ErrorStatus', 'high', 'ReplicaSet in error state', iso),
    ...Array.from({ length: 24 }, () => anomaly('pod', 'CrashLoopBackOff', 'critical', 'Container restarted repeatedly', iso)),
  ];

  const pathNode = (uid: string, types: string[]) => ({
    id: `pn-${uid}`,
    resource: resourceOf(uid),
    anomalies: types.map(t => anomaly(uid, t, 'critical', t, iso)),
  });

  const causalPaths = [
    {
      id: 'path-1',
      candidateRoot: pathNode('hr', ['ReconcileFailed', 'ErrorStatus', ...Array(15).fill('ErrorStatus')]),
      firstAnomalyAt: iso,
      steps: [
        { node: pathNode('hr', ['ReconcileFailed']) },
        { node: pathNode('dep', ['ProgressDeadlineExceeded']), edge: { id: 'e1', relationshipType: 'OWNS', edgeCategory: 'CAUSE_INTRODUCING', causalWeight: 1 } },
        { node: pathNode('rs', ['ErrorStatus']), edge: { id: 'e2', relationshipType: 'OWNS', edgeCategory: 'MATERIALIZATION', causalWeight: 0.8 } },
        { node: pathNode('pod', ['CrashLoopBackOff']), edge: { id: 'e3', relationshipType: 'OWNS', edgeCategory: 'MATERIALIZATION', causalWeight: 0.8 } },
      ],
      confidenceScore: 1,
      explanation: "HelmRelease 'external-secrets' failed to reconcile after an image upgrade. The change propagated through Deployment (owns) and ReplicaSet 'external-secrets-68cfff45c5', whose pod entered CrashLoopBackOff.",
      ranking: { temporalScore: 0.96, effectiveCausalDistance: 1, maxAnomalySeverity: 'critical', severityScore: 1 },
    },
    {
      id: 'path-2',
      candidateRoot: pathNode('hr', ['ErrorStatus', 'ImagePull', ...Array(9).fill('ErrorStatus')]),
      firstAnomalyAt: iso,
      steps: [
        { node: pathNode('hr', ['ErrorStatus']) },
        { node: pathNode('dep', []), edge: { id: 'e4', relationshipType: 'OWNS', edgeCategory: 'CAUSE_INTRODUCING', causalWeight: 1 } },
        { node: pathNode('rs', []), edge: { id: 'e5', relationshipType: 'OWNS', edgeCategory: 'MATERIALIZATION', causalWeight: 0.7 } },
      ],
      confidenceScore: 0.94,
      explanation: 'The webhook container image changed in the same release, leaving its ReplicaSet without ready replicas.',
      ranking: { temporalScore: 0.9, effectiveCausalDistance: 1, maxAnomalySeverity: 'critical', severityScore: 0.9 },
    },
    {
      id: 'path-3',
      candidateRoot: pathNode('es', ['SecretSyncError']),
      firstAnomalyAt: iso,
      steps: [
        { node: pathNode('es', ['SecretSyncError']) },
        { node: pathNode('pod', []), edge: { id: 'e6', relationshipType: 'REFERENCES', edgeCategory: 'CAUSE_INTRODUCING', causalWeight: 0.5 } },
      ],
      confidenceScore: 0.61,
      explanation: 'ExternalSecret db-credentials failed to sync, which the crashing pod mounts.',
      ranking: { temporalScore: 0.6, effectiveCausalDistance: 1, maxAnomalySeverity: 'high', severityScore: 0.6 },
    },
  ];

  return {
    graph: {
      nodes: GRAPH_NODES.map(n => ({
        uid: n.uid,
        kind: n.kind,
        namespace: n.ns ?? 'external-secrets',
        name: n.name,
        status: n.status,
        latestEvent: {
          timestamp: (nowSeconds - 16 * 60) * 1_000_000_000,
          eventType: 'MODIFIED',
          status: n.status,
          description: n.status === 'Ready' ? 'Resource is healthy' : 'Status changed',
          errorMessage: n.status === 'Error' ? 'Back-off restarting failed container external-secrets' : undefined,
          containerIssues: n.uid === 'pod' ? ['CrashLoopBackOff'] : undefined,
          specReplicas: n.kind === 'ReplicaSet' ? 1 : undefined,
        },
        labels: { 'app.kubernetes.io/name': 'external-secrets' },
      })),
      edges: GRAPH_EDGES.map(([source, target, relationshipType], i) => ({ id: `edge-${i}`, source, target, relationshipType })),
    },
    anomalies,
    causalPaths,
    metadata: {
      namespace: 'external-secrets',
      timestamp: nowSeconds * 1_000_000_000,
      nodeCount: GRAPH_NODES.length,
      edgeCount: GRAPH_EDGES.length,
      queryExecutionMs: 612,
      hasMore: false,
    },
  };
}
