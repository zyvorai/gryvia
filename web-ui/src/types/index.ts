export interface FabricAIJob {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace?: string
    creationTimestamp?: string
    labels?: Record<string, string>
  }
  spec: {
    framework: string
    resources: {
      gpuType: string
      gpuCount: number
      memory: string
      cpu: number
    }
    distributed?: {
      enabled: boolean
      strategy?: string
      worldSize?: number
    }
    image: string
    command: string[]
    env?: Array<{ name: string; value: string }>
  }
  status?: {
    phase: 'Pending' | 'Running' | 'Completed' | 'Succeeded' | 'Failed' | 'Queued'
    message?: string
    startTime?: string
    completionTime?: string
  }
}

export interface FabricQuota {
  apiVersion: string
  kind: string
  metadata: {
    name: string
  }
  spec: {
    team: string
    namespaces: string[]
    gpuQuota: {
      maxGPUs: number
      maxGPUsPerJob: number
      allowedGPUTypes: string[]
      maxRunningJobs: number
    }
    budget?: {
      monthlyBudget: number
      alertThreshold: number
      hardLimit: boolean
    }
    priority: number
  }
  status?: {
    phase: string
    currentUsage: {
      allocatedGPUs: number
      runningJobs: number
      queuedJobs: number
      gpuHours: number
    }
    budgetStatus?: {
      spentThisMonth: number
      remainingBudget: number
      percentUsed: number
      projectedSpend: number
    }
  }
}

export interface FabricGpuNode {
  apiVersion: string
  kind: string
  metadata: {
    name: string
  }
  spec: {
    nodeName: string
    gpuType: string
    gpuCount: number
    memory: string
    rdmaEnabled: boolean
  }
  status?: {
    phase: string
    gpus?: Array<{
      index: number
      uuid: string
      temperature: number
      utilization: number
      memoryUsed: number
      memoryTotal: number
    }>
  }
}
