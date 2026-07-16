import type { ImageRecord } from '../types'
import { Button, Link } from '@carbon/react'
import { Launch, Renew } from '@carbon/icons-react'
import StatusBadge from './StatusBadge'
import CopyButton from './CopyButton'
import { formatDigest, formatTime, registryTagURL, relativeTime } from '../utils'

interface ImageDetailProps {
  record: ImageRecord
  shortDigests: boolean
  scanDisabled?: boolean
  onRefreshNamespace?: (namespace: string) => void
  onRefreshWorkload?: (record: ImageRecord) => void
}

export default function ImageDetail({
  record,
  shortDigests,
  scanDisabled = false,
  onRefreshNamespace,
  onRefreshWorkload,
}: ImageDetailProps) {
  const configuredTagURL = record.tag
    ? registryTagURL(record.registry, record.repository, record.tag)
    : null
  const latestTagURL = record.latestTag
    ? registryTagURL(record.registry, record.repository, record.latestTag)
    : null
  const managementText = record.management
    ? record.management.tool === 'Helm'
      ? [
          'Helm',
          record.management.helmReleaseName ? `release ${record.management.helmReleaseName}` : null,
          record.management.helmReleaseNamespace ? `namespace ${record.management.helmReleaseNamespace}` : null,
        ].filter(Boolean).join(' · ')
      : record.management.tool
    : null

  const plainLang = (): string => {
    const isMultiArch = !!record.indexDigest
    switch (record.status) {
      case 'up_to_date':
        return isMultiArch
          ? `The running container's linux/amd64 digest matches the current registry digest for tag "${record.tag}". No update is available.`
          : `The running container matches the current digest for tag "${record.tag}". No update is available.`
      case 'update_available':
        return isMultiArch
          ? `The workload is configured to use "${record.tag}". The running linux/amd64 digest differs from what the registry currently serves for that tag — a newer image is available.`
          : `The workload is configured to use "${record.tag}", but the running digest differs from the digest currently assigned to that tag.`
      case 'check_failed':
        return `Could not retrieve the current digest from ${record.registry}. Check the registry error below.`
      case 'unknown':
      default:
        return `Not enough information is available to determine whether this image is current.`
    }
  }

  const row = (label: string, value: React.ReactNode) => (
    <tr>
      <th style={{ width: 180, fontWeight: 500, paddingRight: 16, paddingBottom: 8, verticalAlign: 'top' }}>
        {label}
      </th>
      <td style={{ paddingBottom: 8, wordBreak: 'break-all' }}>{value}</td>
    </tr>
  )

  return (
    <div style={{ padding: '1rem 1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem', alignItems: 'flex-start', marginBottom: '1rem' }}>
        <p style={{ margin: 0, color: 'var(--cds-text-secondary)', maxWidth: '72rem' }}>{plainLang()}</p>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
          <Button
            kind="tertiary"
            size="sm"
            renderIcon={Renew}
            disabled={scanDisabled}
            onClick={() => onRefreshNamespace?.(record.namespace)}
          >
            Refresh namespace
          </Button>
          <Button
            kind="secondary"
            size="sm"
            renderIcon={Renew}
            disabled={scanDisabled}
            onClick={() => onRefreshWorkload?.(record)}
          >
            Refresh workload
          </Button>
        </div>
      </div>

      <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 14 }}>
        <tbody>
          {row('Status', <StatusBadge status={record.status} />)}
          {row('Namespace', record.namespace)}
          {row('Workload', `${record.workloadKind} / ${record.workloadName}`)}
          {managementText && row('Managed by',
            <span>
              {managementText}
              {record.management?.tool === 'Helm' && (
                <span style={{ color: 'var(--cds-text-secondary)' }}>
                  {' '}· upgrade with Helm chart/release, not by editing the workload image directly
                </span>
              )}
            </span>
          )}
          {row('Container', record.containerName)}
          {row('Configured image',
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <code className="ir-digest">{record.configuredImage}</code>
              <CopyButton text={record.configuredImage} label="Copy image ref" />
            </span>
          )}
          {row('Configured tag',
            record.tag
              ? <span style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <span>{record.tag}</span>
                  <CopyButton text={record.tag} label="Copy tag" />
                  {configuredTagURL && (
                    <Link
                      href={configuredTagURL}
                      target="_blank"
                      rel="noreferrer"
                      renderIcon={Launch}
                      size="sm"
                    >
                      Inspect tag
                    </Link>
                  )}
                </span>
              : '—'
          )}
          {row('Running digest',
            record.runningDigest
              ? <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  <code className="ir-digest">{formatDigest(record.runningDigest, shortDigests)}</code>
                  <CopyButton text={record.runningDigest} label="Copy digest" />
                </span>
              : '—'
          )}
          {row('Registry digest (platform)',
            record.registryDigest
              ? <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  <code className="ir-digest">{formatDigest(record.registryDigest, shortDigests)}</code>
                  <CopyButton text={record.registryDigest} label="Copy digest" />
                </span>
              : '—'
          )}
          {record.indexDigest && row('Registry index digest',
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <code className="ir-digest">{formatDigest(record.indexDigest, shortDigests)}</code>
              <CopyButton text={record.indexDigest} label="Copy index digest" />
            </span>
          )}
          {record.latestTag && row('Latest available tag',
            <span style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <code className="ir-digest" style={{ color: 'var(--cds-support-warning-inverse, #f1c21b)' }}>
                {record.latestTag}
              </code>
              <CopyButton text={record.latestTag} label="Copy tag" />
              {latestTagURL && (
                <Link
                  href={latestTagURL}
                  target="_blank"
                  rel="noreferrer"
                  renderIcon={Launch}
                  size="sm"
                >
                  Inspect tag
                </Link>
              )}
              {record.latestTagDigest && (
                <>
                  <code className="ir-digest" style={{ color: 'var(--cds-text-secondary)' }}>
                    {formatDigest(record.latestTagDigest, shortDigests)}
                  </code>
                  <CopyButton text={record.latestTagDigest} label="Copy latest digest" />
                </>
              )}
            </span>
          )}
          {row('Platform', record.platform || '—')}
          {row('Registry', record.registry)}
          {row('Pods',
            record.podNames && record.podNames.length > 0
              ? record.podNames.join(', ')
              : '—'
          )}
          {row('Last checked',
            record.lastChecked
              ? `${formatTime(record.lastChecked)} (${relativeTime(record.lastChecked)})`
              : '—'
          )}
          {record.error && row('Last error',
            <span style={{ color: 'var(--cds-support-error)' }}>{record.error}</span>
          )}
        </tbody>
      </table>
    </div>
  )
}
