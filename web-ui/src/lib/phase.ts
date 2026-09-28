export function phaseTone(phase?: string): 'ok' | 'warn' | 'bad' | 'info' | '' {
  switch (phase) {
    case 'Completed':
    case 'Succeeded':
      return 'ok'
    case 'Pending':
    case 'Queued':
      return 'warn'
    case 'Failed':
      return 'bad'
    case 'Running':
      return 'info'
    default:
      return ''
  }
}
