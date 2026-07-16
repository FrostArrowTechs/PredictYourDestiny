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
    <div className="max-w-2xl mx-auto py-12">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-8">
        <h1 className="text-2xl font-bold mb-6">{t('account.title')}</h1>

        <div className="space-y-4 mb-8">
          <div>
            <label className="text-sm text-muted">{t('auth.email')}</label>
            <div className="font-medium">{user.email}</div>
          </div>
          <div>
            <label className="text-sm text-muted">{t('auth.displayName')}</label>
            <div className="font-medium">{user.displayName || '-'}</div>
          </div>
        </div>

        <button
          onClick={handleLogout}
          className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
        >
          {t('common.logout')}
        </button>
      </div>
    </div>
  )
}