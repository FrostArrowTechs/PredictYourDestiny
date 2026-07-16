// Top navigation. Routes are wired lazily in App.tsx; this just holds
// the link list so it's easy to reorder/rename from one place.
//
// Primary items show directly in the bar; secondary items collapse into
// a "更多" dropdown so the navbar never overflows as features grow.
import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import LanguageSwitcher from './LanguageSwitcher'

export default function Navbar() {
  const { t } = useTranslation()
  const [moreOpen, setMoreOpen] = useState(false)

  const primary = [
    { to: '/', label: t('nav.home'), end: true },
    { to: '/bazi', label: t('nav.bazi') },
    { to: '/zodiac', label: t('nav.zodiac') },
    { to: '/huangli', label: t('nav.huangli') },
    { to: '/compatibility', label: t('nav.compatibility') },
  ]
  const secondary = [
    { to: '/dream', label: t('nav.dream') },
    { to: '/weighbone', label: t('nav.weighbone') },
    { to: '/divination', label: t('nav.divination') },
    { to: '/plumflower', label: t('nav.plumflower') },
    { to: '/constellation', label: t('nav.constellation') },
    { to: '/tarot', label: t('nav.tarot') },
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
          {primary.map((it) => (
            <NavLink key={it.to} to={it.to} end={it.end} className={linkClass}>
              {it.label}
            </NavLink>
          ))}
          {/* 更多 dropdown */}
          <div className="relative" onMouseLeave={() => setMoreOpen(false)}>
            <button
              type="button"
              onMouseEnter={() => setMoreOpen(true)}
              onClick={() => setMoreOpen((v) => !v)}
              className="px-3 py-1.5 rounded-md text-sm text-muted hover:text-fg hover:bg-surface transition-colors"
            >
              {t('nav.more')} ▾
            </button>
            {moreOpen && (
              <div className="absolute left-0 top-full mt-1 min-w-[8rem] rounded-lg border border-border bg-surface py-1 shadow-lg">
                {secondary.map((it) => (
                  <NavLink
                    key={it.to}
                    to={it.to}
                    onClick={() => setMoreOpen(false)}
                    className={({ isActive }) =>
                      [
                        'block px-4 py-1.5 text-sm transition-colors',
                        isActive ? 'text-primary bg-surface-2' : 'text-muted hover:text-fg hover:bg-surface-2',
                      ].join(' ')
                    }
                  >
                    {it.label}
                  </NavLink>
                ))}
              </div>
            )}
          </div>
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
