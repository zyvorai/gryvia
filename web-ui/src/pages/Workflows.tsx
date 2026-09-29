import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { Workflow, WorkflowStep } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import { formatDate, formatRelative } from '@/lib/format'
import { countByGroup, phaseGroup, phaseTone } from '@/lib/phase'
import { notify } from '@/lib/notify'
import { nameError } from '@/lib/forms'
import { dagLayers } from '@/lib/dag'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'
import Modal from '@/components/Modal'
import Progress from '@/components/Progress'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { TableCaption } from '@/components/TableCaption'

export default function Workflows() {
  useDocumentTitle('Workflows')
  const [showCreate, setShowCreate] = useState(false)
  const { data: workflows, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['workflows'],
    queryFn: api.getWorkflows,
    refetchInterval: 15000,
  })

  if (isError && !workflows) {
    return (
      <>
        <PageHero eyebrow="Workflows" title="Workflows unavailable." tint="red" />
        <ErrorState title="Could not load workflows." error={error} onRetry={() => refetch()} retrying={isRefetching} />
      </>
    )
  }

  const groups = workflows ? countByGroup(workflows, (w) => w.status?.phase) : undefined
  const failed = groups?.failed
  const stepsRunning = workflows?.reduce((sum, w) => sum + (w.status?.steps?.filter((s) => phaseGroup(s.status) === 'running').length ?? 0), 0)

  return (
    <>
      <PageHero eyebrow="Workflows" title="Pipelines, end to end." lede="ML pipeline management." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={
            failed === undefined
              ? undefined
              : failed > 0
                ? `${failed} workflow${failed === 1 ? '' : 's'} failed.`
                : `${groups?.running ?? 0} workflows running, none failed.`
          }
          tone={failed === undefined ? undefined : countTone(failed, 1)}
          figures={[
            { label: 'Active workflows', value: groups?.running },
            { label: 'Completed', value: groups?.completed },
            { label: 'Failed', value: failed, tone: failed === undefined ? undefined : countTone(failed, 1) },
            { label: 'Steps running', value: stepsRunning },
          ]}
        />

        {isError && (
          <div className="span3">
            <ErrorState title="Showing the last data received; refreshing failed." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        )}

        <section className="card span3">
          <p className="eyebrow">Pipelines</p>
          <h2 className="card-title">All workflows</h2>
          <div className="toolbar">
            <button className="primary" onClick={() => setShowCreate(true)}>
              Create workflow
            </button>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
            {workflows && <span className="faint">{workflows.length} total</span>}
          </div>
          {isLoading ? (
            <Skeleton rows={4} />
          ) : (workflows ?? []).length === 0 ? (
            <EmptyState
              title="No workflows yet."
              action={
                <button className="primary" onClick={() => setShowCreate(true)}>
                  Create your first workflow
                </button>
              }
            >
              A workflow chains jobs, scripts and webhooks into a pipeline, with steps that wait for the ones they depend on.
            </EmptyState>
          ) : (
            <div className="table-wrap">
              <table>
                <TableCaption>Workflows</TableCaption>
                <thead>
                  <tr>
                    <th scope="col">Name</th>
                    <th scope="col" className="num">Steps</th>
                    <th scope="col">Status</th>
                    <th scope="col">Progress</th>
                    <th scope="col">Duration</th>
                    <th scope="col">Started</th>
                  </tr>
                </thead>
                <tbody>
                  {(workflows ?? []).map((wf) => (
                    <WorkflowRow key={wf.metadata?.name} workflow={wf} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
      {showCreate && <CreateWorkflowModal onClose={() => setShowCreate(false)} />}
    </>
  )
}

// --- Sub-components ---

const NOT_STARTED = 'Not started'

/** Spec steps carry type and dependsOn; status steps carry progress. Combine them by step name. */
function mergeSteps(workflow: Workflow): WorkflowStep[] {
  const spec = workflow.spec?.steps ?? []
  const status = workflow.status?.steps ?? []
  const statusByName = new Map(status.map((s) => [s.name, s]))
  const specNames = new Set(spec.map((s) => s.name))
  const merged: WorkflowStep[] = spec.map((s) => ({ ...s, ...statusByName.get(s.name), type: statusByName.get(s.name)?.type ?? s.type, dependsOn: statusByName.get(s.name)?.dependsOn ?? s.dependsOn }))
  for (const s of status) if (!specNames.has(s.name)) merged.push(s)
  return merged
}

const DEFAULT_STEPS = JSON.stringify(
  [
    { name: 'prepare', type: 'job', jobTemplate: { type: 'training', image: 'busybox:1.36', gpus: 0, command: ['echo', 'prepare'] } },
    { name: 'train', type: 'job', dependsOn: ['prepare'], jobTemplate: { type: 'training', image: 'busybox:1.36', gpus: 1, command: ['echo', 'train'] } },
  ],
  null,
  2,
)

function CreateWorkflowModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [steps, setSteps] = useState(DEFAULT_STEPS)
  const [submitted, setSubmitted] = useState(false)
  const dirty = name !== '' || steps !== DEFAULT_STEPS

  const createMutation = useMutation({
    mutationFn: (body: { name: string; steps: unknown[] }) =>
      api.createWorkflow({ metadata: { name: body.name }, spec: { steps: body.steps } }),
    onSuccess: (_d, vars) => {
      notify.success(`Created workflow ${vars.name}`)
      queryClient.invalidateQueries({ queryKey: ['workflows'] })
      onClose()
    },
    onError: (err) => notify.error('Could not create workflow', err),
  })

  let parsed: unknown
  let stepsError: string | null = null
  try {
    parsed = JSON.parse(steps)
    if (!Array.isArray(parsed) || parsed.length === 0) stepsError = 'Steps must be a non-empty JSON array.'
  } catch (e) {
    stepsError = `Not valid JSON: ${e instanceof Error ? e.message : 'parse error'}`
  }
  const nameErr = nameError(name)
  const nameShown = submitted || name !== '' ? nameErr : null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (nameErr || stepsError) return
    createMutation.mutate({ name, steps: parsed as unknown[] })
  }

  return (
    <Modal title="Create workflow" onClose={onClose} dirty={dirty}>
      <form onSubmit={handleSubmit} className="stack" noValidate>
        <label className="field">
          <span>Name</span>
          <input type="text" data-autofocus value={name} onChange={(e) => setName(e.target.value)} placeholder="my-pipeline" aria-invalid={!!nameShown} autoComplete="off" />
          {nameShown && <span className="warning">{nameShown}</span>}
        </label>
        <label className="field">
          <span>Steps (JSON)</span>
          <textarea className="codeedit compact" value={steps} onChange={(e) => setSteps(e.target.value)} aria-invalid={!!stepsError} spellCheck={false} />
          {stepsError && <span className="warning">{stepsError}</span>}
        </label>
        <p className="faint">
          Each step has a name, an optional dependsOn list, and a payload: a jobTemplate (type, image, gpus), a script
          (image, command) or a webhook (url).
        </p>
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
            {createMutation.isPending ? 'Creating…' : 'Create workflow'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

function WorkflowRow({ workflow }: { workflow: Workflow }) {
  const [expanded, setExpanded] = useState(false)
  const name = workflow.metadata?.name || 'unknown'
  const phase = workflow.status?.phase || 'Pending'
  const steps = mergeSteps(workflow)
  const totalSteps = steps.length
  const completedSteps = workflow.status?.steps?.filter((s) => phaseGroup(s.status) === 'completed').length ?? 0
  const started = workflow.status?.startedAt
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
        <td className="num muted">{totalSteps}</td>
        <td><span className={`pill ${phaseTone(phase)}`}>{phase}</span></td>
        <td>
          <Progress value={completedSteps} max={totalSteps} label={`${name}: ${completedSteps} of ${totalSteps} steps complete`} tone={phaseGroup(phase) === 'failed' ? 'bad' : undefined} />
          <span className="faint">{completedSteps}/{totalSteps}</span>
        </td>
        <td className="muted">{workflow.status?.duration ?? '—'}</td>
        <td className="faint" title={formatDate(started)}>
          {formatRelative(started)}
        </td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan={6}>
            <p className="eyebrow">Workflow DAG</p>
            <DAGVisualization steps={steps} started={!!workflow.status?.steps} />
          </td>
        </tr>
      )}
    </>
  )
}

function DAGVisualization({ steps, started }: { steps: WorkflowStep[]; started: boolean }) {
  const [selectedName, setSelectedName] = useState<string | null>(null)
  const stepStatus = (s: WorkflowStep) => (started && s.status ? s.status : NOT_STARTED)
  const selectedStep = steps.find((s) => s.name === selectedName)

  if (steps.length === 0) {
    return <EmptyState title="No steps defined." />
  }

  const layers = dagLayers(steps)

  return (
    <div className="stack">
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, overflowX: 'auto', paddingBottom: 4 }} role="group" aria-label="Workflow steps, left to right by dependency">
        {layers.map((layer, i) => (
          <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {i > 0 && <span className="faint" aria-hidden="true">→</span>}
            <div className="stack" style={{ gap: 8 }}>
              {layer.map((step) => {
                const isSelected = selectedName === step.name
                const status = stepStatus(step)
                return (
                  <button
                    key={step.name}
                    type="button"
                    className={isSelected ? 'primary' : 'btn-secondary'}
                    aria-pressed={isSelected}
                    onClick={() => setSelectedName(isSelected ? null : step.name)}
                  >
                    {step.name} <span className={`pill ${phaseTone(status)}`}>{status}</span>
                  </button>
                )
              })}
            </div>
          </div>
        ))}
      </div>

      {selectedStep && (
        <div className="stack">
          <div className="formgrid">
            <div>
              <div className="faint">Step name</div>
              <b>{selectedStep.name}</b>
            </div>
            <div>
              <div className="faint">Type</div>
              <span className="muted">{selectedStep.type ?? '—'}</span>
            </div>
            <div>
              <div className="faint">Depends on</div>
              <span className="muted">{selectedStep.dependsOn && selectedStep.dependsOn.length > 0 ? selectedStep.dependsOn.join(', ') : 'Nothing (starts first)'}</span>
            </div>
            <div>
              <div className="faint">Status</div>
              <span className={`pill ${phaseTone(stepStatus(selectedStep))}`}>{stepStatus(selectedStep)}</span>
            </div>
            <div>
              <div className="faint">Duration</div>
              <span className="muted">{selectedStep.duration ?? '—'}</span>
            </div>
          </div>
          {selectedStep.jobRef && (
            <Link to={`/jobs/${encodeURIComponent(selectedStep.jobRef)}`} className="card-link">
              View job logs: {selectedStep.jobRef} ›
            </Link>
          )}
        </div>
      )}
    </div>
  )
}
