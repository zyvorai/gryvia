import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import type { InferenceService } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
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

  const { data: services, isLoading, isError, refetch, isRefetching, dataUpdatedAt } = useQuery({
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
      <p className="warning" role="alert">Failed to load inference services. Please try again.</p>
    </>
  )

  const activeServices = services?.filter(s => s.status?.phase === 'Running' || s.status?.phase === 'Ready') || []
  const totalReplicas = services?.reduce((sum, s) => sum + (s.status?.readyReplicas || 0), 0) || 0
  const canaryDeployments = services?.filter(s => s.spec?.canary && (s.spec.canary.trafficPercent ?? 0) > 0) || []
  const avgLatency = services && services.length > 0
    ? Math.round(services.reduce((sum, s) => sum + (s.status?.latencyMs || 0), 0) / services.length)
    : 0

  return (
    <>
      <PageHero eyebrow="Inference" title="Serve models at scale." lede="Model serving management." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={`${activeServices.length} of ${services?.length || 0} inference services ready.`}
          tone={services && services.length > activeServices.length ? 'warn' : undefined}
          figures={[
            { label: 'Active services', value: activeServices.length },
            { label: 'Total replicas', value: totalReplicas },
            { label: 'Canary deployments', value: canaryDeployments.length },
            { label: 'Avg latency', value: avgLatency > 0 ? `${avgLatency}ms` : '-' },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">Serving</p>
          <h2 className="card-title">All Services</h2>
          <div className="toolbar">
            <button className="primary" onClick={() => setShowCreateForm(true)}>
              Deploy Model
            </button>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
            <span className="faint">{services?.length || 0} total</span>
          </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (services || []).length === 0 ? (
          <p className="empty-state">No inference services deployed</p>
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
      </div>

      {showCreateForm && (
        <CreateInferenceServiceModal onClose={() => setShowCreateForm(false)} />
      )}
    </>
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
        <h2 className="card-title">Deploy Inference Service</h2>

        <form onSubmit={handleSubmit} className="stack">
          <div className="formgrid">
            <label className="field">
              Service Name
              <input
                type="text"
                required
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="my-inference-svc"
              />
            </label>
            <label className="field">
              Model Reference
              <input
                type="text"
                required
                value={formData.modelRef}
                onChange={(e) => setFormData({ ...formData, modelRef: e.target.value })}
                placeholder="my-model" pattern="[a-z0-9]([\-a-z0-9]*[a-z0-9])?" title="Name of a model in the registry (lowercase letters, digits, hyphens)"
              />
            </label>
          </div>

          <div className="formgrid">
            <label className="field">
              Backend
              <select
                value={formData.backend}
                onChange={(e) => setFormData({ ...formData, backend: e.target.value })}
              >
                <option value="Triton">Triton</option>
                <option value="vLLM">vLLM</option>
                <option value="TensorRT-LLM">TensorRT-LLM</option>
                <option value="TorchServe">TorchServe</option>
              </select>
            </label>
            <label className="field">
              Replicas
              <input
                type="number"
                min="1"
                max="32"
                required
                value={formData.replicas}
                onChange={(e) => setFormData({ ...formData, replicas: parseInt(e.target.value, 10) || 1 })}
              />
            </label>
          </div>

          <div className="stack">
            <p className="eyebrow">Autoscaling</p>
            <div className="formgrid">
              <label className="field">
                Min Replicas
                <input
                  type="number"
                  min="0"
                  max="32"
                  value={formData.minReplicas}
                  onChange={(e) => setFormData({ ...formData, minReplicas: parseInt(e.target.value, 10) || 0 })}
                />
              </label>
              <label className="field">
                Max Replicas
                <input
                  type="number"
                  min="1"
                  max="64"
                  value={formData.maxReplicas}
                  onChange={(e) => setFormData({ ...formData, maxReplicas: parseInt(e.target.value, 10) || 1 })}
                />
              </label>
              <label className="field">
                Target CPU %
                <input
                  type="number"
                  min="10"
                  max="100"
                  value={formData.targetUtilization}
                  onChange={(e) => setFormData({ ...formData, targetUtilization: parseInt(e.target.value, 10) || 80 })}
                />
              </label>
            </div>
          </div>

          {error && <p className="warning" role="alert">{error}</p>}

          {createMutation.isError && (
            <p className="warning" role="alert">
              Error deploying service: {errorMessage(createMutation.error)}
            </p>
          )}

          <div className="toolbar">
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
