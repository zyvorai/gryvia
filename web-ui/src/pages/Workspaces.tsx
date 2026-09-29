import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import type { Workspace } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Workspaces() {
  const queryClient = useQueryClient()
  const [showCreateForm, setShowCreateForm] = useState(false)

  const { data: workspaces, isLoading, isError, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['workspaces'],
    queryFn: api.getWorkspaces,
    refetchInterval: 15000,
  })

  const pauseMutation = useMutation({
    mutationFn: (name: string) => api.pauseWorkspace(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
  })

  const resumeMutation = useMutation({
    mutationFn: (name: string) => api.resumeWorkspace(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.deleteWorkspace(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
  })

  if (isError) return (
    <>
      <PageHero eyebrow="Workspaces" title="Workspaces unavailable." tint="red" />
      <p className="warning" role="alert">Failed to load workspaces. Please try again.</p>
    </>
  )

  const activeWorkspaces = workspaces?.filter(w => w.status?.phase === 'Running') || []
  const idleWorkspaces = workspaces?.filter(w => w.status?.phase === 'Idle') || []
  const pausedWorkspaces = workspaces?.filter(w => w.status?.phase === 'Paused') || []
  const totalGPUs = workspaces?.reduce((sum, w) => sum + (w.spec?.gpuCount || 0), 0) || 0

  return (
    <>
      <PageHero eyebrow="Workspaces" title="Interactive GPU environments." lede="GPU-attached Jupyter and VS Code workspaces." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={`${activeWorkspaces.length} of ${workspaces?.length || 0} workspaces running on ${totalGPUs} GPUs.`}
          figures={[
            { label: 'Active workspaces', value: activeWorkspaces.length },
            { label: 'GPUs allocated', value: totalGPUs },
            { label: idleWorkspaces.length > 0 ? 'Idle · may auto-pause' : 'Idle', value: idleWorkspaces.length, tone: countTone(idleWorkspaces.length) },
            { label: 'Paused', value: pausedWorkspaces.length },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">Actions</p>
          <h2 className="card-title">Workspaces</h2>
          <div className="toolbar">
            <button className="primary" onClick={() => setShowCreateForm(true)}>
              New Workspace
            </button>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
          </div>
        </section>

        {isLoading ? (
          <section className="card span3"><LoadingSpinner /></section>
        ) : (workspaces || []).length === 0 ? (
          <section className="card span3">
            <p className="empty-state">No workspaces yet. Create one to get started.</p>
          </section>
        ) : (
          (workspaces || []).map((ws) => (
            <WorkspaceCard
              key={ws.metadata?.name}
              workspace={ws}
              onPause={(name) => pauseMutation.mutate(name)}
              onResume={(name) => resumeMutation.mutate(name)}
              onDelete={(name) => deleteMutation.mutate(name)}
              isPausing={pauseMutation.isPending}
              isResuming={resumeMutation.isPending}
            />
          ))
        )}
      </div>

      {showCreateForm && (
        <CreateWorkspaceModal onClose={() => setShowCreateForm(false)} />
      )}
    </>
  )
}

// --- Sub-components ---

function WorkspaceCard({ workspace, onPause, onResume, onDelete, isPausing, isResuming }: {
  workspace: Workspace
  onPause: (name: string) => void
  onResume: (name: string) => void
  onDelete: (name: string) => void
  isPausing: boolean
  isResuming: boolean
}) {
  const name = workspace.metadata?.name || 'unknown'
  const phase = workspace.status?.phase || 'Unknown'
  const wsType = workspace.spec?.type || 'jupyter'

  const tone: Record<string, string> = { Running: 'ok', Paused: 'warn', Idle: 'info', Pending: 'warn', Failed: 'bad' }
  const label: Record<string, string> = { jupyter: 'Jupyter', vscode: 'VS Code' }

  return (
    <section className="card">
      <p className="eyebrow">{label[wsType] || label.jupyter}</p>
      <h2 className="card-title">{name}</h2>
      <p><span className={`pill ${tone[phase] || ''}`}>{phase}</span></p>

      <div className="stack">
        <span className="muted">{workspace.spec?.gpuType || 'GPU'} x{workspace.spec?.gpuCount || 0}</span>
        <span className="muted">Storage: {workspace.spec?.storageSize || 'N/A'}</span>
        <span className="muted">Uptime: {workspace.status?.uptime || 'N/A'}</span>
        {workspace.status?.lastActivity && (
          <span className="faint">Last active: {new Date(workspace.status.lastActivity).toLocaleString()}</span>
        )}
      </div>

      <div className="toolbar">
        {phase === 'Running' && workspace.status?.url && (
          <a
            href={workspace.status.url}
            target="_blank"
            rel="noopener noreferrer"
            className="buttonlike btn-secondary"
          >
            Open
          </a>
        )}
        {phase === 'Running' || phase === 'Idle' ? (
          <button className="btn-secondary" onClick={() => onPause(name)} disabled={isPausing}>
            Pause
          </button>
        ) : phase === 'Paused' ? (
          <button className="btn-secondary" onClick={() => onResume(name)} disabled={isResuming}>
            Resume
          </button>
        ) : null}
        <button
          className="danger"
          onClick={() => {
            if (window.confirm(`Delete workspace "${name}"?`)) {
              onDelete(name)
            }
          }}
        >
          Delete
        </button>
      </div>
    </section>
  )
}

function CreateWorkspaceModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [formData, setFormData] = useState({
    name: '',
    type: 'jupyter',
    gpuCount: 1,
    gpuType: 'A100-80G',
    storageSize: '50Gi',
    idleTimeout: '30m',
  })
  const [error, setError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: (data: typeof formData) => api.createWorkspace(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workspaces'] })
      onClose()
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const k8sNameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
    if (!k8sNameRegex.test(formData.name)) {
      setError('Name must consist of lowercase alphanumeric characters or hyphens, and must start and end with an alphanumeric character')
      return
    }

    createMutation.mutate(formData)
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <h2 className="card-title">Create Workspace</h2>

        <form onSubmit={handleSubmit} className="stack">
          <label className="field">
            Name
            <input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="my-workspace"
            />
          </label>

          <div className="formgrid">
            <label className="field">
              Type
              <select
                value={formData.type}
                onChange={(e) => setFormData({ ...formData, type: e.target.value })}
              >
                <option value="jupyter">Jupyter</option>
                <option value="vscode">VS Code</option>
              </select>
            </label>
            <label className="field">
              GPU Type
              <select
                value={formData.gpuType}
                onChange={(e) => setFormData({ ...formData, gpuType: e.target.value })}
              >
                <option value="H100">H100</option>
                <option value="A100-80G">A100-80G</option>
                <option value="A100-40G">A100-40G</option>
                <option value="L40">L40</option>
                <option value="V100">V100</option>
                <option value="T4">T4</option>
              </select>
            </label>
          </div>

          <div className="formgrid">
            <label className="field">
              GPU Count
              <input
                type="number"
                min="1"
                max="8"
                required
                value={formData.gpuCount}
                onChange={(e) => setFormData({ ...formData, gpuCount: parseInt(e.target.value, 10) || 1 })}
              />
            </label>
            <label className="field">
              Storage
              <input
                type="text"
                required
                value={formData.storageSize}
                onChange={(e) => setFormData({ ...formData, storageSize: e.target.value })}
                placeholder="50Gi"
              />
            </label>
            <label className="field">
              Idle Timeout
              <input
                type="text"
                required
                value={formData.idleTimeout}
                onChange={(e) => setFormData({ ...formData, idleTimeout: e.target.value })}
                placeholder="30m"
              />
            </label>
          </div>

          {error && <p className="warning" role="alert">{error}</p>}

          {createMutation.isError && (
            <p className="warning" role="alert">
              Error creating workspace: {errorMessage(createMutation.error)}
            </p>
          )}

          <div className="toolbar">
            <button type="button" className="btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="primary" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create Workspace'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
