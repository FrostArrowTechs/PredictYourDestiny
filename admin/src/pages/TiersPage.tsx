import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Layers, Edit, Trash2, Infinity as InfinityIcon } from 'lucide-react'
import { useAuth } from '../App'
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
  features: string
  priceMonth: number
  sortOrder: number
}

export default function TiersPage() {
  const { t } = useTranslation()
  const { token } = useAuth()
  const [tiers, setTiers] = useState<Tier[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingTier, setEditingTier] = useState<Tier | null>(null)
  const [form, setForm] = useState({
    code: '',
    name: '',
    dailyQuota: 5,
    priceYuan: 0,
  })

  useEffect(() => {
    loadTiers()
  }, [token])

  const loadTiers = async () => {
    setIsLoading(true)
    try {
      const res = await fetch('/api/admin/tiers', {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json()
      setTiers(data.tiers || [])
    } finally {
      setIsLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const url = editingTier ? `/api/admin/tiers/${editingTier.id}` : '/api/admin/tiers'
    const method = editingTier ? 'PUT' : 'POST'

    const body: Record<string, unknown> = {
      name: form.name,
      dailyQuota: form.dailyQuota,
      priceMonth: Math.round(form.priceYuan * 100),
    }
    if (!editingTier) body.code = form.code

    await fetch(url, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })

    setShowModal(false)
    setEditingTier(null)
    setForm({ code: '', name: '', dailyQuota: 5, priceYuan: 0 })
    loadTiers()
  }

  const handleDelete = async (id: number, code: string) => {
    if (code === 'free') {
      alert('免费层级不可删除')
      return
    }
    if (!confirm(`确定要删除层级 "${code}" 吗？`)) return
    const res = await fetch(`/api/admin/tiers/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` },
    })
    if (res.ok) {
      loadTiers()
    } else {
      const data = await res.json()
      alert(data.error || '删除失败')
    }
  }

  const openCreate = () => {
    setEditingTier(null)
    setForm({ code: '', name: '', dailyQuota: 5, priceYuan: 0 })
    setShowModal(true)
  }

  const openEdit = (tier: Tier) => {
    setEditingTier(tier)
    setForm({
      code: tier.code,
      name: tier.name,
      dailyQuota: tier.dailyQuota,
      priceYuan: tier.priceMonth / 100,
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
                <TableHead>{t('tiers.priceMonth')}</TableHead>
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