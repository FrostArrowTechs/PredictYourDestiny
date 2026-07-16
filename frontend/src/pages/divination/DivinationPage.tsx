// 抽签/求签 page. Users enter a question (optional), draw a stick,
// see the poem + interpretation, and get AI 解签.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Divination as DivinationApi,
  type DivinationChart,
  type DivinationInput,
  type InterpretStreamEvent,
  streamDivinationInterpret,
} from '../../api/client'

export default function DivinationPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  const [question, setQuestion] = useState('')
  const [chart, setChart] = useState<DivinationChart | null>(null)
  const [computing, setComputing] = useState(false)
  const [computeErr, setComputeErr] = useState<string | null>(null)

  const [interpretation, setInterpretation] = useState('')
  const [reasoning, setReasoning] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [interpretErr, setInterpretErr] = useState<string | null>(null)
  const [showReasoning, setShowReasoning] = useState(false)
  const [usage, setUsage] = useState<InterpretStreamEvent['usage'] | null>(null)
  const [shaking, setShaking] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const buildInput = useCallback(
    (): DivinationInput => ({ question: question.trim() || undefined, lang }),
    [question, lang],
  )

  const onDraw = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    setChart(null)
    setShaking(true)
    // shake animation duration
    await new Promise((r) => setTimeout(r, 1200))
    setShaking(false)
    try {
      const res = await DivinationApi.compute(buildInput())
      setChart(res.data)
    } catch (e) {
      setComputeErr(e instanceof Error ? e.message : String(e))
    } finally {
      setComputing(false)
    }
  }, [buildInput])

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
      await streamDivinationInterpret(
        buildInput(),
        (ev) => {
          if (ev.error) { setInterpretErr(ev.error); return }
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
        <h1 className="text-3xl font-bold text-fg">{t('divination.title')}</h1>
        <p className="mt-2 text-muted">{t('divination.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="space-y-4">
          <div>
            <label className="mb-1 block text-xs text-muted">{t('divination.form.question')}</label>
            <textarea
              className="w-full rounded-lg border border-border bg-surface px-4 py-3 text-fg outline-none focus:border-primary min-h-[80px] resize-y"
              placeholder={t('divination.form.questionPlaceholder')}
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
            />
          </div>

          <button
            type="button"
            onClick={onDraw}
            disabled={computing}
            className="rounded-xl bg-primary px-8 py-3 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {shaking ? '🎋🎋🎋' : t('divination.form.draw')}
          </button>
        </div>
        {computeErr && <p className="mt-3 text-sm text-red-400">{t('divination.error.compute')}: {computeErr}</p>}
      </section>

      {/* shaking animation */}
      {shaking && (
        <div className="flex justify-center py-8">
          <div className="animate-bounce text-6xl">🎋</div>
        </div>
      )}

      {/* chart */}
      {chart && !shaking && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          {/* tier badge */}
          <div className="mb-4 text-center">
            <span className={`rounded-full px-4 py-1.5 text-sm font-medium ${tierColor(chart.tier)}`}>
              {chart.tier}
            </span>
          </div>

          {/* title */}
          <h2 className="mb-4 text-center text-lg font-semibold text-fg">{chart.title}</h2>

          {/* poem */}
          <div className="mb-4 rounded-lg border border-border bg-bg p-6">
            <p className="whitespace-pre-wrap text-center text-xl leading-loose text-fg" style={{ fontFamily: 'serif' }}>
              {chart.poem}
            </p>
          </div>

          {/* traditional interpretation */}
          <div className="mb-4 rounded-lg border border-border bg-bg p-4 text-sm leading-relaxed text-muted">
            <span className="font-medium text-fg">{t('divination.chart.interpret')}: </span>
            {chart.interpret}
          </div>

          {/* AI interpret */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('divination.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('divination.interpret.stop') : interpretation ? t('divination.interpret.retry') : t('divination.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('divination.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('divination.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('divination.interpret.hideReasoning') : t('divination.interpret.showReasoning')}
                </button>
                {showReasoning && (
                  <pre className="mt-1 max-h-60 overflow-y-auto whitespace-pre-wrap rounded-lg border border-border bg-bg p-3 text-xs text-muted">
                    {reasoning}{streaming && <span className="animate-pulse">▋</span>}
                  </pre>
                )}
              </div>
            )}

            {interpretation && (
              <article className="whitespace-pre-wrap text-sm leading-relaxed text-fg">
                {interpretation}{streaming && <span className="animate-pulse">▋</span>}
              </article>
            )}

            {usage && (
              <div className="mt-3 border-t border-border pt-2 text-xs text-muted">
                {t('divination.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

function tierColor(tier: string): string {
  if (tier.includes('上上')) return 'bg-green-500/20 text-green-300'
  if (tier.includes('上')) return 'bg-green-500/20 text-green-300'
  if (tier.includes('中上')) return 'bg-yellow-500/20 text-yellow-200'
  if (tier.includes('中')) return 'bg-yellow-500/20 text-yellow-300'
  if (tier.includes('下下')) return 'bg-red-500/20 text-red-300'
  if (tier.includes('下')) return 'bg-red-500/20 text-red-300'
  return 'bg-primary/20 text-primary'
}
