import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { InferenceService } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import { phaseTone } from '@/lib/phase'
import { notify } from '@/lib/notify'
import { intError, nameError, parseIntStrict } from '@/lib/forms'
import type { FilterDef, SortAccessor } from '@/lib/tableState'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { useTableState } from '@/hooks/useTableState'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import Modal from '@/components/Modal'
import ConfirmDialog from '@/components/ConfirmDialog'
import CopyButton from '@/components/CopyButton'
import DataTable, { type Column } from '@/components/DataTable'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'

const BACKEND_LABELS: Record<string, string> = {
  triton: 'Triton',
  vllm: 'vLLM',
  'tensorrt-llm': 'TensorRT-LLM',
  torchserve: 'TorchServe',
}

/** The gateway stores backends lowercase (CRD values); show the product names. */
function backendLabel(backend?: string): string {
  if (!backend) return '—'
  return BACKEND_LABELS[backend.toLowerCase()] ?? backend
}

const nameOf = (s: InferenceService) => s.metadata?.name || 'unknown'
const phaseOf = (s: InferenceService) => s.status?.phase || 'Pending'

const FILTERS: FilterDef<InferenceService>[] = [
  { name: 'status', label: 'Status', get: phaseOf },
  { name: 'backend', label: 'Backend', get: (s) => s.spec?.backend },
]
const SORTS: Record<string, SortAccessor<InferenceService>> = {
  name: nameOf,
  model: (s) => s.spec?.modelRef,
  backend: (s) => s.spec?.backend,
  replicas: (s) => s.status?.readyReplicas ?? 0,
  status: phaseOf,
  created: (s) => (s.metadata?.creationTimestamp ? Date.parse(s.metadata.creationTimestamp) : undefined),
}
const searchText = (s: InferenceService) => `${nameOf(s)} ${s.spec?.modelRef ?? ''} ${backendLabel(s.spec?.backend)} ${phaseOf(s)}`
const DEFAULT_SORT = { key: 'name', dir: 'asc' as const }

