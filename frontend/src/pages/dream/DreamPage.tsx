import { useState, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Dream, streamDreamInterpret } from '../../api/client'
import type { DreamChart, InterpretStreamEvent } from '../../api/client'

export default function DreamPage() {
  const { t } = useTranslation()
  const [description, setDescription] = useState('')
  const [chart, setChart] = useState<DreamChart | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [interpretation, setInterpretation] = useState('')
  const [streaming, setStreaming] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const handleCompute = useCallback(async () => {
    if (!description.trim()) return
    setLoading(true)
    setError(null)
    setChart(null)
    setInterpretation('')
    try {
      const res = await Dream.compute({ description: description.trim() })
      setChart(res.data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Request failed')
    } finally {
      setLoading(false)
    }
  }, [description])

  const handleInterpret = useCallback(() => {
    if (!description.trim()) return
    setStreaming(true)
    setInterpretation('')
    abortRef.current = new AbortController()
    streamDreamInterpret(
      { description: description.trim(), stream: true },
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
  }, [description])

  const cancelStream = useCallback(() => {
    abortRef.current?.abort()
    setStreaming(false)
  }, [])

  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <h1 className="mb-2 text-2xl font-bold text-fg">{t('dream.title')}</h1>
      <p className="mb-6 text-sm text-muted">{t('dream.subtitle')}</p>

      {/* Input */}
      <div className="mb-6 rounded-lg border border-border bg-surface p-4">
        <label className="mb-2 block text-sm font-medium text-fg">{t('dream.descriptionLabel')}</label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder={t('dream.descriptionPlaceholder')}
          className="w-full rounded-md border border-border bg-bg px-3 py-2 text-fg outline-none focus:border-primary min-h-[120px] resize-y"
          maxLength={500}
        />
        <div className="mt-3 flex justify-end">
          <button
            onClick={handleCompute}
            disabled={loading || !description.trim()}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-bg hover:bg-primary/90 disabled:opacity-50"
          >
            {loading ? t('dream.analyzing') : t('dream.analyze')}
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
          {/* Summary */}
          <div className="mb-6 rounded-lg border border-border bg-surface p-4">
            <h3 className="mb-2 text-sm font-semibold text-fg">{t('dream.summary')}</h3>
            <p className="text-sm text-muted">{chart.summary}</p>
          </div>

          {/* Matched Symbols */}
          {chart.matches.length > 0 && (
            <div className="mb-6 rounded-lg border border-border bg-surface p-4">
              <h3 className="mb-3 text-sm font-semibold text-fg">{t('dream.matchedSymbols')}</h3>
              <div className="space-y-3">
                {chart.matches.map((m, i) => (
                  <div key={i} className="rounded border border-border/50 bg-bg p-3">
                    <div className="flex items-center gap-2 mb-2">
                      <span className="font-medium text-primary">{m.keyword}</span>
                      <span className="rounded bg-primary/10 px-2 py-0.5 text-xs text-muted">{m.category}</span>
                    </div>
                    <p className="text-sm text-fg leading-relaxed">{m.meaning}</p>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* No Matches */}
          {chart.matches.length === 0 && (
            <div className="mb-6 rounded-lg border border-border bg-surface p-4 text-center text-muted">
              {t('dream.noMatches')}
            </div>
          )}

          {/* Interpret Button */}
          <div className="mb-4 flex justify-center">
            <button
              onClick={handleInterpret}
              disabled={streaming}
              className="rounded-md bg-primary px-6 py-2 text-sm font-medium text-bg hover:bg-primary/90 disabled:opacity-50"
            >
              {streaming ? t('dream.interpreting') : t('dream.interpret')}
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
              <h3 className="mb-2 text-sm font-semibold text-fg">{t('dream.interpretation')}</h3>
              <div className="whitespace-pre-wrap text-sm text-fg leading-relaxed">{interpretation}</div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
