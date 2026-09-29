import { useId, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { FlowPolicy, CreateFlowPolicyRequest } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import ConfirmDialog from '@/components/ConfirmDialog'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { notify } from '@/lib/notify'
import { errorMessage } from '@/lib/errors'
import { phaseTone } from '@/lib/phase'
import { bucketPolicies, INTENT_MAX, validatePolicyForm, type PolicyFormErrors, type PolicyFormValues } from '@/lib/network'

type PolicyAction = 'allow' | 'deny' | 'log'
type PolicyStatusExt = { phase?: string; matchedFlows?: number; ciliumPolicyRef?: string }

const phaseOf = (p: FlowPolicy) => p.status?.phase

export default function NetworkPolicies() {
  useDocumentTitle('Network policies')
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [confirming, setConfirming] = useState<FlowPolicy | null>(null)
  const queryClient = useQueryClient()

  const { data: policies, isLoading, isError, error, refetch, isFetching, dataUpdatedAt } = useQuery({
    queryKey: ['flowPolicies'],
    queryFn: api.getFlowPolicies,
    refetchInterval: 15000,
  })

  const applyMutation = useMutation({
    mutationFn: api.applyFlowPolicy,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flowPolicies'] })
      notify.success('Apply requested — waiting for operator')
    },
    onError: (err) => notify.error('Could not request apply', err),
    onSettled: () => setConfirming(null),
  })

  const createMutation = useMutation({
    mutationFn: (req: CreateFlowPolicyRequest) => api.createFlowPolicy(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flowPolicies'] })
      setShowCreateForm(false)
      notify.success('Policy created')
    },
    onError: (err) => notify.error('Could not create policy', err),
  })

  const applyingName = applyMutation.isPending ? (applyMutation.variables as string) : undefined
  const buckets = bucketPolicies(policies, phaseOf)
  const total = policies?.length

  return (
    <>
      <PageHero eyebrow="Network" title="Flow policies." lede="Manage network flow policies and suggestions" />

      <div className="grid">
        <div className="toolbar span3">
          {!showCreateForm && (
            <button type="button" className="primary" onClick={() => setShowCreateForm(true)}>
              New policy
            </button>
          )}
          <Link to="/network" className="buttonlike btn-secondary">
            Back to network
          </Link>
        </div>

        {isLoading ? (
          <section className="card span3" aria-label="Live summary">
            <Skeleton rows={2} />
          </section>
        ) : isError && !policies ? (
          <div className="span3">
            <ErrorState title="Could not load flow policies." error={error} onRetry={() => refetch()} retrying={isFetching} />
          </div>
        ) : (
          <PagePulse
            updatedAt={dataUpdatedAt}
            error={isError ? errorMessage(error) : undefined}
            headline={
              buckets.suggested.length > 0
                ? `${buckets.suggested.length} suggested polic${buckets.suggested.length === 1 ? 'y' : 'ies'} awaiting review.`
                : `${buckets.enforced.length} polic${buckets.enforced.length === 1 ? 'y' : 'ies'} enforced.`
            }
            tone={buckets.suggested.length > 0 || buckets.pending.length > 0 ? 'warn' : undefined}
            figures={[
              { label: 'enforced', value: buckets.enforced.length },
              { label: 'pending', value: buckets.pending.length, tone: buckets.pending.length > 0 ? 'warn' : undefined },
              { label: 'suggested', value: buckets.suggested.length },
              ...(buckets.other.length > 0 ? [{ label: 'other', value: buckets.other.length }] : []),
              { label: 'total', value: total },
            ]}
          />
        )}

        {showCreateForm && (
          <CreatePolicyForm
            onSubmit={(data) => createMutation.mutate(data)}
            onCancel={() => setShowCreateForm(false)}
            isSubmitting={createMutation.isPending}
            serverError={createMutation.isError ? errorMessage(createMutation.error) : undefined}
          />
        )}

        {policies && (
          <>
            {buckets.suggested.length > 0 && (
              <section className="card span3">
                <p className="eyebrow">{buckets.suggested.length} SUGGESTIONS</p>
                <h2 className="card-title">Suggested policies</h2>
                {buckets.suggested.map((policy) => (
                  <PolicyRow key={policy.metadata.name} policy={policy} onApply={() => setConfirming(policy)} applying={applyingName === policy.metadata.name} />
                ))}
              </section>
            )}

            <section className="card span3">
              <p className="eyebrow">{buckets.enforced.length} ENFORCED</p>
              <h2 className="card-title">Enforced policies</h2>
              {buckets.enforced.length === 0 ? (
                <EmptyState
                  title="No enforced policies"
                  action={
                    !showCreateForm && (
                      <button type="button" className="primary" onClick={() => setShowCreateForm(true)}>
                        New policy
                      </button>
                    )
                  }
                >
                  Policies are FabricFlowPolicy resources; the network operator enforces them. Create one, or wait for the operator to suggest policies from observed traffic.
                </EmptyState>
              ) : (
                buckets.enforced.map((policy) => <PolicyRow key={policy.metadata.name} policy={policy} />)
              )}
            </section>

            {buckets.pending.length > 0 && (
              <section className="card span3">
                <p className="eyebrow">{buckets.pending.length} PENDING</p>
                <h2 className="card-title">Pending policies</h2>
                {buckets.pending.map((policy) => (
                  <PolicyRow key={policy.metadata.name} policy={policy} />
                ))}
              </section>
            )}

            {buckets.other.length > 0 && (
              <section className="card span3">
                <p className="eyebrow">{buckets.other.length} OTHER</p>
                <h2 className="card-title">Other policies</h2>
                {buckets.other.map((policy) => (
                  <PolicyRow key={policy.metadata.name} policy={policy} />
                ))}
              </section>
            )}
          </>
        )}
      </div>

      {confirming && (
        <ConfirmDialog
          title={`Apply policy ${confirming.metadata.name}?`}
          tone="primary"
          confirmLabel="Request apply"
          busy={applyMutation.isPending}
          onCancel={() => setConfirming(null)}
          onConfirm={() => applyMutation.mutate(confirming.metadata.name)}
        >
          This will {(confirming.spec.action as string) === 'log' ? 'log (observe only)' : confirming.spec.action} {confirming.spec.protocol}:{confirming.spec.port} traffic from {confirming.spec.sourceService} to {confirming.spec.destinationService}. Enforcement is requested from the network operator; the policy stays pending until the operator confirms it.
        </ConfirmDialog>
      )}
    </>
  )
}

function ActionPill({ action }: { action: string }) {
  const tone = action === 'deny' ? 'bad' : action === 'log' ? 'info' : ''
  return <span className={`pill ${tone}`}>{action ? action.charAt(0).toUpperCase() + action.slice(1) : '—'}</span>
}

function PhaseBadge({ phase }: { phase: string }) {
  const tone = phase.toLowerCase() === 'suggested' ? 'info' : phaseTone(phase)
  return <span className={`pill ${tone}`}>{phase}</span>
}

function PolicyRow({ policy, onApply, applying }: { policy: FlowPolicy; onApply?: () => void; applying?: boolean }) {
  const status = policy.status as PolicyStatusExt | undefined
  const phase = status?.phase || 'Pending'
  const confidence = typeof policy.spec.confidence === 'number' && policy.spec.confidence > 0 ? policy.spec.confidence : undefined
  return (
    <div className="list-row">
      <div className="grow">
        <b>{policy.metadata.name}</b>
        <small>
          {policy.spec.sourceService} → {policy.spec.destinationService} · {policy.spec.protocol}:{policy.spec.port}
        </small>
        <small>
          Intent: {policy.spec.intent || '—'}
          {status?.matchedFlows !== undefined && ` · ${status.matchedFlows} matched flows`}
          {confidence !== undefined && ` · ${Math.round(confidence * 100)}% confidence`}
        </small>
        {status?.ciliumPolicyRef && (
          <small>
            Cilium policy: <span className="mono">{status.ciliumPolicyRef}</span>
          </small>
        )}
      </div>
      <ActionPill action={policy.spec.action} />
      <PhaseBadge phase={phase} />
      {onApply && (
        <button type="button" className="btn-secondary" onClick={onApply} disabled={applying} aria-label={`Apply policy ${policy.metadata.name}`}>
          {applying ? 'Requesting…' : 'Apply'}
        </button>
      )}
    </div>
  )
}

