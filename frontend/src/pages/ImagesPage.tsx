import { useEffect, useState, useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  DataTable,
  TableContainer,
  Table,
  TableHead,
  TableRow,
  TableHeader,
  TableBody,
  TableCell,
  TableExpandRow,
  TableExpandedRow,
  TableExpandHeader,
  TableToolbar,
  TableToolbarContent,
  TableToolbarSearch,
  Pagination,
  Button,
  Select,
  SelectItem,
  SkeletonText,
  InlineNotification,
  Tag,
} from '@carbon/react'
import { Renew } from '@carbon/icons-react'
import { getImages, triggerScan } from '../api'
import type { ImageRecord, ImagesQuery, Status } from '../types'
import StatusBadge from '../components/StatusBadge'
import ImageDetail from '../components/ImageDetail'
import ScanIndicator from '../components/ScanIndicator'
import { useScanStatus } from '../hooks/useScanStatus'
import { formatDigest } from '../utils'

const PAGE_SIZE = 25
type StatusFilter = Status | 'unknown_failed'
const statusFilters = new Set<StatusFilter>([
  'update_available',
  'check_failed',
  'unknown',
  'unknown_failed',
  'up_to_date',
])

const headers = [
  { key: 'status', header: 'Status' },
  { key: 'configuredImage', header: 'Image' },
  { key: 'tag', header: 'Tag' },
  { key: 'runningDigest', header: 'Running digest' },
  { key: 'registryDigest', header: 'Registry digest' },
  { key: 'namespace', header: 'Namespace' },
  { key: 'workloadName', header: 'Workload' },
  { key: 'containerName', header: 'Container' },
  { key: 'lastChecked', header: 'Last checked' },
]

