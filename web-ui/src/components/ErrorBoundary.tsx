import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="login-shell">
          <div className="card login-card">
            <h2>Something went wrong</h2>
            <p>{this.state.error?.message || 'An unexpected error occurred.'}</p>
            <button
              className="primary"
              onClick={() => {
                this.setState({ hasError: false, error: null })
                window.location.href = '/dashboard'
              }}
            >
              Go to Dashboard
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
