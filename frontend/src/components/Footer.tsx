import { useTranslation } from 'react-i18next'

export default function Footer() {
  const { t } = useTranslation()
  return (
    <footer className="mt-16 border-t border-border bg-surface/40">
      <div className="mx-auto max-w-3xl px-4 py-8 text-sm text-muted">
        <p className="mb-2 font-medium text-accent">{t('home.disclaimerTitle')}</p>
        <p>{t('home.disclaimerBody')}</p>
        <p className="mt-4 text-xs">© {new Date().getFullYear()} {t('app.name')}</p>
      </div>
    </footer>
  )
}
