import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  GitBranch, CheckCircle, XCircle, Loader2,
  RefreshCw, Plus, Clock, ChevronDown, ChevronRight, Activity,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { Workflow, WorkflowStep } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Workflows() {
  const { data: workflows, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['workflows'],
    queryFn: api.getWorkflows,
    refetchInterval: 15000,
  })

  if (isError) return (
    <div className="text-center py-12">
      <p className="text-red-400">Failed to load workflows. Please try again.</p>
    </div>
  )

  const activeWorkflows = workflows?.filter(w => w.status?.phase === 'Running') || []
  const completedWorkflows = workflows?.filter(w => w.status?.phase === 'Completed' || w.status?.phase === 'Succeeded') || []
  const failedWorkflows = workflows?.filter(w => w.status?.phase === 'Failed') || []
  const stepsRunning = workflows?.reduce((sum, w) =>
    sum + (w.status?.steps?.filter(s => s.status === 'Running').length || 0), 0) || 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Workflows</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">ML pipeline management</p>
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
          <Link
            to="/jobs/new"
            className="inline-flex items-center gap-1.5 px-3.5 py-1.5 btn-copper text-sm font-medium rounded-lg"
          >
            <Plus className="h-3.5 w-3.5" />
            Create Workflow
          </Link>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={GitBranch} title="Active Workflows" value={activeWorkflows.length} color="blue" />
        <StatCard icon={CheckCircle} title="Completed" value={completedWorkflows.length} color="green" />
        <StatCard icon={XCircle} title="Failed" value={failedWorkflows.length} color="red" />
        <StatCard icon={Loader2} title="Steps Running" value={stepsRunning} color="cyan" />
      </div>

      {/* Workflows Table */}
      <div className="rounded-xl" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="px-5 py-4" style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
          <div className="flex items-center gap-2">
            <GitBranch className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">All Workflows</h3>
            <span className="text-xs text-[#5a7a9e] ml-auto">{workflows?.length || 0} total</span>
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
                <div className="col-span-1">Steps</div>
                <div className="col-span-2">Status</div>
                <div className="col-span-2">Progress</div>
                <div className="col-span-2">Duration</div>
                <div className="col-span-2">Started</div>
              </div>

              {(workflows || []).map((wf) => (
                <WorkflowRow key={wf.metadata?.name} workflow={wf} />
              ))}

              {(!workflows || workflows.length === 0) && (
                <div className="text-center py-8 text-sm text-[#344e6a]">No workflows yet</div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// --- Sub-components ---

const phaseConfig: Record<string, { icon: React.ElementType; color: string; spinning?: boolean }> = {
  Running: { icon: Loader2, color: 'text-[#7ecbf5]', spinning: true },
  Completed: { icon: CheckCircle, color: 'text-emerald-400' },
  Succeeded: { icon: CheckCircle, color: 'text-emerald-400' },
  Failed: { icon: XCircle, color: 'text-red-400' },
  Pending: { icon: Clock, color: 'text-[#fbbf24]' },
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
        <div className="col-span-1 text-xs text-[#8ba4c0]">{totalSteps}</div>
        <div className="col-span-2">
          <div className="flex items-center gap-1.5">
            <Icon className={`h-3 w-3 ${cfg.color} ${cfg.spinning ? 'animate-spin' : ''}`} />
            <span className={`text-xs font-medium ${cfg.color}`}>{phase}</span>
          </div>
        </div>
        <div className="col-span-2">
          <div className="flex items-center gap-2">
            <div className="flex-1 h-1.5 rounded-full overflow-hidden" style={{
              background: 'rgba(10,14,20,0.6)',
              boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.4)',
            }}>
              <div className="h-full rounded-full transition-all" style={{
                width: totalSteps > 0 ? `${(completedSteps / totalSteps) * 100}%` : '0%',
                background: phase === 'Failed'
                  ? 'linear-gradient(90deg, #7f1d1d, #ef4444)'
                  : 'linear-gradient(90deg, #344e6a, #5fa8d3)',
                boxShadow: '0 0 6px rgba(95,168,211,0.2)',
              }} />
            </div>
            <span className="text-[10px] text-[#5a7a9e] whitespace-nowrap">{completedSteps}/{totalSteps}</span>
          </div>
        </div>
        <div className="col-span-2 text-xs text-[#8ba4c0]">{workflow.status?.duration || '-'}</div>
        <div className="col-span-2 text-xs text-[#5a7a9e]">
          {workflow.status?.startedAt ? new Date(workflow.status.startedAt).toLocaleString() : '-'}
        </div>
      </div>

      {/* DAG Visualization */}
      {expanded && (
        <div className="px-4 py-5 mb-1 rounded-lg animate-fade-in" style={{
          background: 'rgba(10,14,20,0.4)',
          border: '1px solid rgba(192,204,224,0.04)',
        }}>
          <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-3 font-medium">
            Workflow DAG
          </div>
          <DAGVisualization steps={workflow.status?.steps || workflow.spec?.steps || []} />
        </div>
      )}
    </>
  )
}