function CreatePolicyForm({ onSubmit, onCancel, isSubmitting, serverError }: {
  onSubmit: (data: CreateFlowPolicyRequest) => void
  onCancel: () => void
  isSubmitting: boolean
  serverError?: string
}) {
  const uid = useId()
  const [values, setValues] = useState<PolicyFormValues>({ name: '', sourceService: '', destinationService: '', port: '80', intent: '' })
  const [protocol, setProtocol] = useState('TCP')
  const [action, setAction] = useState<PolicyAction>('allow')
  const [submitted, setSubmitted] = useState(false)

  const errors: PolicyFormErrors = submitted ? validatePolicyForm(values) : {}
  const set = (k: keyof PolicyFormValues) => (e: React.ChangeEvent<HTMLInputElement>) => setValues({ ...values, [k]: e.target.value })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (Object.keys(validatePolicyForm(values)).length > 0) return
    const req = {
      sourceService: values.sourceService.trim(),
      destinationService: values.destinationService.trim(),
      port: Number(values.port),
      protocol,
      action,
      intent: values.intent.trim(),
      ...(values.name.trim() ? { name: values.name.trim() } : {}),
    }
    onSubmit(req as unknown as CreateFlowPolicyRequest)
  }

  const field = (k: keyof PolicyFormValues, hint?: string) => {
    const hintId = `${uid}-${k}-hint`
    const errId = `${uid}-${k}-err`
    const describedBy = [hint ? hintId : '', errors[k] ? errId : ''].filter(Boolean).join(' ') || undefined
    return {
      'aria-invalid': errors[k] ? true : undefined,
      'aria-describedby': describedBy,
      hintNode: hint ? (
        <small id={hintId} className="faint">
          {hint}
        </small>
      ) : null,
      errNode: errors[k] ? (
        <small id={errId} className="text-bad" role="alert">
          {errors[k]}
        </small>
      ) : null,
    }
  }
  const fName = field('name', 'Optional. Lowercase letters, digits and "-"; generated if left empty. Created in the gateway’s default namespace.')
  const fSrc = field('sourceService', 'Lowercase Kubernetes name, up to 63 characters.')
  const fDst = field('destinationService', 'Lowercase Kubernetes name, up to 63 characters.')
  const fPort = field('port')
  const fIntent = field('intent', `Plain-language purpose, up to ${INTENT_MAX} characters.`)

  return (
    <section className="card span3">
      <p className="eyebrow">NEW</p>
      <h2 className="card-title">Create flow policy</h2>
      <form onSubmit={handleSubmit} className="stack" noValidate>
        {serverError && (
          <div className="warning" role="alert">
            <strong>The gateway rejected this policy.</strong>
            <div>{serverError}</div>
          </div>
        )}
        <div className="formgrid">
          <label className="field">
            <span>Name</span>
            <input type="text" value={values.name} onChange={set('name')} placeholder="e.g., frontend-to-payment" aria-invalid={fName['aria-invalid']} aria-describedby={fName['aria-describedby']} />
            {fName.hintNode}
            {fName.errNode}
          </label>
          <label className="field">
            <span>Source service *</span>
            <input type="text" value={values.sourceService} onChange={set('sourceService')} placeholder="e.g., frontend" required aria-invalid={fSrc['aria-invalid']} aria-describedby={fSrc['aria-describedby']} />
            {fSrc.hintNode}
            {fSrc.errNode}
          </label>
          <label className="field">
            <span>Destination service *</span>
            <input type="text" value={values.destinationService} onChange={set('destinationService')} placeholder="e.g., payment-api" required aria-invalid={fDst['aria-invalid']} aria-describedby={fDst['aria-describedby']} />
            {fDst.hintNode}
            {fDst.errNode}
          </label>
          <label className="field">
            <span>Port *</span>
            <input type="number" min={1} max={65535} step={1} value={values.port} onChange={set('port')} required aria-invalid={fPort['aria-invalid']} aria-describedby={fPort['aria-describedby']} />
            {fPort.errNode}
          </label>
          <label className="field">
            <span>Protocol</span>
            <select value={protocol} onChange={(e) => setProtocol(e.target.value)}>
              <option value="TCP">TCP</option>
              <option value="UDP">UDP</option>
              <option value="HTTP">HTTP</option>
              <option value="gRPC">gRPC</option>
            </select>
          </label>
          <label className="field">
            <span>Action</span>
            <select value={action} onChange={(e) => setAction(e.target.value as PolicyAction)} aria-describedby={`${uid}-action-hint`}>
              <option value="allow">Allow</option>
              <option value="deny">Deny</option>
              <option value="log">Log</option>
            </select>
            <small id={`${uid}-action-hint`} className="faint">
              log = observe only, safe to trial
            </small>
          </label>
          <label className="field span-all">
            <span>Intent *</span>
            <input type="text" value={values.intent} onChange={set('intent')} placeholder="e.g., allow payment processing" maxLength={INTENT_MAX + 50} required aria-invalid={fIntent['aria-invalid']} aria-describedby={fIntent['aria-describedby']} />
            {fIntent.hintNode}
            {fIntent.errNode}
          </label>
        </div>

        <div className="list-row">
          <div className="grow">
            <small>Policy preview</small>
            <b>
              {values.sourceService || '(source)'} → {values.destinationService || '(destination)'}
            </b>
          </div>
          <span className="pill info">
            {protocol}:{values.port || '—'}
          </span>
          <ActionPill action={action} />
        </div>

        <div className="toolbar">
          <button type="submit" disabled={isSubmitting} className="primary">
            {isSubmitting ? 'Creating…' : 'Create policy'}
          </button>
          <button type="button" className="btn-secondary" onClick={onCancel} disabled={isSubmitting}>
            Cancel
          </button>
        </div>
      </form>
    </section>
  )
}
