import { useState } from 'react'
import axios from 'axios'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import ConfirmDialog from '@/components/ConfirmDialog'
import { ErrorState, Skeleton } from '@/components/StateViews'
import { useNow } from '@/lib/useNow'
import { phaseTone } from '@/lib/phase'
import { durationBetween, formatDate, formatRelative } from '@/lib/format'
import { errorMessage } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { envSource, isSensitiveEnv, jobFramework, type EnvEntry } from '@/lib/jobs'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

const shortDate = (iso?: string) => (iso ? formatDate(iso) : 'Not started')

async function copyText(label: string, text: string | undefined) {
  if (!text) {
    notify.error(`No ${label} to copy`)
    return
  }
  try {
    await navigator.clipboard.writeText(text)
    notify.success(`Copied ${label}`)
  } catch (err) {
    notify.error(`Could not copy the ${label}`, err)
  }
}

export default function JobDetails() {
  const { name } = useParams<{ name: string }>()
  useDocumentTitle(name ? `Job ${name}` : 'Job')
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const now = useNow()
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [revealed, setRevealed] = useState<Set<number>>(new Set())

  const { data: job, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['job', name],
    queryFn: () => api.getJob(name!),
    enabled: !!name,
    refetchInterval: 5000,
    retry: (count, err) => !(axios.isAxiosError(err) && err.response?.status === 404) && count < 2,
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteJob(name!),
    onSuccess: () => {
      notify.success(`Deleted job ${name}`)
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      queryClient.removeQueries({ queryKey: ['job', name] })
      navigate('/jobs')
    },
    onError: (err) => {
      setConfirmDelete(false)
      notify.error(`Could not delete job ${name}`, err)
    },
  })

  if (isLoading) {
    return (
      <>
        <PageHero eyebrow="Job" title={name ?? 'Job'} />
        <Skeleton rows={5} />
      </>
    )
  }

  const notFound = axios.isAxiosError(error) && error.response?.status === 404
  if ((isError && !job) || (!job && !isLoading)) {
    if (notFound || !isError) {
      return (
        <>
          <PageHero eyebrow="Job" title="Job not found" lede={name ? `There is no job named ${name}. It may have been deleted.` : undefined} />
          <Link to="/jobs" className="buttonlike primary">
            Back to jobs
          </Link>
        </>
      )
    }
    return (
      <>
        <PageHero eyebrow="Job" title="Job unavailable" tint="red" />
        <ErrorState title="Could not load this job." error={error} onRetry={() => refetch()} retrying={isRefetching} />
        <p>
          <Link to="/jobs" className="card-link">
            Back to jobs
          </Link>
        </p>
      </>
    )
  }
  if (!job) return null

  const spec = job.spec
  const phase = job.status?.phase || 'Unknown'
  const command = spec?.command?.join(' ')
  const env = (spec?.env ?? []) as EnvEntry[]
  const labels = Object.entries(job.metadata?.labels ?? {})
  const created = job.metadata?.creationTimestamp

  const toggleReveal = (idx: number) =>
    setRevealed((prev) => {
      const next = new Set(prev)
      if (next.has(idx)) next.delete(idx)
      else next.add(idx)
      return next
    })

  return (
    <>
      <PageHero
        eyebrow="Job"
        title={job.metadata.name}
        lede={job.status?.message || `${phase}${created ? ` · created ${formatRelative(created, now)}` : ''}`}
      />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={`${job.metadata.name} is ${phase.toLowerCase()}.`}
          tone={phaseTone(phase) === 'bad' ? 'bad' : undefined}
          figures={[
            { label: 'Created', value: formatDate(created) },
            { label: 'Started', value: shortDate(job.status?.startTime) },
            {
              label: 'Duration',
              value: job.status?.startTime ? durationBetween(job.status.startTime, job.status.completionTime, now) : 'Not started',
            },
            { label: 'GPU type', value: spec?.gpuType || '—' },
            { label: 'GPU count', value: spec?.gpus },
            { label: 'Memory', value: spec?.resources?.requests?.memory ?? '—' },
            { label: 'CPU cores', value: spec?.resources?.requests?.cpu ?? '—' },
          ]}
        />

        {isError && (
          <div className="span3">
            <ErrorState title="Could not refresh this job; showing the last data." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        )}

        <section className="card span3">
          <p className="eyebrow">STATUS</p>
          <h2 className="card-title">Job actions</h2>
          <div className="toolbar">
            <Link to="/jobs" className="buttonlike btn-secondary">
              Back to jobs
            </Link>
            <span className={`pill ${phaseTone(phase)}`}>{phase}</span>
            <button type="button" className="btn-secondary" onClick={() => copyText('job name', job.metadata.name)}>
              Copy name
            </button>
            <button type="button" className="btn-secondary" disabled={!spec?.image} onClick={() => copyText('image', spec?.image)}>
              Copy image
            </button>
            <button type="button" className="btn-secondary" disabled={!command} onClick={() => copyText('command', command)}>
              Copy command
            </button>
            <button type="button" className="danger" onClick={() => setConfirmDelete(true)}>
              Delete
            </button>
          </div>
        </section>

        <section className="card span2">
          <p className="eyebrow">CONFIGURATION</p>
          <h2 className="card-title">Job configuration</h2>
          <div className="formgrid">
            <div>
              <p className="faint">Framework</p>
              <p>{jobFramework(job)}</p>
            </div>
            <div>
              <p className="faint">Image</p>
              <p className="mono">{spec?.image || '—'}</p>
            </div>
            {spec?.distributed?.enabled && (
              <>
                <div>
                  <p className="faint">Distributed training</p>
                  <p>Enabled</p>
                </div>
                {spec.distributed.nodes && (
                  <div>
                    <p className="faint">Nodes</p>
                    <p>{spec.distributed.nodes}</p>
                  </div>
                )}
                {spec.distributed.gpusPerNode && (
                  <div>
                    <p className="faint">GPUs/node</p>
                    <p>{spec.distributed.gpusPerNode}</p>
                  </div>
                )}
              </>
            )}
          </div>
        </section>

        <section className="card">
          <p className="eyebrow">ENTRYPOINT</p>
          <h2 className="card-title">Command</h2>
          <code className="mono">{command || '—'}</code>
        </section>

        {env.length > 0 && (
          <section className="card span2">
            <p className="eyebrow">ENVIRONMENT</p>
            <h2 className="card-title">Environment variables</h2>
            {env.map((e, idx) => {
              const source = envSource(e)
              const sensitive = !source && isSensitiveEnv(e.name)
              const shown = revealed.has(idx)
              const literal = e.value ?? ''
              return (
                <div key={`${e.name}-${idx}`} className="list-row">
                  <span className="grow">{e.name}</span>
                  <span className="mono muted">
                    {source ?? (sensitive && !shown ? '********' : literal === '' ? '(empty)' : literal)}
                  </span>
                  {sensitive && (
                    <button type="button" className="btn-secondary" aria-pressed={shown} aria-label={`${shown ? 'Hide' : 'Reveal'} value of ${e.name}`} onClick={() => toggleReveal(idx)}>
                      {shown ? 'Hide' : 'Reveal'}
                    </button>
                  )}
                </div>
              )
            })}
          </section>
        )}

        {labels.length > 0 && (
          <section className="card">
            <p className="eyebrow">METADATA</p>
            <h2 className="card-title">Labels</h2>
            <div className="row">
              {labels.map(([key, value]) => (
                <span key={key} className="pill mono" style={{ textTransform: 'none' }}>
                  {key}={value}
                </span>
              ))}
            </div>
          </section>
        )}
      </div>

      {confirmDelete && (
        <ConfirmDialog
          title={`Delete job ${job.metadata.name}?`}
          confirmLabel="Delete job"
          busy={deleteMutation.isPending}
          onCancel={() => setConfirmDelete(false)}
          onConfirm={() => deleteMutation.mutate()}
        >
          Job <b>{job.metadata.name}</b> and its pods will be removed. This cannot be undone.
        </ConfirmDialog>
      )}
    </>
  )
}
