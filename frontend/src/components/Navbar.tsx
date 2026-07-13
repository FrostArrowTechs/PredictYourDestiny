// Top navigation. Routes are wired lazily in App.tsx; this just holds
// the link list so it's easy to reorder/rename from one place.
import { NavLink } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import LanguageSwitcher from './LanguageSwitcher'

export default function Navbar() {
  const { t } = useTranslation()
  const items = [
    { to: '/', label: t('nav.home'), end: true },
    { to: '/bazi', label: t('nav.bazi') },
    { to: '/dream', label: t('nav.dream') },
    { to: '/zodiac', label: t('nav.zodiac') },
    { to: '/huangli', label: t('nav.huangli') },
    { to: '/constellation', label: t('nav.constellation') },
    { to: '/tarot', label: t('nav.tarot') },
    { to: '/compatibility', label: t('nav.compatibility') },
  ]

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    [
      'px-3 py-1.5 rounded-md text-sm transition-colors',
      isActive
        ? 'bg-surface-2 text-primary'
        : 'text-muted hover:text-fg hover:bg-surface',
    ].join(' ')

  return (
    <header className="sticky top-0 z-20 border-b border-border bg-bg/80 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center gap-4 px-4 py-3">
        <NavLink to="/" className="flex items-center gap-2 font-semibold text-fg">
          <span className="text-xl">🔮</span>
          <span>{t('app.name')}</span>
        </NavLink>
        <nav className="hidden flex-1 items-center gap-1 md:flex">
          {items.map((it) => (
            <NavLink key={it.to} to={it.to} end={it.end} className={linkClass}>
              {it.label}
            </NavLink>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-3">
          <NavLink to="/account" className="text-sm text-muted hover:text-fg">
            {t('nav.account')}
          </NavLink>
          <LanguageSwitcher />
        </div>
      </div>
    </header>
  )
}
