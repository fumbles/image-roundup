import { useEffect, useState } from 'react'
import { InlineNotification, Link, SkeletonText, Tag, Tile } from '@carbon/react'
import { getHelmReleases, getSettings } from '../api'
import type { HelmRelease, Settings } from '../types'
import { formatTime, relativeTime } from '../utils'

function statusTag(release: HelmRelease) {
  if (release.error) return <Tag type="red" size="sm">Check failed</Tag>
  if (release.updateAvailable) return <Tag type="blue" size="sm">Update available</Tag>
  if (release.latestChartVersion) return <Tag type="green" size="sm">Up to date</Tag>
  return <Tag type="gray" size="sm">Repo unknown</Tag>
}

export default function HelmPage() {
  const [releases, setReleases] = useState<HelmRelease[]>([])
  const [settings, setSettings] = useState<Settings | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([getHelmReleases(), getSettings()])
      .then(([releaseData, settingsData]) => {
        setReleases(releaseData)
        setSettings(settingsData)
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load Helm releases'))
      .finally(() => setLoading(false))
  }, [])

  const updates = releases.filter((release) => release.updateAvailable).length
  const repos = settings?.helmRepositories ?? []
  const repoMappingsMissing = releases.length > 0 && releases.every((release) =>
    !release.repositoryName && !release.latestChartVersion && !release.error
  )

  return (
    <div className="ir-page">
      <div className="ir-page-header">
        <div>
          <h1>Helm</h1>
          <p>
            {loading ? 'Loading releases…' : `${releases.length} release${releases.length === 1 ? '' : 's'} · ${updates} update${updates === 1 ? '' : 's'} available`}
          </p>
        </div>
      </div>

      {error && <p style={{ color: 'var(--cds-support-error)' }}>{error}</p>}
      {repoMappingsMissing && (
        <InlineNotification
          kind="info"
          lowContrast
          title="Chart repositories are not configured"
          subtitle="Add Helm repository mappings in Settings, or set HELM_REPOSITORIES, to check latest chart versions."
          style={{ marginBottom: '1rem' }}
        />
      )}
      {repos.length > 0 && (
        <Tile className="ir-card ir-helm-repos">
          <div>
            <strong>Chart repositories</strong>
            <span className="ir-muted">Latest checks fetch each repository index when this page loads.</span>
          </div>
          <div className="ir-repo-list">
            {repos.map((repo) => (
              <Tag key={`${repo.name}-${repo.url}`} type="cyan" size="sm">
                {repo.name}
              </Tag>
            ))}
          </div>
        </Tile>
      )}

      {loading ? (
        <Tile className="ir-card">
          <SkeletonText />
          <SkeletonText />
          <SkeletonText width="55%" />
        </Tile>
      ) : releases.length === 0 ? (
        <Tile className="ir-card">
          No Helm releases found. The service account may need permission to list Helm release secrets.
        </Tile>
      ) : (
        <div className="ir-table-shell">
          <table className="ir-data-table ir-helm-table">
            <thead>
              <tr>
                <th>Status</th>
                <th>Release</th>
                <th>Namespace</th>
                <th>Chart</th>
                <th>Version</th>
                <th>App</th>
                <th>Images</th>
                <th>Updated</th>
              </tr>
            </thead>
            <tbody>
              {releases.map((release) => (
                <tr key={release.id}>
                  <td>{statusTag(release)}</td>
                  <td>
                    <strong>{release.name}</strong>
                    <span className="ir-muted">rev {release.revision || '—'} · {release.status || 'unknown'}</span>
                  </td>
                  <td>{release.namespace}</td>
                  <td>
                    <strong>{release.chartName || '—'}</strong>
                    {release.repositoryUrl && (
                      <Link href={release.repositoryUrl} target="_blank" rel="noreferrer">
                        {release.repositoryName || release.repositoryUrl}
                      </Link>
                    )}
                  </td>
                  <td>
                    <span>{release.chartVersion || '—'}</span>
                    {release.latestChartVersion && release.latestChartVersion !== release.chartVersion && (
                      <span className="ir-update-pill">↑ {release.latestChartVersion}</span>
                    )}
                    {release.error && <span className="ir-muted">{release.error}</span>}
                  </td>
                  <td>
                    <span>{release.appVersion || '—'}</span>
                    {release.latestAppVersion && release.latestAppVersion !== release.appVersion && (
                      <span className="ir-muted">latest {release.latestAppVersion}</span>
                    )}
                  </td>
                  <td>{release.managedImages}</td>
                  <td title={formatTime(release.updated)}>
                    {relativeTime(release.updated)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
