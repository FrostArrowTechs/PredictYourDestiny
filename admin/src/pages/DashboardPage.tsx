import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../App'

interface Stats {
  todayUsers: number
  todayAiCalls: number
  totalUsers: number
}

export default function DashboardPage() {
  const { t } = useTranslation()
  const { token } = useAuth()
  const [stats, setStats] = useState<Stats>({
    todayUsers: 0,
    todayAiCalls: 0,
    totalUsers: 0,
  })
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // TODO: Add a dedicated dashboard stats endpoint
    // For now, we'll use the users endpoint to get a count
    fetch('/api/admin/users?limit=1', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        setStats({
          todayUsers: 0, // TODO: implement
          todayAiCalls: 0, // TODO: implement
          totalUsers: data.total || 0,
        })
      })
      .finally(() => setIsLoading(false))
  }, [token])

  const statCards = [
    { label: t('dashboard.todayUsers'), value: stats.todayUsers, color: 'bg-blue-500' },
    { label: t('dashboard.todayAiCalls'), value: stats.todayAiCalls, color: 'bg-green-500' },
    { label: t('dashboard.totalUsers'), value: stats.totalUsers, color: 'bg-purple-500' },
  ]

  if (isLoading) {
    return <div className="text-center py-8">{t('common.loading')}</div>
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">{t('dashboard.title')}</h1>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {statCards.map(card => (
          <div key={card.label} className="bg-white rounded-lg shadow p-6">
            <div className={`inline-block px-3 py-1 rounded-full text-white text-sm mb-2 ${card.color}`}>
              {card.label}
            </div>
            <div className="text-3xl font-bold">{card.value}</div>
          </div>
        ))}
      </div>
    </div>
  )
}