function DAGVisualization({ steps }: { steps: WorkflowStep[] }) {
  const [selectedStep, setSelectedStep] = useState<WorkflowStep | null>(null)

  if (steps.length === 0) {
    return <div className="text-xs text-[#344e6a] text-center py-4">No steps defined</div>
  }

  const stepStatusColors: Record<string, { bg: string; border: string; text: string; glow: string }> = {
    Completed: { bg: 'rgba(34,197,94,0.1)', border: 'rgba(34,197,94,0.3)', text: 'text-emerald-400', glow: '0 0 8px rgba(34,197,94,0.15)' },
    Succeeded: { bg: 'rgba(34,197,94,0.1)', border: 'rgba(34,197,94,0.3)', text: 'text-emerald-400', glow: '0 0 8px rgba(34,197,94,0.15)' },
    Running: { bg: 'rgba(95,168,211,0.1)', border: 'rgba(95,168,211,0.3)', text: 'text-[#7ecbf5]', glow: '0 0 8px rgba(95,168,211,0.15)' },
    Pending: { bg: 'rgba(128,144,168,0.06)', border: 'rgba(128,144,168,0.15)', text: 'text-[#5a7a9e]', glow: 'none' },
    Failed: { bg: 'rgba(239,68,68,0.1)', border: 'rgba(239,68,68,0.3)', text: 'text-red-400', glow: '0 0 8px rgba(239,68,68,0.15)' },
  }

  return (
    <div className="flex flex-col gap-2">
      {/* Step boxes */}
      <div className="flex flex-wrap gap-2 items-center">
        {steps.map((step, idx) => {
          const sc = stepStatusColors[step.status || 'Pending'] || stepStatusColors.Pending
          const isSelected = selectedStep?.name === step.name
          return (
            <div key={step.name || idx} className="flex items-center gap-2">
              <button
                onClick={() => setSelectedStep(isSelected ? null : step)}
                className={`px-3 py-2 rounded-lg text-xs font-medium transition-all duration-200 ${isSelected ? 'ring-1 ring-[#d4764e]' : ''}`}
                style={{
                  background: sc.bg,
                  border: `1px solid ${sc.border}`,
                  boxShadow: sc.glow,
                }}
              >
                <div className={`${sc.text} truncate max-w-[120px]`}>{step.name}</div>
                <div className="text-[10px] text-[#5a7a9e] mt-0.5">{step.status || 'Pending'}</div>
              </button>
              {idx < steps.length - 1 && (
                <svg width="24" height="12" viewBox="0 0 24 12" className="flex-shrink-0">
                  <line x1="0" y1="6" x2="18" y2="6" stroke="rgba(192,204,224,0.15)" strokeWidth="1.5" />
                  <polygon points="18,2 24,6 18,10" fill="rgba(192,204,224,0.2)" />
                </svg>
              )}
            </div>
          )
        })}
      </div>

      {/* Step detail */}
      {selectedStep && (
        <div className="mt-3 p-4 rounded-lg animate-fade-in" style={{
          background: 'rgba(10,14,20,0.5)',
          border: '1px solid rgba(192,204,224,0.06)',
        }}>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-1">Step Name</div>
              <div className="text-xs text-[#c0cce0] font-medium">{selectedStep.name}</div>
            </div>
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-1">Type</div>
              <div className="text-xs text-[#8ba4c0]">{selectedStep.type || 'task'}</div>
            </div>
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-1">Status</div>
              <div className={`text-xs font-medium ${(stepStatusColors[selectedStep.status || 'Pending'] || stepStatusColors.Pending).text}`}>
                {selectedStep.status || 'Pending'}
              </div>
            </div>
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-1">Duration</div>
              <div className="text-xs text-[#8ba4c0]">{selectedStep.duration || '-'}</div>
            </div>
          </div>
          {selectedStep.jobRef && (
            <div className="mt-3 pt-3" style={{ borderTop: '1px solid rgba(192,204,224,0.04)' }}>
              <Link to={`/jobs/${selectedStep.jobRef}`} className="inline-flex items-center gap-1 text-xs text-[#e8a87c] hover:text-[#f0c4a0] transition-colors">
                <Activity className="h-3 w-3" />
                View job logs: {selectedStep.jobRef}
              </Link>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
