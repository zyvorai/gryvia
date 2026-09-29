// Pure helpers for the Invoices page. Invoices are estimates built from job run time and catalog rates.
export interface InvoiceLine { sku: string; gpuType: string; gpuHours: number; rate: number; amount: number; jobs: number }
export interface Invoice {
  number: string
  tenant: string
  period: { from: string; to: string }
  currency: string
  status: 'estimate' | string
  generatedAt?: string
  lines: InvoiceLine[]
  subtotal: number
  jobs: number
  open?: boolean
  note?: string
}
export interface InvoiceReport { month: string; items: Invoice[] }

const MONTH_RE = /^(\d{4})-(0[1-9]|1[0-2])$/

export function isValidMonth(m: string | null | undefined): m is string {
  return !!m && MONTH_RE.test(m)
}

/** The current UTC month as YYYY-MM, matching the gateway's default. */
export function currentMonth(now: Date = new Date()): string {
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`
}

/** A valid month from the URL, or the current UTC month. */
export function parseMonth(raw: string | null | undefined, now: Date = new Date()): string {
  return isValidMonth(raw) ? raw : currentMonth(now)
}

/** "September 2026" for display; the input itself when it is not a month. */
export function monthLabel(m: string): string {
  const match = MONTH_RE.exec(m)
  if (!match) return m
  return new Intl.DateTimeFormat('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' }).format(new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, 1)))
}

/** Query params for GET /invoices; blank tenant is dropped. */
export function invoiceParams(month: string, tenant?: string): Record<string, string> {
  const out: Record<string, string> = { month }
  if (tenant) out.tenant = tenant
  return out
}

export function invoiceFilename(tenant: string, month: string, format: 'csv' | 'json'): string {
  return `gryvia-invoice-${tenant}-${month}.${format}`
}

/** Money with the invoice's currency; two decimals for amounts. */
export function formatMoney(amount?: number | null, currency = 'USD'): string {
  if (amount === undefined || amount === null || !Number.isFinite(amount)) return '—'
  try {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency, minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(amount)
  } catch {
    return `${amount.toFixed(2)} ${currency}`
  }
}

/** "Total (estimate)" wording for the subtotal row: running jobs make it provisional. */
export function invoiceTotalLabel(inv: Pick<Invoice, 'open'>): string {
  return inv.open ? 'Subtotal (running jobs included)' : 'Subtotal'
}

export function invoiceTotalDisplay(inv: Pick<Invoice, 'subtotal' | 'currency'>): string {
  return formatMoney(inv.subtotal, inv.currency || 'USD')
}

export function periodLabel(p: { from: string; to: string }): string {
  return `${p.from.slice(0, 10)} to ${p.to.slice(0, 10)}`
}
