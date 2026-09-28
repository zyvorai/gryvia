import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { api } from '@/lib/api'
import type { Workspace } from '@/lib/api'
import PageHero from '@/components/PageHero'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Workspaces() {
  const queryClient = useQueryClient()
  const [showCreateForm, setShowCreateForm] = useState(false)

  const { data: workspaces, isLoading, isError, refetch, isRefetching } = useQuery({
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
      <p className="login-error" role="alert">Failed to load workspaces. Please try again.</p>
    </>
  )

  const activeWorkspaces = workspaces?.filter(w => w.status?.phase === 'Running') || []
  const idleWorkspaces = workspaces?.filter(w => w.status?.phase === 'Idle') || []
  const pausedWorkspaces = workspaces?.filter(w => w.status?.phase === 'Paused') || []
  const totalGPUs = workspaces?.reduce((sum, w) => sum + (w.spec?.gpuCount || 0), 0) || 0

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Workspaces" title="Interactive GPU environments." lede="GPU-attached Jupyter and VS Code workspaces." />

      <div className="page-actions">
        <button className="primary" onClick={() => setShowCreateForm(true)}>
          New Workspace
        </button>
        <button className="btn-secondary" onClick={() => refetch()} disabled={isRefetching}>
          Refresh
        </button>
      </div>

      <div className="apple-metric-band">
        <div><span>Active Workspaces</span><b>{activeWorkspaces.length}</b></div>
        <div><span>Total GPUs Allocated</span><b>{totalGPUs}</b></div>
        <div><span>Idle Workspaces{idleWorkspaces.length > 0 ? ' · may auto-pause' : ''}</span><b>{idleWorkspaces.length}</b></div>
        <div><span>Paused</span><b>{pausedWorkspaces.length}</b></div>
      </div>

      {isLoading ? (
        <LoadingSpinner />
      ) : (workspaces || []).length === 0 ? (
        <div className="list-empty">No workspaces yet. Create one to get started.</div>
      ) : (
        <div className="card-grid">
          {(workspaces || []).map((ws) => (
            <WorkspaceCard
              key={ws.metadata?.name}
              workspace={ws}
              onPause={(name) => pauseMutation.mutate(name)}
              onResume={(name) => resumeMutation.mutate(name)}
              onDelete={(name) => deleteMutation.mutate(name)}
              isPausing={pauseMutation.isPending}
              isResuming={resumeMutation.isPending}
            />
          ))}
        </div>
      )}

      {showCreateForm && (
        <CreateWorkspaceModal onClose={() => setShowCreateForm(false)} />
      )}
    </div>
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
      <div className="stat-head">
        <span className="pill">{label[wsType] || label.jupyter}</span>
        <span className={`pill ${tone[phase] || ''}`}>{phase}</span>
      </div>

      <h3 style={{ margin: '12px 0 8px' }}>{name}</h3>

      <div className="stack" style={{ gap: 4, marginBottom: 16 }}>
        <span className="muted">{workspace.spec?.gpuType || 'GPU'} x{workspace.spec?.gpuCount || 0}</span>
        <span className="muted">Storage: {workspace.spec?.storageSize || 'N/A'}</span>
        <span className="muted">Uptime: {workspace.status?.uptime || 'N/A'}</span>
        {workspace.status?.lastActivity && (
          <span className="faint">Last active: {new Date(workspace.status.lastActivity).toLocaleString()}</span>
        )}
      </div>

      <div className="row">
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
          style={{ marginLeft: 'auto' }}
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
        <div className="stat-head" style={{ marginBottom: 16 }}>
          <h2 style={{ margin: 0 }}>Create Workspace</h2>
          <button type="button" className="icon-button" onClick={onClose} aria-label="Close">
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="stack" style={{ gap: 16 }}>
          <div>
            <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Name</label>
            <input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="my-workspace"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Type</label>
              <select
                value={formData.type}
                onChange={(e) => setFormData({ ...formData, type: e.target.value })}
              >
                <option value="jupyter">Jupyter</option>
                <option value="vscode">VS Code</option>
              </select>
            </div>
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>GPU Type</label>
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
            </div>
          </div>

          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>GPU Count</label>
              <input
                type="number"
                min="1"
                max="8"
                required
                value={formData.gpuCount}
                onChange={(e) => setFormData({ ...formData, gpuCount: parseInt(e.target.value, 10) || 1 })}
              />
            </div>
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Storage</label>
              <input
                type="text"
                required
                value={formData.storageSize}
                onChange={(e) => setFormData({ ...formData, storageSize: e.target.value })}
                placeholder="50Gi"
              />
            </div>
            <div>
              <label className="faint" style={{ display: 'block', fontSize: 12, marginBottom: 4 }}>Idle Timeout</label>
              <input
                type="text"
                required
                value={formData.idleTimeout}
                onChange={(e) => setFormData({ ...formData, idleTimeout: e.target.value })}
                placeholder="30m"
              />
            </div>
          </div>

          {error && <p className="text-warn">{error}</p>}

          {createMutation.isError && (
            <p className="login-error" role="alert">
              Error creating workspace: {createMutation.error instanceof Error ? createMutation.error.message : 'Unknown error'}
            </p>
          )}

          <div className="row" style={{ justifyContent: 'flex-end' }}>
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
