import { useEffect, useState } from 'react'
import { Tile, SkeletonText, Tag } from '@carbon/react'
import { getRegistries } from '../api'
import type { RegistryInfo } from '../types'

export default function RegistriesPage() {
  const [registries, setRegistries] = useState<RegistryInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getRegistries()
      .then(setRegistries)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load registries'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div style={{ padding: '1.5rem' }}>
      <h1 style={{ fontSize: '1.5rem', fontWeight: 600, marginBottom: '1.5rem' }}>Registries</h1>

      {error && <p style={{ color: 'var(--cds-support-error)' }}>{error}</p>}

      {loading ? (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '1rem' }}>
          {[...Array(4)].map((_, i) => (
            <Tile key={i} style={{ padding: '1.25rem' }}>
              <SkeletonText />
              <SkeletonText width="40%" />
            </Tile>
          ))}
        </div>
      ) : registries.length === 0 ? (
        <p style={{ color: 'var(--cds-text-secondary)' }}>No registries found. Run a scan first.</p>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '1rem' }}>
          {registries.map((reg) => (
            <Tile key={reg.hostname} style={{ padding: '1.25rem' }}>
              <p style={{ fontWeight: 600, marginBottom: 8 }}>{reg.hostname}</p>
              <p style={{ fontSize: 13, color: 'var(--cds-text-secondary)', marginBottom: 8 }}>
                {reg.imageCount} image{reg.imageCount !== 1 ? 's' : ''}
              </p>
              <div style={{ display: 'flex', gap: 8 }}>
                {reg.authPresent && <Tag type="blue" size="sm">Auth configured</Tag>}
                {reg.lastError
                  ? <Tag type="red" size="sm">Error</Tag>
                  : <Tag type="green" size="sm">OK</Tag>
                }
              </div>
              {reg.lastError && (
                <p style={{ fontSize: 12, color: 'var(--cds-support-error)', marginTop: 8 }}>
                  {reg.lastError}
                </p>
              )}
            </Tile>
          ))}
        </div>
      )}
    </div>
  )
}
