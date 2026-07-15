import type { Status } from '../types'

interface StatusBadgeProps {
  status: Status
}

const config: Record<Status, { label: string }> = {
  up_to_date: { label: 'Up to date' },
  update_available: { label: 'Update available' },
  unknown: { label: 'Unknown' },
  check_failed: { label: 'Check failed' },
}

export default function StatusBadge({ status }: StatusBadgeProps) {
  const { label } = config[status] ?? config.unknown
  return (
    <span className={`ir-status-badge ir-status-${status}`}>
      <span
        className="ir-status-dot"
        style={{ background: 'currentColor' }}
        aria-hidden="true"
      />
      {label}
    </span>
  )
}
