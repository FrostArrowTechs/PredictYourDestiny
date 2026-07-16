import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../App'

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
    features: '',
    priceMonth: 0,
  })

  useEffect(() => {
    loadTiers()
  }, [token])

  const loadTiers = () => {
    setIsLoading(true)
    fetch('/api/admin/tiers', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        setTiers(data.tiers || [])
      })
      .finally(() => setIsLoading(false))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const url = editingTier
      ? `/api/admin/tiers/${editingTier.id}`
      : '/api/admin/tiers'
    const method = editingTier ? 'PUT' : 'POST'

    const body: Record<string, unknown> = {
      name: form.name,
      dailyQuota: form.dailyQuota,
      features: form.features,
      priceMonth: form.priceMonth,
    }
    if (!editingTier) {
      body.code = form.code
    }

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
    setForm({ code: '', name: '', dailyQuota: 5, features: '', priceMonth: 0 })
    loadTiers()
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure?')) return
    const res = await fetch(`/api/admin/tiers/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` },
    })
    if (res.ok) {
      loadTiers()
    } else {
      const data = await res.json()
      alert(data.error || 'Failed to delete')
    }
  }

  const openEditModal = (tier: Tier) => {
    setEditingTier(tier)
    setForm({
      code: tier.code,
      name: tier.name,
      dailyQuota: tier.dailyQuota,
      features: tier.features,
      priceMonth: tier.priceMonth,
    })
    setShowModal(true)
  }

  const formatPrice = (cents: number) => {
    return (cents / 100).toFixed(2)
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">{t('tiers.title')}</h1>
        <button
          onClick={() => setShowModal(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg"
        >
          {t('tiers.add')}
        </button>
      </div>

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left">{t('tiers.code')}</th>
              <th className="px-4 py-3 text-left">{t('tiers.name')}</th>
              <th className="px-4 py-3 text-left">{t('tiers.dailyQuota')}</th>
              <th className="px-4 py-3 text-left">{t('tiers.priceMonth')}</th>
              <th className="px-4 py-3 text-left">{t('tiers.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={5} className="px-4 py-8 text-center">{t('common.loading')}</td></tr>
            ) : tiers.length === 0 ? (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-500">No tiers</td></tr>
            ) : (
              tiers.map(tier => (
                <tr key={tier.id} className="border-t">
                  <td className="px-4 py-3">
                    <code className="bg-gray-100 px-2 py-1 rounded text-sm">{tier.code}</code>
                  </td>
                  <td className="px-4 py-3 font-medium">{tier.name}</td>
                  <td className="px-4 py-3">
                    {tier.dailyQuota === -1 ? t('tiers.unlimited') : tier.dailyQuota}
                  </td>
                  <td className="px-4 py-3">
                    {tier.priceMonth > 0 ? `¥${formatPrice(tier.priceMonth)}` : 'Free'}
                  </td>
                  <td className="px-4 py-3 space-x-2">
                    <button
                      onClick={() => openEditModal(tier)}
                      className="text-blue-600 hover:underline"
                    >
                      {t('tiers.edit')}
                    </button>
                    {tier.code !== 'free' && (
                      <button
                        onClick={() => handleDelete(tier.id)}
                        className="text-red-600 hover:underline"
                      >
                        {t('tiers.delete')}
                      </button>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white p-6 rounded-lg shadow-lg w-[400px]">
            <h2 className="text-lg font-bold mb-4">
              {editingTier ? t('tiers.edit') : t('tiers.add')}
            </h2>
            <form onSubmit={handleSubmit} className="space-y-4">
              {!editingTier && (
                <div>
                  <label className="block text-sm font-medium mb-1">{t('tiers.code')}</label>
                  <input
                    type="text"
                    value={form.code}
                    onChange={e => setForm({ ...form, code: e.target.value })}
                    className="w-full px-3 py-2 border rounded"
                    required
                  />
                </div>
              )}
              <div>
                <label className="block text-sm font-medium mb-1">{t('tiers.name')}</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={e => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 border rounded"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">{t('tiers.dailyQuota')}</label>
                <input
                  type="number"
                  value={form.dailyQuota}
                  onChange={e => setForm({ ...form, dailyQuota: parseInt(e.target.value) || -1 })}
                  className="w-full px-3 py-2 border rounded"
                />
                <p className="text-xs text-gray-500 mt-1">Use -1 for unlimited</p>
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">{t('tiers.priceMonth')} (元)</label>
                <input
                  type="number"
                  step="0.01"
                  value={form.priceMonth / 100}
                  onChange={e => setForm({ ...form, priceMonth: Math.round(parseFloat(e.target.value) * 100) || 0 })}
                  className="w-full px-3 py-2 border rounded"
                />
              </div>
              <div className="flex gap-2 pt-4">
                <button type="submit" className="flex-1 py-2 bg-blue-600 text-white rounded">
                  {t('common.save')}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setShowModal(false)
                    setEditingTier(null)
                  }}
                  className="flex-1 py-2 bg-gray-200 rounded"
                >
                  {t('common.cancel')}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}