export default function InferenceServices() {
  useDocumentTitle('Inference')
  const queryClient = useQueryClient()
  // /inference?new=1&model=<name> (from the model registry) opens the deploy dialog prefilled.
  const [params, setParams] = useSearchParams()
  const [showCreateForm, setShowCreateForm] = useState(() => params.get('new') === '1')
  const [prefillModel] = useState(() => params.get('model') ?? '')
  const [deleting, setDeleting] = useState<string | null>(null)
  const [openName, setOpenName] = useState<string | null>(null)

  const closeCreate = () => {
    setShowCreateForm(false)
    if (params.has('new') || params.has('model')) {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          next.delete('new')
          next.delete('model')
          return next
        },
        { replace: true },
      )
    }
  }

  const { data: services, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['inferenceServices'],
    queryFn: api.getInferenceServices,
    refetchInterval: 15000,
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.deleteInferenceService(name),
    onSuccess: (_d, name) => {
      notify.success(`Deleted ${name}`)
      setDeleting(null)
      return queryClient.invalidateQueries({ queryKey: ['inferenceServices'] })
    },
    onError: (err, name) => {
      notify.error(`Could not delete ${name}`, err)
      setDeleting(null)
    },
  })

  const table = useTableState({ rows: services, searchText, filters: FILTERS, sortAccessors: SORTS, defaultSort: DEFAULT_SORT })

  if (isError && !services) {
    return (
      <>
        <PageHero eyebrow="Inference" title="Services unavailable." tint="red" />
        <ErrorState title="Could not load inference services." error={error} onRetry={() => refetch()} retrying={isRefetching} />
      </>
    )
  }

  const openService = services?.find((s) => nameOf(s) === openName)
  const toggleOpen = (svc: InferenceService) => setOpenName(openName === nameOf(svc) ? null : nameOf(svc))
  const deletingNow = (name: string) => deleteMutation.isPending && deleteMutation.variables === name
  const columns: Column<InferenceService>[] = [
    {
      key: 'name',
      header: 'Name',
      sortable: true,
      render: (svc) => (
        <button
          type="button"
          className="th-sort"
          aria-expanded={openName === nameOf(svc)}
          aria-controls="service-detail"
          onClick={(e) => {
            e.stopPropagation()
            toggleOpen(svc)
          }}
        >
          <span className="faint" aria-hidden="true">{openName === nameOf(svc) ? '▾' : '▸'}</span>{' '}
          <b>{nameOf(svc)}</b>
        </button>
      ),
    },
    { key: 'model', header: 'Model', sortable: true, render: (svc) => <span className="muted">{svc.spec?.modelRef ?? '—'}</span> },
    { key: 'backend', header: 'Backend', sortable: true, render: (svc) => <span className="pill">{backendLabel(svc.spec?.backend)}</span> },
    {
      key: 'replicas',
      header: 'Replicas',
      sortable: true,
      numeric: true,
      render: (svc) => {
        const auto = svc.spec?.autoscaling
        return (
          <>
            <span>{svc.status?.readyReplicas ?? 0}/{svc.spec?.replicas ?? '—'}</span>
            {auto?.minReplicas !== undefined && auto?.maxReplicas !== undefined && <div className="faint">autoscale {auto.minReplicas}–{auto.maxReplicas}</div>}
          </>
        )
      },
    },
    { key: 'status', header: 'Status', sortable: true, render: (svc) => <span className={`pill ${phaseTone(phaseOf(svc))}`}>{phaseOf(svc)}</span> },
    {
      key: 'endpoint',
      header: 'Endpoint',
      render: (svc) =>
        svc.status?.endpoint ? (
          <div className="toolbar">
            <span className="mono muted">{svc.status.endpoint}</span>
            <CopyButton value={svc.status.endpoint} label="endpoint URL" />
          </div>
        ) : (
          <span className="faint">—</span>
        ),
    },
    {
      key: 'canary',
      header: 'Canary',
      render: (svc) => {
        const canary = svc.spec?.canary?.trafficPercent ?? 0
        return canary > 0 ? <span className="pill info">{canary}%</span> : <span className="faint">—</span>
      },
    },
    {
      key: 'actions',
      header: 'Actions',
      render: (svc) => (
        <button
          className="danger"
          onClick={(e) => {
            e.stopPropagation()
            setDeleting(nameOf(svc))
          }}
          disabled={deletingNow(nameOf(svc))}
          aria-label={`Delete service ${nameOf(svc)}`}
        >
          {deletingNow(nameOf(svc)) ? 'Deleting…' : 'Delete'}
        </button>
      ),
    },
  ]

  const isReady = (s: InferenceService) => ['running', 'ready'].includes((s.status?.phase ?? '').toLowerCase())
  const active = services?.filter(isReady).length
  const notReady = services && active !== undefined ? services.length - active : undefined
  const totalReplicas = services?.reduce((sum, s) => sum + (s.status?.readyReplicas ?? 0), 0)
  const canary = services?.filter((s) => (s.spec?.canary?.trafficPercent ?? 0) > 0).length

  return (
    <>
      <PageHero eyebrow="Inference" title="Serve models at scale." lede="Model serving management." />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={services ? `${active} of ${services.length} inference services ready.` : undefined}
          tone={notReady ? 'warn' : undefined}
          figures={[
            { label: 'Ready services', value: active },
            { label: 'Not ready', value: notReady },
            { label: 'Ready replicas', value: totalReplicas },
            { label: 'Canary deployments', value: canary },
          ]}
        />

        {isError && (
          <div className="span3">
            <ErrorState title="Showing the last data received; refreshing failed." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        )}

        <section className="card span3">
          <p className="eyebrow">Serving</p>
          <h2 className="card-title">All services</h2>
          {isLoading ? (
            <Skeleton rows={4} />
          ) : (
            <>
              {openService && <ServiceDetail service={openService} deleting={deletingNow(nameOf(openService))} onClose={() => setOpenName(null)} onDelete={() => setDeleting(nameOf(openService))} />}
              <DataTable
                caption="Inference services"
                columns={columns}
                state={table}
                rowKey={nameOf}
                onRowClick={toggleOpen}
                actions={
                  <>
                    <button className="primary" onClick={() => setShowCreateForm(true)}>
                      Deploy model
                    </button>
                    <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
                      Refresh
                    </button>
                  </>
                }
                empty={
                  <EmptyState
                    title="No inference services deployed."
                    action={
                      <button className="primary" onClick={() => setShowCreateForm(true)}>
                        Deploy a model
                      </button>
                    }
                  >
                    Serve a registered model behind an autoscaled endpoint.
                  </EmptyState>
                }
              />
            </>
          )}
        </section>
      </div>

      {deleting && (
        <ConfirmDialog
          title={`Delete service ${deleting}?`}
          confirmLabel="Delete service"
          busy={deleteMutation.isPending}
          onCancel={() => setDeleting(null)}
          onConfirm={() => deleteMutation.mutate(deleting)}
        >
          Deleting {deleting} shuts down its replicas and its endpoint stops serving requests.
        </ConfirmDialog>
      )}

      {showCreateForm && <CreateInferenceServiceModal onClose={closeCreate} initialModel={prefillModel} />}
    </>
  )
}

// --- Sub-components ---

function ServiceDetail({ service, onClose, onDelete, deleting }: { service: InferenceService; onClose: () => void; onDelete: () => void; deleting: boolean }) {
  const name = nameOf(service)
  const auto = service.spec?.autoscaling
  const endpoint = service.status?.endpoint
  const model = service.spec?.modelRef
  return (
    <div className="card" id="service-detail" role="region" aria-label={`${name} details`}>
      <div className="row">
        <p className="eyebrow">Service: {name}</p>
        <button type="button" className="btn-secondary" onClick={onClose}>
          Close details
        </button>
      </div>
      <div className="formgrid">
        <div>
          <div className="faint">Endpoint</div>
          {endpoint ? (
            <div className="toolbar">
              <span className="mono muted">{endpoint}</span>
              <CopyButton value={endpoint} label="endpoint URL" />
            </div>
          ) : (
            <span className="faint">Not available yet</span>
          )}
        </div>
        <div>
          <div className="faint">Replicas ready / desired</div>
          <span className="num">{service.status?.readyReplicas ?? 0} / {service.spec?.replicas ?? '—'}</span>
        </div>
        <div>
          <div className="faint">Autoscale range</div>
          <span className="num">{auto?.minReplicas !== undefined && auto?.maxReplicas !== undefined ? `${auto.minReplicas}–${auto.maxReplicas} replicas` : '—'}</span>
        </div>
        <div>
          <div className="faint">Target GPU utilization</div>
          <span className="num">{auto?.targetUtilization !== undefined ? `${auto.targetUtilization}%` : '—'}</span>
        </div>
        <div>
          <div className="faint">Model</div>
          {model ? (
            <Link to={`/models?q=${encodeURIComponent(model)}`} className="card-link">
              {model}
            </Link>
          ) : (
            <span className="faint">—</span>
          )}
        </div>
      </div>
      <div className="toolbar">
        <button className="danger" onClick={onDelete} disabled={deleting} aria-label={`Delete service ${name}`}>
          {deleting ? 'Deleting…' : 'Delete service'}
        </button>
      </div>
    </div>
  )
}

const INITIAL = {
  name: '',
  modelRef: '',
  backend: 'triton',
  minReplicas: '1',
  maxReplicas: '4',
  targetUtilization: '80',
}

