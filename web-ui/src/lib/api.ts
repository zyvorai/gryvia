import axios from 'axios'
import type { FabricAIJob, FabricQuota, FabricGpuNode } from '@/types'
import { getStoredToken, clearToken } from '@/lib/auth'

export interface ClusterStats {
  totalGPUs: number
  availableGPUs: number
  allocatedGPUs: number
  utilizationPercent: number
  totalJobs: number
  runningJobs: number
  pendingJobs: number
  completedJobs: number
  failedJobs: number
  totalNodes: number
  avgGPUUtilization?: number
}

export interface GPUMetric {
  node: string
  gpuIndex: number
  utilization: number
  temperature: number
  memoryUsed: number
  memoryTotal: number
  timestamp: string
}

export interface GPUMetricsResponse {
  timeRange: string
  metrics: GPUMetric[]
}

export interface CostData {
  monthly: Array<{ month: string; cost: number }>
  hasHistoricalData: boolean
  byTeam: Array<{ team: string; cost: number }>
  byGPUType: Array<{ type: string; cost: number; hours: number }>
  totalCost: number
}

// Network intelligence types
export interface NetworkFlow {
  metadata: { name: string; namespace?: string; creationTimestamp?: string }
  spec: {
    timestamp: string
    source: string
    destination: string
    protocol: string
    port: number
    bytes: string
    latency: string
    verdict: 'FORWARDED' | 'DROP' | 'DENIED' | 'ALLOW'
    policy?: string
  }
}

export interface FlowPolicy {
  metadata: { name: string; namespace?: string; creationTimestamp?: string; labels?: Record<string, string> }
  spec: {
    sourceService: string
    destinationService: string
    port: number
    protocol: string
    action: 'allow' | 'deny'
    intent: string
    confidence?: number
  }
  status?: {
    phase: 'Enforced' | 'Pending' | 'Suggested' | 'Active'
    matchedFlows: number
  }
}

export interface ServiceGraphNode {
  id: string
  label: string
  health: 'healthy' | 'warning' | 'critical'
  flowCount: number
}

export interface ServiceGraphEdge {
  source: string
  target: string
  protocol: string
  latency: string
  verdict: string
}

export interface ServiceGraph {
  nodes: ServiceGraphNode[]
  edges: ServiceGraphEdge[]
}

export interface TrafficInsights {
  activeFlows: number
  policiesEnforced: number
  anomaliesDetected: number
  traceSessions: number
}

export interface NetworkAnomaly {
  metadata: { name: string; namespace?: string; creationTimestamp?: string }
  spec: {
    severity: 'critical' | 'high' | 'medium' | 'low'
    service: string
    type: string
    description: string
    detectedAt: string
  }
}

export interface TraceSession {
  metadata: { name: string; namespace?: string; creationTimestamp?: string }
  spec: {
    targetService: string
    duration: string
    captureLevel: string
    namespace: string
  }
  status?: {
    phase: 'Running' | 'Completed' | 'Failed' | 'Active'
    flowsCaptured: number
  }
}

export interface CreateFlowPolicyRequest {
  sourceService: string
  destinationService: string
  port: number
  protocol: string
  action: 'allow' | 'deny'
  intent: string
}

export interface CreateTraceSessionRequest {
  targetService: string
  duration: string
  captureLevel: string
  namespace: string
}

// Security types
export interface SecurityAlert {
  type: string
  severity: 'critical' | 'high' | 'medium' | 'low'
  process: string
  path: string
  sourceIP: string
  message: string
  timestamp: string
}

export interface SecurityPolicy {
  metadata: { name: string; namespace?: string; creationTimestamp?: string }
  spec: {
    targetNamespaces: string[]
    detectionRules: Array<{
      type: string
      enabled: boolean
      sensitivity: string
    }>
    alertWebhook?: string
    autoBlock: boolean
  }
  status?: {
    phase: string
    activeDetections: number
    alertsTriggered: number
    lastAlert?: string
    detectionCounts?: Record<string, number>
  }
}

export interface CreateSecurityPolicyRequest {
  name: string
  targetNamespaces: string[]
  detectionRules: Array<{
    type: string
    enabled: boolean
    sensitivity: string
  }>
  autoBlock: boolean
  alertWebhook?: string
}

