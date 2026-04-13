import axios from 'axios'
import type { FabricAIJob, FabricQuota, FabricGpuNode } from '@/types'

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

const apiClient = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Attach auth token to all requests
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('kubefabric_token') || import.meta.env.VITE_API_TOKEN || ''
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Global error handler for auth failures
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Error is propagated via Promise.reject below; avoid console.error in production
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
}

export default apiClient
