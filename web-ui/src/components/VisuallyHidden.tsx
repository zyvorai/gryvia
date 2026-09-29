import type { ReactNode } from 'react'

const hidden: React.CSSProperties = { position: 'absolute', width: 1, height: 1, margin: -1, padding: 0, overflow: 'hidden', clip: 'rect(0 0 0 0)', whiteSpace: 'nowrap', border: 0 }

/** Content available to screen readers only (e.g. a table alternative to a chart). */
export default function VisuallyHidden({ children }: { children: ReactNode }) {
  return <div style={hidden}>{children}</div>
}
