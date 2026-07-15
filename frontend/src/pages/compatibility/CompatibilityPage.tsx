// Compatibility page. Users enter two people's birth dates to see
// their match scores and AI relationship analysis.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Compatibility as CompatApi,
  type CompatibilityChart,
  type CompatibilityInput,
  type InterpretStreamEvent,
  streamCompatibilityInterpret,
} from '../../api/client'

export default function CompatibilityPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  // Subject 1
  const today = new Date()
  const [y1, setY1] = useState(today.getFullYear() - 28)
  const [m1, setM1] = useState(1)
  const [d1, setD1] = useState(1)
  // Subject 2
  const [y2, setY2] = useState(today.getFullYear() - 26)
  const [m2, setM2] = useState(6)
  const [d2, setD2] = useState(15)

  // result state
  const [chart, setChart] = useState<CompatibilityChart | null>(null)
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
    (): CompatibilityInput => ({
      year: y1, month: m1, day: d1,
      second: { year: y2, month: m2, day: d2 },
      lang,
    }),
    [y1, m1, d1, y2, m2, d2, lang],
  )

  const onCompute = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await CompatApi.compute(buildInput())
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
      await streamCompatibilityInterpret(
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
        <h1 className="text-3xl font-bold text-fg">{t('compatibility.title')}</h1>
        <p className="mt-2 text-muted">{t('compatibility.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="grid gap-6 sm:grid-cols-2">
          {/* Subject 1 */}
          <div>
            <label className="mb-1 block text-xs font-medium text-fg">{t('compatibility.form.subject1')}</label>
            <div className="flex gap-2">
              <select className={inputCls} value={y1} onChange={(e) => setY1(Number(e.target.value))}>
                {years.map((y) => <option key={y} value={y}>{y}</option>)}
              </select>
              <select className={inputCls} value={m1} onChange={(e) => setM1(Number(e.target.value))}>
                {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
              <select className={inputCls} value={d1} onChange={(e) => setD1(Number(e.target.value))}>
                {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => <option key={d} value={d}>{d}</option>)}
              </select>
            </div>
          </div>

          {/* Subject 2 */}
          <div>
            <label className="mb-1 block text-xs font-medium text-fg">{t('compatibility.form.subject2')}</label>
            <div className="flex gap-2">
              <select className={inputCls} value={y2} onChange={(e) => setY2(Number(e.target.value))}>
                {years.map((y) => <option key={y} value={y}>{y}</option>)}
              </select>
              <select className={inputCls} value={m2} onChange={(e) => setM2(Number(e.target.value))}>
                {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
              <select className={inputCls} value={d2} onChange={(e) => setD2(Number(e.target.value))}>
                {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => <option key={d} value={d}>{d}</option>)}
              </select>
            </div>
          </div>
        </div>

        <div className="mt-4">
          <button
            type="button"
            onClick={onCompute}
            disabled={computing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {computing ? t('compatibility.form.computing') : t('compatibility.form.submit')}
          </button>
        </div>

        {computeErr && <p className="mt-3 text-sm text-red-400">{t('compatibility.error.compute')}: {computeErr}</p>}
      </section>

      {/* chart */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          {/* Subjects */}
          <div className="mb-4 flex items-center justify-center gap-4">
            <SubjectCard subject={chart.subject1} label={t('compatibility.form.subject1')} dayMasterLabel={t('compatibility.chart.dayMaster')} />
            <div className="text-2xl text-primary">💕</div>
            <SubjectCard subject={chart.subject2} label={t('compatibility.form.subject2')} dayMasterLabel={t('compatibility.chart.dayMaster')} />
          </div>

          {/* Overall score */}
          <div className="mb-4 rounded-lg border border-primary bg-primary/5 p-4 text-center">
            <div className="text-xs text-muted">{t('compatibility.chart.overall')}</div>
            <div className={`text-4xl font-bold ${scoreColor(chart.overallScore)}`}>{chart.overallScore}</div>
            <div className="mt-1 text-sm text-fg">{chart.summary}</div>
          </div>

          {/* Dimension scores */}
          <div className="mb-4 grid gap-3 sm:grid-cols-3">
            <DimCard label={t('compatibility.chart.chemistry')} score={chart.chemistryScore} />
            <DimCard label={t('compatibility.chart.harmony')} score={chart.harmonyScore} />
            <DimCard label={t('compatibility.chart.stability')} score={chart.stabilityScore} />
          </div>

          {/* Factors */}
          <div className="mb-4">
            <h3 className="mb-2 text-sm font-medium text-fg">{t('compatibility.chart.factors')}</h3>
            <div className="space-y-2">
              {chart.factors.map((f, i) => (
                <div key={i} className="flex items-start gap-3 rounded-lg border border-border bg-bg p-3 text-sm">
                  <span className={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${factorColor(f.score)}`}>
                    {f.factor} {f.score > 0 ? `+${f.score}` : f.score}
                  </span>
                  <span className="text-muted">{f.detail}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Tips */}
          <div className="rounded-lg border border-border bg-bg p-3 text-sm text-fg">
            <span className="font-medium text-primary">{t('compatibility.chart.advice')}: </span>
            {chart.tips}
          </div>

          {/* AI interpret */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('compatibility.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('compatibility.interpret.stop') : interpretation ? t('compatibility.interpret.retry') : t('compatibility.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('compatibility.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('compatibility.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('compatibility.interpret.hideReasoning') : t('compatibility.interpret.showReasoning')}
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
                {t('compatibility.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

function SubjectCard({ subject, label, dayMasterLabel }: { subject: CompatibilityChart['subject1']; label: string; dayMasterLabel: string }) {
  return (
    <div className="rounded-lg border border-border bg-bg p-3 text-center">
      <div className="text-xs text-muted">{label}</div>
      <div className="my-1 text-3xl">{zodiacEmoji(subject.zodiac)}</div>
      <div className="text-sm font-medium text-fg">{subject.zodiac}</div>
      <div className="text-xs text-muted">{subject.yearGanZhi}</div>
      <div className="text-xs text-muted">{dayMasterLabel}: {subject.dayGan}{subject.dayZhi}（{subject.dayWuXing}）</div>
    </div>
  )
}

function DimCard({ label, score }: { label: string; score: number }) {
  return (
    <div className="rounded-lg border border-border bg-bg p-3 text-center">
      <div className="text-xs text-muted">{label}</div>
      <div className={`mt-1 text-2xl font-bold ${scoreColor(score)}`}>{score}</div>
    </div>
  )
}

function scoreColor(score: number): string {
  if (score >= 75) return 'text-green-400'
  if (score >= 55) return 'text-yellow-400'
  return 'text-red-400'
}

function factorColor(score: number): string {
  if (score > 0) return 'bg-green-500/20 text-green-300'
  if (score < 0) return 'bg-red-500/20 text-red-300'
  return 'bg-primary/20 text-primary'
}

function zodiacEmoji(z: string): string {
  const map: Record<string, string> = {
    鼠: '🐭', 牛: '🐮', 虎: '🐯', 兔: '🐰', 龙: '🐲', 蛇: '🐍',
    马: '🐴', 羊: '🐑', 猴: '🐵', 鸡: '🐔', 狗: '🐶', 猪: '🐷',
  }
  return map[z] || '🌟'
}

const inputCls = 'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary'