// Shared mock data + icons for the Spectre redesign mockups.
// Static, no build step: open approach-a.html / approach-b.html with #timeline, #graph or #settings.

const GHOST_PATH =
  'M12 2C7.58172 2 4 5.58172 4 10V19C4 20.6569 5.34315 22 7 22C7.63228 22 8.21952 21.7909 8.70773 21.4312L10.5858 19.5531C10.9609 19.1781 11.4696 18.9674 12 18.9674C12.5304 18.9674 13.0391 19.1781 13.4142 19.5531L15.2923 21.4312C15.7805 21.7909 16.3677 22 17 22C18.6569 22 20 20.6569 20 19V10C20 5.58172 16.4183 2 12 2Z';

function ghostSvg(size = 18, stroke = '#fff') {
  return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="${stroke}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="${GHOST_PATH}"/><path d="M9 10h.01"/><path d="M15 10h.01"/></svg>`;
}

const ICON_PATHS = {
  timeline: '<path d="M2 12h3l2-4 3 8 4-12 5 6 3-2h3"/>',
  graph: '<circle cx="5" cy="6" r="3"/><path d="M5 9v6"/><circle cx="5" cy="18" r="3"/><path d="M12 3v18"/><circle cx="19" cy="6" r="3"/><path d="M16 15.7A9 9 0 0 0 19 9"/>',
  settings: '<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  chevron: '<path d="m6 9 6 6 6-6"/>',
  chevronRight: '<path d="m9 6 6 6-6 6"/>',
  x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  plus: '<path d="M12 5v14"/><path d="M5 12h14"/>',
  minus: '<path d="M5 12h14"/>',
  maximize: '<path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/>',
  filter: '<path d="M3 5h18"/><path d="M6 12h12"/><path d="M10 19h4"/>',
  download: '<path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/>',
  upload: '<path d="M12 21V9"/><path d="m7 14 5-5 5 5"/><path d="M5 3h14"/>',
  panel: '<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M9 3v18"/>',
  more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  copy: '<rect x="8" y="8" width="13" height="13" rx="2"/><path d="M4 16V5a1 1 0 0 1 1-1h11"/>',
  external: '<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M21 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5"/>',
  refresh: '<path d="M21 12a9 9 0 1 1-3-6.7L21 8"/><path d="M21 3v5h-5"/>',
  calendar: '<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4"/><path d="M8 2v4"/><path d="M3 10h18"/>',
  layers: '<path d="m12 2 10 5-10 5L2 7z"/><path d="m2 17 10 5 10-5"/><path d="m2 12 10 5 10-5"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.9 4.9 1.4 1.4"/><path d="m17.7 17.7 1.4 1.4"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.3 17.7-1.4 1.4"/><path d="m19.1 4.9-1.4 1.4"/>',
  moon: '<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z"/>',
  monitor: '<rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8"/><path d="M12 17v4"/>',
  database: '<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.7 4 3 9 3s9-1.3 9-3V5"/><path d="M3 12c0 1.7 4 3 9 3s9-1.3 9-3"/>',
  sliders: '<path d="M4 21v-7"/><path d="M4 10V3"/><path d="M12 21v-9"/><path d="M12 8V3"/><path d="M20 21v-5"/><path d="M20 12V3"/><path d="M2 14h4"/><path d="M10 8h4"/><path d="M18 16h4"/>',
  check: '<path d="M20 6 9 17l-5-5"/>',
  locate: '<circle cx="12" cy="12" r="3"/><path d="M12 2v3"/><path d="M12 19v3"/><path d="M2 12h3"/><path d="M19 12h3"/>',
  pause: '<rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/>',
  arrowRight: '<path d="M5 12h14"/><path d="m13 6 6 6-6 6"/>',
  sparkle: '<path d="M12 3v4"/><path d="M12 17v4"/><path d="M3 12h4"/><path d="M17 12h4"/><path d="m6 6 2.5 2.5"/><path d="m15.5 15.5 2.5 2.5"/><path d="m6 18 2.5-2.5"/><path d="m15.5 8.5 2.5-2.5"/>',
};

function icon(name, size = 16, sw = 1.75) {
  return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${sw}" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICON_PATHS[name]}</svg>`;
}

// ---- Timeline mock (minutes relative to 06:10, window is 40 minutes) ----
const TL_START = { h: 6, m: 10 };
const TL_SPAN = 40;
const TL_TICKS = [0, 5, 10, 15, 20, 25, 30, 35, 40];
function tlLabel(min) {
  const total = TL_START.h * 60 + TL_START.m + min;
  const h = String(Math.floor(total / 60)).padStart(2, '0');
  const m = String(total % 60).padStart(2, '0');
  return `${h}:${m}`;
}

