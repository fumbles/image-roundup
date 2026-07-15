import { useEffect, useState } from 'react'
import {
  Form,
  NumberInput,
  Toggle,
  Select,
  SelectItem,
  TextArea,
  Button,
  InlineNotification,
} from '@carbon/react'
import { getSettings, putSettings } from '../api'
import { SETTINGS_SAVED_EVENT } from '../theme'
import type { Settings } from '../types'

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getSettings()
      .then(setSettings)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load settings'))
  }, [])

  const handleSave = async () => {
    if (!settings) return
    try {
      const updated = await putSettings(settings)
      setSettings(updated)
      window.dispatchEvent(new CustomEvent(SETTINGS_SAVED_EVENT, { detail: updated }))
      setSaved(true)
      setTimeout(() => setSaved(false), 2500)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save settings')
    }
  }

  if (!settings) return <div style={{ padding: '1.5rem' }}>Loading…</div>

  const parseLines = (value: string) => value.split('\n').map((v) => v.trim()).filter(Boolean)

  return (
    <div className="ir-settings">
      <div className="ir-settings-header">
        <h1>Settings</h1>
        <Button onClick={handleSave}>Save settings</Button>
      </div>

      {error && (
        <InlineNotification kind="error" title="Error" subtitle={error} onCloseButtonClick={() => setError(null)} style={{ marginBottom: '1rem' }} />
      )}
      {saved && (
        <InlineNotification kind="success" title="Saved" subtitle="Settings saved successfully." style={{ marginBottom: '1rem' }} />
      )}

      <Form className="ir-settings-form">
        <section className="ir-settings-section">
          <h2>Scan</h2>
          <div className="ir-settings-grid">
            <NumberInput
              id="scan-interval"
              label="Scan interval (seconds)"
              min={10}
              value={settings.scanIntervalSeconds}
              onChange={(_: unknown, { value }: { value: string | number }) =>
                setSettings((s) => s ? { ...s, scanIntervalSeconds: Number(value) } : s)
              }
            />
            <NumberInput
              id="registry-timeout"
              label="Registry timeout (seconds)"
              min={5}
              value={settings.registryTimeoutSeconds}
              onChange={(_: unknown, { value }: { value: string | number }) =>
                setSettings((s) => s ? { ...s, registryTimeoutSeconds: Number(value) } : s)
              }
            />
          </div>
          <div className="ir-toggle-row">
            <Toggle
              id="include-completed"
              labelText="Include completed pods"
              labelA="Off"
              labelB="On"
              toggled={settings.includeCompletedPods}
              onToggle={(v: boolean) => setSettings((s) => s ? { ...s, includeCompletedPods: v } : s)}
            />
            <Toggle
              id="exclude-internal-registry"
              labelText="Exclude internal OpenShift registry images"
              labelA="Off"
              labelB="On"
              toggled={settings.excludeInternalRegistry}
              onToggle={(v: boolean) => setSettings((s) => s ? { ...s, excludeInternalRegistry: v } : s)}
            />
          </div>
        </section>

        <section className="ir-settings-section">
          <h2>Namespaces</h2>
          <div className="ir-settings-grid">
            <TextArea
              id="included-ns"
              labelText="Included namespaces (one per line, empty = all)"
              value={(settings.includedNamespaces ?? []).join('\n')}
              rows={4}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
                setSettings((s) => s ? { ...s, includedNamespaces: parseLines(e.target.value) } : s)
              }
            />
            <TextArea
              id="excluded-ns"
              labelText="Excluded namespaces (one per line, supports trailing *)"
              value={(settings.excludedNamespaces ?? []).join('\n')}
              rows={4}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
                setSettings((s) => s ? { ...s, excludedNamespaces: parseLines(e.target.value) } : s)
              }
            />
          </div>
        </section>

        <section className="ir-settings-section">
          <h2>Display</h2>
          <div className="ir-settings-grid">
            <Select
              id="theme-select"
              labelText="Theme"
              value={settings.theme}
              onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
                setSettings((s) => s ? { ...s, theme: e.target.value as Settings['theme'] } : s)
              }
            >
              <SelectItem value="system" text="System default" />
              <SelectItem value="light" text="Light" />
              <SelectItem value="dark" text="Dark" />
            </Select>
            <div className="ir-settings-toggle-field">
              <Toggle
                id="short-digests"
                labelText="Show short digests"
                labelA="Full"
                labelB="Short"
                toggled={settings.shortDigests}
                onToggle={(v: boolean) => setSettings((s) => s ? { ...s, shortDigests: v } : s)}
              />
            </div>
          </div>
        </section>
      </Form>
    </div>
  )
}
