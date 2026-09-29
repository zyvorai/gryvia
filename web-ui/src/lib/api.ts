import axios from 'axios'
import toast from 'react-hot-toast'
import { notifyUnauthorized } from '@/lib/authEvents'
import type { GryviaAIJob, GryviaQuota, GryviaGpuNode } from '@/types'
import { getStoredToken, clearToken } from '@/lib/auth'
import type { Sku, SkuBody, TenantResource, CreateTenantBody, UsageReport } from '@/lib/cloud'
import type { InvoiceReport } from '@/lib/invoices'

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
  /** 'all-time': the totals cover every billable job since creation, not a calendar month. */
  scope?: string
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
    verdict: 'FORWARDED' | 'DROP' | 'DENIED' | 'ALLOW' | 'ERROR' | ''
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
    action: 'allow' | 'deny' | 'log'
    intent: string
    confidence?: number
  }
  status?: {
    phase: 'Enforced' | 'Pending' | 'Suggested' | 'Active'
    matchedFlows: number
    ciliumPolicyRef?: string
  }
}

export interface ServiceGraphNode {
  id: string
  label: string
  health: 'healthy' | 'warning' | 'critical' | 'unknown'
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
  name?: string
  sourceService: string
  destinationService: string
  port: number
  protocol: string
  action: 'allow' | 'deny' | 'log'
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
  source?: { name: string; namespace: string }
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
  collectors?: { reachable: number; total: number }
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
  collectors?: { reachable: number; total: number }
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
    metricName?: string
    direction?: 'maximize' | 'minimize'
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

// Job runtime details (pods, logs, events)
export interface JobPod {
  name: string
  phase: string
  node?: string
  podIP?: string
  startTime?: string
  restarts: number
  message?: string
  containers: Array<{ name: string; ready: boolean; state: string; restartCount: number; reason?: string }>
}

export interface JobLogs {
  pod: string | null
  container?: string
  lines: string[]
  truncated: boolean
}

export interface JobEvent {
  type: string
  reason: string
  message: string
  count: number
  firstSeen?: string
  lastSeen?: string
  object: string
}

export interface CreateWorkflowRequest {
  metadata: { name: string }
  /** Steps as accepted by the gateway: name, type (job|script|webhook), dependsOn, and the step payload. */
  spec: { steps: unknown[] }
}

export interface CreateTunerRequest {
  name: string
  algorithm: string
  objectiveMetric: string
  direction: 'maximize' | 'minimize'
  maxTrials: number
  parameterSpace: string
  /** The job run for each trial (required by the GryviaAutoTuner CRD). */
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
  const token = getStoredToken() || ''
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Session and rate-limit handling. 401 goes through the router (keeps the return path and shows
// "session expired"); 403/429 tell the user instead of failing silently.
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    if (status === 401) {
      clearToken()
      const path = window.location.pathname
      if (path !== '/login' && path !== '/auth/callback' && !notifyUnauthorized()) {
        window.location.href = '/login'
      }
    } else if (status === 403) {
      toast.error("You don't have permission to do that.", { id: 'http-403' })
    } else if (status === 429) {
      toast.error('Too many requests. Wait a moment and try again.', { id: 'http-429' })
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
  getJobs: async (): Promise<GryviaAIJob[]> => {
    const { data } = await apiClient.get('/jobs')
    return data.items || []
  },

  getJob: async (name: string): Promise<GryviaAIJob> => {
    const { data } = await apiClient.get(`/jobs/${encodeURIComponent(name)}`)
    return data
  },

  createJob: async (job: Partial<GryviaAIJob>): Promise<GryviaAIJob> => {
    const { data } = await apiClient.post('/jobs', job)
    return data
  },

  getJobPods: async (name: string): Promise<JobPod[]> => {
    const { data } = await apiClient.get(`/jobs/${encodeURIComponent(name)}/pods`)
    return data.items || []
  },

  getJobLogs: async (name: string, opts: { pod?: string; tail?: number } = {}): Promise<JobLogs> => {
    const { data } = await apiClient.get(`/jobs/${encodeURIComponent(name)}/logs`, { params: { pod: opts.pod, tail: opts.tail } })
    return data
  },

  getJobEvents: async (name: string): Promise<JobEvent[]> => {
    const { data } = await apiClient.get(`/jobs/${encodeURIComponent(name)}/events`)
    return data.items || []
  },

  deleteJob: async (name: string): Promise<void> => {
    await apiClient.delete(`/jobs/${encodeURIComponent(name)}`)
  },

  // Quotas - routed through API gateway
  getQuotas: async (): Promise<GryviaQuota[]> => {
    const { data } = await apiClient.get('/quotas')
    return data.items || []
  },

  getQuota: async (name: string): Promise<GryviaQuota> => {
    const { data } = await apiClient.get(`/quotas/${encodeURIComponent(name)}`)
    return data
  },

  // Nodes - routed through API gateway
  getNodes: async (): Promise<GryviaGpuNode[]> => {
    const { data } = await apiClient.get('/nodes')
    return data.items || []
  },

  getNode: async (name: string): Promise<GryviaGpuNode> => {
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

  /** Alerts plus whether any event source is connected (false = an empty list does not mean "all clear"). */
  getSecurityAlertsStatus: async (): Promise<{ items: SecurityAlert[]; eventSource: boolean }> => {
    const { data } = await apiClient.get('/security/alerts')
    return { items: data.items || [], eventSource: data.eventSource !== false }
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

  // GPU-as-a-Service: catalog, tenants, usage
  getSkus: async (): Promise<Sku[]> => {
    const { data } = await apiClient.get('/skus')
    return data.items || []
  },

  createSku: async (body: SkuBody & { name: string }): Promise<Sku> => {
    const { data } = await apiClient.post('/skus', body)
    return data
  },

  updateSku: async (name: string, body: SkuBody): Promise<Sku> => {
    const { data } = await apiClient.put(`/skus/${encodeURIComponent(name)}`, body)
    return data
  },

  deleteSku: async (name: string): Promise<void> => {
    await apiClient.delete(`/skus/${encodeURIComponent(name)}`)
  },

  getTenants: async (): Promise<TenantResource[]> => {
    const { data } = await apiClient.get('/tenants')
    return data.items || []
  },

  createTenant: async (body: CreateTenantBody): Promise<TenantResource> => {
    const { data } = await apiClient.post('/tenants', body)
    return data
  },

  deleteTenant: async (name: string): Promise<void> => {
    await apiClient.delete(`/tenants/${encodeURIComponent(name)}`)
  },

  getUsage: async (params: Record<string, string>): Promise<UsageReport> => {
    const { data } = await apiClient.get('/usage', { params })
    return data
  },

  /** Authenticated download (the token is a bearer header, so a plain link would be rejected). */
  exportUsage: async (format: 'csv' | 'json', params: Record<string, string>): Promise<Blob> => {
    const { data } = await apiClient.get('/usage/export', { params: { ...params, format }, responseType: 'blob' })
    return data as Blob
  },

  getInvoices: async (params: Record<string, string>): Promise<InvoiceReport> => {
    const { data } = await apiClient.get('/invoices', { params })
    return data
  },

  /** Authenticated invoice download (bearer header, so not a plain link). */
  downloadInvoice: async (tenant: string, month: string, format: 'csv' | 'json'): Promise<Blob> => {
    const { data } = await apiClient.get(`/invoices/${encodeURIComponent(tenant)}/${encodeURIComponent(month)}`, { params: { format }, responseType: 'blob' })
    return data as Blob
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

  deleteWorkflow: async (name: string): Promise<void> => {
    await apiClient.delete(`/workflows/${encodeURIComponent(name)}`)
  },

  createWorkflow: async (workflow: CreateWorkflowRequest): Promise<Workflow> => {
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

  deleteTuner: async (name: string): Promise<void> => {
    await apiClient.delete(`/tuners/${encodeURIComponent(name)}`)
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
