import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Search, Users as UsersIcon, Shield, ChevronLeft, ChevronRight } from 'lucide-react'
import { useAuth } from '../App'
import { PageHeader, EmptyState, LoadingState } from '../components/PageHeader'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
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
import { Select } from '../components/ui/Select'
import { Label } from '../components/ui/Label'

interface User {
  id: number
  email: string
  displayName: string
  role: string
  tierCode: string
  tierName: string
  createdAt: string
}

interface Tier {
  id: number
  code: string
  name: string
}

export default function UsersPage() {
  const { t } = useTranslation()
  const { token } = useAuth()
  const [users, setUsers] = useState<User[]>([])
  const [tiers, setTiers] = useState<Tier[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [selectedTier, setSelectedTier] = useState<number>(0)

  useEffect(() => {
    loadUsers()
    loadTiers()
  }, [token, page])

  const loadUsers = async () => {
    setIsLoading(true)
    const params = new URLSearchParams({ page: String(page), limit: '20' })
    if (search) params.set('search', search)
    try {
      const res = await fetch(`/api/admin/users?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json()
      setUsers(data.users || [])
      setTotal(data.total || 0)
    } finally {
      setIsLoading(false)
    }
  }

  const loadTiers = async () => {
    const res = await fetch('/api/admin/tiers', { headers: { Authorization: `Bearer ${token}` } })
    const data = await res.json()
    setTiers(data.tiers || [])
  }

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setPage(1)
    loadUsers()
  }

  const handleUpdateTier = async () => {
    if (!editingUser || !selectedTier) return
    await fetch(`/api/admin/users/${editingUser.id}/tier`, {
      method: 'PUT',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ tierId: selectedTier }),
    })
    setEditingUser(null)
    setSelectedTier(0)
    loadUsers()
  }

  const totalPages = Math.ceil(total / 20)

  return (
    <div>
      <PageHeader
        title={t('users.title')}
        description={`共 ${total} 位用户`}
      />

      <Card>
        {/* Search bar */}
        <div className="p-4 border-b border-slate-200">
          <form onSubmit={handleSearch} className="flex items-center gap-2 max-w-md">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
              <Input
                type="text"
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder={t('users.search')}
                className="pl-9"
              />
            </div>
            <Button type="submit" variant="outline">
              搜索
            </Button>
          </form>
        </div>

        {/* Table */}
        {isLoading ? (
          <LoadingState />
        ) : users.length === 0 ? (
          <EmptyState icon={UsersIcon} title="暂无用户" description="没有找到符合条件的用户" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('users.email')}</TableHead>
                <TableHead>{t('users.displayName')}</TableHead>
                <TableHead>{t('users.role')}</TableHead>
                <TableHead>{t('users.tier')}</TableHead>
                <TableHead>{t('users.createdAt')}</TableHead>
                <TableHead className="text-right">{t('users.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map(user => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium text-slate-900">{user.email}</TableCell>
                  <TableCell className="text-slate-600">{user.displayName || '—'}</TableCell>
                  <TableCell>
                    {user.role === 'admin' ? (
                      <Badge variant="default" className="gap-1">
                        <Shield className="w-3 h-3" />
                        {t('users.roleAdmin')}
                      </Badge>
                    ) : (
                      <Badge variant="secondary">{t('users.roleUser')}</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {user.tierName ? (
                      <Badge variant="outline">{user.tierName}</Badge>
                    ) : (
                      <span className="text-slate-400">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-slate-500 text-xs">
                    {new Date(user.createdAt).toLocaleDateString('zh-CN')}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setEditingUser(user)}
                    >
                      {t('users.editTier')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between p-4 border-t border-slate-200">
            <p className="text-sm text-slate-500">
              第 {page} 页 / 共 {totalPages} 页
            </p>
            <div className="flex items-center gap-1">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage(p => Math.max(1, p - 1))}
                disabled={page === 1}
              >
                <ChevronLeft className="w-4 h-4" />
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
              >
                <ChevronRight className="w-4 h-4" />
              </Button>
            </div>
          </div>
        )}
      </Card>

      {/* Edit Tier Dialog */}
      <Dialog open={!!editingUser} onOpenChange={open => !open && setEditingUser(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('users.editTier')}</DialogTitle>
            <DialogDescription>{editingUser?.email}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2 py-2">
            <Label>选择会员层级</Label>
            <Select value={selectedTier} onChange={e => setSelectedTier(Number(e.target.value))}>
              <option value={0}>请选择...</option>
              {tiers.map(tier => (
                <option key={tier.id} value={tier.id}>
                  {tier.name} ({tier.code})
                </option>
              ))}
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditingUser(null)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleUpdateTier} disabled={!selectedTier}>
              {t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}