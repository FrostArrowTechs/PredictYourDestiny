// Bazi feature page. Owns the full input → 排盘 → AI 解读 flow.
//
// Layout:
//   1. Input form (date / time / gender / longitude / depth / model)
//   2. Compute button → /api/bazi/compute → renders BaziChart
//   3. AI interpret button → /api/bazi/interpret (stream=true) →
//      renders the reading with a typewriter effect as deltas arrive
//
// The model picker is fetched from /api/settings (ai.models) so the
// admin can add models without a frontend redeploy. Unknown/missing
// config degrades gracefully: the picker just shows the default.
import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  api,
  Bazi as BaziApi,
  type BaziChart as BaziChartType,
  type BaziInput,
  type InterpretStreamEvent,
  type ModelCatalog,
  streamBaziInterpret,
} from '../../api/client'
import BaziChart from '../../components/charts/BaziChart'

// a handful of common Chinese cities → longitude, so users can pick
// instead of typing a number. The list is intentionally short; the
// manual longitude field covers anything else.
const CITY_LONGITUDES: Record<string, number> = {
  北京: 116.41,
  上海: 121.47,
  广州: 113.27,
  深圳: 114.06,
  成都: 104.07,
  重庆: 106.55,
  杭州: 120.16,
  西安: 108.94,
  武汉: 114.30,
  南京: 118.80,
  天津: 117.20,
  苏州: 120.62,
  长沙: 112.94,
  青岛: 120.38,
  郑州: 113.62,
  沈阳: 123.43,
  哈尔滨: 126.64,
  昆明: 102.72,
  厦门: 118.09,
  福州: 119.30,
  济南: 117.00,
  石家庄: 114.51,
  太原: 112.55,
  兰州: 103.83,
  贵阳: 106.71,
  南宁: 108.37,
  海口: 110.32,
  拉萨: 91.13,
  乌鲁木齐: 87.62,
  呼和浩特: 111.75,
}

