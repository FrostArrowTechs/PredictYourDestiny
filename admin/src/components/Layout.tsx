import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  LayoutDashboard,
  Users,
  Bot,
  Layers,
  Settings,
  LogOut,
  Sparkles,
} from 'lucide-react'
import { useAuth } from '../App'
import { Button } from './ui/Button'
import { cn } from '../lib/utils'

export default function Layout() {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { to: '/dashboard', label: t('nav.dashboard'), icon: LayoutDashboard },
    { to: '/users', label: t('nav.users'), icon: Users },
    { to: '/providers', label: t('nav.providers'), icon: Bot },
    { to: '/tiers', label: t('nav.tiers'), icon: Layers },
    { to: '/settings', label: t('nav.settings'), icon: Settings },
  ]

  return (
    <div className="min-h-screen bg-slate-50 flex">
      {/* Sidebar */}
      <aside className="fixed inset-y-0 left-0 w-60 bg-slate-900 text-slate-100 flex flex-col">
        {/* Logo */}
        <div className="flex items-center gap-2 h-16 px-6 border-b border-slate-800">
          <Sparkles className="w-5 h-5 text-blue-400" />
          <h1 className="text-base font-semibold tracking-tight">知命 · Admin</h1>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-4 space-y-1">
          {navItems.map(item => {
            const Icon = item.icon
            return (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  cn(
                    'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-slate-800 text-white'
                      : 'text-slate-400 hover:bg-slate-800 hover:text-white',
                  )
                }
              >
                <Icon className="w-4 h-4" />
                {item.label}
              </NavLink>
            )
          })}
        </nav>

        {/* User section */}
        <div className="p-3 border-t border-slate-800">
          <div className="px-3 py-2 mb-2">
            <div className="text-sm font-medium text-white truncate">{user?.email}</div>
            <div className="text-xs text-slate-400 mt-0.5">Administrator</div>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleLogout}
            className="w-full justify-start text-slate-400 hover:text-white hover:bg-slate-800"
          >
            <LogOut className="w-4 h-4 mr-2" />
            {t('nav.logout')}
          </Button>
        </div>
      </aside>

      {/* Main content */}
      <main className="ml-60 flex-1">
        <div className="px-8 py-6 max-w-7xl mx-auto">
          <Outlet />
        </div>
      </main>
    </div>
  )
}