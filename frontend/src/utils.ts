// Utility: format/truncate a sha256 digest.
export function formatDigest(digest: string, short = true): string {
  if (!digest) return '—'
  if (!short) return digest
  // sha256:abcdef... → sha256:abcdef (12 chars of hex)
  const match = digest.match(/^(sha256:)([0-9a-f]+)/)
  if (!match) return digest
  return `${match[1]}${match[2].slice(0, 12)}…`
}

// Utility: format ISO timestamp to locale string.
export function formatTime(iso: string | null | undefined): string {
  if (!iso) return 'Never'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

// Utility: relative time.
export function relativeTime(iso: string | null | undefined): string {
  if (!iso) return 'Never'
  const diff = Date.now() - new Date(iso).getTime()
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s ago`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return `${Math.floor(hrs / 24)}d ago`
}
