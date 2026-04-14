import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Shield, ArrowLeft, Plus, ArrowRight, CheckCircle,
  Clock, Zap, X,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { FlowPolicy, CreateFlowPolicyRequest } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
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
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
        Failed to load flow policies. Please check your API connection.
      </div>
    )
  }

  const enforcedPolicies = policies?.filter((p) => p.status?.phase === 'Enforced' || p.status?.phase === 'Active') || []
  const pendingPolicies = policies?.filter((p) => p.status?.phase === 'Pending') || []
  const suggestedPolicies = policies?.filter((p) => p.status?.phase === 'Suggested') || []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link to="/network" className="p-1.5 rounded-lg transition-colors hover:bg-[#1a2332]/60 text-[#5a7a9e] hover:text-[#c0cce0]">
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <div>
            <h2 className="text-2xl font-bold text-gradient-copper">Flow Policies</h2>
            <p className="text-sm text-[#5a7a9e] mt-1">Manage network flow policies and suggestions</p>
          </div>
        </div>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          className="btn-copper text-xs px-3 py-1.5 rounded-lg inline-flex items-center gap-1.5"
        >
          {showCreateForm ? <X className="h-3.5 w-3.5" /> : <Plus className="h-3.5 w-3.5" />}
          {showCreateForm ? 'Cancel' : 'Create Policy'}
        </button>
      </div>

      {/* Create Policy Form */}
      {showCreateForm && (
        <CreatePolicyForm
          onSubmit={(data) => createMutation.mutate(data)}
          isSubmitting={createMutation.isPending}
        />
      )}

      {/* Suggested Policies */}
      {suggestedPolicies.length > 0 && (
        <div className="rounded-xl overflow-hidden" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="px-5 py-4 flex items-center justify-between" style={{ borderBottom: '1px solid rgba(192,204,224,0.04)' }}>
            <div className="flex items-center gap-2">
              <Zap className="h-4 w-4 text-cyan-400" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Suggested Policies</h3>
            </div>
            <span className="text-[10px] font-medium px-2 py-0.5 rounded-full"
              style={{ background: 'rgba(6,182,212,0.08)', color: '#22d3ee' }}
            >
              {suggestedPolicies.length} suggestions
            </span>
          </div>
          <div className="px-5 py-4 space-y-3">
            {suggestedPolicies.map((policy) => (
              <SuggestedPolicyCard
                key={policy.metadata.name}
                policy={policy}
                onApply={() => applyMutation.mutate(policy.metadata.name)}
                isApplying={applyMutation.isPending}
              />
            ))}
          </div>
        </div>
      )}

      {/* Enforced Policies */}
      <div className="rounded-xl overflow-hidden" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="px-5 py-4 flex items-center justify-between" style={{ borderBottom: '1px solid rgba(192,204,224,0.04)' }}>
          <div className="flex items-center gap-2">
            <Shield className="h-4 w-4 text-emerald-400" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">Enforced Policies</h3>
          </div>
          <span className="text-[10px] font-medium px-2 py-0.5 rounded-full"
            style={{ background: 'rgba(34,197,94,0.08)', color: '#4ade80' }}
          >
            {enforcedPolicies.length} active
          </span>
        </div>
        <div className="px-5 py-4">
          {enforcedPolicies.length === 0 ? (
            <div className="text-center py-6 text-sm text-[#344e6a]">No enforced policies</div>
          ) : (
            <div className="space-y-3">
              {enforcedPolicies.map((policy) => (
                <PolicyCard key={policy.metadata.name} policy={policy} />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Pending Policies */}
      {pendingPolicies.length > 0 && (
        <div className="rounded-xl overflow-hidden" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="px-5 py-4 flex items-center justify-between" style={{ borderBottom: '1px solid rgba(192,204,224,0.04)' }}>
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-[#fbbf24]" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Pending Policies</h3>
            </div>
            <span className="text-[10px] font-medium px-2 py-0.5 rounded-full"
              style={{ background: 'rgba(251,191,36,0.08)', color: '#fbbf24' }}
            >
              {pendingPolicies.length} pending
            </span>
          </div>
          <div className="px-5 py-4 space-y-3">
            {pendingPolicies.map((policy) => (
              <PolicyCard key={policy.metadata.name} policy={policy} />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// --- Sub-components ---

function PolicyCard({ policy }: { policy: FlowPolicy }) {
  const isAllow = policy.spec.action === 'allow'
  return (
    <div className="p-3 rounded-lg card-glow" style={{
      background: 'rgba(10,14,20,0.5)',
      border: '1px solid rgba(192,204,224,0.04)',
    }}>
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-medium text-[#c0cce0]">{policy.metadata.name}</span>
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
            background: isAllow ? 'rgba(34,197,94,0.08)' : 'rgba(239,68,68,0.08)',
            color: isAllow ? '#4ade80' : '#f87171',
          }}>
            {policy.spec.action.toUpperCase()}
          </span>
          <PolicyPhaseBadge phase={policy.status?.phase || 'Pending'} />
        </div>
      </div>
      <div className="flex items-center gap-2 text-xs text-[#8ba4c0] mb-1">
        <span className="font-medium text-[#b0c4d8]">{policy.spec.sourceService}</span>
        <ArrowRight className="h-3 w-3 text-[#5a7a9e]" />
        <span className="font-medium text-[#b0c4d8]">{policy.spec.destinationService}</span>
        <span className="text-[10px] px-1.5 py-0.5 rounded" style={{
          background: 'rgba(95,168,211,0.08)',
          color: '#7ecbf5',
        }}>
          {policy.spec.protocol}:{policy.spec.port}
        </span>
      </div>
      <div className="flex items-center justify-between text-[10px] text-[#5a7a9e]">
        <span>Intent: {policy.spec.intent}</span>
        {policy.status?.matchedFlows !== undefined && (
          <span>{policy.status.matchedFlows} matched flows</span>
        )}
      </div>
    </div>
  )
}

function SuggestedPolicyCard({ policy, onApply, isApplying }: {
  policy: FlowPolicy
  onApply: () => void
  isApplying: boolean
}) {
  const isAllow = policy.spec.action === 'allow'
  return (
    <div className="p-3 rounded-lg" style={{
      background: 'rgba(6,182,212,0.04)',
      border: '1px solid rgba(6,182,212,0.1)',
    }}>
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-medium text-[#c0cce0]">{policy.metadata.name}</span>
        <div className="flex items-center gap-2">
          {policy.spec.confidence && (
            <span className="text-[10px] text-[#5a7a9e]">
              {Math.round(policy.spec.confidence * 100)}% confidence
            </span>
          )}
          <button
            onClick={onApply}
            disabled={isApplying}
            className="btn-copper text-[10px] px-2.5 py-1 rounded-lg inline-flex items-center gap-1 disabled:opacity-50"
          >
            <CheckCircle className="h-3 w-3" />
            Apply
          </button>
        </div>
      </div>
      <div className="flex items-center gap-2 text-xs text-[#8ba4c0] mb-1">
        <span className="font-medium text-[#b0c4d8]">{policy.spec.sourceService}</span>
        <ArrowRight className="h-3 w-3 text-[#5a7a9e]" />
        <span className="font-medium text-[#b0c4d8]">{policy.spec.destinationService}</span>
        <span className="text-[10px] px-1.5 py-0.5 rounded" style={{
          background: 'rgba(95,168,211,0.08)',
          color: '#7ecbf5',
        }}>
          {policy.spec.protocol}:{policy.spec.port}
        </span>
        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
          background: isAllow ? 'rgba(34,197,94,0.08)' : 'rgba(239,68,68,0.08)',
          color: isAllow ? '#4ade80' : '#f87171',
        }}>
          {policy.spec.action.toUpperCase()}
        </span>
      </div>
      <div className="text-[10px] text-[#5a7a9e]">
        Intent: {policy.spec.intent}
      </div>
    </div>
  )
}

function PolicyPhaseBadge({ phase }: { phase: string }) {
  const styles: Record<string, { bg: string; color: string }> = {
    Enforced: { bg: 'rgba(34,197,94,0.08)', color: '#4ade80' },
    Active: { bg: 'rgba(34,197,94,0.08)', color: '#4ade80' },
    Pending: { bg: 'rgba(251,191,36,0.08)', color: '#fbbf24' },
    Suggested: { bg: 'rgba(6,182,212,0.08)', color: '#22d3ee' },
  }
  const s = styles[phase] || styles.Pending
  return (
    <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
      background: s.bg,
      color: s.color,
    }}>
      {phase}
    </span>
  )
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

  const inputStyle = {
    background: 'rgba(10,14,20,0.5)',
    border: '1px solid rgba(192,204,224,0.06)',
  }

  return (
    <div className="rounded-xl p-5" style={{
      background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
      border: '1px solid rgba(212,118,78,0.15)',
      boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04), 0 0 20px rgba(212,118,78,0.05)',
    }}>
      <div className="flex items-center gap-2 mb-4">
        <Plus className="h-4 w-4 text-[#e8a87c]" />
        <h3 className="text-sm font-semibold text-[#e8ecf1]">Create Flow Policy</h3>
      </div>
      <form onSubmit={handleSubmit}>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Source Service *</label>
            <input
              type="text"
              value={form.sourceService}
              onChange={(e) => setForm({ ...form, sourceService: e.target.value })}
              placeholder="e.g., frontend"
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] placeholder-[#344e6a] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={inputStyle}
            />
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Destination Service *</label>
            <input
              type="text"
              value={form.destinationService}
              onChange={(e) => setForm({ ...form, destinationService: e.target.value })}
              placeholder="e.g., payment-api"
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] placeholder-[#344e6a] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={inputStyle}
            />
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Port</label>
            <input
              type="number"
              value={form.port}
              onChange={(e) => setForm({ ...form, port: parseInt(e.target.value) || 0 })}
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] placeholder-[#344e6a] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={inputStyle}
            />
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Protocol</label>
            <select
              value={form.protocol}
              onChange={(e) => setForm({ ...form, protocol: e.target.value })}
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={inputStyle}
            >
              <option value="TCP">TCP</option>
              <option value="UDP">UDP</option>
              <option value="HTTP">HTTP</option>
              <option value="gRPC">gRPC</option>
            </select>
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Action</label>
            <select
              value={form.action}
              onChange={(e) => setForm({ ...form, action: e.target.value as 'allow' | 'deny' })}
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={inputStyle}
            >
              <option value="allow">Allow</option>
              <option value="deny">Deny</option>
            </select>
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Intent *</label>
            <input
              type="text"
              value={form.intent}
              onChange={(e) => setForm({ ...form, intent: e.target.value })}
              placeholder="e.g., allow payment processing"
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] placeholder-[#344e6a] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={inputStyle}
            />
          </div>
        </div>

        {/* Preview */}
        <div className="p-3 rounded-lg mb-4" style={{
          background: 'rgba(10,14,20,0.5)',
          border: '1px solid rgba(192,204,224,0.04)',
        }}>
          <div className="text-[10px] text-[#5a7a9e] mb-1">Policy Preview</div>
          <div className="flex items-center gap-2 text-xs">
            <span className="text-[#b0c4d8] font-medium">{form.sourceService || '(source)'}</span>
            <ArrowRight className="h-3 w-3 text-[#5a7a9e]" />
            <span className="text-[#b0c4d8] font-medium">{form.destinationService || '(destination)'}</span>
            <span className="text-[10px] px-1.5 py-0.5 rounded" style={{
              background: 'rgba(95,168,211,0.08)', color: '#7ecbf5',
            }}>
              {form.protocol}:{form.port}
            </span>
            <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
              background: form.action === 'allow' ? 'rgba(34,197,94,0.08)' : 'rgba(239,68,68,0.08)',
              color: form.action === 'allow' ? '#4ade80' : '#f87171',
            }}>
              {form.action.toUpperCase()}
            </span>
          </div>
        </div>

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={isSubmitting}
            className="btn-copper text-xs px-4 py-2 rounded-lg inline-flex items-center gap-1.5 disabled:opacity-50"
          >
            <Shield className="h-3.5 w-3.5" />
            {isSubmitting ? 'Creating...' : 'Create Policy'}
          </button>
        </div>
      </form>
    </div>
  )
}
