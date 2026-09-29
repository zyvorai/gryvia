import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { FabricAIJob } from '@/types'
import GPUChart from '@/components/GPUChart'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import { phaseTone } from '@/lib/phase'
import { jobFramework, jobGpus } from '@/lib/jobs'

export default function Dashboard() {
  const { data: clusterStats, isLoading: statsLoading, isError: statsError } = useQuery({
    queryKey: ['clusterStats'],
    queryFn: api.getClusterStats,
    refetchInterval: 15000,
  })

  const { data: jobs, isLoading: jobsLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.getJobs,
    refetchInterval: 10000,
  })

  const { data: nodes } = useQuery({
    queryKey: ['nodes'],
    queryFn: api.getNodes,
    refetchInterval: 30000,
  })

  const { data: gpuMetrics } = useQuery({
    queryKey: ['gpuMetrics'],
    queryFn: api.getGPUMetrics,
    refetchInterval: 15000,
  })

  if (statsLoading) {
    return <LoadingSpinner />
  }

  if (statsError) {
    return (
      <>
        <PageHero eyebrow="Dashboard" title="Cluster unavailable." tint="red" />
        <p className="warning" role="alert">
          Failed to load cluster stats. Please check your API connection.
        </p>
      </>
    )
  }

  const runningJobs = jobs?.filter((j) => j.status?.phase === 'Running') || []
  const pendingJobs = jobs?.filter((j) => j.status?.phase === 'Pending' || j.status?.phase === 'Queued') || []
  const completedJobs = jobs?.filter((j) => j.status?.phase === 'Completed' || j.status?.phase === 'Succeeded') || []
  const failedJobs = jobs?.filter((j) => j.status?.phase === 'Failed') || []
  const totalJobs = jobs?.length || 0
  const successRate = totalJobs > 0 ? Math.round((completedJobs.length / totalJobs) * 100) : 0
  const utilization = Math.round(clusterStats?.utilizationPercent || 0)

  return (
    <>
      <PageHero eyebrow="Dashboard" title="Your GPU cluster, at a glance." lede="Live capacity, jobs and node health." />

      <div className="grid">
        <div className="apple-metric-band span3">
          <div>
            <span>GPUs · {clusterStats?.availableGPUs || 0} available</span>
            <b>{clusterStats?.totalGPUs || 0}</b>
          </div>
          <div>
            <span>Running jobs{pendingJobs.length > 0 ? ` · ${pendingJobs.length} pending` : ''}</span>
            <b>{runningJobs.length}</b>
          </div>
          <div>
            <span>Completed{successRate > 0 ? ` · ${successRate}% success` : ''}</span>
            <b>{completedJobs.length}</b>
          </div>
          <div>
            <span>GPU utilization</span>
            <b>{utilization}%</b>
          </div>
        </div>

        <section className="card span2">
          <p className="eyebrow">UTILIZATION</p>
          <h2 className="card-title">GPU utilization</h2>
          <GPUChart
            data={gpuMetrics?.metrics?.map((m) => ({
              time: new Date(m.timestamp).toLocaleTimeString(),
              [`${m.node}-GPU${m.gpuIndex}`]: m.utilization,
            }))}
          />
        </section>

        <section className="card">
          <p className="eyebrow">HEALTH</p>
          <h2 className="card-title">Cluster health</h2>
          <HealthCheck label="GPU nodes" tone={(clusterStats?.totalNodes || 0) > 0 ? 'ok' : 'bad'} detail={`${clusterStats?.totalNodes || 0} nodes`} />
          <HealthCheck label="GPU utilization" tone={utilization >= 95 ? 'bad' : utilization > 80 ? 'warn' : 'ok'} detail={`${utilization}%`} />
          <HealthCheck label="Failed jobs" tone={failedJobs.length === 0 ? 'ok' : 'bad'} detail={`${failedJobs.length}`} />
          <HealthCheck label="Available GPUs" tone={(clusterStats?.availableGPUs || 0) > 0 ? 'ok' : 'bad'} detail={`${clusterStats?.availableGPUs || 0}`} />
          <HealthCheck label="Job queue" tone={pendingJobs.length >= 10 ? 'bad' : pendingJobs.length > 5 ? 'warn' : 'ok'} detail={`${pendingJobs.length} pending`} />
        </section>

        <section className="card span2">
          <p className="eyebrow">JOBS · {totalJobs} TOTAL</p>
          <h2 className="card-title">Recent jobs</h2>
          {jobsLoading ? (
            <LoadingSpinner />
          ) : (jobs || []).length === 0 ? (
            <p className="empty-state">No jobs yet</p>
          ) : (
            (jobs || []).slice(0, 5).map((job) => <JobRow key={job.metadata?.name} job={job} />)
          )}
          <p>
            <Link to="/jobs" className="card-link">
              View all jobs ›
            </Link>
          </p>
        </section>

        {nodes && nodes.length > 0 ? (
          <section className="card">
            <p className="eyebrow">NODES</p>
            <h2 className="card-title">GPU nodes</h2>
            {nodes.slice(0, 6).map((node) => (
              <div key={node.metadata?.name} className="list-row">
                <span className={`dot ${node.status?.phase === 'Ready' ? 'ok' : node.status?.phase === 'Degraded' ? 'warn' : 'bad'}`} />
                <div className="grow">
                  <b>{node.spec?.nodeName || node.metadata?.name}</b>
                  <small>
                    {node.spec?.gpuType} × {node.spec?.gpuCount}
                  </small>
                </div>
              </div>
            ))}
            {nodes.length > 6 && (
              <p>
                <Link to="/nodes" className="card-link">
                  View all {nodes.length} nodes ›
                </Link>
              </p>
            )}
          </section>
        ) : null}

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

function HealthCheck({ label, tone, detail }: { label: string; tone: 'ok' | 'warn' | 'bad'; detail?: string }) {
  return (
    <div className="list-row">
      <span className={`dot ${tone}`} />
      <span className="grow">{label}</span>
      {detail && <span className="faint">{detail}</span>}
    </div>
  )
}
