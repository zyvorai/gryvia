import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Package, CheckCircle, FlaskConical, Code2,
  RefreshCw, ChevronDown, ChevronRight, ExternalLink, Link as LinkIcon,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { RegisteredModel } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function ModelRegistry() {
  const { data: models, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['models'],
    queryFn: api.getModels,
    refetchInterval: 30000,
  })

  if (isError) return (
    <div className="text-center py-12">
      <p className="text-red-400">Failed to load model registry. Please try again.</p>
    </div>
  )

  const productionModels = models?.filter(m => m.spec?.stage === 'production') || []
  const stagingModels = models?.filter(m => m.spec?.stage === 'staging') || []
  const devModels = models?.filter(m => m.spec?.stage === 'dev') || []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Model Registry</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">Model versioning and promotion</p>
        </div>
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
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={Package} title="Total Models" value={models?.length || 0} color="blue" />
        <StatCard icon={CheckCircle} title="Production Models" value={productionModels.length} color="green" />
        <StatCard icon={FlaskConical} title="Staging" value={stagingModels.length} color="cyan" />
        <StatCard icon={Code2} title="Dev" value={devModels.length} color="purple" />
      </div>

      {/* Models Table */}
      <div className="rounded-xl" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="px-5 py-4" style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
          <div className="flex items-center gap-2">
            <Package className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">All Models</h3>
            <span className="text-xs text-[#5a7a9e] ml-auto">{models?.length || 0} total</span>
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
                <div className="col-span-1">Version</div>
                <div className="col-span-2">Stage</div>
                <div className="col-span-2">Source Job</div>
                <div className="col-span-2">Created</div>
                <div className="col-span-2">Actions</div>
              </div>

              {(models || []).map((model) => (
                <ModelRow key={`${model.metadata?.name}-${model.spec?.version}`} model={model} />
              ))}

              {(!models || models.length === 0) && (
                <div className="text-center py-8 text-sm text-[#344e6a]">No models registered</div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// --- Sub-components ---

const stageBadgeConfig: Record<string, { bg: string; text: string; border: string }> = {
  dev: { bg: 'rgba(95,168,211,0.08)', text: '#7ecbf5', border: 'rgba(95,168,211,0.2)' },
  staging: { bg: 'rgba(251,191,36,0.08)', text: '#fbbf24', border: 'rgba(251,191,36,0.2)' },
  production: { bg: 'rgba(34,197,94,0.08)', text: '#4ade80', border: 'rgba(34,197,94,0.2)' },
  archived: { bg: 'rgba(128,144,168,0.08)', text: '#8090a8', border: 'rgba(128,144,168,0.2)' },
}

function StageBadge({ stage }: { stage: string }) {
  const cfg = stageBadgeConfig[stage] || stageBadgeConfig.dev
  return (
    <span className="inline-flex items-center text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
      background: cfg.bg,
      color: cfg.text,
      border: `1px solid ${cfg.border}`,
    }}>
      {stage}
    </span>
  )
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
        <div className="col-span-1 text-xs text-[#8ba4c0]">{version}</div>
        <div className="col-span-2">
          <StageBadge stage={stage} />
        </div>
        <div className="col-span-2 text-xs text-[#8ba4c0] truncate">{model.spec?.sourceJob || '-'}</div>
        <div className="col-span-2 text-xs text-[#5a7a9e]">
          {model.metadata?.creationTimestamp ? new Date(model.metadata.creationTimestamp).toLocaleDateString() : '-'}
        </div>
        <div className="col-span-2 relative" onClick={(e) => e.stopPropagation()}>
          {promoteOptions[stage] && promoteOptions[stage].length > 0 && (
            <div className="relative">
              <button
                onClick={() => setPromoteOpen(!promoteOpen)}
                className="inline-flex items-center gap-1 px-2.5 py-1 btn-chrome rounded-lg text-xs font-medium"
              >
                Promote
                <ChevronDown className="h-3 w-3" />
              </button>
              {promoteOpen && (
                <div className="absolute right-0 top-full mt-1 w-36 rounded-lg py-1 z-20 animate-scale-in" style={{
                  background: 'linear-gradient(180deg, #151d28 0%, #111820 100%)',
                  border: '1px solid rgba(192,204,224,0.08)',
                  boxShadow: '0 15px 40px rgba(0,0,0,0.5)',
                }}>
                  {promoteOptions[stage].map((target) => (
                    <button
                      key={target}
                      onClick={() => promoteMutation.mutate({ name, targetStage: target })}
                      disabled={promoteMutation.isPending}
                      className="w-full text-left px-3 py-2 text-xs text-[#8ba4c0] hover:text-[#c0cce0] hover:bg-[#1a2332]/60 transition-colors disabled:opacity-50"
                    >
                      {stage} → {target}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Expanded detail */}
      {expanded && (
        <div className="px-8 py-4 mb-1 rounded-lg animate-fade-in" style={{
          background: 'rgba(10,14,20,0.4)',
          border: '1px solid rgba(192,204,224,0.04)',
        }}>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-1">Artifact Paths</div>
              {model.spec?.artifacts && model.spec.artifacts.length > 0 ? (
                <div className="space-y-1">
                  {model.spec.artifacts.map((a, i) => (
                    <div key={i} className="text-xs text-[#8ba4c0] font-mono truncate">{a}</div>
                  ))}
                </div>
              ) : (
                <div className="text-xs text-[#344e6a]">No artifacts</div>
              )}
            </div>
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-1">Source Job</div>
              {model.spec?.sourceJob ? (
                <a href={`/jobs/${model.spec.sourceJob}`} className="inline-flex items-center gap-1 text-xs text-[#e8a87c] hover:text-[#f0c4a0] transition-colors">
                  <LinkIcon className="h-3 w-3" />
                  {model.spec.sourceJob}
                </a>
              ) : (
                <div className="text-xs text-[#344e6a]">No source job</div>
              )}
            </div>
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#5a7a9e] mb-1">Serving Endpoint</div>
              {stage === 'production' && model.status?.servingEndpoint ? (
                <a href={model.status.servingEndpoint} target="_blank" rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-emerald-400 hover:text-emerald-300 transition-colors">
                  <ExternalLink className="h-3 w-3" />
                  {model.status.servingEndpoint}
                </a>
              ) : (
                <div className="text-xs text-[#344e6a]">{stage === 'production' ? 'Not deployed' : 'Available in production'}</div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  )
}
