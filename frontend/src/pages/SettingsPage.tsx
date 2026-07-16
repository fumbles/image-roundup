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

const parseLines = (value: string) => value.split('\n').map((v) => v.trim()).filter(Boolean)

const formatLines = (values: string[] | undefined) => (values ?? []).join('\n')

const formatHelmRepositories = (repositories: Settings['helmRepositories'] | undefined) => (repositories ?? [])
  .map((repo) => `${repo.name}=${repo.url}`)
  .join('\n')

const parseHelmRepositories = (value: string): Settings['helmRepositories'] => parseLines(value)
  .map((line) => {
    const [name, ...urlParts] = line.split('=')
    return { name: name?.trim() ?? '', url: urlParts.join('=').trim() }
  })
  .filter((repo) => repo.name && repo.url)

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [includedNamespacesText, setIncludedNamespacesText] = useState('')
  const [excludedNamespacesText, setExcludedNamespacesText] = useState('')
  const [helmRepositoriesText, setHelmRepositoriesText] = useState('')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getSettings()
      .then((loaded) => {
        setSettings(loaded)
        setIncludedNamespacesText(formatLines(loaded.includedNamespaces))
        setExcludedNamespacesText(formatLines(loaded.excludedNamespaces))
        setHelmRepositoriesText(formatHelmRepositories(loaded.helmRepositories))
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load settings'))

    const handleSettingsSaved = (event: Event) => {
      const detail = (event as CustomEvent<Settings>).detail
      if (detail) {
        setSettings(detail)
        setIncludedNamespacesText(formatLines(detail.includedNamespaces))
        setExcludedNamespacesText(formatLines(detail.excludedNamespaces))
        setHelmRepositoriesText(formatHelmRepositories(detail.helmRepositories))
      }
    }

    window.addEventListener(SETTINGS_SAVED_EVENT, handleSettingsSaved)
    return () => window.removeEventListener(SETTINGS_SAVED_EVENT, handleSettingsSaved)
  }, [])

  const handleSave = async () => {
    if (!settings) return
    try {
      const updated = await putSettings({
        ...settings,
        includedNamespaces: parseLines(includedNamespacesText),
        excludedNamespaces: parseLines(excludedNamespacesText),
        helmRepositories: parseHelmRepositories(helmRepositoriesText),
      })
      setSettings(updated)
      setIncludedNamespacesText(formatLines(updated.includedNamespaces))
      setExcludedNamespacesText(formatLines(updated.excludedNamespaces))
      setHelmRepositoriesText(formatHelmRepositories(updated.helmRepositories))
      window.dispatchEvent(new CustomEvent(SETTINGS_SAVED_EVENT, { detail: updated }))
      setSaved(true)
      setTimeout(() => setSaved(false), 2500)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save settings')
    }
  }

  if (!settings) return <div style={{ padding: '1.5rem' }}>Loading…</div>

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
              value={includedNamespacesText}
              rows={4}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setIncludedNamespacesText(e.target.value)}
            />
            <TextArea
              id="excluded-ns"
              labelText="Excluded namespaces (one per line, supports trailing *)"
              value={excludedNamespacesText}
              rows={4}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setExcludedNamespacesText(e.target.value)}
            />
          </div>
        </section>

        <section className="ir-settings-section">
          <h2>Helm</h2>
          <TextArea
            id="helm-repositories"
            labelText="Chart repositories (name=url, one per line)"
            helperText="Used to compare installed Helm chart versions against repository index.yaml files."
            value={helmRepositoriesText}
            rows={4}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setHelmRepositoriesText(e.target.value)}
          />
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
