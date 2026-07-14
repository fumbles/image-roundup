import type { Status } from '../types'

interface StatusBadgeProps {
  status: Status
}

const config: Record<Status, { label: string; color: string }> = {
  up_to_date: { label: 'Up to date', color: '#24a148' },
  update_available: { label: 'Update available', color: '#0f62fe' },
  unknown: { label: 'Unknown', color: '#6f6f6f' },
  check_failed: { label: 'Check failed', color: '#da1e28' },
}

export default function StatusBadge({ status }: StatusBadgeProps) {
  const { label, color } = config[status] ?? config.unknown
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
      <span
        className="ir-status-dot"
        style={{ background: color }}
        aria-hidden="true"
      />
      {label}
    </span>
  )
}
