import { useEffect, useMemo, useState } from 'react'
import { DollarSign, Plus, RefreshCw, Coins, MessageSquare, AlertTriangle } from 'lucide-react'
import { apiRequest } from '../api/client'
import { PageHeader, LoadingState } from '../components/PageHeader'
import { Badge } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Label } from '../components/ui/Label'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '../components/ui/Table'

interface Provider {
  id: number
  name: string
  models: string
}

interface Price {
  id: number
  providerId: number
  model: string
  version: string
  inputCostMicrosPerMillion: number
  outputCostMicrosPerMillion: number
  reasoningCostMicrosPerMillion: number
  requestReserveMicros: number
  effectiveFrom: string
  providerName: string
}

interface Totals {
  requests: number
  succeeded: number
  failed: number
  cancelled: number
  unpriced: number
  promptTokens: number
  completionTokens: number
  reasoningTokens: number
  totalTokens: number
  estimatedCostMicros: number
  actualCostMicros: number
  actualCostCount: number
}

interface UsageGroup extends Totals {
  providerId: number | null
  providerName: string
  model: string
}

const today = () => new Date().toISOString().slice(0, 10)
const daysAgo = (days: number) => {
  const date = new Date()
  date.setUTCDate(date.getUTCDate() - days)
  return date.toISOString().slice(0, 10)
}
const money = (micros: number) => `$${(micros / 1_000_000).toFixed(4)}`
const usdToMicros = (value: string) => Math.round(Number(value || 0) * 1_000_000)

function modelIDs(raw: string): string[] {
  try {
    const entries = JSON.parse(raw) as Array<{ id?: string }>
    return entries.map(item => item.id || '').filter(Boolean)
  } catch {
    return []
  }
}

