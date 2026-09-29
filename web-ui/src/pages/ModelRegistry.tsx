import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { RegisteredModel } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import { formatDate, formatRelative } from '@/lib/format'
import { notify } from '@/lib/notify'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import ConfirmDialog from '@/components/ConfirmDialog'
import CopyButton from '@/components/CopyButton'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { TableCaption } from '@/components/TableCaption'

export default function ModelRegistry() {
  useDocumentTitle('Models')
  const { data: models, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['models'],
    queryFn: api.getModels,
    refetchInterval: 30000,
  })

  if (isError && !models) {
    return (
      <>
        <PageHero eyebrow="Models" title="Registry unavailable." tint="red" />
        <ErrorState title="Could not load the model registry." error={error} onRetry={() => refetch()} retrying={isRefetching} />
      </>
    )
  }

  const count = (stage: string) => models?.filter((m) => m.spec?.stage === stage).length

  return (
    <>
      <PageHero eyebrow="Models" title="Every model, versioned." lede="Model versioning and promotion." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={models ? `${count('production')} of ${models.length} models in production.` : undefined}
          figures={[
            { label: 'Total models', value: models?.length },
            { label: 'Production', value: count('production') },
            { label: 'Staging', value: count('staging') },
            { label: 'Dev', value: count('dev') },
          ]}
        />

        {isError && (
          <div className="span3">
            <ErrorState title="Showing the last data received; refreshing failed." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        )}

        <section className="card span3">
          <p className="eyebrow">Registry</p>
          <h2 className="card-title">All models</h2>
          <div className="toolbar">
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
            {models && <span className="faint">{models.length} total</span>}
          </div>
          {isLoading ? (
            <Skeleton rows={4} />
          ) : (models ?? []).length === 0 ? (
            <EmptyState
              title="No models registered yet."
              action={
                <Link to="/jobs/new" className="buttonlike primary">
                  Submit a training job
                </Link>
              }
            >
              Models are registered from training jobs: when a job finishes, its output shows up here.
            </EmptyState>
          ) : (
            <div className="table-wrap">
              <table>
                <TableCaption>Registered models</TableCaption>
                <thead>
                  <tr>
                    <th scope="col">Name</th>
                    <th scope="col">Version</th>
                    <th scope="col">Stage</th>
                    <th scope="col">Source job</th>
                    <th scope="col">Created</th>
                    <th scope="col">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {(models ?? []).map((model) => (
                    <ModelRow key={`${model.metadata?.name}-${model.spec?.version}`} model={model} />
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

const stageTone: Record<string, string> = {
  dev: '',
  staging: 'info',
  production: 'ok',
  archived: '',
}

const PROMOTE_TARGETS: Record<string, string> = {
  dev: 'staging',
  staging: 'production',
  production: 'archived',
}

function StageBadge({ stage }: { stage?: string }) {
  if (!stage) return <span className="faint">—</span>
  return <span className={`pill ${stageTone[stage] ?? ''}`}>{stage}</span>
}

function ModelRow({ model }: { model: RegisteredModel }) {
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = useState(false)
  const [confirming, setConfirming] = useState(false)

  const name = model.metadata?.name || 'unknown'
  const stage = model.spec?.stage
  const version = model.spec?.version
  const sourceJob = model.spec?.sourceJob
  const target = stage ? PROMOTE_TARGETS[stage] : undefined
  const created = model.metadata?.creationTimestamp
  const endpoint = model.status?.servingEndpoint

  const promoteMutation = useMutation({
    mutationFn: (targetStage: string) => api.promoteModel(name, targetStage),
    onSuccess: (_d, targetStage) => {
      notify.success(`Promoted ${name} to ${targetStage}`)
      queryClient.invalidateQueries({ queryKey: ['models'] })
      setConfirming(false)
    },
    // The dialog stays open and shows the gateway's reason (e.g. a 409 conflict) inline too.
    onError: (err) => notify.error(`Could not promote ${name}`, err),
  })

  const toggle = () => setExpanded((v) => !v)
  const dangerous = target === 'production' || target === 'archived'

  return (
    <>
      <tr
        className="table-row-hover"
        data-clickable
        tabIndex={0}
        aria-expanded={expanded}
        onClick={toggle}
        onKeyDown={(e) => {
          if (e.target !== e.currentTarget) return
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            toggle()
          }
        }}
      >
        <td>
          <span className="faint" aria-hidden="true">{expanded ? '▾' : '▸'}</span>{' '}
          <b>{name}</b>
        </td>
        <td className="muted">{version ?? '—'}</td>
        <td>
          <StageBadge stage={stage} />
        </td>
        <td className="muted">
          {sourceJob ? (
            <Link to={`/jobs/${encodeURIComponent(sourceJob)}`} className="card-link" onClick={(e) => e.stopPropagation()}>
              {sourceJob}
            </Link>
          ) : (
            '—'
          )}
        </td>
        <td className="faint" title={formatDate(created)}>
          {formatRelative(created)}
        </td>
        <td onClick={(e) => e.stopPropagation()}>
          {target && (
            <button
              className="btn-secondary"
              onClick={() => {
                promoteMutation.reset()
                setConfirming(true)
              }}
              disabled={promoteMutation.isPending}
              aria-label={`Promote ${name} to ${target}`}
            >
              {promoteMutation.isPending ? 'Promoting…' : `Promote to ${target}`}
            </button>
          )}
        </td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan={6}>
            <div className="formgrid">
              <div>
                <div className="faint">Artifact paths</div>
                {model.spec?.artifacts && model.spec.artifacts.length > 0 ? (
                  <div className="stack">
                    {model.spec.artifacts.map((a) => (
                      <div key={a} className="toolbar">
                        <span className="mono muted">{a}</span>
                        <CopyButton value={a} label="artifact path" />
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="faint">No artifacts</div>
                )}
              </div>
              <div>
                <div className="faint">Source job</div>
                {sourceJob ? (
                  <Link to={`/jobs/${encodeURIComponent(sourceJob)}`} className="card-link">
                    {sourceJob}
                  </Link>
                ) : (
                  <div className="faint">No source job</div>
                )}
              </div>
              <div>
                <div className="faint">Serving endpoint</div>
                {stage === 'production' && endpoint ? (
                  <div className="toolbar">
                    <span className="mono muted">{endpoint}</span>
                    <CopyButton value={endpoint} label="endpoint" />
                  </div>
                ) : (
                  <div className="faint">{stage === 'production' ? 'Not deployed' : 'Available in production'}</div>
                )}
              </div>
            </div>
          </td>
        </tr>
      )}

      {confirming && target && (
        <ConfirmDialog
          title={`Promote ${name} to ${target}?`}
          confirmLabel={`Promote to ${target}`}
          tone={dangerous ? 'danger' : 'primary'}
          busy={promoteMutation.isPending}
          onCancel={() => setConfirming(false)}
          onConfirm={() => promoteMutation.mutate(target)}
        >
          <p>
            {name}
            {version ? ` (${version})` : ''} moves from <b>{stage}</b> to <b>{target}</b>.
          </p>
          {target === 'production' && <p>Production is the version consumers are expected to serve.</p>}
          {target === 'archived' && <p>Archived models are retired and cannot be promoted again from here.</p>}
          {promoteMutation.isError && (
            <p className="warning" role="alert">
              {errorMessage(promoteMutation.error)}
            </p>
          )}
        </ConfirmDialog>
      )}
    </>
  )
}
