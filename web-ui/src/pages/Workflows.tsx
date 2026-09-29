import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { Workflow, WorkflowStep } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Workflows() {
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
            <Link to="/jobs/new" className="buttonlike primary">
              Create Workflow
            </Link>
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
