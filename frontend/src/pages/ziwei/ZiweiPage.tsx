// Ziwei (紫微斗数) natal chart page. Users enter birth date/time +
// gender; we render the 12 palaces and an AI interpretation.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Ziwei as ZiweiApi,
  type ZiweiChart,
  type ZiweiPalace,
  type ZiweiInput,
  type InterpretStreamEvent,
  streamZiweiInterpret,
} from '../../api/client'

const inputCls = 'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary'

export default function ZiweiPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  const today = new Date()
  const [year, setYear] = useState(today.getFullYear() - 25)
  const [month, setMonth] = useState(1)
  const [day, setDay] = useState(1)
  const [hour, setHour] = useState(12)
  const [minute, setMinute] = useState(0)
  const [gender, setGender] = useState<0 | 1>(1)
  const [leapMonthRule, setLeapMonthRule] = useState<ZiweiInput['ziweiLeapMonthRule']>('')

  const [chart, setChart] = useState<ZiweiChart | null>(null)
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
    (): ZiweiInput => ({ year, month, day, hour, minute, gender, lang, ziweiLeapMonthRule: leapMonthRule }),
    [year, month, day, hour, minute, gender, lang, leapMonthRule],
  )

  const onCompute = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await ZiweiApi.compute(buildInput())
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
      await streamZiweiInterpret(
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
        <h1 className="text-3xl font-bold text-fg">{t('ziwei.title')}</h1>
        <p className="mt-2 text-muted">{t('ziwei.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="flex flex-wrap items-end gap-4">
          <div>
            <label className="mb-1 block text-xs text-muted">{t('ziwei.form.birthdate')}</label>
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
            <label className="mb-1 block text-xs text-muted">{t('ziwei.form.birthtime')}</label>
            <div className="flex gap-2">
              <select className={inputCls} value={hour} onChange={(e) => setHour(Number(e.target.value))}>
                {Array.from({ length: 24 }, (_, i) => i).map((h) => <option key={h} value={h}>{String(h).padStart(2, '0')}</option>)}
              </select>
              <select className={inputCls} value={minute} onChange={(e) => setMinute(Number(e.target.value))}>
                {Array.from({ length: 60 }, (_, i) => i).map((m) => <option key={m} value={m}>{String(m).padStart(2, '0')}</option>)}
              </select>
            </div>
          </div>

          <div>
            <label className="mb-1 block text-xs text-muted">{t('ziwei.form.gender')}</label>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => setGender(1)}
                className={`rounded-md px-3 py-1 text-sm ${gender === 1 ? 'bg-primary text-bg' : 'bg-bg text-muted'}`}
              >
                {t('ziwei.form.male')}
              </button>
              <button
                type="button"
                onClick={() => setGender(0)}
                className={`rounded-md px-3 py-1 text-sm ${gender === 0 ? 'bg-primary text-bg' : 'bg-bg text-muted'}`}
              >
                {t('ziwei.form.female')}
              </button>
            </div>
          </div>

          <div>
            <label className="mb-1 block text-xs text-muted">{t('ziwei.form.leapMonthRule')}</label>
            <select className={inputCls} value={leapMonthRule} onChange={(e) => setLeapMonthRule(e.target.value as ZiweiInput['ziweiLeapMonthRule'])}>
              <option value="">{t('ziwei.form.leapMonthOnly')}</option>
              <option value="as_next_month-v1">{t('ziwei.form.leapAsNext')}</option>
              <option value="split_at_day_15-v1">{t('ziwei.form.leapSplit15')}</option>
            </select>
          </div>

          <button
            type="button"
            onClick={onCompute}
            disabled={computing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {computing ? t('ziwei.form.computing') : t('ziwei.form.submit')}
          </button>
        </div>

        {computeErr && <p className="mt-3 text-sm text-red-400">{t('ziwei.error.compute')}: {computeErr}</p>}
      </section>

      {/* chart */}
      {chart && (
        <section className="space-y-4">
          <div className="rounded-lg border border-amber-500/40 bg-amber-500/5 p-4 text-sm">
            <div className="font-medium text-amber-500">{chart.rulePack.version} · {chart.rulePack.status}</div>
            {chart.warnings.map((warning) => <p key={warning} className="mt-2 text-xs text-muted">{warning}</p>)}
            {chart.rulePack.approximateRules.map((rule) => <p key={rule} className="mt-1 text-xs text-muted">• {rule}</p>)}
          </div>
          {/* Summary header */}
          <div className="rounded-2xl border border-border bg-surface p-5">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <Info label={t('ziwei.chart.solar')} value={chart.solarDate} />
              <Info label={t('ziwei.chart.lunar')} value={chart.lunarDate} />
              <Info label={t('ziwei.chart.wuXingJu')} value={chart.wuXingJu} />
              <Info label={t('ziwei.chart.mainStar')} value={chart.mainStarOfLife} highlight />
            </div>
            <div className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-4 border-t border-border pt-3">
              <Info label={t('ziwei.chart.yearGanZhi')} value={chart.yearGanZhi} />
              <Info label={t('ziwei.chart.monthGanZhi')} value={chart.monthGanZhi} />
              <Info label={t('ziwei.chart.dayGanZhi')} value={chart.dayGanZhi} />
              <Info
                label={t('ziwei.chart.daYun')}
                value={`${t('ziwei.chart.daYunStart')}${chart.daYunStartAge} · ${chart.daYunForward ? t('ziwei.chart.daYunForward') : t('ziwei.chart.daYunReverse')}`}
              />
            </div>
            <div className="mt-3 grid gap-4 sm:grid-cols-2 border-t border-border pt-3">
              <Info label={t('ziwei.chart.lifePalace')} value={`${chart.lifePalaceBranch}（命主：${chart.lifeRuler}）`} />
              <Info label={t('ziwei.chart.bodyPalace')} value={`${chart.bodyPalaceBranch}（身主：${chart.bodyRuler}）`} />
            </div>
          </div>

          {/* 12-palace grid */}
          <div className="rounded-2xl border border-border bg-surface p-5">
            <h3 className="mb-3 text-sm font-medium text-fg">{t('ziwei.chart.palaceList')}</h3>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {chart.palaces.map((p, i) => (
                <PalaceCard key={i} palace={p} t={t} />
              ))}
            </div>
          </div>

          {/* AI interpret */}
          <div className="rounded-2xl border border-border bg-surface p-5">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('ziwei.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('ziwei.interpret.stop') : interpretation ? t('ziwei.interpret.retry') : t('ziwei.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('ziwei.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('ziwei.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('ziwei.interpret.hideReasoning') : t('ziwei.interpret.showReasoning')}
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
                {t('ziwei.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

function Info({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div>
      <div className="text-xs text-muted">{label}</div>
      <div className={`mt-0.5 text-sm ${highlight ? 'font-medium text-primary' : 'text-fg'}`}>{value}</div>
    </div>
  )
}

function PalaceCard({ palace, t }: { palace: ZiweiPalace; t: any }) {
  const border = palace.isLife
    ? 'border-primary/60 bg-primary/5'
    : palace.isBody
      ? 'border-yellow-500/60 bg-yellow-500/5'
      : 'border-border bg-bg'
  return (
    <div className={`rounded-lg border p-3 ${border}`}>
      <div className="mb-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="font-medium text-fg">{palace.name}</span>
          <span className="rounded bg-primary/20 px-1.5 py-0.5 text-xs text-primary">{palace.branch}</span>
          {palace.isLife && <span className="rounded bg-primary px-1.5 py-0.5 text-xs text-bg">命</span>}
          {palace.isBody && <span className="rounded bg-yellow-500 px-1.5 py-0.5 text-xs text-bg">身</span>}
        </div>
        <div className="flex flex-wrap gap-1">
          {palace.transformations.map((item) => (
            <span key={`${item.star}-${item.label}`} className="rounded bg-red-500/20 px-2 py-0.5 text-xs text-red-400">{item.star}{item.label}</span>
          ))}
        </div>
      </div>
      {palace.stars.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {palace.stars.map((s: string, i: number) => (
            <span key={i} className="rounded bg-surface-2 px-2 py-0.5 text-xs text-fg">{s}</span>
          ))}
        </div>
      ) : (
        <div className="text-xs text-muted">（{t('ziwei.chart.empty')}）</div>
      )}
    </div>
  )
}
