// Zodiac fortune page. Users enter their birth year to see their
// zodiac fortune scores, relationships, and AI interpretation.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Zodiac as ZodiacApi,
  type ZodiacChart,
  type ZodiacInput,
  type InterpretStreamEvent,
  streamZodiacInterpret,
} from '../../api/client'

export default function ZodiacPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  // form state
  const today = new Date()
  const [year, setYear] = useState(today.getFullYear() - 25)
  const [month, setMonth] = useState(1)
  const [day, setDay] = useState(1)

  // result state
  const [chart, setChart] = useState<ZodiacChart | null>(null)
  const [computing, setComputing] = useState(false)
  const [computeErr, setComputeErr] = useState<string | null>(null)

  // interpret state
  const [interpretation, setInterpretation] = useState('')
  const [reasoning, setReasoning] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [interpretErr, setInterpretErr] = useState<string | null>(null)
  const [showReasoning, setShowReasoning] = useState(false)
  const [usage, setUsage] = useState<InterpretStreamEvent['usage'] | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const buildInput = useCallback(
    (): ZodiacInput => ({ year, month, day, lang }),
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
      const res = await ZodiacApi.compute(buildInput())
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
      await streamZodiacInterpret(
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
        <h1 className="text-3xl font-bold text-fg">{t('zodiac.title')}</h1>
        <p className="mt-2 text-muted">{t('zodiac.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="flex flex-wrap items-end gap-4">
          <div>
            <label className="mb-1 block text-xs text-muted">{t('zodiac.form.birthdate')}</label>
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
            {computing ? t('zodiac.form.computing') : t('zodiac.form.submit')}
          </button>
        </div>

        {computeErr && <p className="mt-3 text-sm text-red-400">{t('zodiac.error.compute')}: {computeErr}</p>}
      </section>

      {/* chart */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          {/* Header with zodiac + year */}
          <div className="mb-4 flex items-center gap-4">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/20 text-3xl">
              {zodiacEmoji(chart.zodiac)}
            </div>
            <div>
              <div className="text-lg font-semibold text-fg">
                {t('zodiac.chart.yourZodiac')}: {chart.zodiac}
              </div>
              <div className="text-sm text-muted">
                {chart.year}{t('zodiac.chart.year')} · {t('zodiac.chart.liuNian')}: {chart.liuNianZodiac}（{chart.liuNianZhi}）
              </div>
            </div>
          </div>

          {/* Scores */}
          <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <ScoreCard label={t('zodiac.chart.overall')} score={chart.overallScore} highlight />
            <ScoreCard label={t('zodiac.chart.career')} score={chart.careerScore} />
            <ScoreCard label={t('zodiac.chart.wealth')} score={chart.wealthScore} />
            <ScoreCard label={t('zodiac.chart.love')} score={chart.loveScore} />
            <ScoreCard label={t('zodiac.chart.health')} score={chart.healthScore} />
          </div>

          {/* Relations */}
          <div className="mb-4">
            <h3 className="mb-2 text-sm font-medium text-fg">{t('zodiac.chart.relations')}</h3>
            <div className="space-y-2">
              {chart.relations.map((rel, i) => (
                <div key={i} className="rounded-lg border border-border bg-bg p-3 text-sm">
                  <span className={`mr-2 rounded px-2 py-0.5 text-xs ${relationColor(rel.type)}`}>
                    {rel.type} · {rel.with}
                  </span>
                  <span className="text-muted">{rel.effect}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Lucky elements */}
          <div className="mb-4 grid gap-3 sm:grid-cols-3">
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('zodiac.chart.luckyColors')}</div>
              <div className="flex flex-wrap gap-1 mt-1">
                {chart.luckyColors.map((c, i) => (
                  <span key={i} className="rounded bg-primary/20 px-2 py-0.5 text-xs text-primary">{c}</span>
                ))}
              </div>
            </div>
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('zodiac.chart.luckyNumbers')}</div>
              <div className="mt-1 text-sm text-fg">{chart.luckyNumbers.join(', ')}</div>
            </div>
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('zodiac.chart.luckyDir')}</div>
              <div className="mt-1 text-sm text-fg">{chart.luckyDir}</div>
            </div>
          </div>

          {/* Tips & Warns */}
          {(chart.tips?.length > 0 || chart.warns?.length > 0) && (
            <div className="grid gap-3 sm:grid-cols-2">
              {chart.tips?.length > 0 && (
                <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-3">
                  <div className="mb-2 text-sm font-medium text-green-400">{t('zodiac.chart.tips')}</div>
                  <ul className="space-y-1 text-xs text-muted">
                    {chart.tips.map((tip, i) => <li key={i}>• {tip}</li>)}
                  </ul>
                </div>
              )}
              {chart.warns?.length > 0 && (
                <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/5 p-3">
                  <div className="mb-2 text-sm font-medium text-yellow-400">{t('zodiac.chart.warns')}</div>
                  <ul className="space-y-1 text-xs text-muted">
                    {chart.warns.map((w, i) => <li key={i}>• {w}</li>)}
                  </ul>
                </div>
              )}
            </div>
          )}

          {/* AI interpret */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('zodiac.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('zodiac.interpret.stop') : interpretation ? t('zodiac.interpret.retry') : t('zodiac.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('zodiac.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('zodiac.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('zodiac.interpret.hideReasoning') : t('zodiac.interpret.showReasoning')}
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
                {t('zodiac.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
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

function relationColor(type: string): string {
  switch (type) {
    case '三合':
    case '六合':
      return 'bg-green-500/20 text-green-300'
    case '相冲':
      return 'bg-red-500/20 text-red-300'
    case '相害':
      return 'bg-yellow-500/20 text-yellow-300'
    default:
      return 'bg-primary/20 text-primary'
  }
}

function zodiacEmoji(z: string): string {
  const map: Record<string, string> = {
    鼠: '🐭', 牛: '🐮', 虎: '🐯', 兔: '🐰', 龙: '🐲', 蛇: '🐍',
    马: '🐴', 羊: '🐑', 猴: '🐵', 鸡: '🐔', 狗: '🐶', 猪: '🐷',
  }
  return map[z] || '🌟'
}

const inputCls = 'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary'