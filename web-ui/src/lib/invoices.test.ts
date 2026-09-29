import { describe, expect, it } from 'vitest'
import { currentMonth, formatMoney, invoiceFilename, invoiceParams, invoiceTotalDisplay, invoiceTotalLabel, isValidMonth, monthLabel, parseMonth, periodLabel } from './invoices'

describe('months', () => {
  it('validates YYYY-MM', () => {
    expect(isValidMonth('2026-09')).toBe(true)
    for (const bad of ['2026-13', '2026-00', '2026-9', '26-09', '', null, undefined, '2026-09-01']) expect(isValidMonth(bad)).toBe(false)
  })
  it('defaults to the current UTC month', () => {
    expect(currentMonth(new Date('2026-09-30T23:59:59Z'))).toBe('2026-09')
    expect(currentMonth(new Date('2026-01-01T00:00:00Z'))).toBe('2026-01')
    expect(parseMonth('bogus', new Date('2026-03-15T00:00:00Z'))).toBe('2026-03')
    expect(parseMonth('2025-12')).toBe('2025-12')
  })
  it('labels months', () => {
    expect(monthLabel('2026-09')).toBe('September 2026')
    expect(monthLabel('nope')).toBe('nope')
  })
})

describe('requests', () => {
  it('builds params and filenames', () => {
    expect(invoiceParams('2026-09')).toEqual({ month: '2026-09' })
    expect(invoiceParams('2026-09', 'acme')).toEqual({ month: '2026-09', tenant: 'acme' })
    expect(invoiceFilename('acme', '2026-09', 'csv')).toBe('gryvia-invoice-acme-2026-09.csv')
  })
})

describe('display', () => {
  it('formats money', () => {
    expect(formatMoney(1234.5)).toBe('$1,234.50')
    expect(formatMoney(null)).toBe('—')
    expect(formatMoney(2, 'ZZ')).toBe('2.00 ZZ')
  })
  it('shows totals and labels', () => {
    expect(invoiceTotalDisplay({ subtotal: 10, currency: 'USD' })).toBe('$10.00')
    expect(invoiceTotalLabel({ open: true })).toMatch(/running/)
    expect(invoiceTotalLabel({})).toBe('Subtotal')
    expect(periodLabel({ from: '2026-09-01T00:00:00Z', to: '2026-09-30' })).toBe('2026-09-01 to 2026-09-30')
  })
})
