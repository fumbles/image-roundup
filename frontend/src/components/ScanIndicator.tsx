import { InlineLoading } from '@carbon/react'
import type { ScanStatus } from '../types'
import { relativeTime } from '../utils'

interface ScanIndicatorProps {
  scanStatus: ScanStatus | null
}

/**
 * Shows a spinning InlineLoading while a scan is running,
 * otherwise shows the last-scan time quietly.
 */
export default function ScanIndicator({ scanStatus }: ScanIndicatorProps) {
  if (!scanStatus) return null

  if (scanStatus.running) {
    return (
      <InlineLoading
        description="Scanning…"
        status="active"
        style={{ fontSize: 13 }}
      />
    )
  }

  return (
    <span style={{ fontSize: 13, color: 'var(--cds-text-secondary)' }}>
      Last scan: {relativeTime(scanStatus.lastScan)}
    </span>
  )
}
