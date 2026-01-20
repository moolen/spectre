/**
 * Root Cause Analysis Types
 * Matches backend schema from internal/analysis/types.go
 */

export interface SymptomResource {
  uid: string;
  kind: string;
  namespace: string;
  name: string;
}

/**
 * Event significance scoring for LLM prioritization
 */
export interface EventSignificance {
  score: number; // 0.0 to 1.0
  reasons: string[]; // Human-readable explanation of significance
}

/**
 * Represents a single change in the diff-based format
 */
export interface EventDiff {
  path: string; // JSON path, e.g., "spec.replicas"
  old?: unknown; // Previous value (undefined for additions)
  new?: unknown; // New value (undefined for removals)
  op: 'add' | 'remove' | 'replace';
}

export interface ChangeEventInfo {
  eventId: string;
  timestamp: string; // ISO timestamp
  eventType: string; // CREATE, UPDATE, DELETE
  status?: string; // Inferred resource status: Ready, Warning, Error, Terminating, Unknown
  configChanged?: boolean;
  statusChanged?: boolean;
  description?: string;

  // Significance scoring for LLM prioritization
  significance?: EventSignificance;

  // Diff-based format (new) - mutually exclusive with data
  diff?: EventDiff[]; // Changes from previous event
  fullSnapshot?: Record<string, unknown>; // Only for first event per resource

  // Legacy format - full resource JSON (deprecated)
  data?: string;
}

export interface K8sEventInfo {
  eventId: string;
  timestamp: string; // ISO timestamp
  reason: string; // e.g., "FailedScheduling", "BackOff", "Pulling"
  message: string; // Human-readable message
  type: string; // "Warning", "Normal", "Error"
  count: number; // How many times this event occurred
  source: string; // Component that generated the event

  // Significance scoring for LLM prioritization
  significance?: EventSignificance;
}

export interface GraphNode {
  id: string;
  resource: SymptomResource;
  changeEvent?: ChangeEventInfo;
  allEvents?: ChangeEventInfo[];
  k8sEvents?: K8sEventInfo[];
  nodeType: 'SPINE' | 'RELATED';
  stepNumber?: number;
  reasoning?: string;
}

export interface GraphEdge {
  id: string;
  from: string;
  to: string;
  relationshipType: string;
  edgeType: 'SPINE' | 'ATTACHMENT';
}

export interface CausalGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface RootCauseHypothesis {
  resource: SymptomResource;
  changeEvent: ChangeEventInfo;
  causationType: string;
  explanation: string;
  timeLagMs: number;
}

export interface ObservedSymptom {
  resource: SymptomResource;
  status: string;
  errorMessage: string;
  observedAt: string; // ISO timestamp
  symptomType: string;
}

export interface ConfidenceFactors {
  directSpecChange: number;
  temporalProximity: number;
  relationshipStrength: number;
  errorMessageMatch: number;
  chainCompleteness: number;
}

export interface ConfidenceScore {
  score: number;
  rationale: string;
  factors: ConfidenceFactors;
}

export interface IncidentAnalysis {
  observedSymptom: ObservedSymptom;
  graph: CausalGraph;
  rootCause: RootCauseHypothesis;
  confidence: ConfidenceScore;
}

export interface EvidenceItem {
  type: string;
  description: string;
  confidence: number;
  details?: Record<string, any>;
}

export interface ExcludedHypothesis {
  resource: SymptomResource;
  hypothesis: string;
  reasonExcluded: string;
}

export interface QueryMetadata {
  queryExecutionMs: number;
  graphNodesVisited?: number;
  algorithmVersion: string;
  executedAt: string; // ISO timestamp
}

export interface RootCauseAnalysisV2 {
  incident: IncidentAnalysis;
  supportingEvidence: EvidenceItem[];
  excludedAlternatives?: ExcludedHypothesis[];
  queryMetadata: QueryMetadata;
}

/**
 * Anomaly Detection Types
 * Matches backend schema from internal/analysis/anomaly/types.go
 */

export type AnomalyCategory = 'event' | 'state' | 'change' | 'frequency';
export type Severity = 'low' | 'medium' | 'high' | 'critical';

export interface AnomalyNode {
  uid: string;
  kind: string;
  namespace: string;
  name: string;
}

export interface Anomaly {
  node: AnomalyNode;
  category: AnomalyCategory;
  type: string;
  severity: Severity;
  timestamp: string; // ISO timestamp
  summary: string;
  details: Record<string, unknown>;
}

export interface TimeWindow {
  start: string; // ISO timestamp
  end: string; // ISO timestamp
}

export interface AnomalyResponseMetadata {
  resourceUID: string;
  timeWindow: TimeWindow;
  nodesAnalyzed: number;
  executionTimeMs?: number;
}

export interface AnomalyResponse {
  anomalies: Anomaly[];
  metadata: AnomalyResponseMetadata;
}

/**
 * UI-specific state for root cause visualization
 */
export interface RootCauseState {
  activeAnalysis: RootCauseAnalysisV2 | null;
  isLoading: boolean;
  error: Error | null;
  causalResourceMap: Map<string, boolean>;
  hiddenCausalResources: Array<{
    resource: SymptomResource;
    step: CausalStep;
  }>;
}

/**
 * Causal Paths API Types
 * Matches backend schema from internal/analysis/causal_paths/types.go
 */

export interface CausalPathsResponse {
  paths: CausalPath[];
  metadata: CausalPathsMetadata;
}

export interface CausalPath {
  id: string;
  candidateRoot: PathNode;
  firstAnomalyAt: string; // ISO timestamp
  steps: PathStep[];
  confidenceScore: number; // 0.0-1.0
  explanation: string;
  ranking: PathRanking;
}

export interface PathStep {
  node: PathNode;
  edge?: PathEdge; // nil for root node
}

export interface PathNode {
  id: string;
  resource: SymptomResource;
  anomalies: Anomaly[];
  primaryEvent?: ChangeEventInfo;
}

export interface PathEdge {
  id: string;
  relationshipType: string;
  edgeCategory: 'CAUSE_INTRODUCING' | 'MATERIALIZATION';
  causalWeight: number;
}

export interface PathRanking {
  temporalScore: number; // 0.0-1.0
  effectiveCausalDistance: number;
  maxAnomalySeverity: string;
  severityScore: number; // 0.0-1.0
}

export interface CausalPathsMetadata {
  queryExecutionMs: number;
  algorithmVersion: string;
  executedAt: string; // ISO timestamp
  nodesExplored: number;
  pathsDiscovered: number;
  pathsReturned: number;
}
