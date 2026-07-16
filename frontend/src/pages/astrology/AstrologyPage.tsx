import { useState, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import type { AstrologyChart, InterpretStreamEvent } from '../../api/client'
import { Astrology, streamAstrologyInterpret } from '../../api/client'

export default function AstrologyPage() {
  const { t } = useTranslation()
  const [year, setYear] = useState(1990)
  const [month, setMonth] = useState(1)
  const [day, setDay] = useState(1)
  const [hour, setHour] = useState(12)
  const [minute, setMinute] = useState(0)
  const [chart, setChart] = useState<AstrologyChart | null>(null)
  const [computing, setComputing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [interpretation, setInterpretation] = useState('')
  const [streaming, setStreaming] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const handleCompute = async () => {
    setComputing(true)
    setError(null)
    setChart(null)
    setInterpretation('')
    try {
      const res = await Astrology.compute({ year, month, day, hour, minute })
      setChart(res.data)
    } catch (e) {
      setError(t('astrology.error.compute'))
    } finally {
      setComputing(false)
    }
  }

  const handleInterpret = useCallback(() => {
    if (!chart) return
    setStreaming(true)
    setInterpretation('')
    abortRef.current = new AbortController()
    streamAstrologyInterpret(
      { year, month, day, hour, minute, stream: true },
      (ev: InterpretStreamEvent) => {
        if (ev.error) {
          setError(ev.error)
          setStreaming(false)
        } else if (ev.done) {
          setStreaming(false)
        } else if (ev.content) {
          setInterpretation((prev) => prev + ev.content)
        }
      },
      abortRef.current.signal,
    ).catch((e) => {
      setError(e instanceof Error ? e.message : 'Stream failed')
      setStreaming(false)
    })
  }, [chart, year, month, day, hour, minute])

  const cancelStream = useCallback(() => {
    abortRef.current?.abort()
    setStreaming(false)
  }, [])

  // Zodiac sign symbols
  const zodiacSymbols: Record<string, string> = {
    白羊座: '♈', 金牛座: '♉', 双子座: '♊', 巨蟹座: '♋',
    狮子座: '♌', 处女座: '♍', 天秤座: '♎', 天蝎座: '♏',
    射手座: '♐', 摩羯座: '♑', 水瓶座: '♒', 双鱼座: '♓',
  }

  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <div className="mb-8 text-center">
        <h1 className="text-2xl font-bold text-fg">{t('astrology.title')}</h1>
        <p className="mt-2 text-muted">{t('astrology.subtitle')}</p>
      </div>

      {/* Input Form */}
      <div className="mb-6 rounded-lg border border-border bg-surface p-6">
        <div className="grid grid-cols-2 gap-4 md:grid-cols-5">
          <div>
            <label className="mb-1 block text-sm text-muted">{t('astrology.form.year')}</label>
            <input
              type="number"
              value={year}
              onChange={(e) => setYear(Number(e.target.value))}
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-fg"
              min={1900}
              max={2100}
            />
          </div>
          <div>
            <label className="mb-1 block text-sm text-muted">{t('astrology.form.month')}</label>
            <input
              type="number"
              value={month}
              onChange={(e) => setMonth(Number(e.target.value))}
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-fg"
              min={1}
              max={12}
            />
          </div>
          <div>
            <label className="mb-1 block text-sm text-muted">{t('astrology.form.day')}</label>
            <input
              type="number"
              value={day}
              onChange={(e) => setDay(Number(e.target.value))}
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-fg"
              min={1}
              max={31}
            />
          </div>
          <div>
            <label className="mb-1 block text-sm text-muted">{t('astrology.form.hour')}</label>
            <input
              type="number"
              value={hour}
              onChange={(e) => setHour(Number(e.target.value))}
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-fg"
              min={0}
              max={23}
            />
          </div>
          <div>
            <label className="mb-1 block text-sm text-muted">{t('astrology.form.minute')}</label>
            <input
              type="number"
              value={minute}
              onChange={(e) => setMinute(Number(e.target.value))}
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-fg"
              min={0}
              max={59}
            />
          </div>
        </div>

        <button
          onClick={handleCompute}
          disabled={computing}
          className="mt-4 rounded-md bg-primary px-6 py-2 text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
        >
          {computing ? t('astrology.form.computing') : t('astrology.form.submit')}
        </button>

        {error && <p className="mt-3 text-sm text-red-500">{error}</p>}
      </div>

      {/* Chart Result */}
      {chart && (
        <div className="space-y-6">
          {/* Core Triad */}
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div className="rounded-lg border border-border bg-surface p-4 text-center">
              <div className="text-3xl">{zodiacSymbols[chart.sunSign] || '☉'}</div>
              <div className="mt-2 text-lg font-semibold text-fg">{t('astrology.chart.sunSign')}</div>
              <div className="text-xl text-primary">{chart.sunSign}</div>
            </div>
            <div className="rounded-lg border border-border bg-surface p-4 text-center">
              <div className="text-3xl">{zodiacSymbols[chart.moonSign] || '☽'}</div>
              <div className="mt-2 text-lg font-semibold text-fg">{t('astrology.chart.moonSign')}</div>
              <div className="text-xl text-primary">{chart.moonSign}</div>
            </div>
            <div className="rounded-lg border border-border bg-surface p-4 text-center">
              <div className="text-3xl">{zodiacSymbols[chart.ascendant] || '↑'}</div>
              <div className="mt-2 text-lg font-semibold text-fg">{t('astrology.chart.ascendant')}</div>
              <div className="text-xl text-primary">{chart.ascendant}</div>
            </div>
          </div>

          {/* Planetary Positions */}
          <div className="rounded-lg border border-border bg-surface p-6">
            <h3 className="mb-4 text-lg font-semibold text-fg">{t('astrology.chart.planets')}</h3>
            <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
              {chart.planets.map((p, i) => (
                <div
                  key={i}
                  className="flex items-center justify-between rounded-md bg-bg/50 px-3 py-2"
                >
                  <div>
                    <span className="font-medium text-fg">{p.name}</span>
                    {p.retrograde && <span className="ml-1 text-xs text-muted">逆行</span>}
                  </div>
                  <div className="text-sm">
                    <span className="text-primary">{p.sign}</span>
                    <span className="ml-1 text-muted">{p.degree.toFixed(1)}°</span>
                    <span className="ml-1 text-xs text-muted">第{p.house}宫</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Houses */}
          <div className="rounded-lg border border-border bg-surface p-6">
            <h3 className="mb-4 text-lg font-semibold text-fg">{t('astrology.chart.houses')}</h3>
            <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
              {chart.houses.map((h, i) => (
                <div
                  key={i}
                  className="rounded-md bg-bg/50 px-3 py-2"
                >
                  <span className="text-sm text-muted">第{h.number}宫</span>
                  <span className="ml-2 font-medium text-primary">{h.sign}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Aspects */}
          {chart.aspects.length > 0 && (
            <div className="rounded-lg border border-border bg-surface p-6">
              <h3 className="mb-4 text-lg font-semibold text-fg">{t('astrology.chart.aspects')}</h3>
              <div className="space-y-1">
                {chart.aspects.slice(0, 10).map((a, i) => (
                  <div
                    key={i}
                    className="flex items-center justify-between rounded-md bg-bg/50 px-3 py-1.5 text-sm"
                  >
                    <span>
                      <span className="text-fg">{a.planet1}</span>
                      <span className="mx-2 text-muted">{a.aspect}</span>
                      <span className="text-fg">{a.planet2}</span>
                    </span>
                    <span className={a.exact ? 'text-primary' : 'text-muted'}>
                      {a.orb.toFixed(1)}°
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* AI Interpretation */}
          <div className="rounded-lg border border-border bg-surface p-6">
            <h3 className="mb-4 text-lg font-semibold text-fg">{t('astrology.interpret.title')}</h3>
            <div className="mb-4">
              {streaming ? (
                <button
                  onClick={cancelStream}
                  className="rounded-md bg-red-500 px-4 py-2 text-sm text-white hover:bg-red-600"
                >
                  {t('astrology.interpret.stop')}
                </button>
              ) : (
                <button
                  onClick={handleInterpret}
                  disabled={!chart}
                  className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                >
                  {t('astrology.interpret.button')}
                </button>
              )}
            </div>
            {interpretation && (
              <div className="whitespace-pre-wrap rounded-md bg-bg/50 p-4 text-sm leading-relaxed text-fg">
                {interpretation}
              </div>
            )}
            {!interpretation && !streaming && (
              <p className="text-sm text-muted">{t('astrology.interpret.empty')}</p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}