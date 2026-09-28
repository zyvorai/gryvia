import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import { phaseTone } from '@/components/JobsTable'

export default function JobDetails() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()

  const { data: job, isLoading, isError } = useQuery({
    queryKey: ['job', name],
    queryFn: () => api.getJob(name!),
    enabled: !!name,
    refetchInterval: 5000,
  })

  if (isLoading) {
    return <LoadingSpinner />
  }

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Job" title="Job unavailable." tint="red" />
        <p className="login-error" role="alert">Failed to load job details. Please try again.</p>
      </>
    )
  }

  if (!job) {
    return <PageHero eyebrow="Job" title="Job not found." />
  }

  const formatDuration = (start?: string, end?: string) => {
    if (!start) return 'N/A'
    const startTime = new Date(start).getTime()
    const endTime = end ? new Date(end).getTime() : Date.now()
    const duration = endTime - startTime
    const hours = Math.floor(duration / 3600000)
    const minutes = Math.floor((duration % 3600000) / 60000)
    return `${hours}h ${minutes}m`
  }

  const phase = job.status?.phase || 'Unknown'
  const sensitivePatterns = /SECRET|PASSWORD|TOKEN|KEY|CREDENTIAL|API_KEY|PRIVATE|AUTH|ACCESS_ID|DATABASE_URL|CONN/i

  return (
    <div className="apple-story-stack">
      <PageHero
        eyebrow="Job"
        title={`${job.metadata.name}.`}
        lede={job.status?.message}
      />

      <div className="page-actions">
        <button className="btn-secondary" onClick={() => navigate('/jobs')}>
          Back to Jobs
        </button>
        <span className={`pill ${phaseTone(phase)}`}>{phase}</span>
      </div>

      <div className="apple-metric-band">
        <div>
          <span>Created</span>
          <b style={{ fontSize: 'var(--fs-h3)' }}>
            {job.metadata.creationTimestamp ? new Date(job.metadata.creationTimestamp).toLocaleString() : 'N/A'}
          </b>
        </div>
        <div>
          <span>Started</span>
          <b style={{ fontSize: 'var(--fs-h3)' }}>
            {job.status?.startTime ? new Date(job.status.startTime).toLocaleString() : 'N/A'}
          </b>
        </div>
        <div>
          <span>Duration</span>
          <b>{formatDuration(job.status?.startTime, job.status?.completionTime)}</b>
        </div>
      </div>

      <section className="card">
        <h2>Resource Allocation</h2>
        <div className="apple-metric-band">
          <div>
            <span>GPU Type</span>
            <b>{job.spec.resources.gpuType}</b>
          </div>
          <div>
            <span>GPU Count</span>
            <b>{job.spec.resources.gpuCount}</b>
          </div>
          <div>
            <span>Memory</span>
            <b>{job.spec.resources.memory}</b>
          </div>
          <div>
            <span>CPU Cores</span>
            <b>{job.spec.resources.cpu}</b>
          </div>
        </div>
      </section>

      <section className="card">
        <h2>Job Configuration</h2>
        <div className="card-grid">
          <div>
            <span className="faint">Framework</span>
            <p>{job.spec.framework}</p>
          </div>
          <div>
            <span className="faint">Image</span>
            <p className="mono">{job.spec.image}</p>
          </div>
          {job.spec.distributed?.enabled && (
            <>
              <div>
                <span className="faint">Distributed Training</span>
                <p>Enabled</p>
              </div>
              <div>
                <span className="faint">Strategy</span>
                <p>{job.spec.distributed.strategy}</p>
              </div>
              {job.spec.distributed.nodes && (
                <div>
                  <span className="faint">Nodes</span>
                  <p>{job.spec.distributed.nodes}</p>
                </div>
              )}
              {job.spec.distributed.gpusPerNode && (
                <div>
                  <span className="faint">GPUs/Node</span>
                  <p>{job.spec.distributed.gpusPerNode}</p>
                </div>
              )}
            </>
          )}
        </div>
      </section>

      <section className="card">
        <h2>Command</h2>
        <code className="mono">{job.spec.command?.join(' ') || 'N/A'}</code>
      </section>

      {job.spec.env && job.spec.env.length > 0 && (
        <section className="card">
          <h2>Environment Variables</h2>
          {job.spec.env.map((env, idx) => {
            const isSensitive = sensitivePatterns.test(env.name)
            return (
              <div key={idx} className="list-row">
                <span className="grow">{env.name}</span>
                <span className="mono muted">{isSensitive ? '********' : env.value || ''}</span>
              </div>
            )
          })}
        </section>
      )}

      {job.metadata.labels && Object.keys(job.metadata.labels).length > 0 && (
        <section className="card">
          <h2>Labels</h2>
          <div className="row">
            {Object.entries(job.metadata.labels).map(([key, value]) => (
              <span key={key} className="pill">
                {key}: {value}
              </span>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}