// Network cost types
export interface NetworkCostReport {
  period: string
  namespace: string
  team: string
  sameZoneBytes: number
  crossZoneBytes: number
  externalBytes: number
  totalCostUSD: number
}

export interface NetworkCostData {
  reports: NetworkCostReport[]
  costPerGB: {
    sameZone: number
    crossZone: number
    internetEgress: number
  }
}

// Training insight types
export interface TrainingInsight {
  rankStats: Array<{
    rank: number
    avgLatencyNs: number
    totalBytes: number
    isStraggler: boolean
  }>
  commPattern: string
  commComputeRatio: number
  stragglers: Array<{
    rank: number
    slowdownFactor: number
    reason: string
  }>
  bottleneck: string
  lastAnalysis?: string
}

// NCCL stats types
export interface NCCLStats {
  operations: Array<{
    opType: string
    count: number
    avgLatencyNs: number
    p99LatencyNs: number
    totalBytes: number
  }>
}

// GPU memory stats types
export interface GPUMemStats {
  h2dBytes: number
  d2hBytes: number
  d2dBytes: number
  h2dCount: number
  d2hCount: number
  d2dCount: number
}

// Workspace types
export interface Workspace {
  metadata?: { name?: string; namespace?: string; creationTimestamp?: string }
  spec?: {
    type?: string
    gpuCount?: number
    gpuType?: string
    storageSize?: string
    idleTimeout?: string
  }
  status?: {
    phase?: string
    url?: string
    uptime?: string
    lastActivity?: string
  }
}

export interface CreateWorkspaceRequest {
  name: string
  type: string
  gpuCount: number
  gpuType: string
  storageSize: string
  idleTimeout: string
}

// Model registry types
export interface RegisteredModel {
  metadata?: { name?: string; namespace?: string; creationTimestamp?: string }
  spec?: {
    version?: string
    stage?: string
    sourceJob?: string
    artifacts?: string[]
  }
  status?: {
    servingEndpoint?: string
  }
}

// Inference service types
export interface InferenceService {
  metadata?: { name?: string; namespace?: string; creationTimestamp?: string }
  spec?: {
    modelRef?: string
    backend?: string
    replicas?: number
    canary?: {
      trafficPercent?: number
    }
    autoscaling?: {
      minReplicas?: number
      maxReplicas?: number
      targetUtilization?: number
    }
  }
  status?: {
    phase?: string
    readyReplicas?: number
    endpoint?: string
    latencyMs?: number
  }
}

export interface CreateInferenceServiceRequest {
  name: string
  modelRef: string
  backend: string
  replicas: number
  minReplicas: number
  maxReplicas: number
  targetUtilization: number
}

// Workflow types
export interface WorkflowStep {
  name: string
  type?: string
  status?: string
  duration?: string
  jobRef?: string
  dependsOn?: string[]
}

export interface Workflow {
  metadata?: { name?: string; namespace?: string; creationTimestamp?: string }
  spec?: {
    steps?: WorkflowStep[]
  }
  status?: {
    phase?: string
    steps?: WorkflowStep[]
    duration?: string
    startedAt?: string
  }
}

// Auto tuner types
export interface TunerTrial {
  trialId: string
  parameters?: Record<string, unknown>
  metricValue?: number
  status?: string
  duration?: string
}

export interface AutoTunerJob {
  metadata?: { name?: string; namespace?: string; creationTimestamp?: string }
  spec?: {
    algorithm?: string
    objectiveMetric?: string
    maxTrials?: number
    parameterSpace?: Record<string, unknown>
  }
  status?: {
    phase?: string
    trialsCompleted?: number
    trialsRunning?: number
    bestMetricValue?: number
    bestTrialId?: string
  }
}

export interface CreateTunerRequest {
  name: string
  algorithm: string
  objectiveMetric: string
  direction: 'maximize' | 'minimize'
  maxTrials: number
  parameterSpace: string
  /** The job run for each trial (required by the FabricAutoTuner CRD). */
  jobTemplate: { type: 'training'; image: string; gpus: number }
  /** Required when algorithm is ASHA. */
  ashaConfig?: { maxEpochs: number }
}

