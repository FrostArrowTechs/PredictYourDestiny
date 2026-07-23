import { useTranslation } from 'react-i18next'

type LegalKind = 'privacy' | 'terms' | 'disclaimer'

const sectionKeys: Record<LegalKind, string[]> = {
  privacy: ['scope', 'collection', 'use', 'storage', 'providers', 'retention', 'rights', 'security', 'children', 'changes'],
  terms: ['service', 'account', 'acceptableUse', 'ai', 'payment', 'intellectualProperty', 'availability', 'liability', 'termination', 'changes'],
  disclaimer: ['entertainment', 'noProfessionalAdvice', 'sensitiveDecisions', 'aiLimitations', 'emergency', 'responsibility'],
}

export default function LegalPage({ kind }: { kind: LegalKind }) {
  const { t } = useTranslation()
  return (
    <article className="mx-auto max-w-3xl rounded-xl border border-border bg-surface p-6 text-fg sm:p-10">
      <h1 className="text-3xl font-bold">{t(`legal.${kind}.title`)}</h1>
      <p className="mt-2 text-sm text-muted">{t('legal.effectiveDate')}</p>
      <p className="mt-6 leading-7 text-muted">{t(`legal.${kind}.intro`)}</p>
      <div className="mt-8 space-y-7">
        {sectionKeys[kind].map((key) => (
          <section key={key}>
            <h2 className="text-lg font-semibold">{t(`legal.${kind}.sections.${key}.title`)}</h2>
            <p className="mt-2 whitespace-pre-line leading-7 text-muted">{t(`legal.${kind}.sections.${key}.body`)}</p>
          </section>
        ))}
      </div>
      <p className="mt-10 border-t border-border pt-5 text-sm text-muted">{t('legal.contact')}</p>
    </article>
  )
}
