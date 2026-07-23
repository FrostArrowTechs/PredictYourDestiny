import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect, createContext, useContext, type ReactNode } from 'react'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import UsersPage from './pages/UsersPage'
import ProvidersPage from './pages/ProvidersPage'
import TiersPage from './pages/TiersPage'
import SettingsPage from './pages/SettingsPage'
import UsagePage from './pages/UsagePage'
import './i18n'
import {
  adminUnauthorizedEvent,
  apiRequest,
  clearAdminToken,
  getAdminToken,
  setAdminToken,
} from './api/client'

// Auth Context
interface User {
  id: number
  email: string
  displayName: string
  role: string
}

interface AuthContextType {
  user: User | null
  token: string | null
  login: (email: string, password: string) => Promise<void>
  logout: () => void
  isLoading: boolean
}

const AuthContext = createContext<AuthContextType | null>(null)

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(getAdminToken())
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // Verify token on mount
    const storedToken = getAdminToken()
    if (storedToken) {
      apiRequest<{ user?: User }>('/auth/me')
        .then(data => {
          if (data.user && data.user.role === 'admin') {
            setUser(data.user)
            setToken(storedToken)
          } else {
            clearAdminToken()
            setToken(null)
          }
        })
        .catch(() => {
          clearAdminToken()
          setToken(null)
        })
        .finally(() => setIsLoading(false))
    } else {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    const handleUnauthorized = () => {
      setToken(null)
      setUser(null)
    }
    window.addEventListener(adminUnauthorizedEvent, handleUnauthorized)
    return () => window.removeEventListener(adminUnauthorizedEvent, handleUnauthorized)
  }, [])

  const login = async (email: string, password: string) => {
    const data = await apiRequest<{ token: string; user: User }>('/auth/login', {
      method: 'POST',
      auth: false,
      body: { email, password },
    })
    if (data.user.role !== 'admin') {
      throw new Error('Admin access required')
    }
    setAdminToken(data.token)
    setToken(data.token)
    setUser(data.user)
  }

  const logout = () => {
    clearAdminToken()
    setToken(null)
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, token, login, logout, isLoading }}>
      {children}
    </AuthContext.Provider>
  )
}

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { user, isLoading } = useAuth()
  if (isLoading) {
    return <div className="min-h-screen flex items-center justify-center">Loading...</div>
  }
  if (!user) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<ProtectedRoute><Layout /></ProtectedRoute>}>
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route path="dashboard" element={<DashboardPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="providers" element={<ProvidersPage />} />
            <Route path="tiers" element={<TiersPage />} />
            <Route path="usage" element={<UsagePage />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
