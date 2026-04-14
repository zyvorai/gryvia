import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Settings, Activity, Trophy, Hash,
  RefreshCw, Plus, X, ChevronDown, ChevronRight, Loader2,
  CheckCircle, XCircle, Clock,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { AutoTunerJob, TunerTrial } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function AutoTuner() {
  const [showCreateForm, setShowCreateForm] = useState(false)

  const { data: tuners, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['tuners'],
    queryFn: api.getTuners,
    refetchInterval: 15000,
  })

  if (isError) return (
    <div className="text-center py-12">
      <p className="text-red-400">Failed to load auto tuners. Please try again.</p>
    </div>
  )

  const activeTuners = tuners?.filter(t => t.status?.phase === 'Running') || []
  const trialsRunning = tuners?.reduce((sum, t) =>
    sum + (t.status?.trialsRunning || 0), 0) || 0
  const bestAccuracy = tuners?.reduce((best, t) => {
    const val = t.status?.bestMetricValue ?? 0
    return val > best ? val : best
  }, 0) || 0
  const trialsCompleted = tuners?.reduce((sum, t) =>
    sum + (t.status?.trialsCompleted || 0), 0) || 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Auto Tuner</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">Hyperparameter tuning</p>
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
            New Tuner
          </button>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={Settings} title="Active Tuners" value={activeTuners.length} color="orange" />
        <StatCard icon={Activity} title="Trials Running" value={trialsRunning} color="blue" />
        <StatCard icon={Trophy} title="Best Accuracy" value={bestAccuracy > 0 ? `${(bestAccuracy * 100).toFixed(1)}%` : '-'} color="green" />
        <StatCard icon={Hash} title="Trials Completed" value={trialsCompleted} color="purple" />
      </div>

      {/* Tuners Table */}
      <div className="rounded-xl" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="px-5 py-4" style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
          <div className="flex items-center gap-2">
            <Settings className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">All Tuners</h3>
            <span className="text-xs text-[#5a7a9e] ml-auto">{tuners?.length || 0} total</span>
          </div>
        </div>
        <div className="p-4">
          {isLoading ? (
            <LoadingSpinner />
          ) : (
            <div className="space-y-1">
              {/* Table header */}
              <div className="grid grid-cols-12 gap-3 px-3 py-2 text-[10px] uppercase tracking-wider text-[#5a7a9e] font-medium">
                <div className="col-span-3">Name</div>
                <div className="col-span-2">Algorithm</div>
                <div className="col-span-2">Trials</div>
                <div className="col-span-2">Best Metric</div>
                <div className="col-span-2">Status</div>
                <div className="col-span-1">Details</div>
              </div>

              {(tuners || []).map((tuner) => (
                <TunerRow key={tuner.metadata?.name} tuner={tuner} />
              ))}

              {(!tuners || tuners.length === 0) && (
                <div className="text-center py-8 text-sm text-[#344e6a]">No tuning jobs yet</div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Create Modal */}
      {showCreateForm && (
        <CreateTunerModal onClose={() => setShowCreateForm(false)} />
      )}
    </div>
  )
}

// --- Sub-components ---

const algorithmBadgeConfig: Record<string, { bg: string; text: string; border: string }> = {
  Grid: { bg: 'rgba(95,168,211,0.08)', text: '#7ecbf5', border: 'rgba(95,168,211,0.2)' },
  Random: { bg: 'rgba(168,85,247,0.08)', text: '#c084fc', border: 'rgba(168,85,247,0.2)' },
  Bayesian: { bg: 'rgba(34,197,94,0.08)', text: '#4ade80', border: 'rgba(34,197,94,0.2)' },
  ASHA: { bg: 'rgba(212,118,78,0.08)', text: '#e8a87c', border: 'rgba(212,118,78,0.2)' },
}

function AlgorithmBadge({ algorithm }: { algorithm: string }) {
  const cfg = algorithmBadgeConfig[algorithm] || algorithmBadgeConfig.Random
  return (
    <span className="inline-flex items-center text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
      background: cfg.bg,
      color: cfg.text,
      border: `1px solid ${cfg.border}`,
    }}>
      {algorithm}
    </span>
  )
}

const phaseConfig: Record<string, { icon: React.ElementType; color: string; spinning?: boolean }> = {
  Running: { icon: Loader2, color: 'text-[#7ecbf5]', spinning: true },
  Completed: { icon: CheckCircle, color: 'text-emerald-400' },
  Succeeded: { icon: CheckCircle, color: 'text-emerald-400' },
  Failed: { icon: XCircle, color: 'text-red-400' },
  Pending: { icon: Clock, color: 'text-[#fbbf24]' },
}

function TunerRow({ tuner }: { tuner: AutoTunerJob }) {
  const [expanded, setExpanded] = useState(false)
  const name = tuner.metadata?.name || 'unknown'
  const phase = tuner.status?.phase || 'Pending'
  const algorithm = tuner.spec?.algorithm || 'Random'
  const trialsCompleted = tuner.status?.trialsCompleted || 0
  const maxTrials = tuner.spec?.maxTrials || 0

  const cfg = phaseConfig[phase] || phaseConfig.Pending
  const Icon = cfg.icon

  return (
    <>
      <div
        className="grid grid-cols-12 gap-3 px-3 py-3 rounded-lg table-row-hover cursor-pointer items-center"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="col-span-3 flex items-center gap-2">
          {expanded ? (
            <ChevronDown className="h-3 w-3 text-[#5a7a9e] flex-shrink-0" />
          ) : (
            <ChevronRight className="h-3 w-3 text-[#5a7a9e] flex-shrink-0" />
          )}
          <span className="text-sm font-medium text-[#c0cce0] truncate">{name}</span>
        </div>
        <div className="col-span-2">
          <AlgorithmBadge algorithm={algorithm} />
        </div>
        <div className="col-span-2">
          <div className="flex items-center gap-2">
            <span className="text-xs text-[#8ba4c0]">{trialsCompleted}/{maxTrials}</span>
            <div className="flex-1 h-1 rounded-full overflow-hidden max-w-[60px]" style={{
              background: 'rgba(10,14,20,0.6)',
              boxShadow: 'inset 0 1px 2px rgba(0,0,0,0.3)',
            }}>
              <div className="h-full rounded-full transition-all" style={{
                width: maxTrials > 0 ? `${(trialsCompleted / maxTrials) * 100}%` : '0%',
                background: 'linear-gradient(90deg, #344e6a, #5fa8d3)',
              }} />
            </div>
          </div>
        </div>
        <div className="col-span-2">
          {tuner.status?.bestMetricValue != null ? (
            <span className="text-xs font-medium text-emerald-400">
              {(tuner.status.bestMetricValue * 100).toFixed(2)}%
            </span>
          ) : (
            <span className="text-xs text-[#344e6a]">-</span>
          )}
        </div>
        <div className="col-span-2">
          <div className="flex items-center gap-1.5">
            <Icon className={`h-3 w-3 ${cfg.color} ${cfg.spinning ? 'animate-spin' : ''}`} />
            <span className={`text-xs font-medium ${cfg.color}`}>{phase}</span>
          </div>
        </div>
        <div className="col-span-1">
          <span className="text-[10px] text-[#5a7a9e]">
            {expanded ? 'Hide' : 'Show'}
          </span>
        </div>
      </div>

      {/* Expanded: Trials + Best trial */}
      {expanded && (
        <TunerDetail tuner={tuner} />
      )}
    </>
  )
}

