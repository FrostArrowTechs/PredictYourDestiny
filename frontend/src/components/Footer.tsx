import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

export default function Footer() {
  const { t } = useTranslation()
  return (
    <footer className="mt-16 border-t border-border bg-surface/40">
      <div className="mx-auto max-w-3xl px-4 py-8 text-sm text-muted">
        <p className="mb-2 font-medium text-accent">{t('home.disclaimerTitle')}</p>
        <p>{t('home.disclaimerBody')}</p>
        <nav className="mt-4 flex flex-wrap gap-x-4 gap-y-2 text-xs">
          <Link className="hover:text-fg hover:underline" to="/privacy">{t('legal.privacy.title')}</Link>
          <Link className="hover:text-fg hover:underline" to="/terms">{t('legal.terms.title')}</Link>
          <Link className="hover:text-fg hover:underline" to="/disclaimer">{t('legal.disclaimer.title')}</Link>
        </nav>
        <p className="mt-4 text-xs">© {new Date().getFullYear()} {t('app.name')}</p>
      </div>
    </footer>
  )
}
