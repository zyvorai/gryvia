import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Zap, Server, GitCompare, Gauge, RefreshCw, Plus, X,
  ExternalLink, Activity, Trash2,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { InferenceService } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function InferenceServices() {
  const queryClient = useQueryClient()
  const [showCreateForm, setShowCreateForm] = useState(false)

  const { data: services, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['inferenceServices'],
    queryFn: api.getInferenceServices,
    refetchInterval: 15000,
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.deleteInferenceService(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['inferenceServices'] }),
  })

  if (isError) return (
    <div className="text-center py-12">
      <p className="text-red-400">Failed to load inference services. Please try again.</p>
    </div>
  )

  const activeServices = services?.filter(s => s.status?.phase === 'Running' || s.status?.phase === 'Ready') || []
  const totalReplicas = services?.reduce((sum, s) => sum + (s.status?.readyReplicas || 0), 0) || 0
  const canaryDeployments = services?.filter(s => s.spec?.canary && (s.spec.canary.trafficPercent ?? 0) > 0) || []
  const avgLatency = services && services.length > 0
    ? Math.round(services.reduce((sum, s) => sum + (s.status?.latencyMs || 0), 0) / services.length)
    : 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Inference Services</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">Model serving management</p>
        </div>
        <div className="flex gap-3">
          <button
            onClick={() => refetch()}
            disabled={isRefetching}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-[#8ba4c0] transition-all duration-200 disabled:opacity-50 hover:text-[#c0cce0]"
            style={{
              border: '1px solid rgba(192,204,224,0.1)',
              background: 'rgba(21,29,40,0.5)',
            }}
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isRefetching ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button
            onClick={() => setShowCreateForm(true)}
            className="inline-flex items-center gap-1.5 px-3.5 py-1.5 btn-copper text-sm font-medium rounded-lg"
          >
            <Plus className="h-3.5 w-3.5" />
            Deploy Model
          </button>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={Zap} title="Active Services" value={activeServices.length} color="cyan" />
        <StatCard icon={Server} title="Total Replicas" value={totalReplicas} color="blue" />
        <StatCard icon={GitCompare} title="Canary Deployments" value={canaryDeployments.length} color="purple" />
        <StatCard icon={Gauge} title="Avg Latency" value={avgLatency > 0 ? `${avgLatency}ms` : '-'} color="green" />
      </div>

      {/* Services Table */}
      <div className="rounded-xl" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="px-5 py-4" style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
          <div className="flex items-center gap-2">
            <Zap className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">All Services</h3>
            <span className="text-xs text-[#5a7a9e] ml-auto">{services?.length || 0} total</span>
          </div>
        </div>
        <div className="p-4">
          {isLoading ? (
            <LoadingSpinner />
          ) : (
            <div className="space-y-1">
              {/* Table header */}
              <div className="grid grid-cols-12 gap-3 px-3 py-2 text-[10px] uppercase tracking-wider text-[#5a7a9e] font-medium">
                <div className="col-span-2">Name</div>
                <div className="col-span-2">Model</div>
                <div className="col-span-1">Backend</div>
                <div className="col-span-1">Replicas</div>
                <div className="col-span-1">Status</div>
                <div className="col-span-2">Endpoint</div>
                <div className="col-span-1">Latency</div>
                <div className="col-span-1">Canary</div>
                <div className="col-span-1">Actions</div>
              </div>

              {(services || []).map((svc) => (
                <ServiceRow
                  key={svc.metadata?.name}
                  service={svc}
                  onDelete={(name) => {
                    if (window.confirm(`Delete service "${name}"?`)) {
                      deleteMutation.mutate(name)
                    }
                  }}
                />
              ))}

              {(!services || services.length === 0) && (
                <div className="text-center py-8 text-sm text-[#344e6a]">No inference services deployed</div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Create Modal */}
      {showCreateForm && (
        <CreateInferenceServiceModal onClose={() => setShowCreateForm(false)} />
      )}
    </div>
  )
}

// --- Sub-components ---

const backendBadgeConfig: Record<string, { bg: string; text: string; border: string }> = {
  Triton: { bg: 'rgba(95,168,211,0.08)', text: '#7ecbf5', border: 'rgba(95,168,211,0.2)' },
  vLLM: { bg: 'rgba(168,85,247,0.08)', text: '#c084fc', border: 'rgba(168,85,247,0.2)' },
  'TensorRT-LLM': { bg: 'rgba(34,197,94,0.08)', text: '#4ade80', border: 'rgba(34,197,94,0.2)' },
  TorchServe: { bg: 'rgba(212,118,78,0.08)', text: '#e8a87c', border: 'rgba(212,118,78,0.2)' },
}

function BackendBadge({ backend }: { backend: string }) {
  const cfg = backendBadgeConfig[backend] || backendBadgeConfig.Triton
  return (
    <span className="inline-flex items-center text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
      background: cfg.bg,
      color: cfg.text,
      border: `1px solid ${cfg.border}`,
    }}>
      {backend}
    </span>
  )
}

