import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../App'

interface Setting {
  key: string
  value: string
  kind: string
  group: string
  label: string
  hint: string
}

export default function SettingsPage() {
  const { t } = useTranslation()
  const { token } = useAuth()
  const [settings, setSettings] = useState<Setting[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [saving, setSaving] = useState<string | null>(null)

  useEffect(() => {
    loadSettings()
  }, [token])

  const loadSettings = () => {
    setIsLoading(true)
    fetch('/api/settings', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => res.json())
      .then(data => {
        setSettings(data.settings || [])
      })
      .finally(() => setIsLoading(false))
  }

  const handleSave = async (key: string, value: string) => {
    setSaving(key)
    await fetch('/api/settings', {
      method: 'PUT',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ settings: [{ key, value }] }),
    })
    setSaving(null)
  }

  const grouped = settings.reduce((acc, s) => {
    const group = s.group || 'general'
    if (!acc[group]) acc[group] = []
    acc[group].push(s)
    return acc
  }, {} as Record<string, Setting[]>)

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">{t('settings.title')}</h1>

      {isLoading ? (
        <div className="text-center py-8">{t('common.loading')}</div>
      ) : (
        <div className="space-y-6">
          {Object.entries(grouped).map(([group, items]) => (
            <div key={group} className="bg-white rounded-lg shadow p-6">
              <h2 className="text-lg font-semibold mb-4 capitalize">{group}</h2>
              <div className="space-y-4">
                {items.map(setting => (
                  <div key={setting.key}>
                    <label className="block text-sm font-medium mb-1">
                      {setting.label}
                    </label>
                    {setting.hint && (
                      <p className="text-xs text-gray-500 mb-1">{setting.hint}</p>
                    )}
                    <div className="flex gap-2">
                      {setting.kind === 'password' ? (
                        <input
                          type="password"
                          defaultValue={setting.value}
                          className="flex-1 px-3 py-2 border rounded"
                          id={setting.key}
                        />
                      ) : setting.kind === 'bool' ? (
                        <select
                          defaultValue={setting.value}
                          className="flex-1 px-3 py-2 border rounded"
                          id={setting.key}
                        >
                          <option value="true">Enabled</option>
                          <option value="false">Disabled</option>
                        </select>
                      ) : setting.kind === 'json' ? (
                        <textarea
                          defaultValue={setting.value}
                          className="flex-1 px-3 py-2 border rounded"
                          rows={3}
                          id={setting.key}
                        />
                      ) : (
                        <input
                          type={setting.kind === 'number' ? 'number' : 'text'}
                          defaultValue={setting.value}
                          className="flex-1 px-3 py-2 border rounded"
                          id={setting.key}
                        />
                      )}
                      <button
                        onClick={() => {
                          const el = document.getElementById(setting.key) as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement
                          handleSave(setting.key, el.value)
                        }}
                        disabled={saving === setting.key}
                        className="px-4 py-2 bg-blue-600 text-white rounded disabled:opacity-50"
                      >
                        {saving === setting.key ? t('common.loading') : t('settings.save')}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}