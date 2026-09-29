import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { AutoTunerJob } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import { formatDate, formatRelative } from '@/lib/format'
import { phaseTone } from '@/lib/phase'
import { notify } from '@/lib/notify'
import { intError, nameError, parseIntStrict } from '@/lib/forms'
import { bestAcross, formatMetric, metricNameOf, validateParameterSpace } from '@/lib/tuner'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import Modal from '@/components/Modal'
import Progress from '@/components/Progress'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { TableCaption } from '@/components/TableCaption'

/** The gateway now also returns spec.direction and spec.metricName; not in the shared type yet. */
type Tuner = AutoTunerJob & { spec?: { direction?: 'maximize' | 'minimize'; metricName?: string } }

const ALGORITHM_LABELS: Record<string, string> = { grid: 'Grid', random: 'Random', bayesian: 'Bayesian', asha: 'ASHA' }
const algorithmLabel = (a?: string) => (a ? (ALGORITHM_LABELS[a.toLowerCase()] ?? a) : '—')

export default function AutoTuner() {
  useDocumentTitle('Auto Tuner')
  const [showCreateForm, setShowCreateForm] = useState(false)

  const { data, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['tuners'],
    queryFn: api.getTuners,
    refetchInterval: 15000,
  })
  const tuners = data as Tuner[] | undefined

  if (isError && !tuners) {
    return (
      <>
        <PageHero eyebrow="Auto Tuner" title="Tuners unavailable." tint="red" />
        <ErrorState title="Could not load auto tuners." error={error} onRetry={() => refetch()} retrying={isRefetching} />
      </>
    )
  }

  const active = tuners?.filter((t) => t.status?.phase === 'Running').length
  const trialsRunning = tuners?.reduce((sum, t) => sum + (t.status?.trialsRunning ?? 0), 0)
  const trialsCompleted = tuners?.reduce((sum, t) => sum + (t.status?.trialsCompleted ?? 0), 0)
  const best = bestAcross(tuners)
  const bestLabel = best.mixed ? 'Best metric' : best.metricName ? `Best ${best.metricName}` : 'Best metric'
  const bestValue = !tuners ? undefined : best.mixed ? 'Varies by tuner' : formatMetric(best.value, best.metricName)

  return (
    <>
      <PageHero eyebrow="Auto Tuner" title="Find the best hyperparameters." lede="Hyperparameter tuning." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={tuners ? `${active} of ${tuners.length} tuners running ${trialsRunning} trials.` : undefined}
          figures={[
            { label: 'Active tuners', value: active },
            { label: 'Trials running', value: trialsRunning },
            { label: bestLabel, value: bestValue },
            { label: 'Trials completed', value: trialsCompleted },
          ]}
        />

        {isError && (
          <div className="span3">
            <ErrorState title="Showing the last data received; refreshing failed." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        )}

        <section className="card span3">
          <p className="eyebrow">Tuning</p>
          <h2 className="card-title">All tuners</h2>
          <div className="toolbar">
            <button className="primary" onClick={() => setShowCreateForm(true)}>
              New tuner
            </button>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
            {tuners && <span className="faint">{tuners.length} total</span>}
          </div>
          {isLoading ? (
            <Skeleton rows={4} />
          ) : (tuners ?? []).length === 0 ? (
            <EmptyState
              title="No tuning jobs yet."
              action={
                <button className="primary" onClick={() => setShowCreateForm(true)}>
                  Create your first tuner
                </button>
              }
            >
              A tuner runs many trials of your training image with different hyperparameters and tracks the best one.
            </EmptyState>
          ) : (
            <div className="table-wrap">
              <table>
                <TableCaption>Auto tuners</TableCaption>
                <thead>
                  <tr>
                    <th scope="col">Name</th>
                    <th scope="col">Algorithm</th>
                    <th scope="col">Trials</th>
                    <th scope="col" className="num">Best metric</th>
                    <th scope="col">Status</th>
                    <th scope="col">Details</th>
                  </tr>
                </thead>
                <tbody>
                  {(tuners ?? []).map((tuner) => (
                    <TunerRow key={tuner.metadata?.name} tuner={tuner} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>

      {showCreateForm && <CreateTunerModal onClose={() => setShowCreateForm(false)} />}
    </>
  )
}

// --- Sub-components ---

function TunerRow({ tuner }: { tuner: Tuner }) {
  const [expanded, setExpanded] = useState(false)
  const name = tuner.metadata?.name || 'unknown'
  const phase = tuner.status?.phase || 'Pending'
  const trialsCompleted = tuner.status?.trialsCompleted ?? 0
  const maxTrials = tuner.spec?.maxTrials ?? 0
  const metricName = metricNameOf(tuner)
  const bestValue = tuner.status?.bestMetricValue
  const toggle = () => setExpanded((v) => !v)

  return (
    <>
      <tr
        className="table-row-hover"
        data-clickable
        tabIndex={0}
        aria-expanded={expanded}
        onClick={toggle}
        onKeyDown={(e) => {
          if (e.target !== e.currentTarget) return
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            toggle()
          }
        }}
      >
        <td>
          <span className="faint" aria-hidden="true">{expanded ? '▾' : '▸'}</span>{' '}
          <b>{name}</b>
        </td>
        <td><span className="pill">{algorithmLabel(tuner.spec?.algorithm)}</span></td>
        <td>
          <span className="muted">{trialsCompleted}/{maxTrials}</span>
          <Progress value={trialsCompleted} max={maxTrials} label={`${name}: ${trialsCompleted} of ${maxTrials} trials complete`} />
        </td>
        <td className="num">
          {bestValue !== undefined && bestValue !== null ? (
            <>
              <b>{formatMetric(bestValue, metricName)}</b>
              {metricName && <div className="faint">{metricName}{tuner.spec?.direction ? ` (${tuner.spec.direction === 'minimize' ? 'lower' : 'higher'} is better)` : ''}</div>}
            </>
          ) : (
            <span className="faint">—</span>
          )}
        </td>
        <td><span className={`pill ${phaseTone(phase)}`}>{phase}</span></td>
        <td>
          <button
            type="button"
            className="btn-secondary"
            aria-expanded={expanded}
            onClick={(e) => {
              e.stopPropagation()
              toggle()
            }}
          >
            {expanded ? 'Hide trials' : 'Show trials'}
          </button>
        </td>
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

function TunerDetail({ tuner }: { tuner: Tuner }) {
  const name = tuner.metadata?.name || 'unknown'
  const metricName = metricNameOf(tuner)

  const { data: trials, isLoading, isError, error, refetch, isRefetching } = useQuery({
    queryKey: ['tunerTrials', name],
    queryFn: () => api.getTunerTrials(name),
    // Poll while anything is still running so the table does not go stale.
    refetchInterval: (query) => (query.state.data?.some((t) => t.status === 'Running') ? 5000 : false),
  })

  // The best trial is decided by the operator (which knows the direction); never recompute it here.
  const bestId = tuner.status?.bestTrialId
  const bestTrial = bestId ? trials?.find((t) => t.trialId === bestId) : undefined
  const bestValue = tuner.status?.bestMetricValue ?? bestTrial?.metricValue

  return (
    <div className="stack">
      {bestId && (
        <div>
          <div className="row">
            <b>
              Best trial: <span className="mono">{bestId}</span>
            </b>
            <b>{formatMetric(bestValue, metricName)}</b>
          </div>
          {bestTrial?.parameters && Object.keys(bestTrial.parameters).length > 0 && (
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
        <p className="eyebrow">Trial results</p>
        {isLoading ? (
          <Skeleton rows={3} />
        ) : isError ? (
          <ErrorState title="Could not load trials." error={error} onRetry={() => refetch()} retrying={isRefetching} />
        ) : (trials ?? []).length === 0 ? (
          <EmptyState title="No trials yet.">Trials appear here once the tuner starts running them.</EmptyState>
        ) : (
          <div className="table-wrap">
            <table>
              <TableCaption>{`Trials for ${name}`}</TableCaption>
              <thead>
                <tr>
                  <th scope="col">Trial ID</th>
                  <th scope="col">Parameters</th>
                  <th scope="col" className="num">{metricName ?? 'Metric'}</th>
                  <th scope="col">Status</th>
                  <th scope="col">Duration</th>
                </tr>
              </thead>
              <tbody>
                {(trials ?? []).map((trial) => (
                  <tr key={trial.trialId}>
                    <td className="mono">
                      {trial.trialId}
                      {trial.trialId === bestId && (
                        <>
                          {' '}
                          <span className="pill ok">best</span>
                        </>
                      )}
                    </td>
                    <td>
                      <div className="row">
                        {trial.parameters &&
                          Object.entries(trial.parameters).map(([k, v]) => (
                            <span key={k} className="pill mono">
                              {k}={String(v)}
                            </span>
                          ))}
                      </div>
                    </td>
                    <td className="num">
                      <b>{formatMetric(trial.metricValue, metricName)}</b>
                    </td>
                    <td><span className={`pill ${phaseTone(trial.status)}`}>{trial.status ?? '—'}</span></td>
                    <td className="faint">{trial.duration ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {tuner.metadata?.creationTimestamp && (
          <p className="faint" title={formatDate(tuner.metadata.creationTimestamp)}>
            Created {formatRelative(tuner.metadata.creationTimestamp)}
          </p>
        )}
      </div>
    </div>
  )
}

const INITIAL = {
  name: '',
  algorithm: 'Bayesian',
  objectiveMetric: 'accuracy',
  direction: 'maximize' as 'maximize' | 'minimize',
  image: '',
  gpus: '1',
  maxTrials: '20',
  parallelism: '0',
  maxEpochs: '10',
  reductionFactor: '',
  minResource: '',
  parameterSpace: '{\n  "learning_rate": {"type": "float", "min": 1e-5, "max": 1e-2},\n  "batch_size": {"type": "choice", "values": [16, 32, 64, 128]},\n  "dropout": {"type": "float", "min": 0.0, "max": 0.5}\n}',
}

function CreateTunerModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [form, setForm] = useState(INITIAL)
  const [touched, setTouched] = useState(false)
  const set = (patch: Partial<typeof INITIAL>) => setForm((f) => ({ ...f, ...patch }))
  const dirty = JSON.stringify(form) !== JSON.stringify(INITIAL)
  const isAsha = form.algorithm === 'ASHA'

  const createMutation = useMutation({
    mutationFn: (body: Parameters<typeof api.createTuner>[0]) => api.createTuner(body),
    onSuccess: (_d, vars) => {
      notify.success(`Created tuner ${vars.name}`)
      queryClient.invalidateQueries({ queryKey: ['tuners'] })
      onClose()
    },
    onError: (err) => notify.error('Could not create tuner', err),
  })

  const optionalInt = (raw: string, min: number, max: number) => (raw.trim() === '' ? null : intError(raw, min, max))
  const errors = {
    name: nameError(form.name),
    objective: /^[A-Za-z0-9_./-]+$/.test(form.objectiveMetric) && form.objectiveMetric.length <= 128 ? null : 'Use letters, digits and _ . / - only.',
    image: form.image.trim() === '' ? 'Enter the container image each trial runs.' : /\s/.test(form.image.trim()) ? 'The image cannot contain spaces.' : null,
    gpus: intError(form.gpus, 0, 1024),
    maxTrials: intError(form.maxTrials, 1, 10000),
    parallelism: intError(form.parallelism, 0, 256),
    maxEpochs: isAsha ? intError(form.maxEpochs, 1, 100000) : null,
    reductionFactor: isAsha ? optionalInt(form.reductionFactor, 2, 100) : null,
    minResource: isAsha ? optionalInt(form.minResource, 1, 100000) : null,
    space: validateParameterSpace(form.parameterSpace),
  }
  const invalid = Object.values(errors).some(Boolean)
  const show = (e: string | null) => (touched ? e : null)
  const nameShown = touched || form.name !== '' ? errors.name : null
  const spaceShown = form.parameterSpace !== INITIAL.parameterSpace || touched ? errors.space : null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setTouched(true)
    if (invalid) return
    const int = (raw: string) => parseIntStrict(raw) as number
    const body = {
      name: form.name,
      algorithm: form.algorithm,
      objectiveMetric: form.objectiveMetric,
      direction: form.direction,
      maxTrials: int(form.maxTrials),
      parallelism: int(form.parallelism),
      parameterSpace: form.parameterSpace,
      jobTemplate: { type: 'training' as const, image: form.image.trim(), gpus: int(form.gpus) },
      ...(isAsha
        ? {
            ashaConfig: {
              maxEpochs: int(form.maxEpochs),
              ...(form.reductionFactor.trim() ? { reductionFactor: int(form.reductionFactor) } : {}),
              ...(form.minResource.trim() ? { minResource: int(form.minResource) } : {}),
            },
          }
        : {}),
    }
    createMutation.mutate(body)
  }

  const num = (label: string, key: keyof typeof INITIAL, err: string | null, hint?: string, min = 0, max?: number, required = true) => (
    <label className="field">
      {label}
      <input type="number" min={min} max={max} required={false} value={form[key]} onChange={(e) => set({ [key]: e.target.value })} aria-invalid={!!show(err)} aria-required={required} />
      {show(err) ? <span className="warning">{err}</span> : hint && <span className="faint">{hint}</span>}
    </label>
  )

  return (
    <Modal title="Create tuner" onClose={onClose} dirty={dirty}>
      <form onSubmit={handleSubmit} className="stack" noValidate>
        <label className="field">
          Name
          <input type="text" data-autofocus value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder="my-tuning-job" aria-invalid={!!nameShown} autoComplete="off" />
          {nameShown && <span className="warning">{nameShown}</span>}
        </label>

        <div className="formgrid">
          <label className="field">
            Algorithm
            <select value={form.algorithm} onChange={(e) => set({ algorithm: e.target.value })}>
              <option value="Grid">Grid search</option>
              <option value="Random">Random search</option>
              <option value="Bayesian">Bayesian optimization</option>
              <option value="ASHA">ASHA</option>
            </select>
          </label>
          <label className="field">
            Objective metric
            <input type="text" value={form.objectiveMetric} onChange={(e) => set({ objectiveMetric: e.target.value })} placeholder="accuracy" aria-invalid={!!show(errors.objective)} />
            {show(errors.objective) && <span className="warning">{errors.objective}</span>}
          </label>
          <label className="field">
            Goal
            <select value={form.direction} onChange={(e) => set({ direction: e.target.value as 'maximize' | 'minimize' })}>
              <option value="maximize">Maximize the metric</option>
              <option value="minimize">Minimize the metric</option>
            </select>
          </label>
        </div>

        <div className="formgrid">
          <label className="field">
            Trial image
            <input type="text" value={form.image} onChange={(e) => set({ image: e.target.value })} placeholder="registry.example.com/train:latest" aria-invalid={!!show(errors.image)} aria-required="true" />
            {show(errors.image) ? <span className="warning">{errors.image}</span> : <span className="faint">Required. The container each trial runs; it must report the objective metric.</span>}
          </label>
          {num('GPUs per trial', 'gpus', errors.gpus, undefined, 0, 1024)}
        </div>

        <div className="formgrid">
          {num('Max trials', 'maxTrials', errors.maxTrials, 'Total trials to run (1 to 10000).', 1, 10000)}
          {num('Parallelism', 'parallelism', errors.parallelism, 'Trials at once. 0 lets the operator decide.', 0, 256)}
        </div>

        {isAsha && (
          <div className="formgrid">
            {num('Max epochs (ASHA)', 'maxEpochs', errors.maxEpochs, 'Longest a single trial may train.', 1, 100000)}
            {num('Reduction factor', 'reductionFactor', errors.reductionFactor, 'Optional, 2 or more. Blank uses the default.', 2, 100, false)}
            {num('Min resource', 'minResource', errors.minResource, 'Optional. Epochs every trial gets before pruning.', 1, 100000, false)}
          </div>
        )}

        <label className="field">
          Parameter space (JSON)
          <textarea className="codeedit" rows={6} value={form.parameterSpace} onChange={(e) => set({ parameterSpace: e.target.value })} aria-invalid={!!spaceShown} spellCheck={false} />
          {spaceShown ? (
            <span className="warning" role="alert">{spaceShown}</span>
          ) : (
            <span className="faint">
              One entry per parameter. float and int take min and max (optional scale "linear" or "log"; int also takes an integer step). choice takes a values list of 1 to 64 numbers or strings.
            </span>
          )}
        </label>

        {createMutation.isError && (
          <p className="warning" role="alert">
            {errorMessage(createMutation.error)}
          </p>
        )}

        <div className="toolbar">
          <button type="button" className="btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="primary" disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Creating…' : 'Create tuner'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
