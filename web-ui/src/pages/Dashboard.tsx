import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { FabricAIJob } from '@/types'
import GPUChart from '@/components/GPUChart'
import PageHero from '@/components/PageHero'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { countByGroup, phaseTone } from '@/lib/phase'
import { jobFramework, jobGpus } from '@/lib/jobs'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

export default function Dashboard() {
  useDocumentTitle('Dashboard')
  const stats = useQuery({ queryKey: ['clusterStats'], queryFn: api.getClusterStats, refetchInterval: 15000 })
  const jobsQ = useQuery({ queryKey: ['jobs'], queryFn: api.getJobs, refetchInterval: 10000 })
  const nodesQ = useQuery({ queryKey: ['nodes'], queryFn: api.getNodes, refetchInterval: 30000 })
  const gpuQ = useQuery({ queryKey: ['gpuMetrics'], queryFn: api.getGPUMetrics, refetchInterval: 15000 })

  const clusterStats = stats.data
  const jobs = jobsQ.data
  const nodes = nodesQ.data
  const gpuMetrics = gpuQ.data

  if (stats.isError && !clusterStats) {
    return (
      <>
        <PageHero eyebrow="Dashboard" title="Cluster unavailable." tint="red" />
        <ErrorState title="Could not load cluster stats." error={stats.error} onRetry={() => stats.refetch()} retrying={stats.isRefetching} />
      </>
    )
  }

  const counts = countByGroup(jobs, (j) => j.status?.phase)
  const finished = counts.completed + counts.failed
  const successRate = finished > 0 ? Math.round((counts.completed / finished) * 100) : undefined
  const utilization = clusterStats ? Math.round(clusterStats.utilizationPercent || 0) : undefined
  const totalGPUs = clusterStats?.totalGPUs ?? 0
  const availableGPUs = clusterStats?.availableGPUs ?? 0
  const allocatedGPUs = clusterStats?.allocatedGPUs ?? Math.max(0, totalGPUs - availableGPUs)
  const num = (n: number | undefined) => (n === undefined ? '—' : n)

  return (
    <>
      <PageHero eyebrow="Dashboard" title="Your GPU cluster, at a glance." lede="Live capacity, jobs and node health." />

      <div className="grid">
        {stats.isError && (
          <div className="span3">
            <ErrorState title="Could not refresh cluster stats; showing the last data." error={stats.error} onRetry={() => stats.refetch()} retrying={stats.isRefetching} />
          </div>
        )}

        <div className="apple-metric-band span3">
          <div>
            <span>GPUs{clusterStats ? ` · ${availableGPUs} available` : ''}</span>
            <b>{num(clusterStats?.totalGPUs)}</b>
          </div>
          <div>
            <span>Running jobs{jobs && counts.pending > 0 ? ` · ${counts.pending} pending` : ''}</span>
            <b>{num(jobs ? counts.running : undefined)}</b>
          </div>
          <div>
            <span>Completed{successRate !== undefined ? ` · ${successRate}% success` : ''}</span>
            <b>{num(jobs ? counts.completed : undefined)}</b>
          </div>
          <div>
            <span>GPU utilization now</span>
            <b>{utilization === undefined ? '—' : `${utilization}%`}</b>
          </div>
        </div>

        <section className="card span2">
          <p className="eyebrow">UTILIZATION</p>
          <h2 className="card-title">GPU utilization now</h2>
          {gpuQ.isError && !gpuMetrics ? (
            <ErrorState title="Could not load GPU metrics." error={gpuQ.error} onRetry={() => gpuQ.refetch()} retrying={gpuQ.isRefetching} />
          ) : gpuQ.isLoading ? (
            <Skeleton rows={4} />
          ) : (
            <GPUChart
              data={gpuMetrics?.metrics?.map((m) => ({
                time: new Date(m.timestamp).toLocaleTimeString(),
                [`${m.node}-GPU${m.gpuIndex}`]: m.utilization,
              }))}
            />
          )}
        </section>

        <section className="card">
          <p className="eyebrow">HEALTH</p>
          <h2 className="card-title">Cluster health</h2>
          {!clusterStats ? (
            <Skeleton rows={5} />
          ) : (
            <>
              <HealthCheck label="GPU nodes" tone={clusterStats.totalNodes > 0 ? 'ok' : 'warn'} detail={`${clusterStats.totalNodes} nodes`} />
              <HealthCheck label="GPU utilization" tone={(utilization ?? 0) >= 95 ? 'bad' : (utilization ?? 0) > 80 ? 'warn' : 'ok'} detail={`${utilization}%`} />
              <HealthCheck label="Failed jobs" tone={!jobs ? undefined : counts.failed === 0 ? 'ok' : 'warn'} detail={jobs ? `${counts.failed}` : '—'} />
              <HealthCheck
                label="GPUs allocated"
                tone={totalGPUs > 0 && availableGPUs === 0 ? 'warn' : 'ok'}
                detail={`${allocatedGPUs}/${totalGPUs} GPUs`}
              />
              <HealthCheck
                label="Job queue"
                tone={!jobs ? undefined : counts.pending >= 10 ? 'bad' : counts.pending > 5 ? 'warn' : 'ok'}
                detail={jobs ? `${counts.pending} pending` : '—'}
              />
            </>
          )}
        </section>

        <section className="card span2">
          <p className="eyebrow">JOBS · {jobs ? `${jobs.length} TOTAL` : '…'}</p>
          <h2 className="card-title">Recent jobs</h2>
          {jobsQ.isError && !jobs ? (
            <ErrorState title="Could not load jobs." error={jobsQ.error} onRetry={() => jobsQ.refetch()} retrying={jobsQ.isRefetching} />
          ) : jobsQ.isLoading ? (
            <Skeleton rows={4} />
          ) : (jobs || []).length === 0 ? (
            <EmptyState
              title="No jobs yet"
              action={
                <Link to="/jobs/new" className="buttonlike primary">
                  Submit a job
                </Link>
              }
            >
              Jobs appear here once you submit a FabricAIJob.
            </EmptyState>
          ) : (
            (jobs || []).slice(0, 5).map((job) => <JobRow key={job.metadata?.name} job={job} />)
          )}
          <p>
            <Link to="/jobs" className="card-link">
              View all jobs ›
            </Link>
          </p>
        </section>

        <section className="card">
          <p className="eyebrow">NODES</p>
          <h2 className="card-title">GPU nodes</h2>
          {nodesQ.isError && !nodes ? (
            <ErrorState title="Could not load nodes." error={nodesQ.error} onRetry={() => nodesQ.refetch()} retrying={nodesQ.isRefetching} />
          ) : nodesQ.isLoading ? (
            <Skeleton rows={3} />
          ) : !nodes || nodes.length === 0 ? (
            <EmptyState title="No GPU nodes registered">Nodes are registered by the GPU operator once it discovers GPUs.</EmptyState>
          ) : (
            <>
              {nodes.slice(0, 6).map((node) => (
                <div key={node.metadata?.name} className="list-row">
                  <span className={`dot ${phaseTone(node.status?.phase)}`} />
                  <div className="grow">
                    <b>{node.spec?.nodeName || node.metadata?.name}</b>
                    <small>
                      {node.spec?.gpuType} × {node.spec?.gpuCount}
                    </small>
                  </div>
                  <span className="faint">{node.status?.phase || 'Unknown'}</span>
                </div>
              ))}
              {nodes.length > 6 && (
                <p>
                  <Link to="/nodes" className="card-link">
                    View all {nodes.length} nodes ›
                  </Link>
                </p>
              )}
            </>
          )}
        </section>

        <section className="card span3">
          <p className="eyebrow">SHORTCUTS</p>
          <h2 className="card-title">Quick actions</h2>
          <div className="toolbar">
            <Link to="/jobs/new" className="buttonlike primary">
              Submit job
            </Link>
            <Link to="/quotas" className="buttonlike btn-secondary">
              Quotas
            </Link>
            <Link to="/costs" className="buttonlike btn-secondary">
              Costs
            </Link>
          </div>
        </section>
      </div>
    </>
  )
}

function JobRow({ job }: { job: FabricAIJob }) {
  const phase = job.status?.phase || 'Unknown'
  return (
    <Link to={`/jobs/${job.metadata?.name}`} className="list-row">
      <div className="grow">
        <b>{job.metadata?.name}</b>
        <small>
          {jobFramework(job)} · {jobGpus(job)}
        </small>
      </div>
      <span className={`pill ${phaseTone(phase)}`}>{phase}</span>
    </Link>
  )
}

function HealthCheck({ label, tone, detail }: { label: string; tone?: 'ok' | 'warn' | 'bad'; detail?: string }) {
  return (
    <div className="list-row">
      <span className={`dot ${tone ?? ''}`} />
      <span className="grow">{label}</span>
      {detail && <span className="faint num">{detail}</span>}
    </div>
  )
}
