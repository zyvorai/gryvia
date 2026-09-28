import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { Workflow, WorkflowStep } from '@/lib/api'
import PageHero from '@/components/PageHero'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Workflows() {
  const { data: workflows, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['workflows'],
    queryFn: api.getWorkflows,
    refetchInterval: 15000,
  })

  if (isError) return (
    <>
      <PageHero eyebrow="Workflows" title="Workflows unavailable." tint="red" />
      <p className="login-error" role="alert">Failed to load workflows. Please try again.</p>
    </>
  )

  const activeWorkflows = workflows?.filter(w => w.status?.phase === 'Running') || []
  const completedWorkflows = workflows?.filter(w => w.status?.phase === 'Completed' || w.status?.phase === 'Succeeded') || []
  const failedWorkflows = workflows?.filter(w => w.status?.phase === 'Failed') || []
  const stepsRunning = workflows?.reduce((sum, w) =>
    sum + (w.status?.steps?.filter(s => s.status === 'Running').length || 0), 0) || 0

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Workflows" title="Pipelines, end to end." lede="ML pipeline management." />

      <div className="page-actions">
        <Link to="/jobs/new" className="buttonlike primary">
          Create Workflow
        </Link>
        <button className="btn-secondary" onClick={() => refetch()} disabled={isRefetching}>
          Refresh
        </button>
      </div>

      <div className="apple-metric-band">
        <div><span>Active Workflows</span><b>{activeWorkflows.length}</b></div>
        <div><span>Completed</span><b>{completedWorkflows.length}</b></div>
        <div><span>Failed</span><b className={failedWorkflows.length > 0 ? 'text-bad' : undefined}>{failedWorkflows.length}</b></div>
        <div><span>Steps Running</span><b>{stepsRunning}</b></div>
      </div>

      <section className="card">
        <div className="stat-head">
          <h2>All Workflows</h2>
          <span className="faint">{workflows?.length || 0} total</span>
        </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (workflows || []).length === 0 ? (
          <div className="list-empty">No workflows yet</div>
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
      <tr className="table-row-hover" style={{ cursor: 'pointer' }} onClick={() => setExpanded(!expanded)}>
        <td>
          <span className="faint" aria-hidden="true">{expanded ? '▾' : '▸'}</span>{' '}
          <b>{name}</b>
        </td>
        <td className="muted">{totalSteps}</td>
        <td><span className={`pill ${phaseTone[phase] || ''}`}>{phase}</span></td>
        <td>
          <div className="row" style={{ gap: 8, flexWrap: 'nowrap' }}>
            <div className={phase === 'Failed' ? 'progress bad' : 'progress'} style={{ flex: 1, minWidth: 60 }}>
              <span style={{ width: totalSteps > 0 ? `${(completedSteps / totalSteps) * 100}%` : '0%' }} />
            </div>
            <span className="faint">{completedSteps}/{totalSteps}</span>
          </div>
        </td>
        <td className="muted">{workflow.status?.duration || '-'}</td>
        <td className="faint">
          {workflow.status?.startedAt ? new Date(workflow.status.startedAt).toLocaleString() : '-'}
        </td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan={6}>
            <div className="faint" style={{ marginBottom: 12 }}>Workflow DAG</div>
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
    return <div className="list-empty">No steps defined</div>
  }

  return (
    <div className="stack" style={{ gap: 12 }}>
      <div className="row" style={{ gap: 8 }}>
        {steps.map((step, idx) => {
          const isSelected = selectedStep?.name === step.name
          return (
            <div key={step.name || idx} className="row" style={{ gap: 8, flexWrap: 'nowrap' }}>
              <button
                className={isSelected ? 'primary' : 'btn-secondary'}
                onClick={() => setSelectedStep(isSelected ? null : step)}
              >
                {step.name}
                <span className={`pill ${stepTone[step.status || 'Pending'] ?? ''}`} style={{ marginLeft: 8 }}>
                  {step.status || 'Pending'}
                </span>
              </button>
              {idx < steps.length - 1 && <span className="faint" aria-hidden="true">›</span>}
            </div>
          )
        })}
      </div>

      {selectedStep && (
        <div className="stack" style={{ gap: 12 }}>
          <div className="card-grid">
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
