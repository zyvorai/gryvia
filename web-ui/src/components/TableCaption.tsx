import type { ReactNode } from 'react'

const HIDDEN = { position: 'absolute', width: 1, height: 1, margin: -1, padding: 0, overflow: 'hidden', clip: 'rect(0 0 0 0)', whiteSpace: 'nowrap', border: 0 } as const

/** Text for screen readers only. */
export function SrOnly({ children }: { children: ReactNode }) {
  return <span style={HIDDEN}>{children}</span>
}

/** A table caption that is announced but not shown. */
export function TableCaption({ children }: { children: ReactNode }) {
  return <caption style={HIDDEN}>{children}</caption>
}