export default function BaziPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  // form state
  const today = new Date()
  const [year, setYear] = useState(today.getFullYear() - 20)
  const [month, setMonth] = useState(1)
  const [day, setDay] = useState(1)
  const [hour, setHour] = useState(12)
  const [minute, setMinute] = useState(0)
  const [gender, setGender] = useState<0 | 1>(1)
  const [city, setCity] = useState<string>('')
  const [longitude, setLongitude] = useState<string>('')

  // result state
  const [chart, setChart] = useState<BaziChartType | null>(null)
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

  // effective longitude: explicit entry wins, else city, else 0 (no correction)
  const effLongitude = (() => {
    const lo = parseFloat(longitude)
    if (!Number.isNaN(lo)) return lo
    if (city && city in CITY_LONGITUDES) return CITY_LONGITUDES[city]
    return 0
  })()

  // fetch the model catalog once so the picker is tier-aware.
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
          /* ignore malformed */
        }
      })
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [])

  const buildInput = useCallback(
    (): BaziInput => ({
      year,
      month,
      day,
      hour,
      minute,
      gender,
      longitude: effLongitude || undefined,
      lang,
      interpretDepth: depth,
      model: modelId || undefined,
    }),
    [year, month, day, hour, minute, gender, effLongitude, lang, depth, modelId],
  )

  const onCompute = useCallback(async () => {
    setComputing(true)
    setComputeErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await BaziApi.compute(buildInput())
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
      await streamBaziInterpret(
        buildInput(),
        (ev) => {
          if (ev.error) {
            setInterpretErr(ev.error)
            return
          }
          if (ev.content) setInterpretation((p) => p + ev.content)
          if (ev.reasoning) setReasoning((p) => p + ev.reasoning)
          if (ev.usage) setUsage(ev.usage)
          if (ev.done) {
            // stream end handled below
          }
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

  // year options: a reasonable range so the select isn't 2000 rows.
  const years: number[] = []
  for (let y = 1900; y <= today.getFullYear() + 1; y++) years.push(y)

  return (
    <div className="space-y-6">
      <header className="text-center">
        <h1 className="text-3xl font-bold text-fg">{t('bazi.title')}</h1>
        <p className="mt-2 text-muted">{t('bazi.subtitle')}</p>
      </header>

      {/* ── input form ── */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
          <Field label={t('bazi.form.birthdate')}>
            <div className="flex gap-2">
              <select
                className={inputCls}
                value={year}
                onChange={(e) => setYear(Number(e.target.value))}
              >
                {years.map((y) => (
                  <option key={y} value={y}>
                    {y}
                  </option>
                ))}
              </select>
              <select
                className={inputCls}
                value={month}
                onChange={(e) => setMonth(Number(e.target.value))}
              >
                {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
              <select
                className={inputCls}
                value={day}
                onChange={(e) => setDay(Number(e.target.value))}
              >
                {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                  <option key={d} value={d}>
                    {d}
                  </option>
                ))}
              </select>
            </div>
          </Field>

          <Field label={t('bazi.form.birthtime')}>
            <div className="flex gap-2">
              <select
                className={inputCls}
                value={hour}
                onChange={(e) => setHour(Number(e.target.value))}
              >
                {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                  <option key={h} value={h}>
                    {String(h).padStart(2, '0')}
                  </option>
                ))}
              </select>
              <select
                className={inputCls}
                value={minute}
                onChange={(e) => setMinute(Number(e.target.value))}
              >
                {Array.from({ length: 60 }, (_, i) => i).map((m) => (
                  <option key={m} value={m}>
                    {String(m).padStart(2, '0')}
                  </option>
                ))}
              </select>
            </div>
          </Field>

          <Field label={t('bazi.form.gender')}>
            <div className="flex gap-4 pt-2">
              <label className="flex items-center gap-1 text-sm">
                <input
                  type="radio"
                  name="gender"
                  checked={gender === 1}
                  onChange={() => setGender(1)}
                />
                {t('bazi.form.male')}
              </label>
              <label className="flex items-center gap-1 text-sm">
                <input
                  type="radio"
                  name="gender"
                  checked={gender === 0}
                  onChange={() => setGender(0)}
                />
                {t('bazi.form.female')}
              </label>
            </div>
          </Field>

          <Field label={t('bazi.form.longitude')}>
            <select
              className={inputCls}
              value={city}
              onChange={(e) => {
                setCity(e.target.value)
                setLongitude('')
              }}
            >
              <option value="">—</option>
              {Object.keys(CITY_LONGITUDES).map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </Field>

          <Field label={t('bazi.interpret.depth')}>
            <div className="flex gap-2 pt-2">
              <button
                type="button"
                onClick={() => setDepth('brief')}
                className={pill(depth === 'brief')}
              >
                {t('bazi.interpret.brief')} · {t('bazi.interpret.tierFree')}
              </button>
              <button
                type="button"
                onClick={() => setDepth('deep')}
                className={pill(depth === 'deep')}
              >
                {t('bazi.interpret.deep')} · {t('bazi.interpret.tierPaid')}
              </button>
            </div>
          </Field>

          <Field label={t('bazi.interpret.model')}>
            <select
              className={inputCls}
              value={modelId}
              onChange={(e) => setModelId(e.target.value)}
              disabled={!catalog}
            >
              <option value="">{t('bazi.interpret.tierFree')}</option>
              {catalog?.free.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.label} · {t('bazi.interpret.tierFree')}
                </option>
              ))}
              {catalog?.paid.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.label} · {t('bazi.interpret.tierPaid')}
                </option>
              ))}
            </select>
          </Field>
        </div>

        {/* optional manual longitude override */}
        <div className="mt-3 flex items-center gap-3 text-xs text-muted">
          <input
            type="number"
            step="0.01"
            min={-180}
            max={180}
            placeholder={t('bazi.form.longitudeHint')}
            value={longitude}
            onChange={(e) => setLongitude(e.target.value)}
            className="w-40 rounded-md border border-border bg-surface px-2 py-1 text-fg outline-none focus:border-primary"
          />
          {effLongitude !== 0 && (
            <span>
              ≈ {effLongitude.toFixed(2)}°E
            </span>
          )}
        </div>

        <div className="mt-4 flex gap-3">
          <button
            type="button"
            onClick={onCompute}
            disabled={computing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {computing ? t('bazi.form.computing') : t('bazi.form.submit')}
          </button>
        </div>

        {computeErr && (
          <p className="mt-3 text-sm text-red-400">{t('bazi.error.compute')} {computeErr}</p>
        )}
      </section>

      {/* ── chart ── */}
      {chart && (
        <>
          <BaziChart chart={chart} />

          {/* ── AI interpret ── */}
          <section className="rounded-2xl border border-border bg-surface p-5">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-fg">{t('bazi.interpret.title')}</h2>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={onInterpret}
                  disabled={!chart || (streaming && false)}
                  className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                    streaming
                      ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30'
                      : 'bg-primary text-bg hover:bg-primary-hover'
                  }`}
                >
                  {streaming ? t('bazi.interpret.stop') : interpretation ? t('bazi.interpret.retry') : t('bazi.interpret.button')}
                </button>
              </div>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-6 text-center text-sm text-muted">{t('bazi.interpret.empty')}</p>
            )}

            {interpretErr && (
              <p className="text-sm text-red-400">
                {t('bazi.interpret.error')}：{interpretErr}
              </p>
            )}

            {reasoning && (
              <div className="mb-3">
                <button
                  type="button"
                  onClick={() => setShowReasoning((v) => !v)}
                  className="text-xs text-muted underline-offset-2 hover:underline"
                >
                  {showReasoning ? t('bazi.interpret.hideReasoning') : t('bazi.interpret.showReasoning')}
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
                {t('bazi.interpret.usage')}：prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </section>
        </>
      )}
    </div>
  )
}

const inputCls =
  'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary disabled:opacity-50'

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <label className="mb-1 block text-xs text-muted">{label}</label>
      {children}
    </div>
  )
}

function pill(active: boolean): string {
  return [
    'rounded-full px-3 py-1 text-xs transition-colors',
    active ? 'bg-primary text-bg' : 'border border-border text-muted hover:text-fg',
  ].join(' ')
}
