import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import type { AutoTunerJob, TunerTrial } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function AutoTuner() {
  const [showCreateForm, setShowCreateForm] = useState(false)

  const { data: tuners, isLoading, isError, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['tuners'],
    queryFn: api.getTuners,
    refetchInterval: 15000,
  })

  if (isError) return (
    <>
      <PageHero eyebrow="Auto Tuner" title="Tuners unavailable." tint="red" />
      <p className="warning" role="alert">Failed to load auto tuners. Please try again.</p>
    </>
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
    <>
      <PageHero eyebrow="Auto Tuner" title="Find the best hyperparameters." lede="Hyperparameter tuning." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={`${activeTuners.length} of ${tuners?.length || 0} tuners running ${trialsRunning} trials.`}
          figures={[
            { label: 'Active tuners', value: activeTuners.length },
            { label: 'Trials running', value: trialsRunning },
            { label: 'Best accuracy', value: bestAccuracy > 0 ? `${(bestAccuracy * 100).toFixed(1)}%` : '-' },
            { label: 'Trials completed', value: trialsCompleted },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">Tuning</p>
          <h2 className="card-title">All Tuners</h2>
          <div className="toolbar">
            <button className="primary" onClick={() => setShowCreateForm(true)}>
              New Tuner
            </button>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
            <span className="faint">{tuners?.length || 0} total</span>
          </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (tuners || []).length === 0 ? (
          <p className="empty-state">No tuning jobs yet</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Algorithm</th>
                  <th>Trials</th>
                  <th>Best Metric</th>
                  <th>Status</th>
                  <th>Details</th>
                </tr>
              </thead>
              <tbody>
                {(tuners || []).map((tuner) => (
                  <TunerRow key={tuner.metadata?.name} tuner={tuner} />
                ))}
              </tbody>
            </table>
          </div>
        )}
        </section>
      </div>

      {showCreateForm && (
        <CreateTunerModal onClose={() => setShowCreateForm(false)} />
      )}
    </>
  )
}

// --- Sub-components ---

const phaseTone: Record<string, string> = {
  Running: 'info',
  Completed: 'ok',
  Succeeded: 'ok',
  Failed: 'bad',
  Pending: 'warn',
}

