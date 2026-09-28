import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { RegisteredModel } from '@/lib/api'
import PageHero from '@/components/PageHero'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function ModelRegistry() {
  const { data: models, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['models'],
    queryFn: api.getModels,
    refetchInterval: 30000,
  })

  if (isError) return (
    <>
      <PageHero eyebrow="Models" title="Registry unavailable." tint="red" />
      <p className="login-error" role="alert">Failed to load model registry. Please try again.</p>
    </>
  )

  const productionModels = models?.filter(m => m.spec?.stage === 'production') || []
  const stagingModels = models?.filter(m => m.spec?.stage === 'staging') || []
  const devModels = models?.filter(m => m.spec?.stage === 'dev') || []

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Models" title="Every model, versioned." lede="Model versioning and promotion." />

      <div className="page-actions">
        <button className="btn-secondary" onClick={() => refetch()} disabled={isRefetching}>
          Refresh
        </button>
      </div>

      <div className="apple-metric-band">
        <div><span>Total Models</span><b>{models?.length || 0}</b></div>
        <div><span>Production Models</span><b>{productionModels.length}</b></div>
        <div><span>Staging</span><b>{stagingModels.length}</b></div>
        <div><span>Dev</span><b>{devModels.length}</b></div>
      </div>

      <section className="card">
        <div className="stat-head">
          <h2>All Models</h2>
          <span className="faint">{models?.length || 0} total</span>
        </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (models || []).length === 0 ? (
          <div className="list-empty">No models registered</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Version</th>
                  <th>Stage</th>
                  <th>Source Job</th>
                  <th>Created</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {(models || []).map((model) => (
                  <ModelRow key={`${model.metadata?.name}-${model.spec?.version}`} model={model} />
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

const stageTone: Record<string, string> = {
  dev: 'info',
  staging: 'warn',
  production: 'ok',
  archived: '',
}

function StageBadge({ stage }: { stage: string }) {
  return <span className={`pill ${stageTone[stage] ?? 'info'}`}>{stage}</span>
}

function ModelRow({ model }: { model: RegisteredModel }) {
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = useState(false)
  const [promoteOpen, setPromoteOpen] = useState(false)

  const promoteMutation = useMutation({
    mutationFn: ({ name, targetStage }: { name: string; targetStage: string }) =>
      api.promoteModel(name, targetStage),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      setPromoteOpen(false)
    },
  })

  const name = model.metadata?.name || 'unknown'
  const stage = model.spec?.stage || 'dev'
  const version = model.spec?.version || 'v1'

  const promoteOptions: Record<string, string[]> = {
    dev: ['staging'],
    staging: ['production'],
    production: ['archived'],
  }

  return (
    <>
      <tr className="table-row-hover" style={{ cursor: 'pointer' }} onClick={() => setExpanded(!expanded)}>
        <td>
          <span className="faint" aria-hidden="true">{expanded ? '▾' : '▸'}</span>{' '}
          <b>{name}</b>
        </td>
        <td className="muted">{version}</td>
        <td>
          <StageBadge stage={stage} />
        </td>
        <td className="muted">{model.spec?.sourceJob || '-'}</td>
        <td className="faint">
          {model.metadata?.creationTimestamp ? new Date(model.metadata.creationTimestamp).toLocaleDateString() : '-'}
        </td>
        <td onClick={(e) => e.stopPropagation()}>
          {promoteOptions[stage] && promoteOptions[stage].length > 0 && (
            <div className="row" style={{ gap: 8 }}>
              <button className="btn-secondary" onClick={() => setPromoteOpen(!promoteOpen)}>
                Promote
              </button>
              {promoteOpen &&
                promoteOptions[stage].map((target) => (
                  <button
                    key={target}
                    className="btn-secondary"
                    onClick={() => promoteMutation.mutate({ name, targetStage: target })}
                    disabled={promoteMutation.isPending}
                  >
                    {stage} → {target}
                  </button>
                ))}
            </div>
          )}
        </td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan={6}>
            <div className="card-grid">
              <div>
                <div className="faint">Artifact Paths</div>
                {model.spec?.artifacts && model.spec.artifacts.length > 0 ? (
                  <div className="stack" style={{ gap: 4 }}>
                    {model.spec.artifacts.map((a, i) => (
                      <div key={i} className="mono muted">{a}</div>
                    ))}
                  </div>
                ) : (
                  <div className="faint">No artifacts</div>
                )}
              </div>
              <div>
                <div className="faint">Source Job</div>
                {model.spec?.sourceJob ? (
                  <a href={`/jobs/${model.spec.sourceJob}`} className="card-link">
                    {model.spec.sourceJob}
                  </a>
                ) : (
                  <div className="faint">No source job</div>
                )}
              </div>
              <div>
                <div className="faint">Serving Endpoint</div>
                {stage === 'production' && model.status?.servingEndpoint ? (
                  <a href={model.status.servingEndpoint} target="_blank" rel="noopener noreferrer" className="card-link">
                    {model.status.servingEndpoint}
                  </a>
                ) : (
                  <div className="faint">{stage === 'production' ? 'Not deployed' : 'Available in production'}</div>
                )}
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}
