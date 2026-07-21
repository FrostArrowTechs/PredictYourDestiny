import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, Settings as SettingsIcon, Check } from 'lucide-react'
import { apiRequest } from '../api/client'
import { PageHeader, LoadingState } from '../components/PageHeader'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Textarea } from '../components/ui/Textarea'
import { Label } from '../components/ui/Label'
import { Select } from '../components/ui/Select'
import { cn } from '../lib/utils'

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
  const [settings, setSettings] = useState<Setting[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [savedKey, setSavedKey] = useState<string | null>(null)
  const [savingKey, setSavingKey] = useState<string | null>(null)
  const [drafts, setDrafts] = useState<Record<string, string>>({})

  useEffect(() => {
    loadSettings()
  }, [])

  const loadSettings = async () => {
    setIsLoading(true)
    try {
      const data = await apiRequest<{ items?: Setting[] }>('/settings')
      setSettings(data.items || [])
      const drafts: Record<string, string> = {}
      data.items?.forEach((s: Setting) => { drafts[s.key] = s.value })
      setDrafts(drafts)
    } finally {
      setIsLoading(false)
    }
  }

  const handleSave = async (key: string) => {
    setSavingKey(key)
    try {
      await apiRequest('/settings', {
        method: 'PUT',
        body: { items: { [key]: drafts[key] } },
      })
      setSavedKey(key)
      setTimeout(() => setSavedKey(null), 2000)
    } finally {
      setSavingKey(null)
    }
  }

  const updateDraft = (key: string, value: string) => {
    setDrafts({ ...drafts, [key]: value })
  }

  const grouped = settings.reduce((acc, s) => {
    const group = s.group || 'general'
    if (!acc[group]) acc[group] = []
    acc[group].push(s)
    return acc
  }, {} as Record<string, Setting[]>)

  const groupLabels: Record<string, string> = {
    ai: 'AI 网关',
    feature: '功能设置',
    general: '通用设置',
  }

  return (
    <div>
      <PageHeader
        title={t('settings.title')}
        description="修改后立即生效，无需重启服务"
      />

      {isLoading ? (
        <LoadingState />
      ) : Object.keys(grouped).length === 0 ? (
        <Card>
          <div className="p-12 text-center">
            <SettingsIcon className="w-8 h-8 text-slate-300 mx-auto mb-3" />
            <p className="text-sm text-slate-500">暂无设置项</p>
          </div>
        </Card>
      ) : (
        <div className="space-y-4">
          {Object.entries(grouped).map(([group, items]) => (
            <Card key={group}>
              <CardHeader>
                <CardTitle className="text-base">
                  {groupLabels[group] || group}
                </CardTitle>
                <CardDescription>
                  {group === 'ai' && '配置 AI 服务的连接信息和模型选择'}
                  {group === 'feature' && '调整平台功能开关与限额'}
                  {group === 'general' && '站点基本信息'}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-5">
                {items.map(setting => {
                  const isSaved = savedKey === setting.key
                  return (
                    <div key={setting.key} className="space-y-1.5">
                      <div className="flex items-center justify-between">
                        <Label htmlFor={setting.key}>
                          {setting.label}
                        </Label>
                        <Button
                          size="sm"
                          variant={isSaved ? 'outline' : 'default'}
                          onClick={() => handleSave(setting.key)}
                          disabled={savingKey === setting.key}
                          className={cn(isSaved && 'text-green-600 border-green-300')}
                        >
                          {isSaved ? (
                            <>
                              <Check className="w-3.5 h-3.5 mr-1" />
                              已保存
                            </>
                          ) : savingKey === setting.key ? (
                            '保存中...'
                          ) : (
                            <>
                              <Save className="w-3.5 h-3.5 mr-1" />
                              保存
                            </>
                          )}
                        </Button>
                      </div>
                      {setting.hint && (
                        <p className="text-xs text-slate-500">{setting.hint}</p>
                      )}
                      {setting.kind === 'password' ? (
                        <Input
                          id={setting.key}
                          type="password"
                          value={drafts[setting.key] || ''}
                          onChange={e => updateDraft(setting.key, e.target.value)}
                          placeholder="••••••••"
                        />
                      ) : setting.kind === 'bool' ? (
                        <Select
                          id={setting.key}
                          value={drafts[setting.key] || ''}
                          onChange={e => updateDraft(setting.key, e.target.value)}
                        >
                          <option value="true">启用</option>
                          <option value="false">禁用</option>
                        </Select>
                      ) : setting.kind === 'json' ? (
                        <Textarea
                          id={setting.key}
                          value={drafts[setting.key] || ''}
                          onChange={e => updateDraft(setting.key, e.target.value)}
                          rows={4}
                          className="font-mono text-xs"
                        />
                      ) : setting.kind === 'number' ? (
                        <Input
                          id={setting.key}
                          type="number"
                          value={drafts[setting.key] || ''}
                          onChange={e => updateDraft(setting.key, e.target.value)}
                        />
                      ) : (
                        <Input
                          id={setting.key}
                          type="text"
                          value={drafts[setting.key] || ''}
                          onChange={e => updateDraft(setting.key, e.target.value)}
                        />
                      )}
                    </div>
                  )
                })}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
