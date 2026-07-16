// Top navigation. Routes are wired lazily in App.tsx; this just holds
// the link list so it's easy to reorder/rename from one place.
//
// Primary items show directly in the bar; secondary items collapse into
// a "更多" dropdown so the navbar never overflows as features grow.
import { useEffect, useRef, useState } from 'react'
import { NavLink, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { LogIn, UserPlus } from 'lucide-react'
import LanguageSwitcher from './LanguageSwitcher'
import ThemeSwitcher from './ThemeSwitcher'
import { useAuth } from '../auth/AuthContext'

export default function Navbar() {
  const { t } = useTranslation()
  const [moreOpen, setMoreOpen] = useState(false)
  const { user, isLoading } = useAuth()
  const moreRef = useRef<HTMLDivElement>(null)
  const closeTimer = useRef<number | null>(null)

  // Cancel any pending close and open the menu immediately.
  const openMore = () => {
    if (closeTimer.current) {
      window.clearTimeout(closeTimer.current)
      closeTimer.current = null
    }
    setMoreOpen(true)
  }
  // Defer close so the user has time to traverse the gap between
  // the trigger and the dropdown without it snapping shut.
  const scheduleCloseMore = () => {
    if (closeTimer.current) window.clearTimeout(closeTimer.current)
    closeTimer.current = window.setTimeout(() => setMoreOpen(false), 120)
  }
  // Keep the menu open while the cursor is over either the trigger
  // button or the dropdown panel itself.
  const keepMoreOpen = () => {
    if (closeTimer.current) {
      window.clearTimeout(closeTimer.current)
      closeTimer.current = null
    }
  }

  // Close the "更多" dropdown when the user clicks anywhere outside it.
  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (moreRef.current && !moreRef.current.contains(e.target as Node)) {
        setMoreOpen(false)
      }
    }
    document.addEventListener('mousedown', onDocClick)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      if (closeTimer.current) window.clearTimeout(closeTimer.current)
    }
  }, [])

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
    { to: '/name', label: t('nav.name') },
    { to: '/astrology', label: t('nav.astrology') },
    { to: '/constellation', label: t('nav.constellation') },
    { to: '/tarot', label: t('nav.tarot') },
    { to: '/ziwei', label: t('nav.ziwei') },
  ]

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    [
      'px-3 py-1.5 rounded-md text-sm transition-colors',
      isActive
        ? 'bg-surface-2 text-primary'
        : 'text-muted hover:text-fg hover:bg-surface-2',
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
          <div
            ref={moreRef}
            className="relative"
            onMouseEnter={openMore}
            onMouseLeave={scheduleCloseMore}
          >
            <button
              type="button"
              onClick={() => setMoreOpen((v) => !v)}
              className="px-3 py-1.5 rounded-md text-sm text-muted hover:text-fg hover:bg-surface transition-colors"
            >
              {t('nav.more')} ▾
            </button>
            {moreOpen && (
              <div
                onMouseEnter={keepMoreOpen}
                onMouseLeave={scheduleCloseMore}
                className="absolute left-0 top-full mt-1 min-w-[8rem] rounded-lg border border-border bg-surface py-1 shadow-lg"
              >
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
        <div className="ml-auto flex items-center gap-2">
          {!isLoading && !user && (
            <>
              <Link
                to="/login"
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm text-muted hover:text-fg hover:bg-surface transition-colors"
              >
                <LogIn className="w-4 h-4" />
                {t('common.login')}
              </Link>
              <Link
                to="/register"
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-primary text-primary-foreground hover:opacity-90 transition-opacity"
              >
                <UserPlus className="w-4 h-4" />
                {t('common.register')}
              </Link>
            </>
          )}
          <NavLink to="/account" className="text-sm text-muted hover:text-fg px-3 py-1.5">
            {user ? user.displayName || user.email : t('nav.account')}
          </NavLink>
          <ThemeSwitcher />
          <LanguageSwitcher />
        </div>
      </div>
    </header>
  )
}
