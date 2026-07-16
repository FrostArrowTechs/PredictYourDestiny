import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../App'

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
  const { token } = useAuth()
  const [providers, setProviders] = useState<Provider[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null)
  const [form, setForm] = useState({
    name: '',
    baseUrl: '',
    apiKey: '',
    models: '',
    isDefault: false,
    isEnabled: true,
  })

  useEffect(() => {
    loadProviders()
  }, [token])

  const loadProviders = () => {
    setIsLoading(true)
    fetch('/api/admin/providers', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        setProviders(data.providers || [])
      })
      .finally(() => setIsLoading(false))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const url = editingProvider
      ? `/api/admin/providers/${editingProvider.id}`
      : '/api/admin/providers'
    const method = editingProvider ? 'PUT' : 'POST'

    await fetch(url, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(form),
    })

    setShowModal(false)
    setEditingProvider(null)
    setForm({ name: '', baseUrl: '', apiKey: '', models: '', isDefault: false, isEnabled: true })
    loadProviders()
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure?')) return
    await fetch(`/api/admin/providers/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` },
    })
    loadProviders()
  }

  const handleSetDefault = async (id: number) => {
    await fetch(`/api/admin/providers/${id}/default`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
    })
    loadProviders()
  }

  const openEditModal = (provider: Provider) => {
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
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">{t('providers.title')}</h1>
        <button
          onClick={() => setShowModal(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg"
        >
          {t('providers.add')}
        </button>
      </div>

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left">{t('providers.name')}</th>
              <th className="px-4 py-3 text-left">{t('providers.baseUrl')}</th>
              <th className="px-4 py-3 text-left">{t('providers.isDefault')}</th>
              <th className="px-4 py-3 text-left">{t('providers.isEnabled')}</th>
              <th className="px-4 py-3 text-left">{t('providers.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={5} className="px-4 py-8 text-center">{t('common.loading')}</td></tr>
            ) : providers.length === 0 ? (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-500">No providers</td></tr>
            ) : (
              providers.map(provider => (
                <tr key={provider.id} className="border-t">
                  <td className="px-4 py-3 font-medium">{provider.name}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{provider.baseUrl}</td>
                  <td className="px-4 py-3">
                    {provider.isDefault && (
                      <span className="px-2 py-1 bg-green-100 text-green-800 rounded text-sm">Default</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded text-sm ${provider.isEnabled ? 'bg-blue-100 text-blue-800' : 'bg-gray-100 text-gray-600'}`}>
                      {provider.isEnabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </td>
                  <td className="px-4 py-3 space-x-2">
                    {!provider.isDefault && (
                      <button
                        onClick={() => handleSetDefault(provider.id)}
                        className="text-green-600 hover:underline"
                      >
                        {t('providers.setDefault')}
                      </button>
                    )}
                    <button
                      onClick={() => openEditModal(provider)}
                      className="text-blue-600 hover:underline"
                    >
                      {t('providers.edit')}
                    </button>
                    <button
                      onClick={() => handleDelete(provider.id)}
                      className="text-red-600 hover:underline"
                    >
                      {t('providers.delete')}
                    </button>
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
          <div className="bg-white p-6 rounded-lg shadow-lg w-[500px]">
            <h2 className="text-lg font-bold mb-4">
              {editingProvider ? t('providers.edit') : t('providers.add')}
            </h2>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1">{t('providers.name')}</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={e => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 border rounded"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">{t('providers.baseUrl')}</label>
                <input
                  type="url"
                  value={form.baseUrl}
                  onChange={e => setForm({ ...form, baseUrl: e.target.value })}
                  className="w-full px-3 py-2 border rounded"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">{t('providers.apiKey')}</label>
                <input
                  type="text"
                  value={form.apiKey}
                  onChange={e => setForm({ ...form, apiKey: e.target.value })}
                  className="w-full px-3 py-2 border rounded"
                  placeholder="Leave empty to keep existing"
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">{t('providers.models')}</label>
                <textarea
                  value={form.models}
                  onChange={e => setForm({ ...form, models: e.target.value })}
                  className="w-full px-3 py-2 border rounded"
                  rows={3}
                  placeholder='[{"id":"gpt-4o","tier":"paid"}]'
                />
              </div>
              <div className="flex gap-4">
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={form.isDefault}
                    onChange={e => setForm({ ...form, isDefault: e.target.checked })}
                  />
                  {t('providers.isDefault')}
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={form.isEnabled}
                    onChange={e => setForm({ ...form, isEnabled: e.target.checked })}
                  />
                  {t('providers.isEnabled')}
                </label>
              </div>
              <div className="flex gap-2 pt-4">
                <button type="submit" className="flex-1 py-2 bg-blue-600 text-white rounded">
                  {t('common.save')}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setShowModal(false)
                    setEditingProvider(null)
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