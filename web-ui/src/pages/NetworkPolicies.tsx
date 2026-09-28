import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { FlowPolicy, CreateFlowPolicyRequest } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import toast from 'react-hot-toast'

export default function NetworkPolicies() {
  const [showCreateForm, setShowCreateForm] = useState(false)
  const queryClient = useQueryClient()

  const { data: policies, isLoading, isError } = useQuery({
    queryKey: ['flowPolicies'],
    queryFn: api.getFlowPolicies,
    refetchInterval: 15000,
  })

  const applyMutation = useMutation({
    mutationFn: api.applyFlowPolicy,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flowPolicies'] })
      toast.success('Policy applied successfully')
    },
    onError: () => {
      toast.error('Failed to apply policy')
    },
  })

  const createMutation = useMutation({
    mutationFn: api.createFlowPolicy,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flowPolicies'] })
      setShowCreateForm(false)
      toast.success('Policy created successfully')
    },
    onError: () => {
      toast.error('Failed to create policy')
    },
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Network" title="Policies unavailable." tint="red" />
        <p className="login-error" role="alert">
          Failed to load flow policies. Please check your API connection.
        </p>
      </>
    )
  }

  const enforcedPolicies = policies?.filter((p) => p.status?.phase === 'Enforced' || p.status?.phase === 'Active') || []
  const pendingPolicies = policies?.filter((p) => p.status?.phase === 'Pending') || []
  const suggestedPolicies = policies?.filter((p) => p.status?.phase === 'Suggested') || []

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Network" title="Flow policies." lede="Manage network flow policies and suggestions" />

      <div className="row">
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          className={showCreateForm ? 'btn-secondary' : 'primary'}
        >
          {showCreateForm ? 'Cancel' : 'Create Policy'}
        </button>
        <Link to="/network" className="buttonlike btn-secondary">
          Back to network
        </Link>
      </div>

      {showCreateForm && (
        <CreatePolicyForm
          onSubmit={(data) => createMutation.mutate(data)}
          isSubmitting={createMutation.isPending}
        />
      )}

      {suggestedPolicies.length > 0 && (
        <section className="card">
          <div className="stat-head">
            <h2>Suggested Policies</h2>
            <span className="pill info">{suggestedPolicies.length} suggestions</span>
          </div>
          {suggestedPolicies.map((policy) => (
            <SuggestedPolicyCard
              key={policy.metadata.name}
              policy={policy}
              onApply={() => applyMutation.mutate(policy.metadata.name)}
              isApplying={applyMutation.isPending}
            />
          ))}
        </section>
      )}

      <section className="card">
        <div className="stat-head">
          <h2>Enforced Policies</h2>
          <span className="pill ok">{enforcedPolicies.length} active</span>
        </div>
        {enforcedPolicies.length === 0 ? (
          <div className="list-empty">No enforced policies</div>
        ) : (
          enforcedPolicies.map((policy) => (
            <PolicyCard key={policy.metadata.name} policy={policy} />
          ))
        )}
      </section>

      {pendingPolicies.length > 0 && (
        <section className="card">
          <div className="stat-head">
            <h2>Pending Policies</h2>
            <span className="pill warn">{pendingPolicies.length} pending</span>
          </div>
          {pendingPolicies.map((policy) => (
            <PolicyCard key={policy.metadata.name} policy={policy} />
          ))}
        </section>
      )}
    </div>
  )
}

// --- Sub-components ---

function ActionPill({ action }: { action: string }) {
  return <span className={`pill ${action === 'allow' ? 'ok' : 'bad'}`}>{action.toUpperCase()}</span>
}

function PolicyCard({ policy }: { policy: FlowPolicy }) {
  return (
    <div className="list-row">
      <div className="grow">
        <b>{policy.metadata.name}</b>
        <small>
          {policy.spec.sourceService} → {policy.spec.destinationService} · {policy.spec.protocol}:{policy.spec.port}
        </small>
        <small>
          Intent: {policy.spec.intent}
          {policy.status?.matchedFlows !== undefined && ` · ${policy.status.matchedFlows} matched flows`}
        </small>
      </div>
      <ActionPill action={policy.spec.action} />
      <PolicyPhaseBadge phase={policy.status?.phase || 'Pending'} />
    </div>
  )
}

function SuggestedPolicyCard({ policy, onApply, isApplying }: {
  policy: FlowPolicy
  onApply: () => void
  isApplying: boolean
}) {
  return (
    <div className="list-row">
      <div className="grow">
        <b>{policy.metadata.name}</b>
        <small>
          {policy.spec.sourceService} → {policy.spec.destinationService} · {policy.spec.protocol}:{policy.spec.port}
        </small>
        <small>
          Intent: {policy.spec.intent}
          {policy.spec.confidence ? ` · ${Math.round(policy.spec.confidence * 100)}% confidence` : ''}
        </small>
      </div>
      <ActionPill action={policy.spec.action} />
      <button className="btn-secondary" onClick={onApply} disabled={isApplying}>
        Apply
      </button>
    </div>
  )
}

function PolicyPhaseBadge({ phase }: { phase: string }) {
  const tones: Record<string, string> = {
    Enforced: 'ok',
    Active: 'ok',
    Pending: 'warn',
    Suggested: 'info',
  }
  return <span className={`pill ${tones[phase] ?? 'warn'}`}>{phase}</span>
}

function CreatePolicyForm({ onSubmit, isSubmitting }: {
  onSubmit: (data: CreateFlowPolicyRequest) => void
  isSubmitting: boolean
}) {
  const [form, setForm] = useState<CreateFlowPolicyRequest>({
    sourceService: '',
    destinationService: '',
    port: 80,
    protocol: 'TCP',
    action: 'allow',
    intent: '',
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.sourceService || !form.destinationService || !form.intent) {
      toast.error('Please fill in all required fields')
      return
    }
    onSubmit(form)
  }

  return (
    <section className="card">
      <h2>Create Flow Policy</h2>
      <form onSubmit={handleSubmit} className="stack">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <label className="stack">
            <span className="faint">Source Service *</span>
            <input
              type="text"
              value={form.sourceService}
              onChange={(e) => setForm({ ...form, sourceService: e.target.value })}
              placeholder="e.g., frontend"
            />
          </label>
          <label className="stack">
            <span className="faint">Destination Service *</span>
            <input
              type="text"
              value={form.destinationService}
              onChange={(e) => setForm({ ...form, destinationService: e.target.value })}
              placeholder="e.g., payment-api"
            />
          </label>
          <label className="stack">
            <span className="faint">Port</span>
            <input
              type="number"
              value={form.port}
              onChange={(e) => setForm({ ...form, port: parseInt(e.target.value) || 0 })}
            />
          </label>
          <label className="stack">
            <span className="faint">Protocol</span>
            <select value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}>
              <option value="TCP">TCP</option>
              <option value="UDP">UDP</option>
              <option value="HTTP">HTTP</option>
              <option value="gRPC">gRPC</option>
            </select>
          </label>
          <label className="stack">
            <span className="faint">Action</span>
            <select
              value={form.action}
              onChange={(e) => setForm({ ...form, action: e.target.value as 'allow' | 'deny' })}
            >
              <option value="allow">Allow</option>
              <option value="deny">Deny</option>
            </select>
          </label>
          <label className="stack">
            <span className="faint">Intent *</span>
            <input
              type="text"
              value={form.intent}
              onChange={(e) => setForm({ ...form, intent: e.target.value })}
              placeholder="e.g., allow payment processing"
            />
          </label>
        </div>

        <div className="list-row">
          <div className="grow">
            <small>Policy Preview</small>
            <b>
              {form.sourceService || '(source)'} → {form.destinationService || '(destination)'}
            </b>
          </div>
          <span className="pill info">{form.protocol}:{form.port}</span>
          <ActionPill action={form.action} />
        </div>

        <div className="row">
          <button type="submit" disabled={isSubmitting} className="primary">
            {isSubmitting ? 'Creating...' : 'Create Policy'}
          </button>
        </div>
      </form>
    </section>
  )
}
