// Constellation (sun-sign) fortune page. Users enter their birth date
// to see their sun-sign traits, daily fortune scores, and AI reading.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Constellation as ConstellationApi,
  type ConstellationChart,
  type ConstellationInput,
  type InterpretStreamEvent,
  streamConstellationInterpret,
} from '../../api/client'

const inputCls = 'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary'

export default function ConstellationPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  const today = new Date()
  const [year, setYear] = useState(today.getFullYear() - 25)
  const [month, setMonth] = useState(1)
  const [day, setDay] = useState(1)

  const [chart, setChart] = useState<ConstellationChart | null>(null)
  const [computing, setComputing] = useState(false)
  const [computeErr, setComputeErr] = useState<string | null>(null)

  const [interpretation, setInterpretation] = useState('')
  const [reasoning, setReasoning] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [interpretErr, setInterpretErr] = useState<string | null>(null)
  const [showReasoning, setShowReasoning] = useState(false)
  const [usage, setUsage] = useState<InterpretStreamEvent['usage'] | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const buildInput = useCallback(
    (): ConstellationInput => ({ year, month, day, lang }),
    [year, month, day, lang],
  )

  const onCompute = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await ConstellationApi.compute(buildInput())
      setChart(res.data)
    } catch (e) {
      setComputeErr(e instanceof Error ? e.message : String(e))
      setChart(null)
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
      await streamConstellationInterpret(
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

  const years: number[] = []
  for (let y = 1900; y <= today.getFullYear() + 1; y++) years.push(y)

  return (
    <div className="space-y-6">
      <header className="text-center">
        <h1 className="text-3xl font-bold text-fg">{t('constellation.title')}</h1>
        <p className="mt-2 text-muted">{t('constellation.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="flex flex-wrap items-end gap-4">
          <div>
            <label className="mb-1 block text-xs text-muted">{t('constellation.form.birthdate')}</label>
            <div className="flex gap-2">
              <select className={inputCls} value={year} onChange={(e) => setYear(Number(e.target.value))}>
                {years.map((y) => <option key={y} value={y}>{y}</option>)}
              </select>
              <select className={inputCls} value={month} onChange={(e) => setMonth(Number(e.target.value))}>
                {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
              <select className={inputCls} value={day} onChange={(e) => setDay(Number(e.target.value))}>
                {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => <option key={d} value={d}>{d}</option>)}
              </select>
            </div>
          </div>

          <button
            type="button"
            onClick={onCompute}
            disabled={computing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {computing ? t('constellation.form.computing') : t('constellation.form.submit')}
          </button>
        </div>

        {computeErr && <p className="mt-3 text-sm text-red-400">{t('constellation.error.compute')}: {computeErr}</p>}
      </section>

      {/* chart */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          {/* header */}
          <div className="mb-4 flex items-center gap-4">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/20 text-3xl">
              {signEmoji(chart.sign)}
            </div>
            <div>
              <div className="text-lg font-semibold text-fg">
                {t('constellation.chart.yourSign')}: {chart.sign}（{chart.signLatin}）
              </div>
              <div className="text-sm text-muted">
                {t('constellation.chart.element')}: {chart.element} · {t('constellation.chart.quality')}: {chart.quality} · {t('constellation.chart.ruler')}: {chart.ruler}
              </div>
              <div className="text-xs text-muted">{t('constellation.chart.dateRange')}: {chart.dateRange}</div>
            </div>
          </div>

          {/* traits */}
          <div className="mb-4 grid gap-3 sm:grid-cols-3">
            <TraitCard title={t('constellation.chart.strengths')} items={chart.strengths} color="green" />
            <TraitCard title={t('constellation.chart.weakness')} items={chart.weakness} color="yellow" />
            <TraitCard title={t('constellation.chart.keywords')} items={chart.keywords} color="primary" />
          </div>

          {/* scores */}
          <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <ScoreCard label={t('constellation.chart.overall')} score={chart.overallScore} highlight />
            <ScoreCard label={t('constellation.chart.career')} score={chart.careerScore} />
            <ScoreCard label={t('constellation.chart.love')} score={chart.loveScore} />
            <ScoreCard label={t('constellation.chart.wealth')} score={chart.wealthScore} />
            <ScoreCard label={t('constellation.chart.health')} score={chart.healthScore} />
          </div>

          {/* lucky + match */}
          <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('constellation.chart.luckyColors')}</div>
              <div className="mt-1 flex flex-wrap gap-1">
                {chart.luckyColors.map((c, i) => (
                  <span key={i} className="rounded bg-primary/20 px-2 py-0.5 text-xs text-primary">{c}</span>
                ))}
              </div>
            </div>
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('constellation.chart.luckyNumbers')}</div>
              <div className="mt-1 text-sm text-fg">{chart.luckyNumbers.join(', ')}</div>
            </div>
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('constellation.chart.luckyDir')}</div>
              <div className="mt-1 text-sm text-fg">{chart.luckyDir}</div>
            </div>
            <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-3">
              <div className="text-xs text-muted">{t('constellation.chart.bestMatch')}</div>
              <div className="mt-1 text-sm text-green-400">{chart.bestMatch}</div>
            </div>
            <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/5 p-3">
              <div className="text-xs text-muted">{t('constellation.chart.worstMatch')}</div>
              <div className="mt-1 text-sm text-yellow-400">{chart.worstMatch}</div>
            </div>
          </div>

          {/* AI interpret */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('constellation.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('constellation.interpret.stop') : interpretation ? t('constellation.interpret.retry') : t('constellation.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('constellation.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('constellation.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('constellation.interpret.hideReasoning') : t('constellation.interpret.showReasoning')}
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
                {t('constellation.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

function ScoreCard({ label, score, highlight }: { label: string; score: number; highlight?: boolean }) {
  const color = score >= 70 ? 'text-green-400' : score >= 50 ? 'text-yellow-400' : 'text-red-400'
  return (
    <div className={`rounded-lg border p-3 text-center ${highlight ? 'border-primary bg-primary/5' : 'border-border bg-bg'}`}>
      <div className="text-xs text-muted">{label}</div>
      <div className={`mt-1 text-2xl font-bold ${color}`}>{score}</div>
    </div>
  )
}

function TraitCard({ title, items, color }: { title: string; items: string[]; color: 'green' | 'yellow' | 'primary' }) {
  const colorCls = color === 'green'
    ? 'border-green-500/30 bg-green-500/5 text-green-400'
    : color === 'yellow'
      ? 'border-yellow-500/30 bg-yellow-500/5 text-yellow-400'
      : 'border-primary/30 bg-primary/5 text-primary'
  return (
    <div className={`rounded-lg border p-3 ${colorCls}`}>
      <div className="mb-2 text-sm font-medium">{title}</div>
      <ul className="space-y-1 text-xs text-muted">
        {items.map((s, i) => <li key={i}>• {s}</li>)}
      </ul>
    </div>
  )
}

function signEmoji(sign: string): string {
  const map: Record<string, string> = {
    白羊座: '♈', 金牛座: '♉', 双子座: '♊', 巨蟹座: '♋',
    狮子座: '♌', 处女座: '♍', 天秤座: '♎', 天蝎座: '♏',
    射手座: '♐', 摩羯座: '♑', 水瓶座: '♒', 双鱼座: '♓',
  }
  return map[sign] || '✨'
}
