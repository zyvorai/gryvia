import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import type { Workflow, WorkflowStep } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Workflows() {
  const [showCreate, setShowCreate] = useState(false)
  const { data: workflows, isLoading, isError, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['workflows'],
    queryFn: api.getWorkflows,
    refetchInterval: 15000,
  })

  if (isError) return (
    <>
      <PageHero eyebrow="Workflows" title="Workflows unavailable." tint="red" />
      <p className="warning" role="alert">Failed to load workflows. Please try again.</p>
    </>
  )

  const activeWorkflows = workflows?.filter(w => w.status?.phase === 'Running') || []
  const completedWorkflows = workflows?.filter(w => w.status?.phase === 'Completed' || w.status?.phase === 'Succeeded') || []
  const failedWorkflows = workflows?.filter(w => w.status?.phase === 'Failed') || []
  const stepsRunning = workflows?.reduce((sum, w) =>
    sum + (w.status?.steps?.filter(s => s.status === 'Running').length || 0), 0) || 0

  return (
    <>
      <PageHero eyebrow="Workflows" title="Pipelines, end to end." lede="ML pipeline management." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={failedWorkflows.length > 0 ? `${failedWorkflows.length} workflow${failedWorkflows.length === 1 ? '' : 's'} failed.` : `${activeWorkflows.length} workflows running, none failed.`}
          tone={countTone(failedWorkflows.length, 1)}
          figures={[
            { label: 'Active workflows', value: activeWorkflows.length },
            { label: 'Completed', value: completedWorkflows.length },
            { label: 'Failed', value: failedWorkflows.length, tone: countTone(failedWorkflows.length, 1) },
            { label: 'Steps running', value: stepsRunning },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">Pipelines</p>
          <h2 className="card-title">All Workflows</h2>
          <div className="toolbar">
            <button className="primary" onClick={() => setShowCreate(true)}>
              Create Workflow
            </button>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
            <span className="faint">{workflows?.length || 0} total</span>
          </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (workflows || []).length === 0 ? (
          <p className="empty-state">No workflows yet</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Steps</th>
                  <th>Status</th>
                  <th>Progress</th>
                  <th>Duration</th>
                  <th>Started</th>
                </tr>
              </thead>
              <tbody>
                {(workflows || []).map((wf) => (
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

const phaseTone: Record<string, string> = {
  Running: 'info',
  Completed: 'ok',
  Succeeded: 'ok',
  Failed: 'bad',
  Pending: 'warn',
}

const stepTone: Record<string, string> = {
  Completed: 'ok',
  Succeeded: 'ok',
  Running: 'info',
  Pending: '',
  Failed: 'bad',
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
  const [error, setError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: (body: { name: string; steps: unknown[] }) =>
      api.createWorkflow({ metadata: { name: body.name }, spec: { steps: body.steps } }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workflows'] })
      onClose()
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    if (!/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(name)) {
      setError('Name must consist of lowercase alphanumeric characters or hyphens')
      return
    }
    let parsed: unknown
    try {
      parsed = JSON.parse(steps)
    } catch {
      setError('Steps must be valid JSON')
      return
    }
    if (!Array.isArray(parsed) || parsed.length === 0) {
      setError('Steps must be a non-empty JSON array')
      return
    }
    createMutation.mutate({ name, steps: parsed })
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <h2 className="card-title">Create Workflow</h2>
        <form onSubmit={handleSubmit} className="stack">
          <label className="field">
            <span>Name</span>
            <input type="text" required value={name} onChange={(e) => setName(e.target.value)} placeholder="my-pipeline" />
          </label>
          <label className="field">
            <span>Steps (JSON)</span>
            <textarea className="codeedit compact" required value={steps} onChange={(e) => setSteps(e.target.value)} />
          </label>
          <p className="faint">
            Each step has a name, an optional dependsOn list, and a payload: a jobTemplate (type, image, gpus), a script
            (image, command) or a webhook (url).
          </p>
          {error && <p className="warning" role="alert">{error}</p>}
          {createMutation.isError && (
            <p className="warning" role="alert">
              Error creating workflow: {errorMessage(createMutation.error)}
            </p>
          )}
          <div className="toolbar">
            <button type="button" className="btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="primary" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create Workflow'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function WorkflowRow({ workflow }: { workflow: Workflow }) {
  const [expanded, setExpanded] = useState(false)
  const name = workflow.metadata?.name || 'unknown'
  const phase = workflow.status?.phase || 'Pending'
  const steps = workflow.status?.steps || workflow.spec?.steps || []
  const totalSteps = steps.length
  const completedSteps = workflow.status?.steps?.filter(
    s => s.status === 'Completed' || s.status === 'Succeeded'
  ).length || 0

  return (
    <>
      <tr className="table-row-hover" onClick={() => setExpanded(!expanded)}>
        <td>
          <span className="faint" aria-hidden="true">{expanded ? '▾' : '▸'}</span>{' '}
          <b>{name}</b>
        </td>
        <td className="muted">{totalSteps}</td>
        <td><span className={`pill ${phaseTone[phase] || ''}`}>{phase}</span></td>
        <td>
          <div className={phase === 'Failed' ? 'progress bad' : 'progress'}>
            <span style={{ width: totalSteps > 0 ? `${(completedSteps / totalSteps) * 100}%` : '0%' }} />
          </div>
          <span className="faint">{completedSteps}/{totalSteps}</span>
        </td>
        <td className="muted">{workflow.status?.duration || '-'}</td>
        <td className="faint">
          {workflow.status?.startedAt ? new Date(workflow.status.startedAt).toLocaleString() : '-'}
        </td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan={6}>
            <p className="eyebrow">Workflow DAG</p>
            <DAGVisualization steps={workflow.status?.steps || workflow.spec?.steps || []} />
          </td>
        </tr>
      )}
    </>
  )
}

function DAGVisualization({ steps }: { steps: WorkflowStep[] }) {
  const [selectedStep, setSelectedStep] = useState<WorkflowStep | null>(null)

  if (steps.length === 0) {
    return <p className="empty-state">No steps defined</p>
  }

  return (
    <div className="stack">
      <div className="toolbar">
        {steps.map((step, idx) => {
          const isSelected = selectedStep?.name === step.name
          return (
            <div key={step.name || idx} className="row">
              <button
                className={isSelected ? 'primary' : 'btn-secondary'}
                onClick={() => setSelectedStep(isSelected ? null : step)}
              >
                {step.name}
                {' '}
                <span className={`pill ${stepTone[step.status || 'Pending'] ?? ''}`}>
                  {step.status || 'Pending'}
                </span>
              </button>
              {idx < steps.length - 1 && <span className="faint" aria-hidden="true">›</span>}
            </div>
          )
        })}
      </div>

      {selectedStep && (
        <div className="stack">
          <div className="formgrid">
            <div>
              <div className="faint">Step Name</div>
              <b>{selectedStep.name}</b>
            </div>
            <div>
              <div className="faint">Type</div>
              <span className="muted">{selectedStep.type || 'task'}</span>
            </div>
            <div>
              <div className="faint">Status</div>
              <span className={`pill ${stepTone[selectedStep.status || 'Pending'] ?? ''}`}>
                {selectedStep.status || 'Pending'}
              </span>
            </div>
            <div>
              <div className="faint">Duration</div>
              <span className="muted">{selectedStep.duration || '-'}</span>
            </div>
          </div>
          {selectedStep.jobRef && (
            <Link to={`/jobs/${selectedStep.jobRef}`} className="card-link">
              View job logs: {selectedStep.jobRef} ›
            </Link>
          )}
        </div>
      )}
    </div>
  )
}