function TunerDetail({ tuner }: { tuner: AutoTunerJob }) {
  const name = tuner.metadata?.name || 'unknown'

  const { data: trials, isLoading } = useQuery({
    queryKey: ['tunerTrials', name],
    queryFn: () => api.getTunerTrials(name),
    enabled: true,
  })

  const bestTrial = trials?.reduce<TunerTrial | null>((best, t) => {
    if (t.status === 'Failed') return best
    if (!best) return t
    return (t.metricValue ?? 0) > (best.metricValue ?? 0) ? t : best
  }, null) ?? null

  return (
    <div className="px-4 py-5 mb-1 rounded-lg animate-fade-in space-y-4" style={{
      background: 'rgba(10,14,20,0.4)',
      border: '1px solid rgba(192,204,224,0.04)',
    }}>
      {/* Best trial highlight */}
      {bestTrial && (
        <div className="p-4 rounded-lg" style={{
          background: 'rgba(34,197,94,0.04)',
          border: '1px solid rgba(34,197,94,0.12)',
        }}>
          <div className="flex items-center gap-2 mb-2">
            <Trophy className="h-4 w-4 text-emerald-400" />
            <span className="text-xs font-semibold text-emerald-400">Best Trial: {bestTrial.trialId}</span>
            <span className="ml-auto text-xs font-bold text-emerald-400">
              {bestTrial.metricValue != null ? `${(bestTrial.metricValue * 100).toFixed(2)}%` : '-'}
            </span>
          </div>
          {bestTrial.parameters && Object.keys(bestTrial.parameters).length > 0 && (
            <div className="flex flex-wrap gap-2">
              {Object.entries(bestTrial.parameters).map(([key, val]) => (
                <span key={key} className="text-[10px] font-mono px-2 py-0.5 rounded" style={{
                  background: 'rgba(34,197,94,0.08)',
                  color: '#4ade80',
                  border: '1px solid rgba(34,197,94,0.15)',
                }}>
                  {key}={String(val)}
                </span>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Trials table */}
      <div>
        <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-2 font-medium">
          Trial Results
        </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (
          <div className="space-y-0.5">
            <div className="grid grid-cols-12 gap-2 px-3 py-1.5 text-[10px] uppercase tracking-wider text-[#5a7a9e] font-medium">
              <div className="col-span-2">Trial ID</div>
              <div className="col-span-4">Parameters</div>
              <div className="col-span-2">Metric</div>
              <div className="col-span-2">Status</div>
              <div className="col-span-2">Duration</div>
            </div>
            {(trials || []).map((trial) => {
              const isBest = bestTrial?.trialId === trial.trialId
              return (
                <div key={trial.trialId} className={`grid grid-cols-12 gap-2 px-3 py-2 rounded items-center ${isBest ? '' : ''}`}
                  style={isBest ? {
                    background: 'rgba(34,197,94,0.04)',
                    border: '1px solid rgba(34,197,94,0.08)',
                  } : {
                    background: 'rgba(10,14,20,0.3)',
                    border: '1px solid transparent',
                  }}
                >
                  <div className="col-span-2 text-xs text-[#c0cce0] font-mono">
                    {isBest && <Trophy className="inline h-3 w-3 text-emerald-400 mr-1" />}
                    {trial.trialId}
                  </div>
                  <div className="col-span-4 flex flex-wrap gap-1">
                    {trial.parameters && Object.entries(trial.parameters).map(([k, v]) => (
                      <span key={k} className="text-[10px] font-mono px-1.5 py-0.5 rounded" style={{
                        background: 'rgba(10,14,20,0.5)',
                        color: '#8ba4c0',
                      }}>
                        {k}={String(v)}
                      </span>
                    ))}
                  </div>
                  <div className="col-span-2 text-xs font-medium text-[#c0cce0]">
                    {trial.metricValue != null ? (trial.metricValue * 100).toFixed(2) + '%' : '-'}
                  </div>
                  <div className="col-span-2">
                    <span className={`text-xs font-medium ${
                      trial.status === 'Completed' || trial.status === 'Succeeded' ? 'text-emerald-400' :
                      trial.status === 'Running' ? 'text-[#7ecbf5]' :
                      trial.status === 'Failed' ? 'text-red-400' : 'text-[#5a7a9e]'
                    }`}>
                      {trial.status}
                    </span>
                  </div>
                  <div className="col-span-2 text-xs text-[#5a7a9e]">{trial.duration || '-'}</div>
                </div>
              )
            })}
            {(!trials || trials.length === 0) && (
              <div className="text-center py-4 text-xs text-[#344e6a]">No trials yet</div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function CreateTunerModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [formData, setFormData] = useState({
    name: '',
    algorithm: 'Bayesian',
    objectiveMetric: 'accuracy',
    maxTrials: 20,
    parameterSpace: '{\n  "learning_rate": {"type": "float", "min": 1e-5, "max": 1e-2},\n  "batch_size": {"type": "choice", "values": [16, 32, 64, 128]},\n  "dropout": {"type": "float", "min": 0.0, "max": 0.5}\n}',
  })
  const [error, setError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: (data: typeof formData) => api.createTuner(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tuners'] })
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

    try {
      JSON.parse(formData.parameterSpace)
    } catch {
      setError('Parameter space must be valid JSON')
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
            <h3 className="text-lg font-bold text-gradient-copper">Create Tuner</h3>
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
              placeholder="my-tuning-job"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Algorithm</label>
              <select
                value={formData.algorithm}
                onChange={(e) => setFormData({ ...formData, algorithm: e.target.value })}
                className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
              >
                <option value="Grid">Grid Search</option>
                <option value="Random">Random Search</option>
                <option value="Bayesian">Bayesian Optimization</option>
                <option value="ASHA">ASHA</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Objective Metric</label>
              <input
                type="text"
                required
                value={formData.objectiveMetric}
                onChange={(e) => setFormData({ ...formData, objectiveMetric: e.target.value })}
                className="block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
                style={inputStyle}
                placeholder="accuracy"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Max Trials</label>
            <input
              type="number"
              min="1"
              max="1000"
              required
              value={formData.maxTrials}
              onChange={(e) => setFormData({ ...formData, maxTrials: parseInt(e.target.value, 10) || 1 })}
              className="block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
              style={inputStyle}
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[#8ba4c0] mb-1">Parameter Space (JSON)</label>
            <textarea
              required
              rows={6}
              value={formData.parameterSpace}
              onChange={(e) => setFormData({ ...formData, parameterSpace: e.target.value })}
              className="block w-full rounded-lg text-sm font-mono placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:outline-none"
              style={inputStyle}
            />
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
                Error creating tuner: {createMutation.error instanceof Error ? createMutation.error.message : 'Unknown error'}
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
              {createMutation.isPending ? 'Creating...' : 'Create Tuner'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
