// Placeholder used by every feature route that isn't built yet.
// Keeps the nav fully clickable from day one so the IA is reviewable.
import { useTranslation } from 'react-i18next'

export default function ComingSoon({ titleKey }: { titleKey: string }) {
  const { t } = useTranslation()
  return (
    <section className="rounded-2xl border border-border bg-surface p-12 text-center">
      <div className="mb-4 text-5xl">✨</div>
      <h1 className="mb-2 text-2xl font-semibold text-fg">{t(titleKey)}</h1>
      <p className="text-muted">{t('common.comingSoon')}</p>
    </section>
  )
}