const TIMELINE_ROWS = [
  { kind: 'HelmRelease', ns: 'external-secrets', name: 'external-secrets', segs: [[0, 40, 'warn']], events: [13.6, 14.4, 26.4, 30.8, 32.6] },
  { kind: 'HelmRelease', ns: 'n8n', name: 'n8n', segs: [[0, 40, 'err']], events: [] },
  { kind: 'Kustomization', ns: 'cert-manager', name: 'cert-manager-crds', segs: [[0, 12.4, 'err'], [12.4, 40, 'err']], events: [1.5, 11.5, 21.5, 27.5, 35.5] },
  { kind: 'Deployment', ns: 'external-secrets', name: 'external-secrets', segs: [[0, 9, 'ok'], [9, 14.4, 'warn'], [14.4, 26.4, 'ok'], [26.4, 33.2, 'warn'], [33.2, 40, 'ok']], events: [9, 14.4, 26.4, 28.6, 33.2], selected: true },
  { kind: 'Deployment', ns: 'monitoring', name: 'spectre', segs: [[0, 4, 'warn'], [4, 40, 'ok']], events: [4] },
  { kind: 'ReplicaSet', ns: 'external-secrets', name: 'external-secrets-68cfff45c5', segs: [[0, 9, 'warn'], [9, 14.4, 'warn'], [14.4, 26.4, 'ok'], [26.4, 33.2, 'warn'], [33.2, 40, 'ok']], events: [7.2, 9, 13.6, 14.4, 26.4, 28.6, 30.6, 32.2] },
  { kind: 'ReplicaSet', ns: 'monitoring', name: 'spectre-694b684cd8', segs: [[0, 34, 'warn'], [34, 40, 'ok']], events: [34] },
  { kind: 'Pod', ns: 'external-secrets', name: 'external-secrets-68cfff45c5-7s2t4', segs: [], burst: { from: 25.6, to: 33.4, count: 24 } },
  { kind: 'Pod', ns: 'external-secrets', name: 'external-secrets-68cfff45c5-88dm4', segs: [], burst: { from: 9, to: 14.6, count: 19 }, events: [31] },
  { kind: 'Pod', ns: 'cert-manager', name: 'cert-manager-c567f7bdd-rpx4w', segs: [[0, 23, 'warn'], [23, 40, 'ok']], events: [23] },
  { kind: 'Pod', ns: 'cert-manager', name: 'cert-manager-cainjector-7f5bd9c869-x2kq9', segs: [[0, 23, 'warn'], [23, 40, 'ok']], events: [23] },
  { kind: 'Pod', ns: 'monitoring', name: 'alertmanager-kube-prometheus-stack-0', segs: [[0, 23, 'warn'], [23, 40, 'ok']], events: [23] },
  { kind: 'Pod', ns: 'monitoring', name: 'kube-prometheus-stack-grafana-c8f6d9b4-tq7lm', segs: [[0, 23, 'warn'], [23, 40, 'ok']], events: [23] },
  { kind: 'Pod', ns: 'monitoring', name: 'prometheus-kube-prometheus-stack-prometheus-0', segs: [[0, 23, 'warn'], [23, 40, 'ok']], events: [23] },
  { kind: 'Pod', ns: 'monitoring', name: 'spectre-694b684cd8-djpzr', segs: [[0, 23, 'warn'], [23, 40, 'ok']], events: [23] },
  { kind: 'Pod', ns: 'n8n', name: 'n8n-6764f65cff-jbvwd', segs: [[0, 40, 'err']], events: [] },
];

const DETAIL = {
  kind: 'Deployment',
  ns: 'external-secrets',
  name: 'external-secrets',
  status: 'Ready',
  meta: [
    ['Replicas', '1 / 1 ready'],
    ['Revision', '14'],
    ['Image', 'external-secrets:v0.10.5'],
    ['Owner', 'HelmRelease/external-secrets'],
  ],
  events: [
    { t: '06:43:12', type: 'Normal', reason: 'ScalingReplicaSet', msg: 'Scaled up replica set external-secrets-68cfff45c5 to 1' },
    { t: '06:38:36', type: 'Warning', reason: 'ProgressDeadlineExceeded', msg: 'ReplicaSet "external-secrets-68cfff45c5" has timed out progressing.' },
    { t: '06:36:24', type: 'Normal', reason: 'ScalingReplicaSet', msg: 'Scaled down replica set external-secrets-7d9b8c6f4 to 0 from 1' },
    { t: '06:24:24', type: 'Warning', reason: 'FailedCreate', msg: 'Error creating: pods "external-secrets-68cfff45c5-" is forbidden: exceeded quota' },
  ],
  diff: [
    [' ', 'spec:'],
    [' ', '  template:'],
    [' ', '    spec:'],
    [' ', '      containers:'],
    [' ', '        - name: external-secrets'],
    ['-', '          image: external-secrets:v0.10.4'],
    ['+', '          image: external-secrets:v0.10.5'],
    [' ', '          resources:'],
    ['-', '            limits: { memory: 128Mi }'],
    ['+', '            limits: { memory: 256Mi }'],
  ],
};

