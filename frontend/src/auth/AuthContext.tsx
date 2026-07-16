import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import { Auth, type AuthUser } from '../api/client'

interface AuthContextType {
  user: AuthUser | null
  isLoading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, displayName?: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // Check if user is logged in on mount
    const token = localStorage.getItem('pyd_token')
    if (token) {
      Auth.me()
        .then(data => setUser(data.user))
        .catch(() => {
          localStorage.removeItem('pyd_token')
        })
        .finally(() => setIsLoading(false))
    } else {
      setIsLoading(false)
    }
  }, [])

  const login = async (email: string, password: string) => {
    const res = await Auth.login({ email, password })
    localStorage.setItem('pyd_token', res.token)
    setUser(res.user)
  }

  const register = async (email: string, password: string, displayName?: string) => {
    const res = await Auth.register({ email, password, displayName })
    localStorage.setItem('pyd_token', res.token)
    setUser(res.user)
  }

  const logout = () => {
    localStorage.removeItem('pyd_token')
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, isLoading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}