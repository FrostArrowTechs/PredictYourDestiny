import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../App'

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

  const loadUsers = () => {
    setIsLoading(true)
    const params = new URLSearchParams({ page: String(page), limit: '20' })
    if (search) params.set('search', search)
    fetch(`/api/admin/users?${params}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        setUsers(data.users || [])
        setTotal(data.total || 0)
      })
      .finally(() => setIsLoading(false))
  }

  const loadTiers = () => {
    fetch('/api/admin/tiers', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => setTiers(data.tiers || []))
  }

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setPage(1)
    loadUsers()
  }

  const handleUpdateTier = async (userId: number) => {
    if (!selectedTier) return
    await fetch(`/api/admin/users/${userId}/tier`, {
      method: 'PUT',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ tierId: selectedTier }),
    })
    setEditingUser(null)
    loadUsers()
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">{t('users.title')}</h1>
      
      {/* Search */}
      <form onSubmit={handleSearch} className="mb-4">
        <input
          type="text"
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder={t('users.search')}
          className="px-4 py-2 border rounded-lg w-64"
        />
        <button type="submit" className="ml-2 px-4 py-2 bg-blue-600 text-white rounded-lg">
          {t('common.confirm')}
        </button>
      </form>

      {/* Table */}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left">{t('users.email')}</th>
              <th className="px-4 py-3 text-left">{t('users.displayName')}</th>
              <th className="px-4 py-3 text-left">{t('users.role')}</th>
              <th className="px-4 py-3 text-left">{t('users.tier')}</th>
              <th className="px-4 py-3 text-left">{t('users.createdAt')}</th>
              <th className="px-4 py-3 text-left">{t('users.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center">{t('common.loading')}</td></tr>
            ) : users.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-500">No users found</td></tr>
            ) : (
              users.map(user => (
                <tr key={user.id} className="border-t">
                  <td className="px-4 py-3">{user.email}</td>
                  <td className="px-4 py-3">{user.displayName || '-'}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded text-sm ${user.role === 'admin' ? 'bg-red-100 text-red-800' : 'bg-gray-100'}`}>
                      {user.role === 'admin' ? t('users.roleAdmin') : t('users.roleUser')}
                    </span>
                  </td>
                  <td className="px-4 py-3">{user.tierName || '-'}</td>
                  <td className="px-4 py-3 text-sm text-gray-500">
                    {new Date(user.createdAt).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => setEditingUser(user)}
                      className="text-blue-600 hover:underline"
                    >
                      {t('users.editTier')}
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {total > 20 && (
        <div className="mt-4 flex gap-2">
          <button
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page === 1}
            className="px-4 py-2 bg-gray-200 rounded disabled:opacity-50"
          >
            Previous
          </button>
          <span className="px-4 py-2">Page {page}</span>
          <button
            onClick={() => setPage(p => p + 1)}
            disabled={page * 20 >= total}
            className="px-4 py-2 bg-gray-200 rounded disabled:opacity-50"
          >
            Next
          </button>
        </div>
      )}

      {/* Edit Tier Modal */}
      {editingUser && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white p-6 rounded-lg shadow-lg w-96">
            <h2 className="text-lg font-bold mb-4">{t('users.editTier')}</h2>
            <p className="text-sm text-gray-600 mb-4">{editingUser.email}</p>
            <select
              value={selectedTier}
              onChange={e => setSelectedTier(Number(e.target.value))}
              className="w-full px-3 py-2 border rounded mb-4"
            >
              <option value={0}>Select tier...</option>
              {tiers.map(tier => (
                <option key={tier.id} value={tier.id}>{tier.name} ({tier.code})</option>
              ))}
            </select>
            <div className="flex gap-2">
              <button
                onClick={() => handleUpdateTier(editingUser.id)}
                disabled={!selectedTier}
                className="flex-1 py-2 bg-blue-600 text-white rounded disabled:opacity-50"
              >
                {t('common.confirm')}
              </button>
              <button
                onClick={() => setEditingUser(null)}
                className="flex-1 py-2 bg-gray-200 rounded"
              >
                {t('common.cancel')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}