function CreateInferenceServiceModal({ onClose, initialModel = '' }: { onClose: () => void; initialModel?: string }) {
  const queryClient = useQueryClient()
  const [initial] = useState(() => ({ ...INITIAL, modelRef: initialModel }))
  const [form, setForm] = useState(initial)
  const [touched, setTouched] = useState(false)
  const set = (patch: Partial<typeof INITIAL>) => setForm((f) => ({ ...f, ...patch }))
  const dirty = JSON.stringify(form) !== JSON.stringify(initial)

  const modelsQuery = useQuery({ queryKey: ['models'], queryFn: api.getModels })
  // A prefilled model stays selectable even before (or without) the registry listing it.
  const modelNames = Array.from(new Set([...(initialModel ? [initialModel] : []), ...(modelsQuery.data ?? []).map((m) => m.metadata?.name).filter((n): n is string => !!n)]))

  const createMutation = useMutation({
    mutationFn: (data: Parameters<typeof api.createInferenceService>[0]) => api.createInferenceService(data),
    onSuccess: (_d, vars) => {
      notify.success(`Deploying ${vars.name}`)
      queryClient.invalidateQueries({ queryKey: ['inferenceServices'] })
      onClose()
    },
    onError: (err) => notify.error('Could not deploy service', err),
  })

  const minN = parseIntStrict(form.minReplicas)
  const maxN = parseIntStrict(form.maxReplicas)
  const errors = {
    name: nameError(form.name),
    modelRef: form.modelRef ? null : 'Choose a model.',
    min: intError(form.minReplicas, 1, 100),
    max: intError(form.maxReplicas, 1, 100) ?? (minN !== null && maxN !== null && maxN < minN ? 'Must be at least the minimum.' : null),
    target: intError(form.targetUtilization, 1, 100),
  }
  const invalid = Object.values(errors).some(Boolean)
  const show = (e: string | null) => (touched ? e : null)
  const nameShown = touched || form.name !== '' ? errors.name : null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setTouched(true)
    if (invalid) return
    createMutation.mutate({
      name: form.name,
      modelRef: form.modelRef,
      backend: form.backend,
      // The gateway takes an initial replica count as well; start at the autoscaling minimum.
      replicas: minN as number,
      minReplicas: minN as number,
      maxReplicas: maxN as number,
      targetUtilization: parseIntStrict(form.targetUtilization) as number,
    })
  }

  return (
    <Modal title="Deploy inference service" onClose={onClose} dirty={dirty}>
      <form onSubmit={handleSubmit} className="stack" noValidate>
        <div className="formgrid">
          <label className="field">
            Service name
            <input
              type="text"
              data-autofocus
              value={form.name}
              onChange={(e) => set({ name: e.target.value })}
              placeholder="my-inference-svc"
              aria-invalid={!!nameShown}
              autoComplete="off"
            />
            {nameShown && <span className="warning">{nameShown}</span>}
          </label>
          <label className="field">
            Model
            <select value={form.modelRef} onChange={(e) => set({ modelRef: e.target.value })} disabled={modelNames.length === 0} aria-invalid={!!show(errors.modelRef)}>
              <option value="">{modelsQuery.isLoading ? 'Loading models…' : 'Select a model'}</option>
              {modelNames.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
            {show(errors.modelRef) && modelNames.length > 0 && <span className="warning">{errors.modelRef}</span>}
          </label>
        </div>

        {modelsQuery.isError && <ErrorState title="Could not load models." error={modelsQuery.error} onRetry={() => modelsQuery.refetch()} />}
        {!modelsQuery.isLoading && !modelsQuery.isError && modelNames.length === 0 && (
          <EmptyState title="No models registered yet.">
            Register a model first: it appears in the <Link to="/models" onClick={onClose}>model registry</Link> when a training job finishes.
          </EmptyState>
        )}

        <label className="field">
          Backend
          <select value={form.backend} onChange={(e) => set({ backend: e.target.value })}>
            {Object.entries(BACKEND_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>

        <div className="stack">
          <p className="eyebrow">Autoscaling</p>
          <div className="formgrid">
            <label className="field">
              Min replicas
              <input type="number" min={1} max={100} value={form.minReplicas} onChange={(e) => set({ minReplicas: e.target.value })} aria-invalid={!!show(errors.min)} />
              {show(errors.min) && <span className="warning">{errors.min}</span>}
            </label>
            <label className="field">
              Max replicas
              <input type="number" min={1} max={100} value={form.maxReplicas} onChange={(e) => set({ maxReplicas: e.target.value })} aria-invalid={!!show(errors.max)} />
              {show(errors.max) && <span className="warning">{errors.max}</span>}
            </label>
            <label className="field">
              Target GPU %
              <input type="number" min={1} max={100} value={form.targetUtilization} onChange={(e) => set({ targetUtilization: e.target.value })} aria-invalid={!!show(errors.target)} />
              {show(errors.target) && <span className="warning">{errors.target}</span>}
            </label>
          </div>
          <p className="faint">The service starts at the minimum and scales up to the maximum when GPU utilization passes the target.</p>
        </div>

        {createMutation.isError && (
          <p className="warning" role="alert">
            {errorMessage(createMutation.error)}
          </p>
        )}

        <div className="toolbar">
          <button type="button" className="btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="primary" disabled={createMutation.isPending || modelNames.length === 0}>
            {createMutation.isPending ? 'Deploying…' : 'Deploy service'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
