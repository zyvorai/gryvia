import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { RegisteredModel } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function ModelRegistry() {
  const { data: models, isLoading, isError, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['models'],
    queryFn: api.getModels,
    refetchInterval: 30000,
  })

  if (isError) return (
    <>
      <PageHero eyebrow="Models" title="Registry unavailable." tint="red" />
      <p className="warning" role="alert">Failed to load model registry. Please try again.</p>
    </>
  )

  const productionModels = models?.filter(m => m.spec?.stage === 'production') || []
  const stagingModels = models?.filter(m => m.spec?.stage === 'staging') || []
  const devModels = models?.filter(m => m.spec?.stage === 'dev') || []

  return (
    <>
      <PageHero eyebrow="Models" title="Every model, versioned." lede="Model versioning and promotion." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={`${productionModels.length} of ${models?.length || 0} models in production.`}
          figures={[
            { label: 'Total models', value: models?.length || 0 },
            { label: 'Production', value: productionModels.length },
            { label: 'Staging', value: stagingModels.length },
            { label: 'Dev', value: devModels.length },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">Registry</p>
          <h2 className="card-title">All Models</h2>
          <div className="toolbar">
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
            <span className="faint">{models?.length || 0} total</span>
          </div>
        {isLoading ? (
          <LoadingSpinner />
        ) : (models || []).length === 0 ? (
          <p className="empty-state">No models registered</p>
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
    </>
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
      <tr className="table-row-hover" onClick={() => setExpanded(!expanded)}>
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
            <div className="toolbar">
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
            <div className="formgrid">
              <div>
                <div className="faint">Artifact Paths</div>
                {model.spec?.artifacts && model.spec.artifacts.length > 0 ? (
                  <div className="stack">
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
