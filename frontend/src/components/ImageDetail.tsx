import type { ImageRecord } from '../types'
import StatusBadge from './StatusBadge'
import CopyButton from './CopyButton'
import { formatDigest, formatTime, relativeTime } from '../utils'

interface ImageDetailProps {
  record: ImageRecord
  shortDigests: boolean
}

export default function ImageDetail({ record, shortDigests }: ImageDetailProps) {
  const plainLang = (): string => {
    switch (record.status) {
      case 'up_to_date':
        return `The running container matches the current digest for tag "${record.tag}". No update is available.`
      case 'update_available':
        return `The workload is configured to use "${record.tag}", but the running digest differs from the digest currently assigned to that tag.`
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
      <p style={{ marginBottom: '1rem', color: 'var(--cds-text-secondary)' }}>{plainLang()}</p>

      <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 14 }}>
        <tbody>
          {row('Status', <StatusBadge status={record.status} />)}
          {row('Namespace', record.namespace)}
          {row('Workload', `${record.workloadKind} / ${record.workloadName}`)}
          {row('Container', record.containerName)}
          {row('Configured image',
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <code className="ir-digest">{record.configuredImage}</code>
              <CopyButton text={record.configuredImage} label="Copy image ref" />
            </span>
          )}
          {row('Configured tag', record.tag || '—')}
          {row('Running digest',
            record.runningDigest
              ? <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  <code className="ir-digest">{formatDigest(record.runningDigest, shortDigests)}</code>
                  <CopyButton text={record.runningDigest} label="Copy digest" />
                </span>
              : '—'
          )}
          {row('Registry digest',
            record.registryDigest
              ? <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  <code className="ir-digest">{formatDigest(record.registryDigest, shortDigests)}</code>
                  <CopyButton text={record.registryDigest} label="Copy digest" />
                </span>
              : '—'
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
