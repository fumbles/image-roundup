// Shared TypeScript types mirroring the Go models.

export type Status = 'up_to_date' | 'update_available' | 'unknown' | 'check_failed'

export interface ImageRecord {
  id: string
  namespace: string
  workloadKind: string
  workloadName: string
  containerName: string
  configuredImage: string
  registry: string
  repository: string
  tag: string
  runningDigest: string
  registryDigest: string
  indexDigest?: string
  latestTag?: string
  latestTagDigest?: string
  platform: string
  status: Status
  podNames: string[]
  lastChecked: string | null
  error?: string
}

export interface Summary {
  totalImages: number
  upToDate: number
  updatesAvailable: number
  unknown: number
  checkFailed: number
  uniqueRegistries: number
  lastScan: string | null
}

export interface ScanStatus {
  running: boolean
  lastScan: string | null
  imageCount: number
  errors: string[]
}

export interface ScanRequest {
  namespace?: string
  workloadKind?: string
  workloadName?: string
}

export interface RegistryInfo {
  hostname: string
  imageCount: number
  reachable: boolean | null
  authPresent: boolean
  lastError?: string
}

export interface Settings {
  scanIntervalSeconds: number
  includedNamespaces: string[]
  excludedNamespaces: string[]
  includeCompletedPods: boolean
  excludeInternalRegistry: boolean
  registryTimeoutSeconds: number
  theme: 'system' | 'light' | 'dark'
  shortDigests: boolean
}

export interface ImagesQuery {
  search?: string
  namespace?: string
  registry?: string
  kind?: string
  status?: Status
}
