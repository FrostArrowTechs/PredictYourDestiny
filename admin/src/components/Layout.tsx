import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../App'

export default function Layout() {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { to: '/dashboard', label: t('nav.dashboard') },
    { to: '/users', label: t('nav.users') },
    { to: '/providers', label: t('nav.providers') },
    { to: '/tiers', label: t('nav.tiers') },
    { to: '/settings', label: t('nav.settings') },
  ]

  return (
    <div className="min-h-screen bg-gray-100">
      {/* Sidebar */}
      <aside className="fixed inset-y-0 left-0 w-64 bg-slate-800 text-white">
        <div className="flex items-center justify-center h-16 border-b border-slate-700">
          <h1 className="text-xl font-bold">知命 Admin</h1>
        </div>
        <nav className="p-4 space-y-2">
          {navItems.map(item => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `block px-4 py-2 rounded-lg transition-colors ${
                  isActive
                    ? 'bg-slate-700 text-white'
                    : 'text-slate-300 hover:bg-slate-700 hover:text-white'
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="absolute bottom-0 left-0 right-0 p-4 border-t border-slate-700">
          <div className="text-sm text-slate-400 mb-2">{user?.email}</div>
          <button
            onClick={handleLogout}
            className="w-full px-4 py-2 text-left text-slate-300 hover:text-white hover:bg-slate-700 rounded-lg transition-colors"
          >
            {t('nav.logout')}
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="ml-64 p-8">
        <Outlet />
      </main>
    </div>
  )
}