function TunerRow({ tuner }: { tuner: AutoTunerJob }) {
  const [expanded, setExpanded] = useState(false)
  const name = tuner.metadata?.name || 'unknown'
  const phase = tuner.status?.phase || 'Pending'
  const algorithm = tuner.spec?.algorithm || 'Random'
  const trialsCompleted = tuner.status?.trialsCompleted || 0
  const maxTrials = tuner.spec?.maxTrials || 0

  return (
    <>
      <tr className="table-row-hover" onClick={() => setExpanded(!expanded)}>
        <td>
          <span className="faint" aria-hidden="true">{expanded ? '▾' : '▸'}</span>{' '}
          <b>{name}</b>
        </td>
        <td><span className="pill">{algorithm}</span></td>
        <td>
          <span className="muted">{trialsCompleted}/{maxTrials}</span>
          <div className="progress">
            <span style={{ width: maxTrials > 0 ? `${(trialsCompleted / maxTrials) * 100}%` : '0%' }} />
          </div>
        </td>
        <td>
          {tuner.status?.bestMetricValue != null ? (
            <b>{(tuner.status.bestMetricValue * 100).toFixed(2)}%</b>
          ) : (
            <span className="faint">-</span>
          )}
        </td>
        <td><span className={`pill ${phaseTone[phase] || ''}`}>{phase}</span></td>
        <td className="faint">{expanded ? 'Hide' : 'Show'}</td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan={6}>
            <TunerDetail tuner={tuner} />
          </td>
        </tr>
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

  const trialTone = (status?: string) =>
    status === 'Completed' || status === 'Succeeded' ? 'ok' : status === 'Running' ? 'info' : status === 'Failed' ? 'bad' : ''

  return (
    <div className="stack">
      {bestTrial && (
        <div>
          <div className="row">
            <b>Best Trial: <span className="mono">{bestTrial.trialId}</span></b>
            <b>{bestTrial.metricValue != null ? `${(bestTrial.metricValue * 100).toFixed(2)}%` : '-'}</b>
          </div>
          {bestTrial.parameters && Object.keys(bestTrial.parameters).length > 0 && (
            <div className="row">
              {Object.entries(bestTrial.parameters).map(([key, val]) => (
                <span key={key} className="pill mono">
                  {key}={String(val)}
                </span>
              ))}
            </div>
          )}
        </div>
      )}

      <div>
        <p className="eyebrow">Trial Results</p>
        {isLoading ? (
          <LoadingSpinner />
        ) : (trials || []).length === 0 ? (
          <p className="empty-state">No trials yet</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Trial ID</th>
                  <th>Parameters</th>
                  <th>Metric</th>
                  <th>Status</th>
                  <th>Duration</th>
                </tr>
              </thead>
              <tbody>
                {(trials || []).map((trial) => {
                  const isBest = bestTrial?.trialId === trial.trialId
                  return (
                    <tr key={trial.trialId}>
                      <td className="mono">
                        {trial.trialId}
                        {isBest && <> <span className="pill ok">best</span></>}
                      </td>
                      <td>
                        <div className="row">
                          {trial.parameters && Object.entries(trial.parameters).map(([k, v]) => (
                            <span key={k} className="pill mono">
                              {k}={String(v)}
                            </span>
                          ))}
                        </div>
                      </td>
                      <td>
                        <b>{trial.metricValue != null ? (trial.metricValue * 100).toFixed(2) + '%' : '-'}</b>
                      </td>
                      <td><span className={`pill ${trialTone(trial.status)}`}>{trial.status}</span></td>
                      <td className="faint">{trial.duration || '-'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
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
    direction: 'maximize' as 'maximize' | 'minimize',
    image: '',
    gpus: 1,
    maxEpochs: 10,
    maxTrials: 20,
    parameterSpace: '{\n  "learning_rate": {"type": "float", "min": 1e-5, "max": 1e-2},\n  "batch_size": {"type": "choice", "values": [16, 32, 64, 128]},\n  "dropout": {"type": "float", "min": 0.0, "max": 0.5}\n}',
  })
  const [error, setError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: (data: typeof formData) =>
      api.createTuner({
        name: data.name,
        algorithm: data.algorithm,
        objectiveMetric: data.objectiveMetric,
        direction: data.direction,
        maxTrials: data.maxTrials,
        parameterSpace: data.parameterSpace,
        jobTemplate: { type: 'training', image: data.image.trim(), gpus: data.gpus },
        ...(data.algorithm === 'ASHA' ? { ashaConfig: { maxEpochs: data.maxEpochs } } : {}),
      }),
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

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <h2 className="card-title">Create Tuner</h2>

        <form onSubmit={handleSubmit} className="stack">
          <label className="field">
            Name
            <input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="my-tuning-job"
            />
          </label>

          <div className="formgrid">
            <label className="field">
              Algorithm
              <select
                value={formData.algorithm}
                onChange={(e) => setFormData({ ...formData, algorithm: e.target.value })}
              >
                <option value="Grid">Grid Search</option>
                <option value="Random">Random Search</option>
                <option value="Bayesian">Bayesian Optimization</option>
                <option value="ASHA">ASHA</option>
              </select>
            </label>
            <label className="field">
              Objective Metric
              <input
                type="text"
                required
                value={formData.objectiveMetric}
                onChange={(e) => setFormData({ ...formData, objectiveMetric: e.target.value })}
                placeholder="accuracy"
              />
            </label>
          </div>

          <div className="formgrid">
            <label className="field">
              Goal
              <select
                value={formData.direction}
                onChange={(e) => setFormData({ ...formData, direction: e.target.value as 'maximize' | 'minimize' })}
              >
                <option value="maximize">Maximize the metric</option>
                <option value="minimize">Minimize the metric</option>
              </select>
            </label>
            {formData.algorithm === 'ASHA' && (
              <label className="field">
                Max Epochs (ASHA)
                <input
                  type="number"
                  min="1"
                  required
                  value={formData.maxEpochs}
                  onChange={(e) => setFormData({ ...formData, maxEpochs: parseInt(e.target.value, 10) || 1 })}
                />
              </label>
            )}
          </div>

          <div className="formgrid">
            <label className="field">
              Trial Image
              <input
                type="text"
                required
                value={formData.image}
                onChange={(e) => setFormData({ ...formData, image: e.target.value })}
                placeholder="registry.example.com/train:latest"
              />
            </label>
            <label className="field">
              GPUs per Trial
              <input
                type="number"
                min="0"
                max="1024"
                required
                value={formData.gpus}
                onChange={(e) => setFormData({ ...formData, gpus: parseInt(e.target.value, 10) || 0 })}
              />
            </label>
          </div>

          <label className="field">
            Max Trials
            <input
              type="number"
              min="1"
              max="1000"
              required
              value={formData.maxTrials}
              onChange={(e) => setFormData({ ...formData, maxTrials: parseInt(e.target.value, 10) || 1 })}
            />
          </label>

          <label className="field">
            Parameter Space (JSON)
            <textarea
              className="codeedit"
              required
              rows={6}
              value={formData.parameterSpace}
              onChange={(e) => setFormData({ ...formData, parameterSpace: e.target.value })}
            />
          </label>

          {error && <p className="warning" role="alert">{error}</p>}

          {createMutation.isError && (
            <p className="warning" role="alert">
              Error creating tuner: {errorMessage(createMutation.error)}
            </p>
          )}

          <div className="toolbar">
            <button type="button" className="btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="primary" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create Tuner'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
