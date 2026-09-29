import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { api } from '@/lib/api'
import type { InferenceService } from '@/lib/api'
import PageHero from '@/components/PageHero'
import LoadingSpinner from '@/components/LoadingSpinner'

const BACKEND_LABELS: Record<string, string> = {
  triton: 'Triton',
  vllm: 'vLLM',
  'tensorrt-llm': 'TensorRT-LLM',
  torchserve: 'TorchServe',
}

/** The gateway stores backends lowercase (CRD values); show the product names. */
function backendLabel(backend?: string): string {
  if (!backend) return 'Triton'
  return BACKEND_LABELS[backend.toLowerCase()] ?? backend
}

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
    <>
      <PageHero eyebrow="Inference" title="Services unavailable." tint="red" />
      <p className="login-error" role="alert">Failed to load inference services. Please try again.</p>
    </>
  )

  const activeServices = services?.filter(s => s.status?.phase === 'Running' || s.status?.phase === 'Ready') || []
  const totalReplicas = services?.reduce((sum, s) => sum + (s.status?.readyReplicas || 0), 0) || 0
  const canaryDeployments = services?.filter(s => s.spec?.canary && (s.spec.canary.trafficPercent ?? 0) > 0) || []
  const avgLatency = services && services.length > 0
    ? Math.round(services.reduce((sum, s) => sum + (s.status?.latencyMs || 0), 0) / services.length)
    : 0

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Inference" title="Serve models at scale." lede="Model serving management." />

      <div className="page-actions">
        <button className="primary" onClick={() => setShowCreateForm(true)}>
          Deploy Model
        </button>
        <button className="btn-secondary" onClick={() => refetch()} disabled={isRefetching}>
          Refresh
        </button>
      </div>

      <div className="apple-metric-band">
        <div><span>Active Services</span><b>{activeServices.length}</b></div>
        <div><span>Total Replicas</span><b>{totalReplicas}</b></div>
        <div><span>Canary Deployments</span><b>{canaryDeployments.length}</b></div>
        <div><span>Avg Latency</span><b>{avgLatency > 0 ? `${avgLatency}ms` : '-'}</b></div>
      </div>

      <section className="card">
        <div className="stat-head">
          <h2>All Services</h2>
          <span className="faint">{services?.length || 0} total</span>
        </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (services || []).length === 0 ? (
          <div className="list-empty">No inference services deployed</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Model</th>
                  <th>Backend</th>
                  <th>Replicas</th>
                  <th>Status</th>
                  <th>Endpoint</th>
                  <th>Latency</th>
                  <th>Canary</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
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
              </tbody>
            </table>
          </div>
        )}
      </section>

      {showCreateForm && (
        <CreateInferenceServiceModal onClose={() => setShowCreateForm(false)} />
      )}
    </div>
  )
}

// --- Sub-components ---

function ServiceRow({ service, onDelete }: { service: InferenceService; onDelete: (name: string) => void }) {
  const name = service.metadata?.name || 'unknown'
  const phase = service.status?.phase || 'Unknown'
  const tone: Record<string, string> = {
    Running: 'ok',
    Ready: 'ok',
    Pending: 'warn',
    Failed: 'bad',
    Scaling: 'info',
  }

  return (
    <tr>
      <td><b>{name}</b></td>
      <td className="muted">{service.spec?.modelRef || '-'}</td>
      <td><span className="pill">{backendLabel(service.spec?.backend)}</span></td>
      <td className="muted">
        {service.status?.readyReplicas || 0}/{service.spec?.replicas || 0}
      </td>
      <td><span className={`pill ${tone[phase] || ''}`}>{phase}</span></td>
      <td>
        {service.status?.endpoint ? (
          <a href={service.status.endpoint} target="_blank" rel="noopener noreferrer" className="card-link mono">
            {service.status.endpoint}
          </a>
        ) : (
          <span className="faint">-</span>
        )}
      </td>
      <td className="muted">
        {service.status?.latencyMs ? `${service.status.latencyMs}ms` : '-'}
      </td>
      <td>
        {service.spec?.canary && (service.spec.canary.trafficPercent ?? 0) > 0 ? (
          <span className="pill info">{service.spec.canary.trafficPercent}%</span>
        ) : (
          <span className="faint">-</span>
        )}
      </td>
      <td>
        <button className="danger" onClick={() => onDelete(name)}>
          Delete
        </button>
      </td>
    </tr>
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

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="stat-head" style={{ marginBottom: 16 }}>
          <h2 style={{ margin: 0 }}>Deploy Inference Service</h2>
          <button type="button" className="icon-button" onClick={onClose} aria-label="Close">
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="stack" style={{ gap: 16 }}>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Service Name</label>
              <input
                type="text"
                required
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="my-inference-svc"
              />
            </div>
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Model Reference</label>
              <input
                type="text"
                required
                value={formData.modelRef}
                onChange={(e) => setFormData({ ...formData, modelRef: e.target.value })}
                placeholder="my-model:v2"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Backend</label>
              <select
                value={formData.backend}
                onChange={(e) => setFormData({ ...formData, backend: e.target.value })}
              >
                <option value="Triton">Triton</option>
                <option value="vLLM">vLLM</option>
                <option value="TensorRT-LLM">TensorRT-LLM</option>
                <option value="TorchServe">TorchServe</option>
              </select>
            </div>
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Replicas</label>
              <input
                type="number"
                min="1"
                max="32"
                required
                value={formData.replicas}
                onChange={(e) => setFormData({ ...formData, replicas: parseInt(e.target.value, 10) || 1 })}
              />
            </div>
          </div>

          <div>
            <h3>Autoscaling</h3>
            <div className="grid grid-cols-3 gap-4">
              <div>
                <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Min Replicas</label>
                <input
                  type="number"
                  min="0"
                  max="32"
                  value={formData.minReplicas}
                  onChange={(e) => setFormData({ ...formData, minReplicas: parseInt(e.target.value, 10) || 0 })}
                />
              </div>
              <div>
                <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Max Replicas</label>
                <input
                  type="number"
                  min="1"
                  max="64"
                  value={formData.maxReplicas}
                  onChange={(e) => setFormData({ ...formData, maxReplicas: parseInt(e.target.value, 10) || 1 })}
                />
              </div>
              <div>
                <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Target CPU %</label>
                <input
                  type="number"
                  min="10"
                  max="100"
                  value={formData.targetUtilization}
                  onChange={(e) => setFormData({ ...formData, targetUtilization: parseInt(e.target.value, 10) || 80 })}
                />
              </div>
            </div>
          </div>

          {error && <p className="text-warn">{error}</p>}

          {createMutation.isError && (
            <p className="login-error" role="alert">
              Error deploying service: {createMutation.error instanceof Error ? createMutation.error.message : 'Unknown error'}
            </p>
          )}

          <div className="row" style={{ justifyContent: 'flex-end' }}>
            <button type="button" className="btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="primary" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Deploying...' : 'Deploy Service'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
