import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Users, Bot, Layers, TrendingUp } from 'lucide-react'
import { useAuth } from '../App'
import { PageHeader } from '../components/PageHeader'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/Card'
import { Skeleton } from '../components/ui/Skeleton'

interface Stats {
  totalUsers: number
  totalProviders: number
  totalTiers: number
}

export default function DashboardPage() {
  const { t } = useTranslation()
  const { token } = useAuth()
  const [stats, setStats] = useState<Stats | null>(null)

  useEffect(() => {
    const headers = { Authorization: `Bearer ${token}` }

    Promise.all([
      fetch('/api/admin/users?limit=1', { headers }).then(r => r.json()),
      fetch('/api/admin/providers', { headers }).then(r => r.json()),
      fetch('/api/admin/tiers', { headers }).then(r => r.json()),
    ]).then(([users, providers, tiers]) => {
      setStats({
        totalUsers: users.total || 0,
        totalProviders: (providers.providers || []).length,
        totalTiers: (tiers.tiers || []).length,
      })
    })
  }, [token])

  const cards = [
    { label: t('dashboard.totalUsers'), value: stats?.totalUsers, icon: Users, color: 'text-blue-600', bg: 'bg-blue-50' },
    { label: t('nav.providers'), value: stats?.totalProviders, icon: Bot, color: 'text-purple-600', bg: 'bg-purple-50' },
    { label: t('nav.tiers'), value: stats?.totalTiers, icon: Layers, color: 'text-green-600', bg: 'bg-green-50' },
  ]

  return (
    <div>
      <PageHeader
        title={t('dashboard.title')}
        description="系统核心指标一览"
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        {cards.map(card => {
          const Icon = card.icon
          return (
            <Card key={card.label}>
              <CardContent className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-slate-500">{card.label}</p>
                    {stats ? (
                      <p className="text-3xl font-semibold text-slate-900 mt-2">{card.value}</p>
                    ) : (
                      <Skeleton className="h-9 w-16 mt-2" />
                    )}
                  </div>
                  <div className={`w-10 h-10 rounded-lg ${card.bg} flex items-center justify-center`}>
                    <Icon className={`w-5 h-5 ${card.color}`} />
                  </div>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <TrendingUp className="w-4 h-4" />
            快速入口
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {[
              { href: '/users', label: '用户管理', icon: Users },
              { href: '/providers', label: 'AI 供应商', icon: Bot },
              { href: '/tiers', label: '会员层级', icon: Layers },
              { href: '/settings', label: '系统设置', icon: TrendingUp },
            ].map(item => {
              const Icon = item.icon
              return (
                <a
                  key={item.href}
                  href={item.href}
                  className="flex items-center gap-3 p-4 rounded-lg border border-slate-200 hover:border-blue-500 hover:bg-blue-50/50 transition-colors"
                >
                  <Icon className="w-4 h-4 text-slate-500" />
                  <span className="text-sm font-medium text-slate-700">{item.label}</span>
                </a>
              )
            })}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}