import { useEffect, useState, useCallback } from 'react'
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
} from '@carbon/react'
import { Renew } from '@carbon/icons-react'
import { getImages, triggerScan } from '../api'
import type { ImageRecord, ImagesQuery, Status } from '../types'
import StatusBadge from '../components/StatusBadge'
import ImageDetail from '../components/ImageDetail'
import { formatDigest, relativeTime } from '../utils'

const PAGE_SIZE = 25

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
  const [query, setQuery] = useState<ImagesQuery>({})

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getImages(query)
      setRecords(data)
      setPage(1)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load images')
    } finally {
      setLoading(false)
    }
  }, [query])

  useEffect(() => { load() }, [load])

  const handleRefresh = async () => {
    try { await triggerScan() } catch { /* best-effort */ }
    setTimeout(load, 1500)
  }

  const paged = records.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

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
    lastChecked: relativeTime(r.lastChecked),
    _record: r,
  }))

  const namespaces = Array.from(new Set(records.map((r) => r.namespace))).sort()
  const registries = Array.from(new Set(records.map((r) => r.registry))).sort()
  const kinds = Array.from(new Set(records.map((r) => r.workloadKind))).sort()

  const statusOptions: { value: Status | ''; label: string }[] = [
    { value: '', label: 'All statuses' },
    { value: 'update_available', label: 'Update available' },
    { value: 'check_failed', label: 'Check failed' },
    { value: 'unknown', label: 'Unknown' },
    { value: 'up_to_date', label: 'Up to date' },
  ]

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.5rem', fontWeight: 600, margin: 0 }}>Images</h1>
        <Button kind="secondary" size="sm" renderIcon={Renew} onClick={handleRefresh}>
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
          onChange={(e) => setQuery((q) => ({ ...q, namespace: e.target.value || undefined }))}
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
          onChange={(e) => setQuery((q) => ({ ...q, registry: e.target.value || undefined }))}
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
          onChange={(e) => setQuery((q) => ({ ...q, kind: e.target.value || undefined }))}
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
          onChange={(e) => setQuery((q) => ({ ...q, status: (e.target.value as Status) || undefined }))}
          style={{ minWidth: 180 }}
        >
          {statusOptions.map((o) => <SelectItem key={o.value} value={o.value} text={o.label} />)}
        </Select>
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
                    value={query.search ?? ''}
                    onChange={(_event: unknown, value?: string) =>
                      setQuery((q) => ({ ...q, search: value || undefined }))
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
                              {cell.info.header === 'status' && original
                                ? <StatusBadge status={original.status} />
                                : cell.value}
                            </TableCell>
                          ))}
                        </TableExpandRow>
                        <TableExpandedRow colSpan={tableHeaders.length + 1} key={`${row.id}-exp`}>
                          {original && (
                            <ImageDetail record={original} shortDigests={true} />
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

      {!loading && records.length > PAGE_SIZE && (
        <Pagination
          totalItems={records.length}
          pageSize={PAGE_SIZE}
          page={page}
          pageSizes={[25, 50, 100]}
          onChange={({ page: p }: { page: number }) => setPage(p)}
          style={{ marginTop: '1rem' }}
        />
      )}

      {!loading && records.length === 0 && (
        <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--cds-text-secondary)' }}>
          No images found. {query.search || query.namespace || query.status ? 'Try clearing filters.' : 'Run a scan to discover images.'}
        </div>
      )}
    </div>
  )
}
