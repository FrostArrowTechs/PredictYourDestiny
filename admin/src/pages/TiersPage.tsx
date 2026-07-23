import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Layers, Edit, Trash2, Infinity as InfinityIcon } from 'lucide-react'
import { apiRequest, ApiError } from '../api/client'
import { PageHeader, EmptyState, LoadingState } from '../components/PageHeader'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Label } from '../components/ui/Label'
import { Badge } from '../components/ui/Badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../components/ui/Table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/Dialog'

interface Tier {
  id: number
  code: string
  name: string
  dailyQuota: number
  dailyCostBudgetMicros: number
  features: string
  priceMonth: number
  sortOrder: number
  isEnabled: boolean
}

export default function TiersPage() {
  const { t } = useTranslation()
  const [tiers, setTiers] = useState<Tier[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingTier, setEditingTier] = useState<Tier | null>(null)
  const [form, setForm] = useState({
    code: '',
    name: '',
    dailyQuota: 5,
    dailyCostBudgetUsd: -1,
    priceYuan: 0,
    isEnabled: true,
  })

  useEffect(() => {
    loadTiers()
  }, [])

  const loadTiers = async () => {
    setIsLoading(true)
    try {
      const data = await apiRequest<{ tiers?: Tier[] }>('/admin/tiers')
      setTiers(data.tiers || [])
    } finally {
      setIsLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const url = editingTier ? `/admin/tiers/${editingTier.id}` : '/admin/tiers'
    const method = editingTier ? 'PUT' : 'POST'

    const body: Record<string, unknown> = {
      name: form.name,
      dailyQuota: form.dailyQuota,
      dailyCostBudgetMicros: form.dailyCostBudgetUsd < 0 ? -1 : Math.round(form.dailyCostBudgetUsd * 1_000_000),
      priceMonth: Math.round(form.priceYuan * 100),
      isEnabled: form.isEnabled,
    }
    if (!editingTier) body.code = form.code

    await apiRequest(url, {
      method,
      body,
    })

    setShowModal(false)
    setEditingTier(null)
    setForm({ code: '', name: '', dailyQuota: 5, dailyCostBudgetUsd: -1, priceYuan: 0, isEnabled: true })
    loadTiers()
  }

  const handleDelete = async (id: number, code: string) => {
    if (code === 'free') {
      alert('免费层级不可删除')
      return
    }
    if (!confirm(`确定要删除层级 "${code}" 吗？`)) return
    try {
      await apiRequest(`/admin/tiers/${id}`, { method: 'DELETE' })
      loadTiers()
    } catch (error) {
      alert(error instanceof ApiError ? error.message : '删除失败')
    }
  }

  const openCreate = () => {
    setEditingTier(null)
    setForm({ code: '', name: '', dailyQuota: 5, dailyCostBudgetUsd: -1, priceYuan: 0, isEnabled: true })
    setShowModal(true)
  }

  const openEdit = (tier: Tier) => {
    setEditingTier(tier)
    setForm({
      code: tier.code,
      name: tier.name,
      dailyQuota: tier.dailyQuota,
      dailyCostBudgetUsd: tier.dailyCostBudgetMicros < 0 ? -1 : tier.dailyCostBudgetMicros / 1_000_000,
      priceYuan: tier.priceMonth / 100,
      isEnabled: tier.isEnabled,
    })
    setShowModal(true)
  }

  return (
    <div>
      <PageHeader
        title={t('tiers.title')}
        description="管理会员层级、每日配额与定价"
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1.5" />
            {t('tiers.add')}
          </Button>
        }
      />

      <Card>
        {isLoading ? (
          <LoadingState />
        ) : tiers.length === 0 ? (
          <EmptyState icon={Layers} title="还没有层级" description="添加第一个会员层级" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('tiers.code')}</TableHead>
                <TableHead>{t('tiers.name')}</TableHead>
                <TableHead>{t('tiers.dailyQuota')}</TableHead>
                <TableHead>每日成本预算</TableHead>
                <TableHead>{t('tiers.priceMonth')}</TableHead>
                <TableHead>状态</TableHead>
                <TableHead className="text-right">{t('tiers.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tiers.map(tier => (
                <TableRow key={tier.id}>
                  <TableCell>
                    <code className="px-2 py-0.5 rounded bg-slate-100 text-xs text-slate-700 font-mono">
                      {tier.code}
                    </code>
                  </TableCell>
                  <TableCell>{tier.dailyCostBudgetMicros < 0 ? <Badge variant="secondary">未启用</Badge> : `$${(tier.dailyCostBudgetMicros / 1_000_000).toFixed(4)}`}</TableCell>
                  <TableCell className="font-medium text-slate-900">{tier.name}</TableCell>
                  <TableCell>
                    {tier.dailyQuota === -1 ? (
                      <Badge variant="success" className="gap-1">
                        <InfinityIcon className="w-3 h-3" />
                        无限
                      </Badge>
                    ) : (
                      <span className="text-slate-700">{tier.dailyQuota} 次/日</span>
                    )}
                  </TableCell>
                  <TableCell>
                    {tier.priceMonth > 0 ? (
                      <span className="text-slate-900 font-medium">
                        ¥{(tier.priceMonth / 100).toFixed(2)}
                      </span>
                    ) : (
                      <Badge variant="secondary">免费</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant={tier.isEnabled ? 'success' : 'secondary'}>
                      {tier.isEnabled ? '启用' : '禁用'}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button variant="ghost" size="sm" onClick={() => openEdit(tier)}>
                        <Edit className="w-4 h-4" />
                      </Button>
                      {tier.code !== 'free' && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDelete(tier.id, tier.code)}
                          className="text-red-600 hover:text-red-700 hover:bg-red-50"
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </Card>

      <Dialog open={showModal} onOpenChange={open => !open && setShowModal(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingTier ? '编辑层级' : '添加层级'}</DialogTitle>
            <DialogDescription>
              {editingTier ? `修改 ${editingTier.code} 层级配置` : '创建一个新的会员层级'}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit} className="space-y-4">
            {!editingTier && (
              <div className="space-y-1.5">
                <Label htmlFor="tcode">代码</Label>
                <Input
                  id="tcode"
                  value={form.code}
                  onChange={e => setForm({ ...form, code: e.target.value })}
                  placeholder="free / basic / premium"
                  required
                />
              </div>
            )}
            <div className="space-y-1.5">
              <Label htmlFor="tcost">每日 AI 成本预算 (USD)</Label>
              <Input id="tcost" type="number" min="-1" step="0.000001" value={form.dailyCostBudgetUsd}
                onChange={e => setForm({ ...form, dailyCostBudgetUsd: Number(e.target.value) })} />
              <p className="text-xs text-slate-500">填 -1 表示暂不执行成本预算；启用前应先为模型配置单请求预留额</p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tname">名称</Label>
              <Input
                id="tname"
                value={form.name}
                onChange={e => setForm({ ...form, name: e.target.value })}
                placeholder="免费用户 / 基础会员"
                required
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tquota">每日配额</Label>
              <Input
                id="tquota"
                type="number"
                value={form.dailyQuota}
                onChange={e => setForm({ ...form, dailyQuota: parseInt(e.target.value) || -1 })}
                placeholder="5"
              />
              <p className="text-xs text-slate-500">填 -1 表示无限制</p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tprice">月费 (元)</Label>
              <Input
                id="tprice"
                type="number"
                step="0.01"
                value={form.priceYuan}
                onChange={e => setForm({ ...form, priceYuan: parseFloat(e.target.value) || 0 })}
                placeholder="0.00"
              />
              <p className="text-xs text-slate-500">填 0 表示免费</p>
            </div>
            <label className="flex items-center gap-2 text-sm text-slate-700">
              <input
                type="checkbox"
                checked={form.isEnabled}
                disabled={editingTier?.code === 'free'}
                onChange={e => setForm({ ...form, isEnabled: e.target.checked })}
              />
              启用该会员层级
            </label>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowModal(false)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSubmit}>{t('common.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