export default function ImagesPage() {
  const [records, setRecords] = useState<ImageRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [searchParams, setSearchParams] = useSearchParams()

  const { scanStatus, refetchScan } = useScanStatus()

  const query = useMemo(() => {
    const status = searchParams.get('status')
    return {
      namespace: searchParams.get('namespace') || undefined,
      registry: searchParams.get('registry') || undefined,
      kind: searchParams.get('kind') || undefined,
      status: status && statusFilters.has(status as StatusFilter)
        ? status as StatusFilter
        : undefined,
    }
  }, [searchParams])

  const apiQuery = useMemo<ImagesQuery>(() => ({
    namespace: query.namespace,
    registry: query.registry,
    kind: query.kind,
    status: query.status === 'unknown_failed' ? undefined : query.status,
  }), [query])

  const setFilter = (key: keyof typeof query, value?: string) => {
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
      if (value) {
        next.set(key, value)
      } else {
        next.delete(key)
      }
      return next
    })
  }

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getImages(apiQuery)
      setRecords(data)
      setPage(1)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load images')
    } finally {
      setLoading(false)
    }
  }, [apiQuery])

  useEffect(() => { load() }, [load])
  useEffect(() => { setPage(1) }, [search])

  // Reload table data once a running scan finishes
  const wasRunning = scanStatus?.running
  useEffect(() => {
    if (wasRunning === false) load()
  }, [wasRunning]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleRefresh = async () => {
    try {
      await triggerScan()
      await refetchScan()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to start scan')
    }
  }

  const handleNamespaceRefresh = async (namespace: string) => {
    try {
      await triggerScan({ namespace })
      await refetchScan()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to start namespace scan')
    }
  }

  const handleWorkloadRefresh = async (record: ImageRecord) => {
    try {
      await triggerScan({
        namespace: record.namespace,
        workloadKind: record.workloadKind,
        workloadName: record.workloadName,
      })
      await refetchScan()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to start workload scan')
    }
  }

  const filteredRecords = useMemo(() => {
    const term = search.trim().toLowerCase()
    const statusFiltered = query.status === 'unknown_failed'
      ? records.filter((r) => r.status === 'unknown' || r.status === 'check_failed')
      : records
    if (!term) return statusFiltered

    return statusFiltered.filter((r) => {
      const haystack = [
        r.configuredImage,
        r.registry,
        r.repository,
        r.tag,
        r.runningDigest,
        r.registryDigest,
        r.indexDigest,
        r.namespace,
        r.workloadKind,
        r.workloadName,
        r.containerName,
        r.status,
        ...(r.podNames ?? []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()

      return haystack.includes(term)
    })
  }, [records, search, query.status])

  const paged = filteredRecords.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  const rows = paged.map((r) => ({
    id: r.id,
    status: r.status,
    configuredImage: r.configuredImage,
    tag: r.tag || '—',
    runningDigest: formatDigest(r.runningDigest),
    registryDigest: formatDigest(r.registryDigest),
    namespace: r.namespace,
    workloadName: `${r.workloadKind} / ${r.workloadName}`,
    containerName: r.containerName,
    lastChecked: r.lastChecked ? new Date(r.lastChecked).toLocaleString() : '—',
    // _latestTag is a carry-through used when rendering the tag cell
    _latestTag: r.latestTag,
    _record: r,
  }))

  const namespaces = Array.from(new Set(records.map((r) => r.namespace))).sort()
  const registries = Array.from(new Set(records.map((r) => r.registry))).sort()
  const kinds = Array.from(new Set(records.map((r) => r.workloadKind))).sort()

  const statusOptions: { value: StatusFilter | ''; label: string }[] = [
    { value: '', label: 'All statuses' },
    { value: 'update_available', label: 'Update available' },
    { value: 'unknown_failed', label: 'Unknown / failed' },
    { value: 'check_failed', label: 'Check failed' },
    { value: 'unknown', label: 'Unknown' },
    { value: 'up_to_date', label: 'Up to date' },
  ]

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <div>
          <h1 style={{ fontSize: '1.5rem', fontWeight: 600, margin: 0 }}>Images</h1>
          <div style={{ marginTop: 4 }}>
            <ScanIndicator scanStatus={scanStatus} />
          </div>
        </div>
        <Button
          kind="secondary"
          size="sm"
          renderIcon={Renew}
          onClick={handleRefresh}
          disabled={scanStatus?.running}
        >
          Refresh
        </Button>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
        <Select
          id="ns-filter"
          labelText="Namespace"
          size="sm"
          value={query.namespace ?? ''}
          onChange={(e) => setFilter('namespace', e.target.value || undefined)}
          style={{ minWidth: 180 }}
        >
          <SelectItem value="" text="All namespaces" />
          {namespaces.map((ns) => <SelectItem key={ns} value={ns} text={ns} />)}
        </Select>

        <Select
          id="reg-filter"
          labelText="Registry"
          size="sm"
          value={query.registry ?? ''}
          onChange={(e) => setFilter('registry', e.target.value || undefined)}
          style={{ minWidth: 180 }}
        >
          <SelectItem value="" text="All registries" />
          {registries.map((r) => <SelectItem key={r} value={r} text={r} />)}
        </Select>

        <Select
          id="kind-filter"
          labelText="Workload kind"
          size="sm"
          value={query.kind ?? ''}
          onChange={(e) => setFilter('kind', e.target.value || undefined)}
          style={{ minWidth: 180 }}
        >
          <SelectItem value="" text="All kinds" />
          {kinds.map((k) => <SelectItem key={k} value={k} text={k} />)}
        </Select>

        <Select
          id="status-filter"
          labelText="Status"
          size="sm"
          value={query.status ?? ''}
          onChange={(e) => setFilter('status', e.target.value || undefined)}
          style={{ minWidth: 180 }}
        >
          {statusOptions.map((o) => <SelectItem key={o.value} value={o.value} text={o.label} />)}
        </Select>

        {query.namespace && (
          <Button
            kind="tertiary"
            size="sm"
            renderIcon={Renew}
            disabled={scanStatus?.running}
            onClick={() => handleNamespaceRefresh(query.namespace as string)}
            style={{ alignSelf: 'flex-end' }}
          >
            Refresh namespace
          </Button>
        )}
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

      {loading ? (
        <div style={{ padding: '2rem 0' }}>
          {[...Array(8)].map((_, i) => (
            <div key={i} style={{ marginBottom: 12 }}><SkeletonText /></div>
          ))}
        </div>
      ) : (
        <DataTable rows={rows} headers={headers}>
          {({
            rows: tableRows,
            headers: tableHeaders,
            getTableProps,
            getHeaderProps,
            getRowProps,
            getToolbarProps,
          }: any) => (
            <TableContainer>
              <TableToolbar {...getToolbarProps()}>
                <TableToolbarContent>
                  <TableToolbarSearch
                    persistent
                    placeholder="Search images, namespaces, workloads…"
                    value={search}
                    onChange={(_event: unknown, value?: string) =>
                      setSearch(value ?? '')
                    }
                  />
                </TableToolbarContent>
              </TableToolbar>
              <Table {...getTableProps()} size="sm">
                <TableHead>
                  <TableRow>
                    <TableExpandHeader />
                    {tableHeaders.map((header: any) => (
                      <TableHeader {...getHeaderProps({ header })} key={header.key}>
                        {header.header}
                      </TableHeader>
                    ))}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {tableRows.map((row: any) => {
                    const original = paged.find((r) => r.id === row.id)
                    return (
                      <>
                        <TableExpandRow {...getRowProps({ row })} key={row.id}>
                          {row.cells.map((cell: any) => (
                            <TableCell key={cell.id}>
                              {cell.info.header === 'status' && original ? (
                                <StatusBadge status={original.status} />
                              ) : cell.info.header === 'tag' && original?.latestTag ? (
                                <span style={{ display: 'flex', alignItems: 'center', gap: 4, flexWrap: 'wrap' }}>
                                  {cell.value}
                                  <Tag type="warm-gray" size="sm" title={`Latest: ${original.latestTag}`}>
                                    ↑ {original.latestTag}
                                  </Tag>
                                </span>
                              ) : (
                                cell.value
                              )}
                            </TableCell>
                          ))}
                        </TableExpandRow>
                        <TableExpandedRow colSpan={tableHeaders.length + 1} key={`${row.id}-exp`}>
                          {original && (
                            <ImageDetail
                              record={original}
                              shortDigests={true}
                              scanDisabled={scanStatus?.running}
                              onRefreshNamespace={handleNamespaceRefresh}
                              onRefreshWorkload={handleWorkloadRefresh}
                            />
                          )}
                        </TableExpandedRow>
                      </>
                    )
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </DataTable>
      )}

      {!loading && filteredRecords.length > PAGE_SIZE && (
        <Pagination
          totalItems={filteredRecords.length}
          pageSize={PAGE_SIZE}
          page={page}
          pageSizes={[25, 50, 100]}
          onChange={({ page: p }: { page: number }) => setPage(p)}
          style={{ marginTop: '1rem' }}
        />
      )}

      {!loading && filteredRecords.length === 0 && (
        <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--cds-text-secondary)' }}>
          No images found. {search || query.namespace || query.status ? 'Try clearing filters.' : 'Run a scan to discover images.'}
        </div>
      )}
    </div>
  )
}