const apiClient = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Attach auth token to all requests
apiClient.interceptors.request.use((config) => {
  const token = getStoredToken() || import.meta.env.VITE_API_TOKEN || ''
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Global error handler for auth failures - redirect to login on 401
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      clearToken()
      // Redirect to login unless already on login/callback pages
      const path = window.location.pathname
      if (path !== '/login' && path !== '/auth/callback') {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  }
)

export const api = {
  // Cluster stats
  getClusterStats: async (): Promise<ClusterStats> => {
    const { data } = await apiClient.get('/cluster/stats')
    return data
  },

  // Jobs - routed through API gateway
  getJobs: async (): Promise<FabricAIJob[]> => {
    const { data } = await apiClient.get('/jobs')
    return data.items || []
  },

  getJob: async (name: string): Promise<FabricAIJob> => {
    const { data } = await apiClient.get(`/jobs/${encodeURIComponent(name)}`)
    return data
  },

  createJob: async (job: Partial<FabricAIJob>): Promise<FabricAIJob> => {
    const { data } = await apiClient.post('/jobs', job)
    return data
  },

  deleteJob: async (name: string): Promise<void> => {
    await apiClient.delete(`/jobs/${encodeURIComponent(name)}`)
  },

  // Quotas - routed through API gateway
  getQuotas: async (): Promise<FabricQuota[]> => {
    const { data } = await apiClient.get('/quotas')
    return data.items || []
  },

  getQuota: async (name: string): Promise<FabricQuota> => {
    const { data } = await apiClient.get(`/quotas/${encodeURIComponent(name)}`)
    return data
  },

  // Nodes - routed through API gateway
  getNodes: async (): Promise<FabricGpuNode[]> => {
    const { data } = await apiClient.get('/nodes')
    return data.items || []
  },

  getNode: async (name: string): Promise<FabricGpuNode> => {
    const { data } = await apiClient.get(`/nodes/${encodeURIComponent(name)}`)
    return data
  },

  // GPU Metrics
  getGPUMetrics: async (): Promise<GPUMetricsResponse> => {
    const { data } = await apiClient.get('/metrics/gpu')
    return data
  },

  // Cost data
  getCostData: async (): Promise<CostData> => {
    const { data } = await apiClient.get('/metrics/costs')
    return data
  },

  // Network intelligence
  getFlowPolicies: async (): Promise<FlowPolicy[]> => {
    const { data } = await apiClient.get('/network/policies')
    return data.items || []
  },

  createFlowPolicy: async (policy: CreateFlowPolicyRequest): Promise<FlowPolicy> => {
    const { data } = await apiClient.post('/network/policies', policy)
    return data
  },

  applyFlowPolicy: async (name: string): Promise<void> => {
    await apiClient.post(`/network/policies/${encodeURIComponent(name)}/apply`)
  },

  getTrafficInsights: async (): Promise<TrafficInsights> => {
    const { data } = await apiClient.get('/network/insights')
    return data
  },

  getServiceGraph: async (): Promise<ServiceGraph> => {
    const { data } = await apiClient.get('/network/graph')
    return data
  },

  getNetworkAnomalies: async (): Promise<NetworkAnomaly[]> => {
    const { data } = await apiClient.get('/network/anomalies')
    return data.items || []
  },

  getNetworkFlows: async (): Promise<NetworkFlow[]> => {
    const { data } = await apiClient.get('/network/flows')
    return data.items || []
  },

  createTraceSession: async (req: CreateTraceSessionRequest): Promise<TraceSession> => {
    const { data } = await apiClient.post('/network/traces', req)
    return data
  },

  getTraceSessions: async (): Promise<TraceSession[]> => {
    const { data } = await apiClient.get('/network/traces')
    return data.items || []
  },

  getTraceSession: async (name: string): Promise<TraceSession> => {
    const { data } = await apiClient.get(`/network/traces/${encodeURIComponent(name)}`)
    return data
  },

  // Security
  getSecurityAlerts: async (): Promise<SecurityAlert[]> => {
    const { data } = await apiClient.get('/security/alerts')
    return data.items || []
  },

  getSecurityPolicies: async (): Promise<SecurityPolicy[]> => {
    const { data } = await apiClient.get('/security/policies')
    return data.items || []
  },

  createSecurityPolicy: async (policy: CreateSecurityPolicyRequest): Promise<SecurityPolicy> => {
    const { data } = await apiClient.post('/security/policies', policy)
    return data
  },

  // Network costs
  getNetworkCosts: async (): Promise<NetworkCostData> => {
    const { data } = await apiClient.get('/network/costs')
    return data
  },

  // Training insights
  getTrainingInsight: async (): Promise<TrainingInsight> => {
    const { data } = await apiClient.get('/ai/training/insight')
    return data
  },

  // NCCL stats
  getNCCLStats: async (): Promise<NCCLStats> => {
    const { data } = await apiClient.get('/ai/training/nccl')
    return data
  },

  // GPU memory stats
  getGPUMemoryStats: async (): Promise<GPUMemStats> => {
    const { data } = await apiClient.get('/gpu/memory')
    return data
  },

  // Workspaces
  getWorkspaces: async (): Promise<Workspace[]> => {
    const { data } = await apiClient.get('/workspaces')
    return data.items || []
  },

  getWorkspace: async (name: string): Promise<Workspace> => {
    const { data } = await apiClient.get(`/workspaces/${encodeURIComponent(name)}`)
    return data
  },

  createWorkspace: async (req: CreateWorkspaceRequest): Promise<Workspace> => {
    const { data } = await apiClient.post('/workspaces', req)
    return data
  },

  pauseWorkspace: async (name: string): Promise<void> => {
    await apiClient.post(`/workspaces/${encodeURIComponent(name)}/pause`)
  },

  resumeWorkspace: async (name: string): Promise<void> => {
    await apiClient.post(`/workspaces/${encodeURIComponent(name)}/resume`)
  },

  deleteWorkspace: async (name: string): Promise<void> => {
    await apiClient.delete(`/workspaces/${encodeURIComponent(name)}`)
  },

  // Model Registry
  getModels: async (): Promise<RegisteredModel[]> => {
    const { data } = await apiClient.get('/models')
    return data.items || []
  },

  getModel: async (name: string): Promise<RegisteredModel> => {
    const { data } = await apiClient.get(`/models/${encodeURIComponent(name)}`)
    return data
  },

  promoteModel: async (name: string, targetStage: string): Promise<RegisteredModel> => {
    const { data } = await apiClient.post(`/models/${encodeURIComponent(name)}/promote`, { targetStage })
    return data
  },

  // Inference Services
  getInferenceServices: async (): Promise<InferenceService[]> => {
    const { data } = await apiClient.get('/inference')
    return data.items || []
  },

  getInferenceService: async (name: string): Promise<InferenceService> => {
    const { data } = await apiClient.get(`/inference/${encodeURIComponent(name)}`)
    return data
  },

  createInferenceService: async (req: CreateInferenceServiceRequest): Promise<InferenceService> => {
    const { data } = await apiClient.post('/inference', req)
    return data
  },

  deleteInferenceService: async (name: string): Promise<void> => {
    await apiClient.delete(`/inference/${encodeURIComponent(name)}`)
  },

  // Workflows
  getWorkflows: async (): Promise<Workflow[]> => {
    const { data } = await apiClient.get('/workflows')
    return data.items || []
  },

  getWorkflow: async (name: string): Promise<Workflow> => {
    const { data } = await apiClient.get(`/workflows/${encodeURIComponent(name)}`)
    return data
  },

  createWorkflow: async (workflow: Partial<Workflow>): Promise<Workflow> => {
    const { data } = await apiClient.post('/workflows', workflow)
    return data
  },

  // Auto Tuners
  getTuners: async (): Promise<AutoTunerJob[]> => {
    const { data } = await apiClient.get('/tuners')
    return data.items || []
  },

  getTuner: async (name: string): Promise<AutoTunerJob> => {
    const { data } = await apiClient.get(`/tuners/${encodeURIComponent(name)}`)
    return data
  },

  createTuner: async (req: CreateTunerRequest): Promise<AutoTunerJob> => {
    const { data } = await apiClient.post('/tuners', req)
    return data
  },

  getTunerTrials: async (name: string): Promise<TunerTrial[]> => {
    const { data } = await apiClient.get(`/tuners/${encodeURIComponent(name)}/trials`)
    return data.items || []
  },
}

export default apiClient
