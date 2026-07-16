// 称骨算命 page. Users enter birth date/time to see their bone weight
// and traditional fortune poem, plus AI interpretation.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Weighbone as WeighboneApi,
  type WeighboneChart,
  type WeighboneInput,
  type InterpretStreamEvent,
  streamWeighboneInterpret,
} from '../../api/client'

export default function WeighbonePage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  const today = new Date()
  const [year, setYear] = useState(today.getFullYear() - 30)
  const [month, setMonth] = useState(1)
  const [day, setDay] = useState(1)
  const [hour, setHour] = useState(12)
  const [minute, setMinute] = useState(0)

  const [chart, setChart] = useState<WeighboneChart | null>(null)
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
    (): WeighboneInput => ({ year, month, day, hour, minute, lang }),
    [year, month, day, hour, minute, lang],
  )

  const onCompute = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await WeighboneApi.compute(buildInput())
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
      await streamWeighboneInterpret(
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
        <h1 className="text-3xl font-bold text-fg">{t('weighbone.title')}</h1>
        <p className="mt-2 text-muted">{t('weighbone.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="flex flex-wrap items-end gap-4">
          <div>
            <label className="mb-1 block text-xs text-muted">{t('weighbone.form.birthdate')}</label>
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

          <div>
            <label className="mb-1 block text-xs text-muted">{t('weighbone.form.birthtime')}</label>
            <div className="flex gap-2">
              <select className={inputCls} value={hour} onChange={(e) => setHour(Number(e.target.value))}>
                {Array.from({ length: 24 }, (_, i) => i).map((h) => <option key={h} value={h}>{String(h).padStart(2, '0')}</option>)}
              </select>
              <select className={inputCls} value={minute} onChange={(e) => setMinute(Number(e.target.value))}>
                {Array.from({ length: 60 }, (_, i) => i).map((m) => <option key={m} value={m}>{String(m).padStart(2, '0')}</option>)}
              </select>
            </div>
          </div>

          <button
            type="button"
            onClick={onCompute}
            disabled={computing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {computing ? t('weighbone.form.computing') : t('weighbone.form.submit')}
          </button>
        </div>
        {computeErr && <p className="mt-3 text-sm text-red-400">{t('weighbone.error.compute')}: {computeErr}</p>}
      </section>

      {/* chart */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          {/* total weight hero */}
          <div className="mb-4 rounded-lg border border-primary bg-primary/5 p-6 text-center">
            <div className="text-xs text-muted">{t('weighbone.chart.totalWeight')}</div>
            <div className="my-2 text-5xl font-bold text-primary">{chart.totalWeight}</div>
            <span className={`rounded-full px-3 py-1 text-xs font-medium ${categoryColor(chart.category)}`}>
              {t('weighbone.chart.category')}: {chart.category}
            </span>
          </div>

          {/* breakdown */}
          <div className="mb-4 grid gap-3 sm:grid-cols-4">
            <WeightCard label={t('weighbone.chart.yearWeight')} value={chart.yearWeight} />
            <WeightCard label={t('weighbone.chart.monthWeight')} value={chart.monthWeight} />
            <WeightCard label={t('weighbone.chart.dayWeight')} value={chart.dayWeight} />
            <WeightCard label={t('weighbone.chart.hourWeight')} value={chart.hourWeight} />
          </div>

          {/* poem */}
          <div className="mb-4 rounded-lg border border-border bg-bg p-4">
            <div className="mb-2 text-sm font-medium text-fg">{t('weighbone.chart.poem')}</div>
            <p className="whitespace-pre-wrap text-center text-lg leading-relaxed text-fg" style={{ fontFamily: 'serif' }}>
              {chart.poem}
            </p>
          </div>

          {/* description */}
          <div className="mb-4 rounded-lg border border-border bg-bg p-4 text-sm text-muted">
            {chart.description}
          </div>

          {/* AI interpret */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('weighbone.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('weighbone.interpret.stop') : interpretation ? t('weighbone.interpret.retry') : t('weighbone.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('weighbone.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('weighbone.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('weighbone.interpret.hideReasoning') : t('weighbone.interpret.showReasoning')}
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
                {t('weighbone.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

function WeightCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-bg p-3 text-center">
      <div className="text-xs text-muted">{label}</div>
      <div className="mt-1 text-lg font-medium text-fg">{value}</div>
    </div>
  )
}

function categoryColor(cat: string): string {
  if (cat.startsWith('上')) return 'bg-green-500/20 text-green-300'
  if (cat === '中上') return 'bg-green-500/20 text-green-300'
  if (cat === '中') return 'bg-yellow-500/20 text-yellow-300'
  if (cat === '中下' || cat === '下下') return 'bg-red-500/20 text-red-300'
  return 'bg-primary/20 text-primary'
}

const inputCls = 'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary'