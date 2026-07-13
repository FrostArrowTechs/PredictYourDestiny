// A compact language picker. Persists the choice via rememberLang so
// it survives reloads; react-i18next handles the actual re-render.
import { useTranslation } from 'react-i18next'
import { DEFAULT_LANG, rememberLang, SUPPORTED_LANGS, type Lang } from '../i18n'

export default function LanguageSwitcher() {
  const { i18n, t } = useTranslation()
  const current = (i18n.resolvedLanguage as Lang) ?? DEFAULT_LANG

  return (
    <label className="flex items-center gap-2 text-sm text-muted">
      <span aria-hidden>🌐</span>
      <select
        value={current}
        onChange={(e) => {
          const lang = e.target.value as Lang
          void i18n.changeLanguage(lang)
          rememberLang(lang)
        }}
        className="rounded-md border border-border bg-surface px-2 py-1 text-fg outline-none focus:border-primary"
        aria-label="Language"
      >
        {SUPPORTED_LANGS.map((lang) => (
          <option key={lang} value={lang}>
            {t(`lang.${lang}` as const)}
          </option>
        ))}
      </select>
    </label>
  )
}
