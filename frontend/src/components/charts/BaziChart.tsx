// Visualization for a bazi chart: four-pillar table, dayun timeline,
// and the wuxing distribution bar chart. Pure presentational — takes
// the chart data and renders it; no fetching of its own.
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { BaziChart, DaYun, Pillar, WuXingStat } from '../../api/client'

// element → tailwind color token, used by the wuxing chart and the
// pillar element badges. Keeping the palette here means a restyle is
// one edit.
const WUXING_COLOR: Record<string, string> = {
  金: 'text-yellow-300',
  木: 'text-green-400',
  水: 'text-blue-400',
  火: 'text-red-400',
  土: 'text-amber-500',
}
const WUXING_BAR: Record<string, string> = {
  金: 'bg-yellow-300',
  木: 'bg-green-400',
  水: 'bg-blue-400',
  火: 'bg-red-400',
  土: 'bg-amber-500',
}

function elementChar(wx: string, idx: number): string {
  // wx looks like "木水" — one char per element. Split by rune.
  const chars = Array.from(wx)
  return chars[idx] ?? ''
}

function PillarColumn({ p }: { p: Pillar }) {
  const { t } = useTranslation()
  const ganWX = elementChar(p.wuXing, 0)
  const zhiWX = elementChar(p.wuXing, 1)
  return (
    <div className="rounded-xl border border-border bg-surface-2 p-3 text-center">
      <div className="mb-2 text-xs text-muted">
        {t(`bazi.chart.${p.position === '年' ? 'year' : p.position === '月' ? 'month' : p.position === '日' ? 'day' : 'hour'}`)}
      </div>
      <div className="mb-1 flex items-baseline justify-center gap-1">
        <span className={`text-2xl font-bold ${WUXING_COLOR[ganWX] ?? 'text-fg'}`}>{p.gan}</span>
        <span className={`text-2xl font-bold ${WUXING_COLOR[zhiWX] ?? 'text-fg'}`}>{p.zhi}</span>
      </div>
      <div className="text-xs text-muted">{p.naYin}</div>
      <div className="mt-2 space-y-1 text-xs">
        <Row label={t('bazi.chart.shiShenGan')} value={p.shiShenGan} />
        <Row label={t('bazi.chart.shiShenZhi')} value={p.shiShenZhi.join(' / ')} />
        <Row label={t('bazi.chart.hideGan')} value={p.hideGan.join(' / ')} />
        <Row label={t('bazi.chart.diShi')} value={p.diShi} />
        <Row label={t('bazi.chart.xunKong')} value={p.xunKong} />
      </div>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  if (!value) return null
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted">{label}</span>
      <span className="text-fg">{value}</span>
    </div>
  )
}

function DaYunTimeline({ daYun, forward }: { daYun: DaYun[]; forward: boolean }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState<number | null>(null)
  return (
    <div>
      <div className="mb-2 text-xs text-muted">
        {t('bazi.chart.startAge')}：{forward ? t('bazi.chart.forward') : t('bazi.chart.reverse')}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 lg:grid-cols-5">
        {daYun
          .filter((d) => d.index > 0)
          .map((d) => (
            <div
              key={d.index}
              className="cursor-pointer rounded-lg border border-border bg-surface-2 p-2 text-center transition-colors hover:border-primary"
              onClick={() => setOpen(open === d.index ? null : d.index)}
            >
              <div className={`text-lg font-bold ${WUXING_COLOR[elementChar(d.ganZhi, 0)] ?? 'text-fg'}`}>
                {d.ganZhi}
              </div>
              <div className="text-xs text-muted">
                {d.startAge}-{d.endAge}{t('bazi.chart.liuNian').slice(0, 1)}
              </div>
              <div className="text-[10px] text-muted">
                {d.startYear}-{d.endYear}
              </div>
              {open === d.index && (
                <div className="mt-2 max-h-32 overflow-y-auto border-t border-border pt-1 text-[10px]">
                  {d.liuNian.map((ln) => (
                    <div key={ln.year} className="flex justify-between">
                      <span className="text-muted">{ln.year}</span>
                      <span className="text-fg">{ln.ganZhi}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
      </div>
    </div>
  )
}

// tiny local hook to avoid importing useState in two places; keeps
// the file self-contained for the timeline expand state.
function WuXingChart({ stats }: { stats: WuXingStat[] }) {
  const { t } = useTranslation()
  const max = Math.max(...stats.map((s) => s.count), 1)
  return (
    <div className="space-y-2">
      {stats.map((s) => (
        <div key={s.element} className="flex items-center gap-2">
          <span className={`w-6 text-sm font-medium ${WUXING_COLOR[s.element] ?? 'text-fg'}`}>
            {s.element}
          </span>
          <div className="h-3 flex-1 overflow-hidden rounded-full bg-bg">
            <div
              className={`h-full rounded-full ${WUXING_BAR[s.element] ?? 'bg-primary'}`}
              style={{ width: `${(s.count / max) * 100}%` }}
            />
          </div>
          <span className="w-16 text-right text-xs text-muted">
            {s.count} · {s.percent}%
          </span>
        </div>
      ))}
      <div className="pt-1 text-xs text-muted">{t('bazi.chart.wuXingStats')}</div>
    </div>
  )
}

export default function BaziChart({ chart }: { chart: BaziChart }) {
  const { t } = useTranslation()
  return (
    <div className="space-y-6">
      {/* provenance */}
      <div className="rounded-xl border border-border bg-surface p-4 text-sm">
        <div className="flex flex-wrap gap-x-6 gap-y-1">
          <span>
            <span className="text-muted">{t('bazi.chart.solar')}：</span>
            <span className="text-fg">{chart.solar}</span>
          </span>
          <span>
            <span className="text-muted">{t('bazi.chart.lunar')}：</span>
            <span className="text-fg">{chart.lunar}</span>
          </span>
          {chart.trueSolar && (
            <span>
              <span className="text-muted">{t('bazi.chart.correction')}：</span>
              <span className="text-fg">{chart.correction}</span>
            </span>
          )}
        </div>
        <div className="mt-2 text-xs text-muted">
          规则：{chart.ruleSetVersion} · 日界：{chart.dayBoundary === 'midnight' ? '00:00 换日' : '23:00 子初换日'} · 起运：{chart.yunMethod} · 历法：{chart.calendarLibraryVersion}
        </div>
        <div className="mt-1 text-xs text-muted">
          前一节令：{chart.previousJie.name} {chart.previousJie.time} · 后一节令：{chart.nextJie.name} {chart.nextJie.time}
        </div>
      </div>

      {/* four pillars */}
      <section>
        <h3 className="mb-2 text-sm font-semibold text-fg">{t('bazi.chart.pillars')}</h3>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {chart.pillars.map((p) => (
            <PillarColumn key={p.position} p={p} />
          ))}
        </div>
      </section>

      {/* derived: 胎元/命宫/身宫 */}
      <section className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <DerivedCard label={t('bazi.chart.taiYuan')} value={chart.taiYuan} nayin={chart.taiYuanNaYin} />
        <DerivedCard label={t('bazi.chart.taiXi')} value={chart.taiXi} nayin={chart.taiXiNaYin} />
        <DerivedCard label={t('bazi.chart.mingGong')} value={chart.mingGong} nayin={chart.mingGongNaYin} />
        <DerivedCard label={t('bazi.chart.shenGong')} value={chart.shenGong} nayin={chart.shenGongNaYin} />
      </section>

      {/* 神煞 */}
      {chart.shenSha.length > 0 && (
        <section>
          <h3 className="mb-2 text-sm font-semibold text-fg">{t('bazi.chart.shenSha')}</h3>
          <div className="flex flex-wrap gap-2">
            {chart.shenSha.map((s, i) => (
              <span
                key={`${s.name}-${s.position}-${i}`}
                className="rounded-full border border-border bg-surface-2 px-3 py-1 text-xs"
                title={s.note}
              >
                <span className="text-primary">{s.name}</span>
                <span className="ml-1 text-muted">{s.position}</span>
              </span>
            ))}
          </div>
        </section>
      )}

      {/* 大运 */}
      <section>
        <h3 className="mb-2 text-sm font-semibold text-fg">{t('bazi.chart.daYun')}</h3>
        <DaYunTimeline daYun={chart.daYun} forward={chart.forward} />
      </section>

      {/* 五行 + 旺衰 + 用神 */}
      <section className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-xl border border-border bg-surface p-4">
          <h3 className="mb-3 text-sm font-semibold text-fg">{t('bazi.chart.wuXingStats')}</h3>
          <WuXingChart stats={chart.wuXingStats} />
        </div>
        <div className="space-y-3">
          <div className="rounded-xl border border-border bg-surface p-4">
            <h3 className="mb-2 text-sm font-semibold text-fg">{t('bazi.chart.wangShuai')}</h3>
            <p className="text-sm text-fg">{chart.interpretation.wangShuai.summary}</p>
            <div className="mt-2 flex gap-4 text-xs">
              <span className="text-muted">
                {t('bazi.chart.strong')}：
                <span className={WUXING_COLOR[chart.interpretation.wangShuai.strong] ?? 'text-fg'}>
                  {chart.interpretation.wangShuai.strong}
                </span>
              </span>
              <span className="text-muted">
                {t('bazi.chart.weak')}：
                <span className={WUXING_COLOR[chart.interpretation.wangShuai.weak] ?? 'text-fg'}>
                  {chart.interpretation.wangShuai.weak}
                </span>
              </span>
            </div>
          </div>
          <div className="rounded-xl border border-border bg-surface p-4">
            <h3 className="mb-2 text-sm font-semibold text-fg">
              {t('bazi.chart.yongYin')}
              <span className="ml-2 text-xs text-muted">({chart.interpretation.yongYin.confidence})</span>
            </h3>
            <div className="mb-2 flex flex-wrap gap-3 text-xs">
              <span className="text-muted">
                {t('bazi.chart.yongShen')}：
                <span className={WUXING_COLOR[chart.interpretation.yongYin.yongShen] ?? 'text-fg'}>
                  {chart.interpretation.yongYin.yongShen}
                </span>
              </span>
              <span className="text-muted">
                {t('bazi.chart.xi')}：{chart.interpretation.yongYin.xi.join(' / ')}
              </span>
              <span className="text-muted">
                {t('bazi.chart.ji')}：{chart.interpretation.yongYin.ji.join(' / ')}
              </span>
            </div>
            <p className="text-xs text-muted">{chart.interpretation.yongYin.reason}</p>
            {chart.interpretation.warnings.map((warning) => (
              <p key={warning} className="mt-2 text-xs text-amber-600 dark:text-amber-400">{warning}</p>
            ))}
          </div>
        </div>
      </section>
    </div>
  )
}

function DerivedCard({ label, value, nayin }: { label: string; value: string; nayin: string }) {
  return (
    <div className="rounded-xl border border-border bg-surface p-3 text-center">
      <div className="text-xs text-muted">{label}</div>
      <div className="text-lg font-semibold text-fg">{value}</div>
      <div className="text-xs text-muted">{nayin}</div>
    </div>
  )
}
