// Huangli (万年历黄历) page. Users select a date to view traditional
// calendar information including 宜/忌, 吉神/凶煞, etc.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Huangli as HuangliApi,
  type HuangliChart,
  type HuangliInput,
  type InterpretStreamEvent,
  streamHuangliInterpret,
} from '../../api/client'

export default function HuangliPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  // form state - default to today
  const today = new Date()
  const [year, setYear] = useState(today.getFullYear())
  const [month, setMonth] = useState(today.getMonth() + 1)
  const [day, setDay] = useState(today.getDate())

  // result state
  const [chart, setChart] = useState<HuangliChart | null>(null)
  const [computing, setComputing] = useState(false)
  const [computeErr, setComputeErr] = useState<string | null>(null)

  // interpret state
  const depth = 'brief'
  const modelId = ''
  const [interpretation, setInterpretation] = useState('')
  const [reasoning, setReasoning] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [interpretErr, setInterpretErr] = useState<string | null>(null)
  const [showReasoning, setShowReasoning] = useState(false)
  const [usage, setUsage] = useState<InterpretStreamEvent['usage'] | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const buildInput = useCallback(
    (): HuangliInput => ({
      year,
      month,
      day,
      lang,
      interpretDepth: depth,
      model: modelId || undefined,
    }),
    [year, month, day, lang, depth, modelId],
  )

  const onCompute = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await HuangliApi.compute(buildInput())
      setChart(res.data)
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      setComputeErr(msg)
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
      await streamHuangliInterpret(
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

  // year options
  const years: number[] = []
  for (let y = 1900; y <= today.getFullYear() + 1; y++) years.push(y)

  return (
    <div className="space-y-6">
      <header className="text-center">
        <h1 className="text-3xl font-bold text-fg">{t('huangli.title')}</h1>
        <p className="mt-2 text-muted">{t('huangli.subtitle')}</p>
      </header>

      {/* ── input form ── */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="flex flex-wrap items-end gap-4">
          <div>
            <label className="mb-1 block text-xs text-muted">{t('huangli.form.date')}</label>
            <div className="flex gap-2">
              <select
                className="rounded-md border border-border bg-surface px-3 py-2 text-fg outline-none focus:border-primary"
                value={year}
                onChange={(e) => setYear(Number(e.target.value))}
              >
                {years.map((y) => (
                  <option key={y} value={y}>{y}</option>
                ))}
              </select>
              <select
                className="rounded-md border border-border bg-surface px-3 py-2 text-fg outline-none focus:border-primary"
                value={month}
                onChange={(e) => setMonth(Number(e.target.value))}
              >
                {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
              <select
                className="rounded-md border border-border bg-surface px-3 py-2 text-fg outline-none focus:border-primary"
                value={day}
                onChange={(e) => setDay(Number(e.target.value))}
              >
                {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                  <option key={d} value={d}>{d}</option>
                ))}
              </select>
            </div>
          </div>

          <button
            type="button"
            onClick={onCompute}
            disabled={computing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {computing ? t('huangli.form.computing') : t('huangli.form.submit')}
          </button>
        </div>

        {computeErr && (
          <p className="mt-3 text-sm text-red-400">{t('huangli.error.compute')}: {computeErr}</p>
        )}
      </section>

      {/* ── chart ── */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          <div className="mb-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('huangli.chart.solar')}</div>
              <div className="text-sm font-medium text-fg">{chart.solar}</div>
            </div>
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('huangli.chart.lunar')}</div>
              <div className="text-sm font-medium text-fg">{chart.lunar}</div>
            </div>
            <div className="rounded-lg border border-border bg-bg p-3">
              <div className="text-xs text-muted">{t('huangli.chart.ganZhi')}</div>
              <div className="text-sm font-medium text-fg">{chart.yearGanZhi}年 {chart.monthGanZhi}月 {chart.dayGanZhi}日</div>
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            {/* 宜 */}
            <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-3">
              <div className="mb-2 text-sm font-medium text-green-400">{t('huangli.chart.yi')}</div>
              <div className="flex flex-wrap gap-1">
                {chart.yi.map((item, i) => (
                  <span key={i} className="rounded bg-green-500/20 px-2 py-0.5 text-xs text-green-300">{item}</span>
                ))}
              </div>
            </div>

            {/* 忌 */}
            <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-3">
              <div className="mb-2 text-sm font-medium text-red-400">{t('huangli.chart.ji')}</div>
              <div className="flex flex-wrap gap-1">
                {chart.ji.map((item, i) => (
                  <span key={i} className="rounded bg-red-500/20 px-2 py-0.5 text-xs text-red-300">{item}</span>
                ))}
              </div>
            </div>
          </div>

          {/* 其他信息 */}
          <div className="mt-4 grid gap-2 text-sm">
            <div className="flex gap-4">
              <span className="text-muted">{t('huangli.chart.jiShen')}:</span>
              <span className="text-fg">{chart.jiShen.join('、')}</span>
            </div>
            <div className="flex gap-4">
              <span className="text-muted">{t('huangli.chart.xiongSha')}:</span>
              <span className="text-fg">{chart.xiongSha.join('、')}</span>
            </div>
            <div className="flex gap-4">
              <span className="text-muted">{t('huangli.chart.pengZu')}:</span>
              <span className="text-fg">{chart.pengZu}</span>
            </div>
            <div className="flex gap-4">
              <span className="text-muted">{t('huangli.chart.chong')}:</span>
              <span className="text-fg">{chart.chong}</span>
              <span className="text-muted ml-4">{t('huangli.chart.sha')}:</span>
              <span className="text-fg">{chart.sha}</span>
            </div>
            <div className="flex gap-4">
              <span className="text-muted">{t('huangli.chart.xingZuo')}:</span>
              <span className="text-fg">{chart.xingZuo}</span>
              <span className="text-muted ml-4">{t('huangli.chart.erShiBaXiu')}:</span>
              <span className="text-fg">{chart.erShiBaXiu}</span>
            </div>
          </div>

          {/* ── AI interpret ── */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('huangli.interpret.title')}</h3>
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
                {streaming ? t('huangli.interpret.stop') : interpretation ? t('huangli.interpret.retry') : t('huangli.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('huangli.interpret.empty')}</p>
            )}

            {interpretErr && (
              <p className="text-sm text-red-400">{t('huangli.interpret.error')}: {interpretErr}</p>
            )}

            {reasoning && (
              <div className="mb-3">
                <button
                  type="button"
                  onClick={() => setShowReasoning((v) => !v)}
                  className="text-xs text-muted underline-offset-2 hover:underline"
                >
                  {showReasoning ? t('huangli.interpret.hideReasoning') : t('huangli.interpret.showReasoning')}
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
                {t('huangli.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}