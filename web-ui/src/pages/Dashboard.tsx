import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { FabricAIJob } from '@/types'
import GPUChart from '@/components/GPUChart'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import Reveal from '@/components/Reveal'
import { phaseTone } from '@/lib/phase'

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
        <p className="login-error" role="alert">
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
    <div className="apple-story-stack">
      <PageHero eyebrow="Dashboard" title="Your GPU cluster, at a glance." lede="Live capacity, jobs and node health." />

      <div className="apple-metric-band">
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

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 stack">
          <Reveal>
            <section className="card">
              <h2>GPU utilization</h2>
              <GPUChart
                data={gpuMetrics?.metrics?.map((m) => ({
                  time: new Date(m.timestamp).toLocaleTimeString(),
                  [`${m.node}-GPU${m.gpuIndex}`]: m.utilization,
                }))}
              />
            </section>
          </Reveal>

          <Reveal delay={80}>
            <section className="card">
              <div className="stat-head">
                <h2>Recent jobs</h2>
                <span className="faint">{totalJobs} total</span>
              </div>
              {jobsLoading ? (
                <LoadingSpinner />
              ) : (jobs || []).length === 0 ? (
                <div className="list-empty">No jobs yet</div>
              ) : (
                (jobs || []).slice(0, 5).map((job) => <JobRow key={job.metadata?.name} job={job} />)
              )}
              <p style={{ margin: '12px 0 0' }}>
                <Link to="/jobs" className="card-link">
                  View all jobs ›
                </Link>
              </p>
            </section>
          </Reveal>
        </div>

        <div className="stack">
          <Reveal>
            <section className="card">
              <h2>Cluster health</h2>
              <HealthCheck label="GPU nodes" tone={(clusterStats?.totalNodes || 0) > 0 ? 'ok' : 'bad'} detail={`${clusterStats?.totalNodes || 0} nodes`} />
              <HealthCheck label="GPU utilization" tone={utilization >= 95 ? 'bad' : utilization > 80 ? 'warn' : 'ok'} detail={`${utilization}%`} />
              <HealthCheck label="Failed jobs" tone={failedJobs.length === 0 ? 'ok' : 'bad'} detail={`${failedJobs.length}`} />
              <HealthCheck label="Available GPUs" tone={(clusterStats?.availableGPUs || 0) > 0 ? 'ok' : 'bad'} detail={`${clusterStats?.availableGPUs || 0}`} />
              <HealthCheck label="Job queue" tone={pendingJobs.length >= 10 ? 'bad' : pendingJobs.length > 5 ? 'warn' : 'ok'} detail={`${pendingJobs.length} pending`} />
            </section>
          </Reveal>

          {nodes && nodes.length > 0 && (
            <Reveal delay={80}>
              <section className="card">
                <h2>GPU nodes</h2>
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
                  <p style={{ margin: '12px 0 0' }}>
                    <Link to="/nodes" className="card-link">
                      View all {nodes.length} nodes ›
                    </Link>
                  </p>
                )}
              </section>
            </Reveal>
          )}

          <Reveal delay={160}>
            <section className="card">
              <h2>Quick actions</h2>
              <div className="row">
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
          </Reveal>
        </div>
      </div>
    </div>
  )
}

function JobRow({ job }: { job: FabricAIJob }) {
  const phase = job.status?.phase || 'Unknown'
  return (
    <Link to={`/jobs/${job.metadata?.name}`} className="list-row">
      <div className="grow">
        <b>{job.metadata?.name}</b>
        <small>
          {job.spec?.framework || 'job'} · {job.spec?.resources?.gpuType || 'GPU'} × {job.spec?.resources?.gpuCount || '?'}
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
