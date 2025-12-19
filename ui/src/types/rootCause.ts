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

export interface ChangeEventInfo {
  eventId: string;
  timestamp: string; // ISO timestamp
  eventType: string; // CREATE, UPDATE, DELETE
  status?: string; // Inferred resource status: Ready, Warning, Error, Terminating, Unknown
  configChanged?: boolean;
  statusChanged?: boolean;
  description?: string;
  data?: string; // Full resource JSON for diff calculation
}

export interface K8sEventInfo {
  eventId: string;
  timestamp: string; // ISO timestamp
  reason: string; // e.g., "FailedScheduling", "BackOff", "Pulling"
  message: string; // Human-readable message
  type: string; // "Warning", "Normal", "Error"
  count: number; // How many times this event occurred
  source: string; // Component that generated the event
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
