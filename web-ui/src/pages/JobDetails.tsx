import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { useNow } from '@/lib/useNow'
import { phaseTone } from '@/lib/phase'
import { jobFramework } from '@/lib/jobs'

const shortDate = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })

export default function JobDetails() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const now = useNow()

  const { data: job, isLoading, isError, dataUpdatedAt } = useQuery({
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
        <p className="warning" role="alert">Failed to load job details. Please try again.</p>
      </>
    )
  }

  if (!job) {
    return <PageHero eyebrow="Job" title="Job not found." />
  }

  const formatDuration = (start?: string, end?: string) => {
    if (!start) return 'N/A'
    const startTime = new Date(start).getTime()
    const endTime = end ? new Date(end).getTime() : now
    const duration = endTime - startTime
    const hours = Math.floor(duration / 3600000)
    const minutes = Math.floor((duration % 3600000) / 60000)
    return `${hours}h ${minutes}m`
  }

  const phase = job.status?.phase || 'Unknown'
  const sensitivePatterns = /SECRET|PASSWORD|TOKEN|KEY|CREDENTIAL|API_KEY|PRIVATE|AUTH|ACCESS_ID|DATABASE_URL|CONN/i

  return (
    <>
      <PageHero
        eyebrow="Job"
        title={`${job.metadata.name}.`}
        lede={job.status?.message}
      />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={`${job.metadata.name} is ${phase.toLowerCase()}.`}
          tone={phase === 'Failed' ? 'bad' : undefined}
          figures={[
            { label: 'Created', value: job.metadata.creationTimestamp ? shortDate(job.metadata.creationTimestamp) : 'N/A' },
            { label: 'Started', value: job.status?.startTime ? shortDate(job.status.startTime) : 'N/A' },
            { label: 'Duration', value: formatDuration(job.status?.startTime, job.status?.completionTime) },
            { label: 'GPU type', value: job.spec.gpuType || '-' },
            { label: 'GPU count', value: job.spec.gpus },
            { label: 'Memory', value: job.spec.resources?.requests?.memory ?? '-' },
            { label: 'CPU cores', value: job.spec.resources?.requests?.cpu ?? '-' },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">STATUS</p>
          <h2 className="card-title">Job actions</h2>
          <div className="toolbar">
            <button className="btn-secondary" onClick={() => navigate('/jobs')}>
              Back to Jobs
            </button>
            <span className={`pill ${phaseTone(phase)}`}>{phase}</span>
          </div>
        </section>

        <section className="card span2">
          <p className="eyebrow">CONFIGURATION</p>
          <h2 className="card-title">Job Configuration</h2>
          <div className="formgrid">
            <div>
              <p className="faint">Framework</p>
              <p>{jobFramework(job)}</p>
            </div>
            <div>
              <p className="faint">Image</p>
              <p className="mono">{job.spec.image}</p>
            </div>
            {job.spec.distributed?.enabled && (
              <>
                <div>
                  <p className="faint">Distributed Training</p>
                  <p>Enabled</p>
                </div>
                {job.spec.distributed.nodes && (
                  <div>
                    <p className="faint">Nodes</p>
                    <p>{job.spec.distributed.nodes}</p>
                  </div>
                )}
                {job.spec.distributed.gpusPerNode && (
                  <div>
                    <p className="faint">GPUs/Node</p>
                    <p>{job.spec.distributed.gpusPerNode}</p>
                  </div>
                )}
              </>
            )}
          </div>
        </section>

        <section className="card">
          <p className="eyebrow">ENTRYPOINT</p>
          <h2 className="card-title">Command</h2>
          <code className="mono">{job.spec.command?.join(' ') || 'N/A'}</code>
        </section>

        {job.spec.env && job.spec.env.length > 0 && (
          <section className="card span2">
            <p className="eyebrow">ENVIRONMENT</p>
            <h2 className="card-title">Environment Variables</h2>
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
            <p className="eyebrow">METADATA</p>
            <h2 className="card-title">Labels</h2>
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
    </>
  )
}
