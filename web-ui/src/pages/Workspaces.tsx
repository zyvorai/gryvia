import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Monitor, Cpu, Clock, Pause, Play, Plus, ExternalLink,
  RefreshCw, HardDrive, Activity, X,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { Workspace } from '@/lib/api'
import StatCard from '@/components/StatCard'
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
    <div className="text-center py-12">
      <p className="text-red-400">Failed to load workspaces. Please try again.</p>
    </div>
  )

  const activeWorkspaces = workspaces?.filter(w => w.status?.phase === 'Running') || []
  const idleWorkspaces = workspaces?.filter(w => w.status?.phase === 'Idle') || []
  const pausedWorkspaces = workspaces?.filter(w => w.status?.phase === 'Paused') || []
  const totalGPUs = workspaces?.reduce((sum, w) => sum + (w.spec?.gpuCount || 0), 0) || 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Workspaces</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">GPU-attached interactive environments</p>
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
            New Workspace
          </button>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={Monitor} title="Active Workspaces" value={activeWorkspaces.length} color="purple" />
        <StatCard icon={Cpu} title="Total GPUs Allocated" value={totalGPUs} color="blue" />
        <StatCard icon={Clock} title="Idle Workspaces" value={idleWorkspaces.length} subtitle={idleWorkspaces.length > 0 ? 'may auto-pause' : undefined} color="orange" />
        <StatCard icon={Pause} title="Paused" value={pausedWorkspaces.length} color="cyan" />
      </div>

      {/* Workspace Grid */}
      {isLoading ? (
        <LoadingSpinner />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
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
          {(!workspaces || workspaces.length === 0) && (
            <div className="col-span-full text-center py-12">
              <Monitor className="h-12 w-12 mx-auto mb-3 text-[#344e6a]" />
              <p className="text-sm text-[#5a7a9e]">No workspaces yet. Create one to get started.</p>
            </div>
          )}
        </div>
      )}

      {/* Create Workspace Modal */}
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

  const statusColors: Record<string, { dot: string; bg: string; text: string }> = {
    Running: { dot: 'bg-emerald-400', bg: 'rgba(34,197,94,0.08)', text: 'text-emerald-400' },
    Paused: { dot: 'bg-yellow-400', bg: 'rgba(251,191,36,0.08)', text: 'text-[#fbbf24]' },
    Idle: { dot: 'bg-cyan-400', bg: 'rgba(6,182,212,0.08)', text: 'text-cyan-400' },
    Pending: { dot: 'bg-[#fbbf24]', bg: 'rgba(251,191,36,0.08)', text: 'text-[#fbbf24]' },
    Failed: { dot: 'bg-red-400', bg: 'rgba(239,68,68,0.08)', text: 'text-red-400' },
  }
  const sc = statusColors[phase] || statusColors.Pending

  const typeIcons: Record<string, { label: string; color: string }> = {
    jupyter: { label: 'Jupyter', color: 'text-[#f37626]' },
    vscode: { label: 'VS Code', color: 'text-[#007acc]' },
  }
  const ti = typeIcons[wsType] || typeIcons.jupyter

  return (
    <div className="rounded-xl p-5 metal-card card-glow transition-all duration-300 hover:scale-[1.01]">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Monitor className={`h-4 w-4 ${ti.color}`} />
          <span className="text-xs font-medium px-2 py-0.5 rounded-full" style={{
            background: 'rgba(10,14,20,0.5)',
            border: '1px solid rgba(192,204,224,0.08)',
            color: ti.color === 'text-[#f37626]' ? '#f37626' : '#007acc',
          }}>
            {ti.label}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className={`w-2 h-2 rounded-full ${sc.dot}`} style={{
            boxShadow: `0 0 6px currentColor`,
          }} />
          <span className={`text-[10px] font-medium ${sc.text}`}>{phase}</span>
        </div>
      </div>

      <h3 className="text-sm font-semibold text-[#e8ecf1] truncate mb-2">{name}</h3>

      <div className="space-y-1.5 mb-4">
        <div className="flex items-center gap-2 text-xs text-[#8ba4c0]">
          <Cpu className="h-3 w-3 text-[#5a7a9e]" />
          <span>{workspace.spec?.gpuType || 'GPU'} x{workspace.spec?.gpuCount || 0}</span>
        </div>
        <div className="flex items-center gap-2 text-xs text-[#8ba4c0]">
          <HardDrive className="h-3 w-3 text-[#5a7a9e]" />
          <span>{workspace.spec?.storageSize || 'N/A'}</span>
        </div>
        <div className="flex items-center gap-2 text-xs text-[#8ba4c0]">
          <Clock className="h-3 w-3 text-[#5a7a9e]" />
          <span>Uptime: {workspace.status?.uptime || 'N/A'}</span>
        </div>
        {workspace.status?.lastActivity && (
          <div className="flex items-center gap-2 text-xs text-[#5a7a9e]">
            <Activity className="h-3 w-3" />
            <span>Last active: {new Date(workspace.status.lastActivity).toLocaleString()}</span>
          </div>
        )}
      </div>

      <div className="flex items-center gap-2 pt-3" style={{ borderTop: '1px solid rgba(192,204,224,0.06)' }}>
        {phase === 'Running' && workspace.status?.url && (
          <a
            href={workspace.status.url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 px-2.5 py-1.5 btn-copper rounded-lg text-xs font-medium"
          >
            <ExternalLink className="h-3 w-3" />
            Open
          </a>
        )}
        {phase === 'Running' || phase === 'Idle' ? (
          <button
            onClick={() => onPause(name)}
            disabled={isPausing}
            className="inline-flex items-center gap-1 px-2.5 py-1.5 btn-chrome rounded-lg text-xs font-medium disabled:opacity-50"
          >
            <Pause className="h-3 w-3" />
            Pause
          </button>
        ) : phase === 'Paused' ? (
          <button
            onClick={() => onResume(name)}
            disabled={isResuming}
            className="inline-flex items-center gap-1 px-2.5 py-1.5 btn-chrome rounded-lg text-xs font-medium disabled:opacity-50"
          >
            <Play className="h-3 w-3" />
            Resume
          </button>
        ) : null}
        <button
          onClick={() => {
            if (window.confirm(`Delete workspace "${name}"?`)) {
              onDelete(name)
            }
          }}
          className="ml-auto p-1.5 text-[#5a7a9e] hover:text-red-400 transition-colors rounded-lg"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
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
            <h3 className="text-lg font-bold text-gradient-copper">Create Workspace</h3>
            <button onClick={onClose} className="text-[#5a7a9e] hover:text-[#c0cce0] transition-colors">
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <div>
            <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Name</label>
            <input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
              style={inputStyle}
              placeholder="my-workspace"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Type</label>
              <select
                value={formData.type}
                onChange={(e) => setFormData({ ...formData, type: e.target.value })}
                className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
              >
                <option value="jupyter">Jupyter</option>
                <option value="vscode">VS Code</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">GPU Type</label>
              <select
                value={formData.gpuType}
                onChange={(e) => setFormData({ ...formData, gpuType: e.target.value })}
                className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
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
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">GPU Count</label>
              <input
                type="number"
                min="1"
                max="8"
                required
                value={formData.gpuCount}
                onChange={(e) => setFormData({ ...formData, gpuCount: parseInt(e.target.value, 10) || 1 })}
                className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Storage</label>
              <input
                type="text"
                required
                value={formData.storageSize}
                onChange={(e) => setFormData({ ...formData, storageSize: e.target.value })}
                className="block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
                placeholder="50Gi"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Idle Timeout</label>
              <input
                type="text"
                required
                value={formData.idleTimeout}
                onChange={(e) => setFormData({ ...formData, idleTimeout: e.target.value })}
                className="block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
                placeholder="30m"
              />
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
                Error creating workspace: {createMutation.error instanceof Error ? createMutation.error.message : 'Unknown error'}
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
              {createMutation.isPending ? 'Creating...' : 'Create Workspace'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
