import { Link, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'

const featureRoutes = new Set([
  '/bazi', '/dream', '/huangli', '/zodiac', '/compatibility', '/weighbone',
  '/divination', '/plumflower', '/name', '/astrology', '/constellation', '/tarot', '/ziwei',
])

export default function SafetyNotice() {
  const { pathname } = useLocation()
  const { t } = useTranslation()
  if (!featureRoutes.has(pathname)) return null
  return (
    <aside className="mx-auto mt-8 max-w-4xl rounded-lg border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-fg">
      <div className="font-semibold">{t('legal.safety.title')}</div>
      <p className="mt-1 text-muted">{t('legal.safety.body')}</p>
      <Link to="/disclaimer" className="mt-2 inline-block text-primary hover:underline">{t('legal.safety.learnMore')}</Link>
    </aside>
  )
}
