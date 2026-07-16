import { useState, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Name, streamNameInterpret } from '../../api/client'
import type { NameChart, InterpretStreamEvent } from '../../api/client'

export default function NamePage() {
  const { t } = useTranslation()
  const [fullName, setFullName] = useState('')
  const [chart, setChart] = useState<NameChart | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [interpretation, setInterpretation] = useState('')
  const [streaming, setStreaming] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const handleCompute = useCallback(async () => {
    if (!fullName.trim()) return
    setLoading(true)
    setError(null)
    setChart(null)
    setInterpretation('')
    try {
      const res = await Name.compute({ fullName: fullName.trim() })
      setChart(res.data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Request failed')
    } finally {
      setLoading(false)
    }
  }, [fullName])

  const handleInterpret = useCallback(() => {
    if (!fullName.trim()) return
    setStreaming(true)
    setInterpretation('')
    abortRef.current = new AbortController()
    streamNameInterpret(
      { fullName: fullName.trim(), stream: true },
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
  }, [fullName])

  const cancelStream = useCallback(() => {
    abortRef.current?.abort()
    setStreaming(false)
  }, [])

  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <h1 className="mb-2 text-2xl font-bold text-fg">{t('name.title')}</h1>
      <p className="mb-6 text-sm text-muted">{t('name.subtitle')}</p>

      {/* Input */}
      <div className="mb-6 rounded-lg border border-border bg-surface p-4">
        <label className="mb-2 block text-sm font-medium text-fg">{t('name.fullNameLabel')}</label>
        <div className="flex gap-2">
          <input
            type="text"
            value={fullName}
            onChange={(e) => setFullName(e.target.value)}
            placeholder={t('name.fullNamePlaceholder')}
            className="flex-1 rounded-md border border-border bg-bg px-3 py-2 text-fg outline-none focus:border-primary"
            maxLength={20}
          />
          <button
            onClick={handleCompute}
            disabled={loading || !fullName.trim()}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-bg hover:bg-primary/90 disabled:opacity-50"
          >
            {loading ? t('name.analyzing') : t('name.analyze')}
          </button>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* Result */}
      {chart && (
        <>
          {/* Score Hero */}
          <div className="mb-6 rounded-lg border border-border bg-surface p-6 text-center">
            <div className="mb-2 text-sm text-muted">{t('name.overallScore')}</div>
            <div className="text-5xl font-bold text-primary">{chart.score}</div>
            <div className="mt-1 text-lg text-fg">{chart.scoreDesc}</div>
            <div className="mt-3 text-sm text-muted">{t('name.sanCai')}: {chart.sanCai}</div>
          </div>

          {/* Five格 Cards */}
          <div className="mb-6 grid grid-cols-2 gap-4 md:grid-cols-5">
            <GeCard name={t('name.tianGe')} value={chart.tianGe} luck={chart.tianGeLuck} />
            <GeCard name={t('name.renGe')} value={chart.renGe} luck={chart.renGeLuck} highlight />
            <GeCard name={t('name.diGe')} value={chart.diGe} luck={chart.diGeLuck} />
            <GeCard name={t('name.waiGe')} value={chart.waiGe} luck={chart.waiGeLuck} />
            <GeCard name={t('name.zongGe')} value={chart.zongGe} luck={chart.zongGeLuck} />
          </div>

          {/* Stroke Details */}
          <div className="mb-6 rounded-lg border border-border bg-surface p-4">
            <h3 className="mb-3 text-sm font-semibold text-fg">{t('name.strokeDetails')}</h3>
            <div className="space-y-2">
              {chart.strokeDetails.map((d, i) => (
                <div key={i} className="flex items-center justify-between rounded border border-border/50 bg-bg px-3 py-2 text-sm">
                  <span className="font-medium text-fg">{d.char}</span>
                  <span className="text-muted">{d.position}</span>
                  <span className="text-muted">{t('name.strokesCount', { n: d.strokes })}</span>
                  {d.wuXing && <span className="rounded bg-primary/10 px-2 py-0.5 text-xs text-primary">{d.wuXing}</span>}
                </div>
              ))}
            </div>
          </div>

          {/* Interpret Button */}
          <div className="mb-4 flex justify-center">
            <button
              onClick={handleInterpret}
              disabled={streaming}
              className="rounded-md bg-primary px-6 py-2 text-sm font-medium text-bg hover:bg-primary/90 disabled:opacity-50"
            >
              {streaming ? t('name.interpreting') : t('name.interpret')}
            </button>
            {streaming && (
              <button onClick={cancelStream} className="ml-2 rounded-md border border-border px-4 py-2 text-sm text-muted hover:text-fg">
                {t('common.cancel')}
              </button>
            )}
          </div>

          {/* Interpretation */}
          {interpretation && (
            <div className="rounded-lg border border-border bg-surface p-4">
              <h3 className="mb-2 text-sm font-semibold text-fg">{t('name.interpretation')}</h3>
              <div className="whitespace-pre-wrap text-sm text-fg leading-relaxed">{interpretation}</div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// GeCard displays one of the Five格
function GeCard({ name, value, luck, highlight }: { name: string; value: number; luck: string; highlight?: boolean }) {
  const luckColor = luck === '吉' ? 'text-green-400' : luck === '凶' ? 'text-red-400' : 'text-yellow-400'
  return (
    <div className={`rounded-lg border p-3 text-center ${highlight ? 'border-primary bg-primary/5' : 'border-border bg-surface'}`}>
      <div className="text-xs text-muted">{name}</div>
      <div className="mt-1 text-2xl font-bold text-fg">{value}</div>
      <div className={`mt-1 text-xs ${luckColor}`}>{luck}</div>
    </div>
  )
}