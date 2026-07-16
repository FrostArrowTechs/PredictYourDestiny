import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../auth/AuthContext'

export default function AccountPage() {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/')
  }

  if (!user) {
    return (
      <div className="max-w-2xl mx-auto py-12 text-center">
        <p className="text-muted">{t('auth.notLoggedIn')}</p>
      </div>
    )
  }

  return (
    <div className="max-w-2xl mx-auto py-12 px-4">
      <div className="rounded-lg border border-border bg-surface shadow p-8">
        <h1 className="text-2xl font-bold text-fg mb-6">{t('account.title')}</h1>

        <div className="space-y-4 mb-8">
          <div>
            <label className="text-sm text-muted">{t('auth.email')}</label>
            <div className="font-medium text-fg">{user.email}</div>
          </div>
          <div>
            <label className="text-sm text-muted">{t('auth.displayName')}</label>
            <div className="font-medium text-fg">{user.displayName || '-'}</div>
          </div>
        </div>

        <button
          onClick={handleLogout}
          className="px-4 py-2 rounded-md bg-red-600 text-white hover:bg-red-700"
        >
          {t('common.logout')}
        </button>
      </div>
    </div>
  )
}