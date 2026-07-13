// Landing page. Demonstrates the full loop: fetches /api/health,
// renders i18n'd copy, and links into each feature. Other feature
// pages start as ComingSoon and get fleshed out stage by stage.
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Health, type HealthResponse } from '../../api/client'

export default function HomePage() {
  const { t } = useTranslation()
  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [online, setOnline] = useState<boolean | null>(null)

  useEffect(() => {
    let alive = true
    Health.check()
      .then((h) => {
        if (alive) {
          setHealth(h)
          setOnline(true)
        }
      })
      .catch(() => alive && setOnline(false))
    return () => {
      alive = false
    }
  }, [])

  const features = [
    { to: '/bazi', icon: '🀄', label: t('nav.bazi') },
    { to: '/dream', icon: '💭', label: t('nav.dream') },
    { to: '/zodiac', icon: '🐉', label: t('nav.zodiac') },
    { to: '/huangli', icon: '📅', label: t('nav.huangli') },
    { to: '/constellation', icon: '♈', label: t('nav.constellation') },
    { to: '/tarot', icon: '🃏', label: t('nav.tarot') },
    { to: '/compatibility', icon: '💞', label: t('nav.compatibility') },
  ]

  return (
    <div className="space-y-12">
      <section className="text-center">
        <h1 className="bg-gradient-to-r from-primary via-primary-hover to-accent bg-clip-text text-5xl font-bold text-transparent sm:text-6xl">
          {t('home.heroTitle')}
        </h1>
        <p className="mx-auto mt-4 max-w-2xl text-lg text-muted">{t('home.heroSubtitle')}</p>
        <Link
          to="/bazi"
          className="mt-8 inline-block rounded-xl bg-primary px-8 py-3 font-medium text-bg transition-colors hover:bg-primary-hover"
        >
          {t('home.cta')}
        </Link>
      </section>

      <section className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        {features.map((f) => (
          <Link
            key={f.to}
            to={f.to}
            className="group rounded-2xl border border-border bg-surface p-6 text-center transition-colors hover:border-primary hover:bg-surface-2"
          >
            <div className="mb-2 text-4xl transition-transform group-hover:scale-110">{f.icon}</div>
            <div className="font-medium text-fg">{f.label}</div>
          </Link>
        ))}
      </section>

      <section className="flex items-center justify-center gap-2 text-sm text-muted">
        <span
          className={`inline-block h-2 w-2 rounded-full ${
            online === null ? 'bg-muted' : online ? 'bg-green-400' : 'bg-red-400'
          }`}
        />
        {online === null
          ? t('common.loading')
          : online
            ? t('health.online')
            : t('health.offline')}
        {health && <span className="text-xs">· {t('health.version')} {health.version}</span>}
      </section>
    </div>
  )
}
