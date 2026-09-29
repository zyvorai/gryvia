import { useState, useEffect, useCallback, type ReactNode } from 'react'
import {
  AuthContext,
  type AuthConfig,
  type UserInfo,
  fetchAuthConfig,
  fetchUserInfo,
  loginWithCredentials,
  loginErrorMessage,
  exchangeCodeForToken,
  buildAuthorizationURLWithPKCE,
  storeToken,
  clearToken,
  storeOIDCState,
  getOIDCState,
  getStoredToken,
} from '@/lib/auth'

interface AuthProviderProps {
  children: ReactNode
}

export default function AuthProvider({ children }: AuthProviderProps) {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [isLoading, setIsLoading] = useState(() => getStoredToken() !== null)
  const [user, setUser] = useState<UserInfo | null>(null)
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null)
  const [error, setError] = useState<string | null>(null)

  // Load auth config on mount
  useEffect(() => {
    fetchAuthConfig()
      .then((config) => setAuthConfig(config))
      .catch(() => {
        // If config endpoint is unreachable, default to API key only
        setAuthConfig({ oidcEnabled: false, apiKeyEnabled: true })
      })
  }, [])

  // Validate existing token on mount
  useEffect(() => {
    const token = getStoredToken()
    if (!token) return

    fetchUserInfo(token)
      .then((userInfo) => {
        setUser(userInfo)
        setIsAuthenticated(true)
      })
      .catch(() => {
        // Token is invalid or expired
        clearToken()
      })
      .finally(() => setIsLoading(false))
  }, [])

  const loginWithApiKey = useCallback(async (apiKey: string) => {
    setError(null)
    try {
      const userInfo = await fetchUserInfo(apiKey)
      storeToken(apiKey, 'api_key')
      setUser(userInfo)
      setIsAuthenticated(true)
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status
      const message =
        status === 403
          ? 'Invalid API key'
          : status === 401
            ? 'Authentication failed'
            : 'Connection error. Check that the API gateway is running.'
      setError(message)
      throw new Error(message, { cause: err })
    }
  }, [])

  const loginWithPassword = useCallback(async (username: string, password: string) => {
    setError(null)
    if (username.trim() === '' || password === '') {
      const message = 'Wrong username or password.'
      setError(message)
      throw new Error(message)
    }
    try {
      const bearer = await loginWithCredentials(username.trim(), password)
      const userInfo = await fetchUserInfo(bearer)
      storeToken(bearer, 'api_key')
      setUser(userInfo)
      setIsAuthenticated(true)
    } catch (err) {
      const message = loginErrorMessage(err)
      setError(message)
      throw new Error(message, { cause: err })
    }
  }, [])

  const loginWithSSO = useCallback(async () => {
    if (!authConfig?.oidcEnabled || !authConfig.authorizationEndpoint) {
      setError('SSO is not configured')
      return
    }

    try {
      const { url, state, verifier } = await buildAuthorizationURLWithPKCE(authConfig)
      storeOIDCState(state, verifier)
      window.location.href = url
    } catch {
      setError('Failed to initiate SSO login')
    }
  }, [authConfig])

  const handleOIDCCallback = useCallback(
    async (code: string) => {
      setError(null)
      setIsLoading(true)

      const { state: storedState, verifier } = getOIDCState()
      const urlState = new URLSearchParams(window.location.search).get('state')

      if (!storedState || storedState !== urlState) {
        setError('Invalid OAuth state. Please try logging in again.')
        setIsLoading(false)
        return
      }

      if (!verifier || !authConfig?.tokenEndpoint || !authConfig?.clientId) {
        setError('Missing OIDC configuration for token exchange')
        setIsLoading(false)
        return
      }

      try {
        const token = await exchangeCodeForToken(
          code,
          authConfig.tokenEndpoint,
          authConfig.clientId,
          verifier,
        )

        storeToken(token, 'oidc')
        const userInfo = await fetchUserInfo(token)
        setUser(userInfo)
        setIsAuthenticated(true)
      } catch {
        setError('Failed to complete SSO login. Please try again.')
        clearToken()
      } finally {
        setIsLoading(false)
      }
    },
    [authConfig],
  )

  const logout = useCallback(() => {
    clearToken()
    setUser(null)
    setIsAuthenticated(false)
    setError(null)
  }, [])

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated,
        isLoading,
        user,
        authConfig,
        loginWithApiKey,
        loginWithPassword,
        loginWithSSO,
        handleOIDCCallback,
        logout,
        error,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}