export default function UsagePage() {
  const [providers, setProviders] = useState<Provider[]>([])
  const [prices, setPrices] = useState<Price[]>([])
  const [totals, setTotals] = useState<Totals | null>(null)
  const [groups, setGroups] = useState<UsageGroup[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [from, setFrom] = useState(daysAgo(29))
  const [to, setTo] = useState(today())
  const [form, setForm] = useState({
    providerId: '', model: '', version: '', input: '0', output: '0',
    reasoning: '0', reserve: '0', effectiveFrom: new Date().toISOString().slice(0, 16),
  })

  const selectedProvider = providers.find(provider => String(provider.id) === form.providerId)
  const models = useMemo(() => modelIDs(selectedProvider?.models || '[]'), [selectedProvider])

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const [providerData, priceData, usageData] = await Promise.all([
        apiRequest<{ providers?: Provider[] }>('/admin/providers'),
        apiRequest<{ prices?: Price[] }>('/admin/ai/prices'),
        apiRequest<{ totals: Totals; groups?: UsageGroup[] }>(
          `/admin/ai/usage/summary?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
        ),
      ])
      setProviders(providerData.providers || [])
      setPrices(priceData.prices || [])
      setTotals(usageData.totals)
      setGroups(usageData.groups || [])
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const selectProvider = (providerId: string) => {
    const provider = providers.find(item => String(item.id) === providerId)
    setForm(current => ({ ...current, providerId, model: modelIDs(provider?.models || '[]')[0] || '' }))
  }

  const submitPrice = async (event: React.FormEvent) => {
    event.preventDefault()
    setSaving(true)
    setError('')
    try {
      await apiRequest('/admin/ai/prices', {
        method: 'POST',
        body: {
          providerId: Number(form.providerId),
          model: form.model,
          version: form.version.trim(),
          inputCostMicrosPerMillion: usdToMicros(form.input),
          outputCostMicrosPerMillion: usdToMicros(form.output),
          reasoningCostMicrosPerMillion: usdToMicros(form.reasoning),
          requestReserveMicros: usdToMicros(form.reserve),
          effectiveFrom: new Date(form.effectiveFrom).toISOString(),
        },
      })
      setForm(current => ({ ...current, version: '' }))
      await load()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const cards = [
    { label: '请求数', value: totals?.requests.toLocaleString() || '0', icon: MessageSquare },
    { label: '总 Token', value: totals?.totalTokens.toLocaleString() || '0', icon: Coins },
    { label: '估算成本', value: money(totals?.estimatedCostMicros || 0), icon: DollarSign },
    { label: '未定价请求', value: totals?.unpriced.toLocaleString() || '0', icon: AlertTriangle },
  ]

  return (
    <div>
      <PageHeader
        title="AI 用量与成本"
        description="价格版本不可修改；新价格只影响其生效时间之后发起的请求。金额单位为 USD。"
        actions={<Button variant="outline" onClick={load}><RefreshCw className="w-4 h-4 mr-1.5" />刷新</Button>}
      />

      {error && <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}

      <div className="flex items-end gap-3 mb-5">
        <div><Label htmlFor="usage-from">开始日期</Label><Input id="usage-from" type="date" value={from} onChange={e => setFrom(e.target.value)} /></div>
        <div><Label htmlFor="usage-to">结束日期</Label><Input id="usage-to" type="date" value={to} onChange={e => setTo(e.target.value)} /></div>
        <Button onClick={load}>应用范围</Button>
      </div>

      {loading ? <LoadingState /> : (
        <>
          <div className="grid grid-cols-4 gap-4 mb-6">
            {cards.map(({ label, value, icon: Icon }) => (
              <Card key={label}><CardContent className="pt-6 flex items-center justify-between">
                <div><div className="text-sm text-slate-500">{label}</div><div className="text-2xl font-semibold mt-1">{value}</div></div>
                <Icon className="w-5 h-5 text-slate-400" />
              </CardContent></Card>
            ))}
          </div>

          <Card className="mb-6">
            <CardHeader><CardTitle>供应商 / 模型汇总</CardTitle></CardHeader>
            <Table>
              <TableHeader><TableRow>
                <TableHead>供应商</TableHead><TableHead>模型</TableHead><TableHead>请求</TableHead>
                <TableHead>Token</TableHead><TableHead>估算成本</TableHead><TableHead>定价状态</TableHead>
              </TableRow></TableHeader>
              <TableBody>{groups.map(group => (
                <TableRow key={`${group.providerId}-${group.model}`}>
                  <TableCell>{group.providerName || '未知供应商'}</TableCell>
                  <TableCell className="font-mono text-xs">{group.model}</TableCell>
                  <TableCell>{group.requests}（成功 {group.succeeded}）</TableCell>
                  <TableCell>{group.totalTokens.toLocaleString()}</TableCell>
                  <TableCell>{money(group.estimatedCostMicros)}</TableCell>
                  <TableCell>{group.unpriced ? <Badge variant="warning">{group.unpriced} 条未定价</Badge> : <Badge variant="success">已定价</Badge>}</TableCell>
                </TableRow>
              ))}</TableBody>
            </Table>
          </Card>
        </>
      )}

      <Card className="mb-6">
        <CardHeader><CardTitle>新增价格版本</CardTitle></CardHeader>
        <CardContent>
          <form onSubmit={submitPrice} className="grid grid-cols-4 gap-4 items-end">
            <div><Label>供应商</Label><select className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm" value={form.providerId} onChange={e => selectProvider(e.target.value)} required>
              <option value="">请选择</option>{providers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select></div>
            <div><Label>模型</Label><select className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm" value={form.model} onChange={e => setForm({ ...form, model: e.target.value })} required>
              <option value="">请选择</option>{models.map(id => <option key={id} value={id}>{id}</option>)}
            </select></div>
            <div><Label>版本标识</Label><Input value={form.version} onChange={e => setForm({ ...form, version: e.target.value })} placeholder="例如 2026-07" required /></div>
            <div><Label>生效时间</Label><Input type="datetime-local" value={form.effectiveFrom} onChange={e => setForm({ ...form, effectiveFrom: e.target.value })} required /></div>
            <div><Label>输入 USD / 1M Token</Label><Input type="number" min="0" step="0.000001" value={form.input} onChange={e => setForm({ ...form, input: e.target.value })} required /></div>
            <div><Label>输出 USD / 1M Token</Label><Input type="number" min="0" step="0.000001" value={form.output} onChange={e => setForm({ ...form, output: e.target.value })} required /></div>
            <div><Label>推理附加费 USD / 1M Token</Label><Input type="number" min="0" step="0.000001" value={form.reasoning} onChange={e => setForm({ ...form, reasoning: e.target.value })} required /></div>
            <div><Label>单请求预留 USD</Label><Input type="number" min="0" step="0.000001" value={form.reserve} onChange={e => setForm({ ...form, reserve: e.target.value })} required /></div>
            <Button type="submit" disabled={saving}><Plus className="w-4 h-4 mr-1.5" />{saving ? '保存中…' : '新增版本'}</Button>
          </form>
          <p className="mt-3 text-xs text-slate-500">推理附加费会叠加在输出成本之上；若供应商已将推理 token 计入普通输出价，请填写 0。</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>价格版本</CardTitle></CardHeader>
        <Table>
          <TableHeader><TableRow>
            <TableHead>供应商 / 模型</TableHead><TableHead>版本</TableHead><TableHead>输入</TableHead>
            <TableHead>输出</TableHead><TableHead>推理附加费</TableHead><TableHead>请求预留</TableHead><TableHead>生效时间</TableHead>
          </TableRow></TableHeader>
          <TableBody>{prices.map(price => (
            <TableRow key={price.id}>
              <TableCell>{price.providerName}<div className="font-mono text-xs text-slate-500">{price.model}</div></TableCell>
              <TableCell>{price.version}</TableCell>
              <TableCell>{money(price.inputCostMicrosPerMillion)}</TableCell>
              <TableCell>{money(price.outputCostMicrosPerMillion)}</TableCell>
              <TableCell>{money(price.reasoningCostMicrosPerMillion)}</TableCell>
              <TableCell>{money(price.requestReserveMicros)}</TableCell>
              <TableCell>{new Date(price.effectiveFrom).toLocaleString()}</TableCell>
            </TableRow>
          ))}</TableBody>
        </Table>
      </Card>
    </div>
  )
}