function ServiceRow({ service, onDelete }: { service: InferenceService; onDelete: (name: string) => void }) {
  const name = service.metadata?.name || 'unknown'
  const phase = service.status?.phase || 'Unknown'
  const statusColors: Record<string, string> = {
    Running: 'text-emerald-400',
    Ready: 'text-emerald-400',
    Pending: 'text-[#fbbf24]',
    Failed: 'text-red-400',
    Scaling: 'text-cyan-400',
  }

  return (
    <div className="grid grid-cols-12 gap-3 px-3 py-3 rounded-lg table-row-hover items-center">
      <div className="col-span-2 text-sm font-medium text-[#c0cce0] truncate">{name}</div>
      <div className="col-span-2 text-xs text-[#8ba4c0] truncate">{service.spec?.modelRef || '-'}</div>
      <div className="col-span-1">
        <BackendBadge backend={service.spec?.backend || 'Triton'} />
      </div>
      <div className="col-span-1 text-xs text-[#8ba4c0]">
        {service.status?.readyReplicas || 0}/{service.spec?.replicas || 0}
      </div>
      <div className="col-span-1">
        <div className="flex items-center gap-1">
          <Activity className={`h-3 w-3 ${statusColors[phase] || 'text-[#5a7a9e]'}`} />
          <span className={`text-xs font-medium ${statusColors[phase] || 'text-[#5a7a9e]'}`}>{phase}</span>
        </div>
      </div>
      <div className="col-span-2">
        {service.status?.endpoint ? (
          <a href={service.status.endpoint} target="_blank" rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-[10px] text-[#e8a87c] hover:text-[#f0c4a0] transition-colors truncate max-w-full">
            <ExternalLink className="h-3 w-3 flex-shrink-0" />
            <span className="truncate">{service.status.endpoint}</span>
          </a>
        ) : (
          <span className="text-xs text-[#344e6a]">-</span>
        )}
      </div>
      <div className="col-span-1 text-xs text-[#8ba4c0]">
        {service.status?.latencyMs ? `${service.status.latencyMs}ms` : '-'}
      </div>
      <div className="col-span-1">
        {service.spec?.canary && (service.spec.canary.trafficPercent ?? 0) > 0 ? (
          <span className="inline-flex items-center text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
            background: 'rgba(168,85,247,0.08)',
            color: '#c084fc',
            border: '1px solid rgba(168,85,247,0.2)',
          }}>
            {service.spec.canary.trafficPercent}%
          </span>
        ) : (
          <span className="text-xs text-[#344e6a]">-</span>
        )}
      </div>
      <div className="col-span-1">
        <button
          onClick={() => onDelete(name)}
          className="p-1.5 text-[#5a7a9e] hover:text-red-400 transition-colors rounded-lg"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  )
}

function CreateInferenceServiceModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [formData, setFormData] = useState({
    name: '',
    modelRef: '',
    backend: 'Triton',
    replicas: 1,
    minReplicas: 1,
    maxReplicas: 4,
    targetUtilization: 80,
  })
  const [error, setError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: (data: typeof formData) => api.createInferenceService(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inferenceServices'] })
      onClose()
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const k8sNameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
    if (!k8sNameRegex.test(formData.name)) {
      setError('Name must consist of lowercase alphanumeric characters or hyphens')
      return
    }
    if (!formData.modelRef.trim()) {
      setError('Model reference is required')
      return
    }

    createMutation.mutate(formData)
  }

  const inputStyle = {
    background: 'rgba(10,14,20,0.6)',
    border: '1px solid rgba(192,204,224,0.08)',
    boxShadow: 'inset 0 2px 4px rgba(0,0,0,0.3)',
    color: '#d0dae6',
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card w-full max-w-lg mx-4" onClick={(e) => e.stopPropagation()}>
        <div className="px-6 py-4" style={{
          borderBottom: '1px solid rgba(192,204,224,0.06)',
          background: 'rgba(10,14,20,0.3)',
        }}>
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-bold text-gradient-copper">Deploy Inference Service</h3>
            <button onClick={onClose} className="text-[#5a7a9e] hover:text-[#c0cce0] transition-colors">
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Service Name</label>
              <input
                type="text"
                required
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                className="block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
                placeholder="my-inference-svc"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Model Reference</label>
              <input
                type="text"
                required
                value={formData.modelRef}
                onChange={(e) => setFormData({ ...formData, modelRef: e.target.value })}
                className="block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
                placeholder="my-model:v2"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Backend</label>
              <select
                value={formData.backend}
                onChange={(e) => setFormData({ ...formData, backend: e.target.value })}
                className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
              >
                <option value="Triton">Triton</option>
                <option value="vLLM">vLLM</option>
                <option value="TensorRT-LLM">TensorRT-LLM</option>
                <option value="TorchServe">TorchServe</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Replicas</label>
              <input
                type="number"
                min="1"
                max="32"
                required
                value={formData.replicas}
                onChange={(e) => setFormData({ ...formData, replicas: parseInt(e.target.value, 10) || 1 })}
                className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
              />
            </div>
          </div>

          <div>
            <h4 className="text-xs font-semibold text-[#e8ecf1] mb-3">Autoscaling</h4>
            <div className="grid grid-cols-3 gap-4">
              <div>
                <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Min Replicas</label>
                <input
                  type="number"
                  min="0"
                  max="32"
                  value={formData.minReplicas}
                  onChange={(e) => setFormData({ ...formData, minReplicas: parseInt(e.target.value, 10) || 0 })}
                  className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                  style={inputStyle}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Max Replicas</label>
                <input
                  type="number"
                  min="1"
                  max="64"
                  value={formData.maxReplicas}
                  onChange={(e) => setFormData({ ...formData, maxReplicas: parseInt(e.target.value, 10) || 1 })}
                  className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                  style={inputStyle}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Target CPU %</label>
                <input
                  type="number"
                  min="10"
                  max="100"
                  value={formData.targetUtilization}
                  onChange={(e) => setFormData({ ...formData, targetUtilization: parseInt(e.target.value, 10) || 80 })}
                  className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                  style={inputStyle}
                />
              </div>
            </div>
          </div>

          {error && (
            <div className="rounded-xl p-3" style={{
              background: 'rgba(251,191,36,0.06)',
              border: '1px solid rgba(251,191,36,0.12)',
            }}>
              <p className="text-xs text-[#fbbf24]">{error}</p>
            </div>
          )}

          {createMutation.isError && (
            <div className="rounded-xl p-3" style={{
              background: 'rgba(239,68,68,0.06)',
              border: '1px solid rgba(239,68,68,0.12)',
            }}>
              <p className="text-xs text-[#f87171]">
                Error deploying service: {createMutation.error instanceof Error ? createMutation.error.message : 'Unknown error'}
              </p>
            </div>
          )}

          <div className="flex justify-end gap-3 pt-4" style={{ borderTop: '1px solid rgba(192,204,224,0.04)' }}>
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-lg text-sm font-medium text-[#8ba4c0] hover:text-[#c0cce0] transition-all"
              style={{
                border: '1px solid rgba(192,204,224,0.1)',
                background: 'rgba(21,29,40,0.5)',
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={createMutation.isPending}
              className="px-4 py-2 btn-copper rounded-lg text-sm font-medium disabled:opacity-50"
            >
              {createMutation.isPending ? 'Deploying...' : 'Deploy Service'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
