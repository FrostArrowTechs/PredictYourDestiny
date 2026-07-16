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

// a more complete city → longitude table. Grouped by region so the
// dropdown is easy to scan. Users can also type a custom longitude
// in the optional override field.
type CityEntry = { name: string; lon: number }
const CITY_GROUPS: { label: string; cities: CityEntry[] }[] = [
  {
    label: '直辖市',
    cities: [
      { name: '北京', lon: 116.41 },
      { name: '上海', lon: 121.47 },
      { name: '天津', lon: 117.20 },
      { name: '重庆', lon: 106.55 },
    ],
  },
  {
    label: '省会与主要城市',
    cities: [
      { name: '广州', lon: 113.27 },
      { name: '深圳', lon: 114.06 },
      { name: '成都', lon: 104.07 },
      { name: '杭州', lon: 120.16 },
      { name: '西安', lon: 108.94 },
      { name: '武汉', lon: 114.30 },
      { name: '南京', lon: 118.80 },
      { name: '苏州', lon: 120.62 },
      { name: '长沙', lon: 112.94 },
      { name: '青岛', lon: 120.38 },
      { name: '郑州', lon: 113.62 },
      { name: '沈阳', lon: 123.43 },
      { name: '哈尔滨', lon: 126.64 },
      { name: '昆明', lon: 102.72 },
      { name: '厦门', lon: 118.09 },
      { name: '福州', lon: 119.30 },
      { name: '济南', lon: 117.00 },
      { name: '石家庄', lon: 114.51 },
      { name: '太原', lon: 112.55 },
      { name: '兰州', lon: 103.83 },
      { name: '贵阳', lon: 106.71 },
      { name: '南宁', lon: 108.37 },
      { name: '海口', lon: 110.32 },
      { name: '拉萨', lon: 91.13 },
      { name: '乌鲁木齐', lon: 87.62 },
      { name: '呼和浩特', lon: 111.75 },
      { name: '银川', lon: 106.23 },
      { name: '西宁', lon: 101.78 },
      { name: '南昌', lon: 115.86 },
      { name: '合肥', lon: 117.28 },
      { name: '长春', lon: 125.33 },
      { name: '大连', lon: 121.62 },
    ],
  },
  {
    label: '港澳台',
    cities: [
      { name: '香港', lon: 114.17 },
      { name: '澳门', lon: 113.55 },
      { name: '台北', lon: 121.56 },
    ],
  },
  {
    label: '海外',
    cities: [
      { name: '东京', lon: 139.69 },
      { name: '首尔', lon: 126.98 },
      { name: '新加坡', lon: 103.82 },
      { name: '吉隆坡', lon: 101.69 },
      { name: '曼谷', lon: 100.50 },
      { name: '纽约', lon: -74.01 },
      { name: '洛杉矶', lon: -118.24 },
      { name: '伦敦', lon: -0.13 },
      { name: '巴黎', lon: 2.35 },
      { name: '悉尼', lon: 151.21 },
    ],
  },
]

// Flatten the grouped list to O(1) lookup: name → longitude.
const CITY_LONGITUDES: Record<string, number> = Object.fromEntries(
  CITY_GROUPS.flatMap((g) => g.cities.map((c) => [c.name, c.lon])),
)

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
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          <Field label={t('bazi.form.birthdate')}>
            <div className="flex gap-2">
              <select
                className={`${inputCls} flex-1`}
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
                className={`${inputCls} w-20`}
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
                className={`${inputCls} w-20`}
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
                className={`${inputCls} flex-1`}
                value={hour}
                onChange={(e) => setHour(Number(e.target.value))}
              >
                {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                  <option key={h} value={h}>
                    {String(h).padStart(2, '0')}
                  </option>
                ))}
              </select>
              <span className="self-center text-muted">:</span>
              <select
                className={`${inputCls} flex-1`}
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
              <label className="flex items-center gap-2 text-sm text-fg cursor-pointer">
                <input
                  type="radio"
                  name="gender"
                  checked={gender === 1}
                  onChange={() => setGender(1)}
                  className="accent-primary"
                />
                {t('bazi.form.male')}
              </label>
              <label className="flex items-center gap-2 text-sm text-fg cursor-pointer">
                <input
                  type="radio"
                  name="gender"
                  checked={gender === 0}
                  onChange={() => setGender(0)}
                  className="accent-primary"
                />
                {t('bazi.form.female')}
              </label>
            </div>
          </Field>

          <Field label={t('bazi.form.birthplace')}>
            <select
              className={inputCls}
              value={city}
              onChange={(e) => {
                setCity(e.target.value)
                setLongitude('')
              }}
            >
              <option value="">{t('bazi.form.birthplaceNone')}</option>
              {CITY_GROUPS.map((g) => (
                <optgroup key={g.label} label={g.label}>
                  {g.cities.map((c) => (
                    <option key={c.name} value={c.name}>
                      {c.name}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          </Field>

          <Field label={t('bazi.interpret.depth')}>
            <div className="flex gap-2 pt-1">
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
        <div className="mt-4 flex flex-wrap items-center gap-3 text-sm">
          <label className="text-muted">{t('bazi.form.longitudeLabel')}</label>
          <input
            type="number"
            step="0.01"
            min={-180}
            max={180}
            placeholder={t('bazi.form.longitudeHint')}
            value={longitude}
            onChange={(e) => setLongitude(e.target.value)}
            className="w-44 rounded-md border border-border bg-bg px-3 py-1.5 text-fg outline-none focus:border-primary"
          />
          {effLongitude !== 0 && (
            <span className="text-xs text-muted">
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
