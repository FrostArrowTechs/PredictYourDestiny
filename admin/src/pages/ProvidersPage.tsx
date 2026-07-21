import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Bot, Trash2, Edit, Check, Star, Activity } from 'lucide-react'
import { apiRequest } from '../api/client'
import { PageHeader, EmptyState, LoadingState } from '../components/PageHeader'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Textarea } from '../components/ui/Textarea'
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

interface Provider {
  id: number
  name: string
  baseUrl: string
  models: string
  isDefault: boolean
  isEnabled: boolean
  sortOrder: number
  }

export default function ProvidersPage() {
  const { t } = useTranslation()
  const [providers, setProviders] = useState<Provider[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null)
  const [checkingProvider, setCheckingProvider] = useState<number | null>(null)
  const [form, setForm] = useState({
    name: '',
    baseUrl: '',
    apiKey: '',
    models: '[]',
    isDefault: false,
    isEnabled: true,
  })

  useEffect(() => {
    loadProviders()
  }, [])

  const loadProviders = async () => {
    setIsLoading(true)
    try {
      const data = await apiRequest<{ providers?: Provider[] }>('/admin/providers')
      setProviders(data.providers || [])
    } finally {
      setIsLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const url = editingProvider ? `/admin/providers/${editingProvider.id}` : '/admin/providers'
    const method = editingProvider ? 'PUT' : 'POST'

    await apiRequest(url, {
      method,
      body: form,
    })

    setShowModal(false)
    setEditingProvider(null)
    setForm({ name: '', baseUrl: '', apiKey: '', models: '[]', isDefault: false, isEnabled: true })
    loadProviders()
  }

  const handleDelete = async (id: number, name: string) => {
    if (!confirm(`确定要删除供应商 "${name}" 吗？`)) return
    await apiRequest(`/admin/providers/${id}`, { method: 'DELETE' })
    loadProviders()
  }

  const handleSetDefault = async (id: number) => {
    await apiRequest(`/admin/providers/${id}/default`, { method: 'POST' })
    loadProviders()
	}

  const handleHealthCheck = async (id: number) => {
    setCheckingProvider(id)
    try {
      const result = await apiRequest<{ latencyMs: number }>(`/admin/providers/${id}/health`, { method: 'POST' })
      alert(`供应商连接正常，延迟 ${result.latencyMs} ms`)
    } catch (error) {
      alert(error instanceof Error ? error.message : '供应商健康检查失败')
    } finally {
      setCheckingProvider(null)
    }
  }

  const openCreate = () => {
    setEditingProvider(null)
    setForm({ name: '', baseUrl: '', apiKey: '', models: '[]', isDefault: false, isEnabled: true })
    setShowModal(true)
  }

  const openEdit = (provider: Provider) => {
    setEditingProvider(provider)
    setForm({
      name: provider.name,
      baseUrl: provider.baseUrl,
      apiKey: '',
      models: provider.models,
      isDefault: provider.isDefault,
      isEnabled: provider.isEnabled,
    })
    setShowModal(true)
  }

  return (
    <div>
      <PageHeader
        title={t('providers.title')}
        description="管理 AI 模型供应商，可随时切换默认或新增"
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4 mr-1.5" />
            {t('providers.add')}
          </Button>
        }
      />

      <Card>
        {isLoading ? (
          <LoadingState />
        ) : providers.length === 0 ? (
          <EmptyState
            icon={Bot}
            title="还没有供应商"
            description="添加一个 OpenAI 兼容的 API 端点开始使用"
            action={
              <Button onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1.5" />
                添加第一个供应商
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('providers.name')}</TableHead>
                <TableHead>{t('providers.baseUrl')}</TableHead>
                <TableHead>状态</TableHead>
                <TableHead className="text-right">{t('providers.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {providers.map(provider => (
                <TableRow key={provider.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-slate-900">{provider.name}</span>
                      {provider.isDefault && (
                        <Badge variant="default" className="gap-1">
                          <Star className="w-3 h-3 fill-current" />
                          默认
                        </Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {provider.baseUrl}
                  </TableCell>
                  <TableCell>
                    {provider.isEnabled ? (
                      <Badge variant="success">已启用</Badge>
                    ) : (
                      <Badge variant="secondary">已禁用</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleHealthCheck(provider.id)}
                        disabled={checkingProvider === provider.id}
                        title="检查连接"
                      >
                        <Activity className="w-4 h-4" />
                      </Button>
                      {!provider.isDefault && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleSetDefault(provider.id)}
                          title="设为默认"
                        >
                          <Check className="w-4 h-4" />
                        </Button>
                      )}
                      <Button variant="ghost" size="sm" onClick={() => openEdit(provider)}>
                        <Edit className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDelete(provider.id, provider.name)}
                        className="text-red-600 hover:text-red-700 hover:bg-red-50"
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </Card>

      {/* Modal */}
      <Dialog open={showModal} onOpenChange={open => !open && setShowModal(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingProvider ? '编辑供应商' : '添加供应商'}
            </DialogTitle>
            <DialogDescription>
              填写 OpenAI 兼容 API 的连接信息
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="pname">名称</Label>
              <Input
                id="pname"
                value={form.name}
                onChange={e => setForm({ ...form, name: e.target.value })}
                placeholder="例如：OpenAI / DeepSeek"
                required
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="purl">Base URL</Label>
              <Input
                id="purl"
                value={form.baseUrl}
                onChange={e => setForm({ ...form, baseUrl: e.target.value })}
                placeholder="https://api.openai.com/v1"
                required
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pkey">API Key</Label>
              <Input
                id="pkey"
                value={form.apiKey}
                onChange={e => setForm({ ...form, apiKey: e.target.value })}
                placeholder={editingProvider ? '留空保留原值' : 'sk-...'}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pmodels">模型列表 (JSON)</Label>
              <Textarea
                id="pmodels"
                value={form.models}
                onChange={e => setForm({ ...form, models: e.target.value })}
                rows={4}
                className="font-mono text-xs"
                placeholder='[{"id":"gpt-4o","tier":"paid"}]'
              />
            </div>
            <div className="flex gap-6 pt-1">
              <label className="flex items-center gap-2 text-sm text-slate-700 cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.isDefault}
                  onChange={e => setForm({ ...form, isDefault: e.target.checked })}
                  className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                />
                设为默认
              </label>
              <label className="flex items-center gap-2 text-sm text-slate-700 cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.isEnabled}
                  onChange={e => setForm({ ...form, isEnabled: e.target.checked })}
                  className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                />
                启用
              </label>
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
