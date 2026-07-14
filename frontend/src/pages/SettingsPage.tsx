import { useEffect, useState } from 'react'
import {
  Form,
  FormGroup,
  NumberInput,
  Toggle,
  Select,
  SelectItem,
  TextArea,
  Button,
  InlineNotification,
} from '@carbon/react'
import { getSettings, putSettings } from '../api'
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
      setSaved(true)
      setTimeout(() => setSaved(false), 2500)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save settings')
    }
  }

  if (!settings) return <div style={{ padding: '1.5rem' }}>Loading…</div>

  return (
    <div style={{ padding: '1.5rem', maxWidth: 640 }}>
      <h1 style={{ fontSize: '1.5rem', fontWeight: 600, marginBottom: '1.5rem' }}>Settings</h1>

      {error && (
        <InlineNotification kind="error" title="Error" subtitle={error} onCloseButtonClick={() => setError(null)} style={{ marginBottom: '1rem' }} />
      )}
      {saved && (
        <InlineNotification kind="success" title="Saved" subtitle="Settings saved successfully." style={{ marginBottom: '1rem' }} />
      )}

      <Form>
        <FormGroup legendText="Scan">
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
            style={{ marginTop: '1rem' }}
            onChange={(_: unknown, { value }: { value: string | number }) =>
              setSettings((s) => s ? { ...s, registryTimeoutSeconds: Number(value) } : s)
            }
          />
          <Toggle
            id="include-completed"
            labelText="Include completed pods"
            labelA="Off"
            labelB="On"
            toggled={settings.includeCompletedPods}
            onToggle={(v: boolean) => setSettings((s) => s ? { ...s, includeCompletedPods: v } : s)}
            style={{ marginTop: '1rem' }}
          />
        </FormGroup>

        <FormGroup legendText="Namespaces" style={{ marginTop: '1.5rem' }}>
          <TextArea
            id="included-ns"
            labelText="Included namespaces (one per line, empty = all)"
            value={(settings.includedNamespaces ?? []).join('\n')}
            rows={4}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
              setSettings((s) => s ? { ...s, includedNamespaces: e.target.value.split('\n').filter(Boolean) } : s)
            }
          />
          <TextArea
            id="excluded-ns"
            labelText="Excluded namespaces (one per line)"
            value={(settings.excludedNamespaces ?? []).join('\n')}
            rows={4}
            style={{ marginTop: '1rem' }}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
              setSettings((s) => s ? { ...s, excludedNamespaces: e.target.value.split('\n').filter(Boolean) } : s)
            }
          />
        </FormGroup>

        <FormGroup legendText="Display" style={{ marginTop: '1.5rem' }}>
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
          <Toggle
            id="short-digests"
            labelText="Show short digests"
            labelA="Full"
            labelB="Short"
            toggled={settings.shortDigests}
            onToggle={(v: boolean) => setSettings((s) => s ? { ...s, shortDigests: v } : s)}
            style={{ marginTop: '1rem' }}
          />
        </FormGroup>

        <Button onClick={handleSave} style={{ marginTop: '2rem' }}>
          Save settings
        </Button>
      </Form>
    </div>
  )
}
