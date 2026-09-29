import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { api } from '@/lib/api'
import { notify } from '@/lib/notify'
import { useIsAdmin } from '@/lib/useRole'
import { saveBlob } from '@/lib/download'
import { formatNumber } from '@/lib/format'
import { formatRate } from '@/lib/cloud'
import { classLabel, formatMoney, invoiceFilename, networkRateDisplay, invoiceParams, invoiceTotalDisplay, invoiceTotalLabel, monthLabel, parseMonth, periodLabel, type Invoice } from '@/lib/invoices'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import PageHero from '@/components/PageHero'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'

export default function Invoices() {
  useDocumentTitle('Invoices')
  const admin = useIsAdmin()
  const [params, setParams] = useSearchParams()
  const month = useMemo(() => parseMonth(params.get('month')), [params])
  const tenant = admin ? (params.get('tenant') ?? '') : ''
  const [busy, setBusy] = useState<string | null>(null)

  const setParam = (name: string, value: string) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value) next.set(name, value)
        else next.delete(name)
        return next
      },
      { replace: true },
    )

  const tenants = useQuery({ queryKey: ['tenants'], queryFn: api.getTenants, enabled: admin })
  const { data, isLoading, isError, error, refetch, isRefetching } = useQuery({
    queryKey: ['invoices', month, tenant],
    queryFn: () => api.getInvoices(invoiceParams(month, tenant)),
  })

  const download = async (inv: Invoice, format: 'csv' | 'json') => {
    const key = `${inv.number}:${format}`
    setBusy(key)
    try {
      const blob = await api.downloadInvoice(inv.tenant, month, format)
      saveBlob(blob, invoiceFilename(inv.tenant, month, format))
    } catch (err) {
      notify.error(`Could not download ${format.toUpperCase()}`, err)
    } finally {
      setBusy(null)
    }
  }

  return (
    <>
      <PageHero
        eyebrow="Invoices"
        title={admin ? 'Invoices.' : 'Your invoices.'}
        lede="Monthly statements built from job run time and the catalog rates. These are estimates, not bills; nothing is charged."
      />
      <div className="grid">
        <section className="card span3">
          <p className="eyebrow">Filters</p>
          <div className="toolbar">
            <label>
              Month
              <input type="month" value={month} onChange={(e) => setParam('month', e.target.value)} />
            </label>
            {admin && (
              <label>
                Tenant
                <select value={tenant} onChange={(e) => setParam('tenant', e.target.value)}>
                  <option value="">All tenants</option>
                  {(tenants.data ?? []).map((t) => (
                    <option key={t.metadata.name} value={t.metadata.name}>
                      {t.spec?.displayName || t.metadata.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
          </div>
        </section>

        {isLoading ? (
          <section className="card span3">
            <Skeleton rows={3} />
          </section>
        ) : isError ? (
          <div className="span3">
            <ErrorState title="Could not load invoices." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        ) : data && data.items.length === 0 ? (
          <div className="span3">
            <EmptyState title={`No usage in ${monthLabel(month)}`}>Invoices appear here once jobs have run in the selected month.</EmptyState>
          </div>
        ) : (
          (data?.items ?? []).map((inv) => (
            <section key={inv.number} className="card span3" aria-labelledby={`inv-${inv.number}`}>
              <p className="eyebrow">{periodLabel(inv.period)}</p>
              <h2 className="card-title" id={`inv-${inv.number}`}>
                <span className="mono">{inv.number}</span> · {inv.tenant}
              </h2>
              <p>
                <span className="pill">Estimate</span> <span className="faint">{inv.currency}</span>
                {inv.open && <span className="faint"> Includes running jobs; the amount can still change.</span>}
              </p>
              <div className="table-wrap">
                <table>
                  <caption className="sr-only">{`Invoice ${inv.number} lines, ${monthLabel(month)}`}</caption>
                  <thead>
                    <tr>
                      <th scope="col">SKU</th>
                      <th scope="col">GPU type</th>
                      <th scope="col" className="num">GPU hours</th>
                      <th scope="col" className="num">Rate/hour</th>
                      <th scope="col" className="num">Amount</th>
                      <th scope="col" className="num">Jobs</th>
                    </tr>
                  </thead>
                  <tbody>
                    {inv.lines.map((l) => (
                      <tr key={`${l.sku}:${l.gpuType}`}>
                        <td className="mono">{l.sku}</td>
                        <td>{l.gpuType}</td>
                        <td className="num">{formatNumber(Math.round(l.gpuHours * 100) / 100)}</td>
                        <td className="num">{formatRate(l.rate, inv.currency)}</td>
                        <td className="num">{formatMoney(l.amount, inv.currency)}</td>
                        <td className="num">{formatNumber(l.jobs)}</td>
                      </tr>
                    ))}
                  </tbody>
                  <tfoot>
                    <tr>
                      <th scope="row" colSpan={4}>{invoiceTotalLabel(inv)}</th>
                      <td className="num"><b>{invoiceTotalDisplay(inv)}</b></td>
                      <td className="num">{formatNumber(inv.jobs)}</td>
                    </tr>
                  </tfoot>
                </table>
              </div>
              {inv.note && <p className="faint">{inv.note}</p>}
              {inv.networkLines && inv.networkLines.length > 0 && (
                <>
                  <h3 className="card-title">Network egress (estimate)</h3>
                  <div className="table-wrap">
                    <table>
                      <caption className="sr-only">{`Invoice ${inv.number} network egress lines, ${monthLabel(month)}`}</caption>
                      <thead>
                        <tr>
                          <th scope="col">Peer</th>
                          <th scope="col">Zone</th>
                          <th scope="col" className="num">Egress GB</th>
                          <th scope="col" className="num">Rate</th>
                          <th scope="col" className="num">Amount</th>
                        </tr>
                      </thead>
                      <tbody>
                        {inv.networkLines.map((l) => (
                          <tr key={`${l.peerClass}:${l.zoneClass}`}>
                            <td>{classLabel(l.peerClass)}</td>
                            <td>{classLabel(l.zoneClass)}</td>
                            <td className="num">{formatNumber(Math.round(l.egressGB * 1000) / 1000)}</td>
                            <td className="num">{networkRateDisplay(l)}</td>
                            <td className="num">{formatMoney(l.amount, l.currency || inv.currency)}</td>
                          </tr>
                        ))}
                      </tbody>
                      <tfoot>
                        <tr>
                          <th scope="row" colSpan={4}>Network subtotal (separate from the GPU subtotal)</th>
                          <td className="num"><b>{formatMoney(inv.networkSubtotal, inv.networkLines[0]?.currency || inv.currency)}</b></td>
                        </tr>
                      </tfoot>
                    </table>
                  </div>
                  {inv.networkNote && <p className="faint">{inv.networkNote}</p>}
                </>
              )}
              <div className="toolbar">
                {(['csv', 'json'] as const).map((f) => (
                  <button key={f} type="button" className="btn-secondary" onClick={() => download(inv, f)} disabled={busy !== null} aria-busy={busy === `${inv.number}:${f}`} aria-label={`Download ${f.toUpperCase()} for ${inv.number}`}>
                    {busy === `${inv.number}:${f}` ? 'Downloading…' : `Download ${f.toUpperCase()}`}
                  </button>
                ))}
              </div>
            </section>
          ))
        )}
      </div>
    </>
  )
}
