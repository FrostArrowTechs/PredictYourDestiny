// 梅花易数 page. Users can cast by time or by 3 numbers, see the
// hexagrams (本卦/互卦/变卦), 体用 analysis, and AI interpretation.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  PlumFlower as PlumFlowerApi,
  type PlumFlowerChart,
  type PlumFlowerInput,
  type Hexagram,
  type InterpretStreamEvent,
  streamPlumFlowerInterpret,
} from '../../api/client'

export default function PlumFlowerPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  const [mode, setMode] = useState<'time' | 'number'>('time')
  const today = new Date()
  const [year, setYear] = useState(today.getFullYear())
  const [month, setMonth] = useState(today.getMonth() + 1)
  const [day, setDay] = useState(today.getDate())
  const [hour, setHour] = useState(today.getHours())
  const [nums, setNums] = useState('')
  const [question, setQuestion] = useState('')

  const [chart, setChart] = useState<PlumFlowerChart | null>(null)
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
    (): PlumFlowerInput => {
      const base: PlumFlowerInput = { lang, question: question.trim() || undefined }
      if (mode === 'time') {
        return { ...base, year, month, day, hour }
      }
      // number mode: pass numbers as question string for engine to parse
      return { ...base, question: nums }
    },
    [mode, year, month, day, hour, nums, question, lang],
  )

  const onCompute = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await PlumFlowerApi.compute(buildInput())
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
      await streamPlumFlowerInterpret(
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

  const years: number[] = []
  for (let y = 1900; y <= today.getFullYear() + 1; y++) years.push(y)

  return (
    <div className="space-y-6">
      <header className="text-center">
        <h1 className="text-3xl font-bold text-fg">{t('plumflower.title')}</h1>
        <p className="mt-2 text-muted">{t('plumflower.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        {/* mode switch */}
        <div className="mb-4 flex gap-2">
          <button type="button" onClick={() => setMode('time')} className={pill(mode === 'time')}>
            {t('plumflower.form.timeMode')}
          </button>
          <button type="button" onClick={() => setMode('number')} className={pill(mode === 'number')}>
            {t('plumflower.form.numberMode')}
          </button>
        </div>

        {mode === 'time' ? (
          <div className="flex flex-wrap items-end gap-4">
            <div>
              <label className="mb-1 block text-xs text-muted">{t('plumflower.form.datetime')}</label>
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
                <select className={inputCls} value={hour} onChange={(e) => setHour(Number(e.target.value))}>
                  {Array.from({ length: 24 }, (_, i) => i).map((h) => <option key={h} value={h}>{String(h).padStart(2, '0')}</option>)}
                </select>
              </div>
            </div>
          </div>
        ) : (
          <div>
            <label className="mb-1 block text-xs text-muted">{t('plumflower.form.numbers')}</label>
            <input
              type="text"
              className={inputCls + ' w-full'}
              placeholder={t('plumflower.form.numbersHint')}
              value={nums}
              onChange={(e) => setNums(e.target.value)}
            />
          </div>
        )}

        {mode === 'time' && (
          <div className="mt-4">
            <label className="mb-1 block text-xs text-muted">{t('plumflower.form.question')}</label>
            <input
              type="text"
              className={inputCls + ' w-full'}
              placeholder={t('plumflower.form.questionHint')}
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
            />
          </div>
        )}

        <div className="mt-4">
          <button
            type="button"
            onClick={onCompute}
            disabled={computing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {computing ? t('plumflower.form.computing') : t('plumflower.form.submit')}
          </button>
        </div>
        {computeErr && <p className="mt-3 text-sm text-red-400">{t('plumflower.error.compute')}: {computeErr}</p>}
      </section>

      {/* chart */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          {/* three hexagrams */}
          <div className="mb-4 grid gap-3 sm:grid-cols-3">
            <HexagramCard hex={chart.original} label={t('plumflower.chart.original')} upperLowerLabel={t('plumflower.chart.upperLower')} highlight />
            <HexagramCard hex={chart.mutual} label={t('plumflower.chart.mutual')} upperLowerLabel={t('plumflower.chart.upperLower')} />
            <HexagramCard hex={chart.changed} label={t('plumflower.chart.changed')} upperLowerLabel={t('plumflower.chart.upperLower')} />
          </div>

          {/* changing line + body/use */}
          <div className="mb-4 rounded-lg border border-border bg-bg p-4">
            <div className="mb-2 text-sm font-medium text-fg">{t('plumflower.chart.analysisTitle')}</div>
            <div className="grid gap-2 text-sm sm:grid-cols-2">
              <div>
                <span className="text-muted">{t('plumflower.chart.changingLine')}: </span>
                <span className="text-fg">{t('plumflower.chart.lineNum', { n: chart.changingLine })}</span>
              </div>
              <div>
                <span className="text-muted">{t('plumflower.chart.bodyTrig')}: </span>
                <span className="text-fg">{chart.bodyTrigram}（{chart.bodyWuXing}）</span>
              </div>
              <div>
                <span className="text-muted">{t('plumflower.chart.useTrig')}: </span>
                <span className="text-fg">{chart.useTrigram}（{chart.useWuXing}）</span>
              </div>
              <div>
                <span className="text-muted">{t('plumflower.chart.relationship')}: </span>
                <span className={`font-medium ${trendColor(chart.trend)}`}>{chart.relationship}</span>
              </div>
            </div>
            <div className="mt-2 text-sm text-muted">{chart.analysis}</div>
          </div>

          {/* AI interpret */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('plumflower.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('plumflower.interpret.stop') : interpretation ? t('plumflower.interpret.retry') : t('plumflower.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('plumflower.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('plumflower.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('plumflower.interpret.hideReasoning') : t('plumflower.interpret.showReasoning')}
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
                {t('plumflower.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

// Render a hexagram with its trigram lines (visual yang/yin lines).
function HexagramCard({ hex, label, upperLowerLabel, highlight }: { hex: Hexagram; label: string; upperLowerLabel: string; highlight?: boolean }) {
  return (
    <div className={`rounded-lg border p-4 text-center ${highlight ? 'border-primary bg-primary/5' : 'border-border bg-bg'}`}>
      <div className="text-xs text-muted">{label}</div>
      <div className="my-2 text-lg font-semibold text-fg" style={{ fontFamily: 'serif' }}>{hex.name}</div>
      {/* lines: render top to bottom (index 5 → 0) */}
      <div className="flex flex-col-reverse items-center gap-1 py-2">
        {hex.lines.map((line, i) => (
          <div key={i} className="h-1.5">
            {line === 1 ? (
              <div className="h-1.5 w-12 rounded bg-fg" />
            ) : (
              <div className="flex gap-1">
                <div className="h-1.5 w-5 rounded bg-fg" />
                <div className="h-1.5 w-5 rounded bg-fg" />
              </div>
            )}
          </div>
        ))}
      </div>
      <div className="text-xs text-muted">
        {upperLowerLabel.replace('{upper}', hex.upperTrig).replace('{lower}', hex.lowerTrig)}
      </div>
    </div>
  )
}

function trendColor(trend: string): string {
  if (trend === '吉') return 'text-green-400'
  if (trend === '凶') return 'text-red-400'
  return 'text-yellow-400'
}

function pill(active: boolean): string {
  return [
    'rounded-full px-3 py-1 text-xs transition-colors',
    active ? 'bg-primary text-bg' : 'border border-border text-muted hover:text-fg',
  ].join(' ')
}

const inputCls = 'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary'