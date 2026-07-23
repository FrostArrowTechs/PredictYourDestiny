import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import {
  Entitlements,
  Account,
  Quota,
  Records,
  type EntitlementResponse,
  type FortuneRecord,
  type QuotaResponse,
} from '../../api/client'
import { useAuth } from '../../auth/AuthContext'

const PAGE_SIZE = 20

function prettyJSON(value: string): string {
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

export default function AccountPage() {
  const { t, i18n } = useTranslation()
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [entitlement, setEntitlement] = useState<EntitlementResponse | null>(null)
  const [quota, setQuota] = useState<QuotaResponse | null>(null)
  const [records, setRecords] = useState<FortuneRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [detail, setDetail] = useState<FortuneRecord | null>(null)
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [deletingID, setDeletingID] = useState<number | null>(null)
  const [error, setError] = useState('')
  const [password, setPassword] = useState('')
  const [accountAction, setAccountAction] = useState<'export' | 'clear' | 'delete' | null>(null)

  const loadAccount = useCallback(async (targetPage: number) => {
    setLoading(true)
    setError('')
    const [entitlementResult, quotaResult, recordsResult] = await Promise.allSettled([
      Entitlements.get(),
      Quota.get(),
      Records.list({ page: targetPage, limit: PAGE_SIZE }),
    ])
    if (entitlementResult.status === 'fulfilled') setEntitlement(entitlementResult.value)
    if (quotaResult.status === 'fulfilled') setQuota(quotaResult.value)
    if (recordsResult.status === 'fulfilled') {
      setRecords(recordsResult.value.records)
      setTotal(recordsResult.value.total)
      setPage(recordsResult.value.page)
    }
    if ([entitlementResult, quotaResult, recordsResult].some((result) => result.status === 'rejected')) {
      setError(t('account.partialLoadError'))
    }
    setLoading(false)
  }, [t])

  useEffect(() => {
    if (user) void loadAccount(1)
  }, [loadAccount, user])

  const handleLogout = () => {
    logout()
    navigate('/')
  }

  const showDetail = async (id: number) => {
    setDetailLoading(true)
    setError('')
    try {
      setDetail(await Records.get(id))
    } catch {
      setError(t('account.detailError'))
    } finally {
      setDetailLoading(false)
    }
  }

  const deleteRecord = async (record: FortuneRecord) => {
    if (!window.confirm(t('account.deleteConfirm', { title: record.title || record.kind }))) return
    setDeletingID(record.id)
    setError('')
    try {
      await Records.delete(record.id)
      if (detail?.id === record.id) setDetail(null)
      const nextPage = records.length === 1 && page > 1 ? page - 1 : page
      await loadAccount(nextPage)
    } catch {
      setError(t('account.deleteError'))
    } finally {
      setDeletingID(null)
    }
  }

  const exportAccount = async () => {
    setAccountAction('export')
    setError('')
    try {
      const data = await Account.export()
      const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }))
      const link = document.createElement('a')
      link.href = url
      link.download = `predictdestiny-account-${new Date().toISOString().slice(0, 10)}.json`
      link.click()
      URL.revokeObjectURL(url)
    } catch {
      setError(t('account.exportError'))
    } finally {
      setAccountAction(null)
    }
  }

  const clearHistory = async () => {
    if (!window.confirm(t('account.clearHistoryConfirm'))) return
    setAccountAction('clear')
    setError('')
    try {
      await Account.clearHistory()
      setDetail(null)
      await loadAccount(1)
    } catch {
      setError(t('account.clearHistoryError'))
    } finally {
      setAccountAction(null)
    }
  }

  const deleteAccount = async () => {
    if (!password || !window.confirm(t('account.deleteAccountConfirm'))) return
    setAccountAction('delete')
    setError('')
    try {
      await Account.delete(password)
      logout()
      navigate('/')
    } catch {
      setError(t('account.deleteAccountError'))
      setAccountAction(null)
    }
  }

  if (!user) {
    return (
      <div className="mx-auto max-w-2xl py-12 text-center">
        <p className="text-muted">{t('auth.notLoggedIn')}</p>
      </div>
    )
  }

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const dateLocale = i18n.language === 'zh-TW' ? 'zh-TW' : 'zh-CN'

  return (
    <div className="mx-auto max-w-4xl space-y-6 px-4 py-12">
      <div className="rounded-lg border border-border bg-surface p-6 shadow">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="mb-4 text-2xl font-bold text-fg">{t('account.title')}</h1>
            <div className="space-y-2">
              <div><span className="text-sm text-muted">{t('auth.email')}</span><div className="font-medium text-fg">{user.email}</div></div>
              <div><span className="text-sm text-muted">{t('auth.displayName')}</span><div className="font-medium text-fg">{user.displayName || '-'}</div></div>
            </div>
          </div>
          <button onClick={handleLogout} className="rounded-md bg-red-600 px-4 py-2 text-white hover:bg-red-700">
            {t('common.logout')}
          </button>
        </div>
      </div>

      {error && <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-500">{error}</div>}

      <div className="grid gap-4 md:grid-cols-2">
        <section className="rounded-lg border border-border bg-surface p-5">
          <h2 className="font-semibold text-fg">{t('account.membership')}</h2>
          {entitlement ? (
            <div className="mt-3 space-y-1 text-sm">
              <div className="text-lg font-medium text-primary">{entitlement.tierName}</div>
              <div className="text-muted">{t('account.tierCode')}: {entitlement.effectiveTier}</div>
              <div className="text-muted">{t('account.expiresAt')}: {entitlement.expiresAt ? new Date(entitlement.expiresAt).toLocaleString(dateLocale) : t('account.noExpiry')}</div>
              {entitlement.fellBackToFree && <div className="text-amber-500">{t('account.fellBackToFree')}</div>}
            </div>
          ) : <div className="mt-3 text-sm text-muted">{loading ? t('account.loading') : t('account.unavailable')}</div>}
        </section>

        <section className="rounded-lg border border-border bg-surface p-5">
          <h2 className="font-semibold text-fg">{t('account.todayQuota')}</h2>
          {quota ? (
            <div className="mt-3">
              <div className="text-2xl font-semibold text-primary">
                {quota.limit < 0 ? t('account.unlimited') : `${quota.remaining} / ${quota.limit}`}
              </div>
              <div className="text-sm text-muted">{t('account.usedToday', { count: quota.used })}</div>
              {quota.costBudgetMicros >= 0 && (
                <div className="mt-1 text-xs text-muted">
                  {t('account.costBudget', {
                    remaining: (quota.costRemainingMicros / 1_000_000).toFixed(4),
                    budget: (quota.costBudgetMicros / 1_000_000).toFixed(4),
                    reserved: (quota.reservedCostMicros / 1_000_000).toFixed(4),
                  })}
                </div>
              )}
            </div>
          ) : <div className="mt-3 text-sm text-muted">{loading ? t('account.loading') : t('account.unavailable')}</div>}
        </section>
      </div>

      <section className="rounded-lg border border-border bg-surface p-5">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="font-semibold text-fg">{t('account.history')}</h2>
          <span className="text-sm text-muted">{t('account.recordCount', { count: total })}</span>
        </div>
        {loading && records.length === 0 ? <div className="text-sm text-muted">{t('account.loading')}</div> : records.length === 0 ? (
          <div className="text-sm text-muted">{t('account.noRecords')}</div>
        ) : (
          <div className="divide-y divide-border">
            {records.map((record) => (
              <div key={record.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                <button className="min-w-0 flex-1 text-left" onClick={() => void showDetail(record.id)} disabled={detailLoading}>
                  <div className="truncate font-medium text-fg">{record.title || record.kind}</div>
                  <div className="text-xs text-muted">{record.kind} · {new Date(record.createdAt).toLocaleString(dateLocale)} · {record.serverGenerated ? t('account.serverGenerated') : t('account.legacyRecord')}</div>
                </button>
                <button
                  onClick={() => void deleteRecord(record)}
                  disabled={deletingID === record.id}
                  className="rounded border border-red-500/50 px-3 py-1 text-sm text-red-500 disabled:opacity-50"
                >
                  {deletingID === record.id ? t('account.deleting') : t('account.delete')}
                </button>
              </div>
            ))}
          </div>
        )}
        {pageCount > 1 && (
          <div className="mt-4 flex items-center justify-center gap-3 text-sm">
            <button className="rounded border border-border px-3 py-1 disabled:opacity-40" disabled={page <= 1 || loading} onClick={() => void loadAccount(page - 1)}>{t('account.previous')}</button>
            <span className="text-muted">{page} / {pageCount}</span>
            <button className="rounded border border-border px-3 py-1 disabled:opacity-40" disabled={page >= pageCount || loading} onClick={() => void loadAccount(page + 1)}>{t('account.next')}</button>
          </div>
        )}
      </section>

      {detail && (
        <section className="rounded-lg border border-primary/40 bg-surface p-5">
          <div className="flex items-start justify-between gap-3">
            <div><h2 className="font-semibold text-fg">{detail.title || detail.kind}</h2><div className="text-xs text-muted">#{detail.id} · {detail.kind} · {detail.serverGenerated ? t('account.serverGenerated') : t('account.legacyRecord')}</div></div>
            <button className="text-sm text-muted hover:text-fg" onClick={() => setDetail(null)}>{t('account.close')}</button>
          </div>
          {detail.note && <p className="mt-4 whitespace-pre-wrap text-sm text-fg">{detail.note}</p>}
          <details className="mt-4"><summary className="cursor-pointer text-sm text-primary">{t('account.input')}</summary><pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap rounded bg-bg p-3 text-xs text-muted">{prettyJSON(detail.inputJson)}</pre></details>
          <details className="mt-3" open><summary className="cursor-pointer text-sm text-primary">{t('account.result')}</summary><pre className="mt-2 max-h-96 overflow-auto whitespace-pre-wrap rounded bg-bg p-3 text-xs text-muted">{prettyJSON(detail.resultJson)}</pre></details>
        </section>
      )}

      <section className="rounded-lg border border-border bg-surface p-5">
        <h2 className="font-semibold text-fg">{t('account.dataControls')}</h2>
        <p className="mt-1 text-sm text-muted">{t('account.retentionNotice')}</p>
        <div className="mt-4 flex flex-wrap gap-3">
          <button disabled={accountAction !== null} onClick={() => void exportAccount()} className="rounded border border-border px-4 py-2 text-sm text-fg disabled:opacity-50">{accountAction === 'export' ? t('account.exporting') : t('account.exportData')}</button>
          <button disabled={accountAction !== null} onClick={() => void clearHistory()} className="rounded border border-amber-500/60 px-4 py-2 text-sm text-amber-500 disabled:opacity-50">{accountAction === 'clear' ? t('account.clearing') : t('account.clearHistory')}</button>
        </div>
        <div className="mt-6 border-t border-border pt-4">
          <h3 className="font-medium text-red-500">{t('account.deleteAccount')}</h3>
          <p className="mt-1 text-sm text-muted">{t('account.deleteAccountHint')}</p>
          <div className="mt-3 flex flex-col gap-3 sm:flex-row">
            <input type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder={t('auth.password')} autoComplete="current-password" className="rounded-md border border-border bg-bg px-3 py-2 text-fg" />
            <button disabled={!password || accountAction !== null} onClick={() => void deleteAccount()} className="rounded bg-red-600 px-4 py-2 text-sm text-white disabled:opacity-50">{accountAction === 'delete' ? t('account.deletingAccount') : t('account.deleteAccount')}</button>
          </div>
        </div>
      </section>
    </div>
  )
}