// ---- Graph mock (coordinates in a 1000x640 space) ----
const GRAPH_NODES = [
  { id: 'hr', kind: 'HelmRelease', name: 'external-secrets', x: 110, y: 300, status: 'err', path: true, badge: '9+' },
  { id: 'dep2', kind: 'Deployment', name: 'external-secrets-webhook', x: 330, y: 130, status: 'ok' },
  { id: 'dep', kind: 'Deployment', name: 'external-secrets', x: 330, y: 300, status: 'warn', path: true, badge: '9+' },
  { id: 'dep3', kind: 'Deployment', name: 'external-secrets-cert-ctl', x: 330, y: 460, status: 'ok' },
  { id: 'es', kind: 'ExternalSecret', name: 'db-credentials', x: 330, y: 600, status: 'ok' },
  { id: 'rs4', kind: 'ReplicaSet', name: 'external-secrets-webhook-6b', x: 550, y: 80, status: 'ok' },
  { id: 'rs2', kind: 'ReplicaSet', name: 'external-secrets-7d9b8c6f4', x: 550, y: 190, status: 'ok' },
  { id: 'rs', kind: 'ReplicaSet', name: 'external-secrets-68cfff45c5', x: 550, y: 300, status: 'warn', path: true, badge: '2' },
  { id: 'rs3', kind: 'ReplicaSet', name: 'external-secrets-5f6c7d8b9', x: 550, y: 400, status: 'ok' },
  { id: 'rs5', kind: 'ReplicaSet', name: 'external-secrets-cert-ctl-8c', x: 550, y: 500, status: 'ok' },
  { id: 'sa', kind: 'ServiceAccount', name: 'external-secrets', x: 550, y: 600, status: 'ok' },
  { id: 'pod2', kind: 'Pod', name: 'external-secrets-webhook-k2', x: 770, y: 80, status: 'ok' },
  { id: 'cm', kind: 'ConfigMap', name: 'external-secrets-config', x: 770, y: 190, status: 'ok' },
  { id: 'pod', kind: 'Pod', name: 'external-secrets-7s2t4', x: 770, y: 300, status: 'err', path: true, badge: '24' },
  { id: 'pod4', kind: 'Pod', name: 'external-secrets-88dm4', x: 770, y: 400, status: 'ok' },
  { id: 'pod3', kind: 'Pod', name: 'external-secrets-cert-ctl-9x', x: 770, y: 500, status: 'ok' },
  { id: 'svc', kind: 'Service', name: 'external-secrets-webhook', x: 770, y: 600, status: 'ok' },
  { id: 'n2', kind: 'Node', name: 't440', x: 970, y: 140, status: 'ok' },
  { id: 'n1', kind: 'Node', name: 'z440', x: 970, y: 400, status: 'ok' },
];
const GRAPH_EDGES = [
  ['hr', 'dep', true], ['dep', 'rs', true], ['rs', 'pod', true],
  ['dep', 'rs2'], ['dep', 'rs3'], ['hr', 'dep2'], ['dep2', 'rs4'], ['rs4', 'pod2'], ['hr', 'dep3'], ['dep3', 'rs5'], ['rs5', 'pod3'],
  ['rs', 'pod4'], ['pod', 'n1'], ['pod4', 'n1'], ['pod2', 'n2'], ['pod3', 'n2'], ['dep2', 'svc'], ['hr', 'es'], ['rs2', 'cm'], ['dep3', 'sa'],
];

const CAUSAL_PATHS = [
  {
    rank: 1, confidence: 100, severity: 'critical', root: 'HelmRelease/external-secrets', steps: 3,
    tags: ['ReconcileFailed', 'ErrorStatus', '+15'],
    summary: "HelmRelease external-secrets failed to reconcile at 06:24:28. The upgrade propagated through Deployment (owns) and ReplicaSet external-secrets-68cfff45c5, whose pod entered CrashLoopBackOff.",
    chain: ['HelmRelease', 'Deployment', 'ReplicaSet', 'Pod'],
  },
  {
    rank: 2, confidence: 94, severity: 'critical', root: 'HelmRelease/external-secrets', steps: 4,
    tags: ['ErrorStatus', 'ImagePull', '+9'],
    summary: 'Image tag change on the webhook container left ReplicaSet external-secrets-webhook-6b without ready replicas.',
    chain: ['HelmRelease', 'Deployment', 'ReplicaSet', 'Pod'],
  },
  {
    rank: 3, confidence: 61, severity: 'warning', root: 'Kustomization/cert-manager-crds', steps: 2,
    tags: ['DependencyNotReady', '+3'],
    summary: 'CRD apply failures in cert-manager-crds block certificate issuance for the webhook service.',
    chain: ['Kustomization', 'Certificate', 'Service'],
  },
];

const RESOURCE_KINDS = ['Pod', 'Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob', 'Service', 'Ingress', 'ConfigMap', 'Secret', 'Node', 'HelmRelease', 'Kustomization', 'PersistentVolumeClaim'];
const DEFAULT_KINDS = ['Pod', 'Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet', 'HelmRelease', 'Kustomization'];

function currentView() {
  return (location.hash || '#timeline').slice(1);
}
