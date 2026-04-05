import axios from 'axios'
import type { FabricAIJob, FabricQuota, FabricGpuNode } from '@/types'

const apiClient = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

export const api = {
  // Cluster stats
  getClusterStats: async () => {
    const { data } = await apiClient.get('/cluster/stats')
    return data
  },

  // Jobs
  getJobs: async (): Promise<FabricAIJob[]> => {
    const { data } = await apiClient.get('/apis/kubefabric.ai/v1/namespaces/default/fabricaijobs')
    return data.items || []
  },

  getJob: async (name: string): Promise<FabricAIJob> => {
    const { data } = await apiClient.get(`/apis/kubefabric.ai/v1/namespaces/default/fabricaijobs/${name}`)
    return data
  },

  createJob: async (job: Partial<FabricAIJob>): Promise<FabricAIJob> => {
    const { data } = await apiClient.post('/apis/kubefabric.ai/v1/namespaces/default/fabricaijobs', job)
    return data
  },

  deleteJob: async (name: string): Promise<void> => {
    await apiClient.delete(`/apis/kubefabric.ai/v1/namespaces/default/fabricaijobs/${name}`)
  },

  // Quotas
  getQuotas: async (): Promise<FabricQuota[]> => {
    const { data } = await apiClient.get('/apis/kubefabric.ai/v1/fabricquotas')
    return data.items || []
  },

  getQuota: async (name: string): Promise<FabricQuota> => {
    const { data } = await apiClient.get(`/apis/kubefabric.ai/v1/fabricquotas/${name}`)
    return data
  },

  // Nodes
  getNodes: async (): Promise<FabricGpuNode[]> => {
    const { data } = await apiClient.get('/apis/kubefabric.ai/v1/fabricgpunodes')
    return data.items || []
  },

  getNode: async (name: string): Promise<FabricGpuNode> => {
    const { data } = await apiClient.get(`/apis/kubefabric.ai/v1/fabricgpunodes/${name}`)
    return data
  },

  // GPU Metrics
  getGPUMetrics: async () => {
    const { data } = await apiClient.get('/metrics/gpu')
    return data
  },

  // Cost data
  getCostData: async () => {
    const { data } = await apiClient.get('/metrics/costs')
    return data
  },
}

export default apiClient
