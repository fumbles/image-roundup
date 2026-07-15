import { useEffect, useRef, useState } from 'react'
import { getScan } from '../api'
import type { ScanStatus } from '../types'

const POLL_INTERVAL_MS = 3000

/**
 * Polls GET /api/v1/scan every 3 seconds while a scan is running,
 * then drops back to a slower check after it finishes.
 * Returns the latest ScanStatus and a manual refetch function.
 */
export function useScanStatus() {
  const [status, setStatus] = useState<ScanStatus | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const fetch = async () => {
    try {
      const s = await getScan()
      setStatus(s)
      return s
    } catch {
      return null
    }
  }

  const schedule = (wasRunning: boolean) => {
    // Poll quickly while running, slow down once idle
    const delay = wasRunning ? POLL_INTERVAL_MS : 30_000
    timerRef.current = setTimeout(async () => {
      const s = await fetch()
      schedule(s?.running ?? false)
    }, delay)
  }

  useEffect(() => {
    fetch().then((s) => schedule(s?.running ?? false))
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return { scanStatus: status, refetchScan: fetch }
}
