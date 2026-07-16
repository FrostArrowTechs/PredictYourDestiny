// Tarot page. Users pick a spread, optionally ask a question, draw cards,
// and get an AI interpretation of the spread.
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Tarot as TarotApi,
  type TarotChart,
  type TarotInput,
  type InterpretStreamEvent,
  streamTarotInterpret,
} from '../../api/client'

const inputCls = 'rounded-md border border-border bg-surface px-2 py-1 text-sm text-fg outline-none focus:border-primary'

export default function TarotPage() {
  const { t, i18n } = useTranslation()
  const lang = (i18n.resolvedLanguage ?? 'zh-CN') as string

  const [spread, setSpread] = useState<'single' | 'three' | 'celtic'>('three')
  const [question, setQuestion] = useState('')

  const [chart, setChart] = useState<TarotChart | null>(null)
  const [drawing, setDrawing] = useState(false)
  const [drawErr, setDrawErr] = useState<string | null>(null)

  const [interpretation, setInterpretation] = useState('')
  const [reasoning, setReasoning] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [interpretErr, setInterpretErr] = useState<string | null>(null)
  const [showReasoning, setShowReasoning] = useState(false)
  const [usage, setUsage] = useState<InterpretStreamEvent['usage'] | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const buildInput = useCallback(
    (): TarotInput => ({ spread, question: question.trim(), lang }),
    [spread, question, lang],
  )

  const onDraw = useCallback(async () => {
    setDrawing(true)
    setDrawErr(null)
    setInterpretation('')
    setReasoning('')
    setInterpretErr(null)
    setUsage(null)
    try {
      const res = await TarotApi.draw(buildInput())
      setChart(res.data)
    } catch (e) {
      setDrawErr(e instanceof Error ? e.message : String(e))
      setChart(null)
    } finally {
      setDrawing(false)
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
      await streamTarotInterpret(
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

  return (
    <div className="space-y-6">
      <header className="text-center">
        <h1 className="text-3xl font-bold text-fg">{t('tarot.title')}</h1>
        <p className="mt-2 text-muted">{t('tarot.subtitle')}</p>
      </header>

      {/* input form */}
      <section className="rounded-2xl border border-border bg-surface p-5">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:flex-wrap">
          <div>
            <label className="mb-1 block text-xs text-muted">{t('tarot.form.spread')}</label>
            <select className={inputCls} value={spread} onChange={(e) => setSpread(e.target.value as 'single' | 'three' | 'celtic')}>
              <option value="single">{t('tarot.form.spreadSingle')}</option>
              <option value="three">{t('tarot.form.spreadThree')}</option>
              <option value="celtic">{t('tarot.form.spreadCeltic')}</option>
            </select>
          </div>
          <div className="flex-1 min-w-[200px]">
            <label className="mb-1 block text-xs text-muted">{t('tarot.form.question')}</label>
            <input
              className={`${inputCls} w-full`}
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder={t('tarot.form.questionPlaceholder')}
              maxLength={100}
            />
          </div>
          <button
            type="button"
            onClick={onDraw}
            disabled={drawing}
            className="rounded-xl bg-primary px-6 py-2 font-medium text-bg transition-colors hover:bg-primary-hover disabled:opacity-50"
          >
            {drawing ? t('tarot.form.drawing') : t('tarot.form.draw')}
          </button>
        </div>

        {drawErr && <p className="mt-3 text-sm text-red-400">{t('tarot.error.draw')}: {drawErr}</p>}
      </section>

      {/* chart */}
      {chart && (
        <section className="rounded-2xl border border-border bg-surface p-5">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-fg">{chart.spread.name}</h2>
            <span className="text-xs text-muted">{chart.cards.length} 张</span>
          </div>

          {/* cards */}
          <div className={`grid gap-3 ${chart.cards.length === 1 ? 'grid-cols-1' : 'grid-cols-2 lg:grid-cols-3'}`}>
            {chart.cards.map((card, i) => (
              <div
                key={i}
                className={`rounded-lg border p-4 ${card.reversed ? 'border-purple-500/40 bg-purple-500/5' : 'border-border bg-bg'}`}
              >
                <div className="mb-1 flex items-center justify-between">
                  <span className="rounded bg-primary/20 px-2 py-0.5 text-xs text-primary">{card.positionLabel}</span>
                  <span className={`text-xs ${card.reversed ? 'text-purple-400' : 'text-green-400'}`}>
                    {card.reversed ? t('tarot.chart.reversed') : t('tarot.chart.upright')}
                  </span>
                </div>
                <div className="mb-2 text-center">
                  <div className="text-2xl">{majorArcanaEmoji(card.name)}</div>
                  <div className="mt-1 font-medium text-fg">{card.name}</div>
                  <div className="text-xs text-muted">{card.nameLatin}</div>
                </div>
                <div className="space-y-1 text-xs text-muted">
                  <div>
                    <span className="text-fg/70">{t('tarot.chart.meaning')}：</span>
                    {card.meaning}
                  </div>
                  {card.keywords && (
                    <div>
                      <span className="text-fg/70">{t('tarot.chart.keywords')}：</span>
                      {card.keywords}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* AI interpret */}
          <div className="mt-6 border-t border-border pt-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="font-medium text-fg">{t('tarot.interpret.title')}</h3>
              <button
                type="button"
                onClick={onInterpret}
                disabled={!chart || streaming}
                className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
                  streaming ? 'bg-red-500/20 text-red-300 hover:bg-red-500/30' : 'bg-primary text-bg hover:bg-primary-hover'
                }`}
              >
                {streaming ? t('tarot.interpret.stop') : interpretation ? t('tarot.interpret.retry') : t('tarot.interpret.button')}
              </button>
            </div>

            {!interpretation && !streaming && !interpretErr && (
              <p className="py-4 text-center text-sm text-muted">{t('tarot.interpret.empty')}</p>
            )}
            {interpretErr && <p className="text-sm text-red-400">{t('tarot.interpret.error')}: {interpretErr}</p>}

            {reasoning && (
              <div className="mb-3">
                <button type="button" onClick={() => setShowReasoning((v) => !v)} className="text-xs text-muted underline-offset-2 hover:underline">
                  {showReasoning ? t('tarot.interpret.hideReasoning') : t('tarot.interpret.showReasoning')}
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
                {t('tarot.interpret.usage')}: prompt {usage.prompt_tokens} · completion {usage.completion_tokens}
                {usage.reasoning_tokens ? ` · reasoning ${usage.reasoning_tokens}` : ''} · total {usage.total_tokens}
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}

// majorArcanaEmoji returns a representative glyph for a card.
// Major arcana get a thematic emoji; minor arcana use a suit symbol.
function majorArcanaEmoji(name: string): string {
  const majorMap: Record<string, string> = {
    愚者: '🃏', 魔术师: '🎩', 女祭司: '🌙', 皇后: '👑',
    皇帝: '🏰', 教皇: '⛪', 恋人: '💞', 战车: '⚔️',
    力量: '🦁', 隐士: '🏮', 命运之轮: '🎡', 正义: '⚖️',
    倒吊人: '🪢', 死神: '💀', 节制: '🕊️', 恶魔: '😈',
    高塔: '🗼', 星星: '⭐', 月亮: '🌕', 太阳: '☀️',
    审判: '📯', 世界: '🌍',
  }
  if (majorMap[name]) return majorMap[name]
  // Minor arcana: use suit symbol
  if (name.includes('权杖')) return '🔥'
  if (name.includes('圣杯')) return '💧'
  if (name.includes('宝剑')) return '💨'
  if (name.includes('星币')) return '🪙'
  return '🃏'
}
