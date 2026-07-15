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

export function registryTagURL(registry: string, repository: string, tag: string): string | null {
  if (!registry || !repository || !tag) return null

  const encodedTag = encodeURIComponent(tag)
  const normalizedRegistry = registry.toLowerCase()

  if (normalizedRegistry === 'docker.io' || normalizedRegistry === 'index.docker.io') {
    if (repository.startsWith('library/')) {
      return `https://hub.docker.com/_/${encodeURIComponent(repository.slice('library/'.length))}/tags?name=${encodedTag}`
    }
    return `https://hub.docker.com/r/${repository}/tags?name=${encodedTag}`
  }

  if (normalizedRegistry === 'quay.io') {
    return `https://quay.io/repository/${repository}?tab=tags&tag=${encodedTag}`
  }

  if (normalizedRegistry === 'ghcr.io') {
    const parts = repository.split('/')
    if (parts.length >= 2) {
      const owner = encodeURIComponent(parts[0])
      const repo = encodeURIComponent(parts[1])
      const pkg = encodeURIComponent(parts.slice(1).join('/'))
      return `https://github.com/${owner}/${repo}/pkgs/container/${pkg}?tag=${encodedTag}`
    }
    return null
  }

  if (normalizedRegistry === 'lscr.io') {
    const parts = repository.split('/')
    if (parts[0] === 'linuxserver' && parts[1]) {
      return `https://docs.linuxserver.io/images/docker-${encodeURIComponent(parts[1])}/`
    }
    return null
  }

  if (normalizedRegistry === 'registry.access.redhat.com' || normalizedRegistry === 'registry.redhat.io') {
    return `https://catalog.redhat.com/search?searchType=containers&gs&q=${encodeURIComponent(repository)}`
  }

  return null
}
