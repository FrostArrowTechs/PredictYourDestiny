// Dream interpretation page. Users enter their dream description,
// see matching traditional meanings (free), then optionally get an
// AI-powered personalized interpretation (tiered).
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  api,
  Dream as DreamApi,
  type DreamChart,
  type DreamInput,
  type InterpretStreamEvent,
  type ModelCatalog,
  streamDreamInterpret,
} from '../../api/client'

export default function DreamPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  // form state
  const [question, setQuestion] = useState('')

  // result state
  const [chart, setChart] = useState<DreamChart | null>(null)
  const [computing, setComputing] = useState(false)
  const [computeErr, setComputeErr] = useState<string | null>(null)

  // interpret state
  const [depth, setDepth] = useState<'brief' | 'deep'>('brief')
  const [modelId, setModelId] = useState<string>('')
  const [catalog, setCatalog] = useState<ModelCatalog | null>(null)
  const [interpretation, setInterpretation] = useState('')
  const [reasoning, setReasoning] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [interpretErr, setInterpretErr] = useState<string | null>(null)
  const [showReasoning, setShowReasoning] = useState(false)
  const [usage, setUsage] = useState<InterpretStreamEvent['usage'] | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  // fetch model catalog on mount
  useEffect(() => {
    let alive = true
    api
      .get<{ items: { Key: string; Value: string }[] }>('/settings')
      .then((res) => {
        if (!alive) return
        const row = res.items.find((r) => r.Key === 'ai.models')
        if (!row) return
        try {
          const entries = JSON.parse(row.Value) as { id: string; label?: string; tier?: string }[]
          const cat: ModelCatalog = { free: [], paid: [] }
          for (const e of entries) {
            const entry = { id: e.id, label: e.label ?? e.id, tier: (e.tier === 'paid' ? 'paid' : 'free') as 'free' | 'paid' }
            ;(entry.tier === 'paid' ? cat.paid : cat.free).push(entry)
          }
          if (alive) setCatalog(cat)
        } catch {
          /* ignore */
        }
      })
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [])

  const buildInput = useCallback(
    (): DreamInput => ({
      question,
      lang,
      interpretDepth: depth,
      model: modelId || undefined,
    }),
    [question, lang, depth, modelId],
  )

  const onCompute = useCallback(async () => {
    if (!question.trim()) return
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await DreamApi.compute(buildInput())
      setChart(res.data)
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      setComputeErr(msg)
      setChart(null)
    } finally {
      setComputing(false)
    }
  }, [buildInput, question])

  const onInterpret = useCallback(async () => {
    if (streaming) {
      abortRef.current?.abort()
      return
    }
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    setStreaming(true)
    setShowReasoning(false)
    const ac = new AbortController()
    abortRef.current = ac
    try {
      await streamDreamInterpret(
        buildInput(),
        (ev) => {
          if (ev.error) {
            setInterpretErr(ev.error)
            return
          }
          if (ev.content) setInterpretation((p) => p + ev.content)
          if (ev.reasoning) setReasoning((p) => p + ev.reasoning)
          if (ev.usage) setUsage(ev.usage)
        },
        ac.signal,
      )
    } catch (e) {
      if ((e as Error).name !== 'AbortError') {
        setInterpretErr(e instanceof Error ? e.message : String(e))
      }
    } finally {
      setStreaming(false)
      abortRef.current = null
    }
  }, [buildInput, streaming])

  return (
    <div className="space-y-6">
      <header className="text-center">
        <h1 className="text-3xl font-bold text-fg">{t('dream.title')}</h1>
        <p className="mt-2 text-muted">{t('dream.subtitle')}</p>
      </header>

      {/* ── input form ── */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="space-y-4">
          <div>
            <label className="mb-2 block text-sm font-medium text-fg">
              {t('dream.form.question')}
            </label>
            <textarea
              className="w-full rounded-lg border border-border bg-surface px-4 py-3 text-fg outline-none focus:border-primary min-h-[120px] resize-y"
              placeholder={t('dream.form.questionPlaceholder')}
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
            />
          </div>

          <div className="flex flex-wrap items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted">{t('dream.interpret.depth')}:</span>
              <button
                type="button"
                onClick={() => setDepth('brief')}
                className={pill(depth === 'brief')}
              >
                {t('dream.interpret.brief')}
              </button>
              <button
                type="button"
                onClick={() => setDepth('deep')}
                className={pill(depth === 'deep')}
              >
                {t('dream.interpret.deep')}
              </button>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-sm text-muted">{t('dream.interpret.model')}:</span>
              <select
                className="rounded-md border border-border bg-surface px-3 py-1.5 text-sm text-fg outline-none focus:border-primary"
                value={modelId}
                onChange={(e) => setModelId(e.target.value)}
                disabled={!catalog}
              >
                <option value="">{t('dream.interpret.defaultModel')}</option>
                {catalog?.free.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.label} ({t('dream.interpret.tierFree')})
                  </option>
                ))}
                {catalog?.paid.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.label} ({t('dream.interpret.tierPaid')})
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="flex gap-3">
            <button
              type="button"
              onClick={onCompute}
              disabled={computing || !question.trim()}
              className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
            >
              {computing ? t('dream.form.searching') : t('dream.form.search')}
            </button>
          </div>

          {computeErr && (
            <p className="text-sm text-red-400">{t('dream.error.compute')}: {computeErr}</p>
          )}
        </div>
      </section>

      {/* ── matched results ── */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          <h2 className="mb-4 text-lg font-semibold text-fg">
            {t('dream.matches.title')} ({chart.totalMatches})
          </h2>

          {chart.matches.length === 0 ? (
            <p className="text-sm text-muted">{t('dream.matches.empty')}</p>
          ) : (
            <div className="space-y-3">
              {chart.matches.map((m, i) => (
                <div
                  key={i}
                  className="rounded-lg border border-border bg-bg p-4"
                >
                  <div className="mb-1 flex items-center gap-2">
                    <span className="font-medium text-fg">{m.keyword}</span>
                    <span className="rounded-full bg-primary/20 px-2 py-0.5 text-xs text-primary">
                      {m.category}
                    </span>
                  </div>
                  <p className="text-sm leading-relaxed text-muted">{m.meaning}</p>
                </div>
              ))}
            </div>
          )}

          {/* ── AI interpret ── */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('dream.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming
                    ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30'
                    : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('dream.interpret.stop') : interpretation ? t('dream.interpret.retry') : t('dream.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('dream.interpret.empty')}</p>
            )}

            {interpretErr && (
              <p className="text-sm text-red-400">{t('dream.interpret.error')}: {interpretErr}</p>
            )}

            {reasoning && (
              <div className="mb-3">
                <button
                  type="button"
                  onClick={() => setShowReasoning((v) => !v)}
                  className="text-xs text-muted underline-offset-2 hover:underline"
                >
                  {showReasoning ? t('dream.interpret.hideReasoning') : t('dream.interpret.showReasoning')}
                </button>
                {showReasoning && (
                  <pre className="mt-1 max-h-60 overflow-y-auto whitespace-pre-wrap rounded-lg border border-border bg-bg p-3 text-xs text-muted">
                    {reasoning}
                    {streaming && <span className="animate-pulse">▋</span>}
                  </pre>
                )}
              </div>
            )}

            {interpretation && (
              <article className="whitespace-pre-wrap text-sm leading-relaxed text-fg">
                {interpretation}
                {streaming && <span className="animate-pulse">▋</span>}
              </article>
            )}

            {usage && (
              <div className="mt-3 border-t border-border pt-2 text-xs text-muted">
                {t('dream.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

function pill(active: boolean): string {
  return [
    'rounded-full px-3 py-1 text-xs transition-colors',
    active ? 'bg-primary text-bg' : 'border border-border text-muted hover:text-fg',
  ].join(' ')
}