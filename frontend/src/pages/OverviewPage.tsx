import { useEffect, useState } from 'react'
import {
  Tile,
  SkeletonText,
  InlineNotification,
  Button,
} from '@carbon/react'
import { Renew } from '@carbon/icons-react'
import { getSummary, triggerScan } from '../api'
import type { Summary, ImageRecord } from '../types'
import { getImages } from '../api'
import StatusBadge from '../components/StatusBadge'
import ScanIndicator from '../components/ScanIndicator'
import { useScanStatus } from '../hooks/useScanStatus'

export default function OverviewPage() {
  const [summary, setSummary] = useState<Summary | null>(null)
  const [attention, setAttention] = useState<ImageRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const { scanStatus, refetchScan } = useScanStatus()

  const load = async () => {
    try {
      setLoading(true)
      const [s, imgs] = await Promise.all([
        getSummary(),
        getImages({ status: 'update_available' }),
      ])
      setSummary(s)
      setAttention(imgs.slice(0, 10))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load data')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  // Reload page data once a running scan finishes
  const wasRunning = scanStatus?.running
  useEffect(() => {
    if (wasRunning === false) load()
  }, [wasRunning]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleScan = async () => {
    try {
      await triggerScan()
      await refetchScan()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Scan trigger failed')
    }
  }

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <div>
          <h1 style={{ fontSize: '1.5rem', fontWeight: 600, margin: 0 }}>Overview</h1>
          <div style={{ marginTop: 4 }}>
            <ScanIndicator scanStatus={scanStatus} />
          </div>
        </div>
        <Button
          kind="secondary"
          size="sm"
          renderIcon={Renew}
          onClick={handleScan}
          disabled={scanStatus?.running}
        >
          Refresh
        </Button>
      </div>

      {error && (
        <InlineNotification
          kind="error"
          title="Error"
          subtitle={error}
          style={{ marginBottom: '1rem' }}
          onCloseButtonClick={() => setError(null)}
        />
      )}

      {/* Summary tiles */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', gap: '1rem', marginBottom: '2rem' }}>
        <SummaryTile label="Total images" value={summary?.totalImages} loading={loading} />
        <SummaryTile label="Up to date" value={summary?.upToDate} loading={loading} accent="#24a148" />
        <SummaryTile label="Updates available" value={summary?.updatesAvailable} loading={loading} accent="#0f62fe" />
        <SummaryTile label="Unknown / failed" value={summary ? summary.unknown + summary.checkFailed : undefined} loading={loading} accent="#6f6f6f" />
        <SummaryTile label="Registries" value={summary?.uniqueRegistries} loading={loading} />
      </div>

      {/* Attention list */}
      {!loading && attention.length > 0 && (
        <>
          <h2 style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '0.75rem' }}>
            Images needing attention
          </h2>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--cds-border-subtle)' }}>
                <th style={{ textAlign: 'left', paddingBottom: 8, fontWeight: 600 }}>Image</th>
                <th style={{ textAlign: 'left', paddingBottom: 8, fontWeight: 600 }}>Namespace</th>
                <th style={{ textAlign: 'left', paddingBottom: 8, fontWeight: 600 }}>Status</th>
              </tr>
            </thead>
            <tbody>
              {attention.map((img) => (
                <tr key={img.id} style={{ borderBottom: '1px solid var(--cds-border-subtle-00)' }}>
                  <td style={{ padding: '8px 0', wordBreak: 'break-all' }}>{img.configuredImage}</td>
                  <td style={{ padding: '8px 0' }}>{img.namespace}</td>
                  <td style={{ padding: '8px 0' }}><StatusBadge status={img.status} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      {!loading && attention.length === 0 && summary && summary.updatesAvailable === 0 && (
        <Tile style={{ textAlign: 'center', color: 'var(--cds-text-secondary)', padding: '2rem' }}>
          All images are up to date.
        </Tile>
      )}
    </div>
  )
}

interface TileProps {
  label: string
  value: number | undefined
  loading: boolean
  accent?: string
}

function SummaryTile({ label, value, loading, accent }: TileProps) {
  return (
    <Tile style={{ padding: '1.25rem 1rem' }}>
      {loading ? (
        <SkeletonText width="60%" />
      ) : (
        <span style={{ fontSize: '2rem', fontWeight: 700, color: accent ?? 'var(--cds-text-primary)' }}>
          {value ?? 0}
        </span>
      )}
      <p style={{ fontSize: 13, color: 'var(--cds-text-secondary)', margin: '4px 0 0' }}>{label}</p>
    </Tile>
  )
}
