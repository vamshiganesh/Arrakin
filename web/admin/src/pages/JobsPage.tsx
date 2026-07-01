import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { settlementApi } from '../api'
import type { JobStatus, SettlementJob } from '../api/types'
import { EmptyState } from '../components/EmptyState'
import { ErrorState } from '../components/ErrorState'
import { LoadingState } from '../components/LoadingState'
import { StatusBadge } from '../components/StatusBadge'
import { formatDateTime, formatMoney } from '../lib/format'

const statuses: { value: JobStatus | ''; label: string }[] = [
  { value: '', label: 'All statuses' },
  { value: 'pending', label: 'Pending' },
  { value: 'processing', label: 'Processing' },
  { value: 'succeeded', label: 'Succeeded' },
  { value: 'failed', label: 'Failed' },
  { value: 'dead_letter', label: 'Dead letter' },
]

export function JobsPage() {
  const [status, setStatus] = useState<JobStatus | ''>('')
  const [jobs, setJobs] = useState<SettlementJob[]>([])
  const [cursor, setCursor] = useState<string | undefined>()
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(
    async (nextCursor?: string, append = false) => {
      setLoading(true)
      setError(null)
      try {
        const res = await settlementApi.listJobs({
          status: status || undefined,
          limit: 25,
          cursor: nextCursor,
        })
        setJobs((prev) => (append ? [...prev, ...res.items] : res.items))
        setCursor(res.page.next_cursor)
        setHasMore(res.page.has_more)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load jobs')
      } finally {
        setLoading(false)
      }
    },
    [status],
  )

  useEffect(() => {
    load()
    const id = window.setInterval(() => load(), 10000)
    return () => window.clearInterval(id)
  }, [load])

  return (
    <>
      <header className="page-header">
        <div>
          <h2>Settlement jobs</h2>
          <p>Queue status, amounts, and payout lifecycle</p>
        </div>
      </header>

      <section className="panel">
        <div className="filter-bar">
          <label htmlFor="status-filter">Status</label>
          <select
            id="status-filter"
            value={status}
            onChange={(e) => setStatus(e.target.value as JobStatus | '')}
          >
            {statuses.map((s) => (
              <option key={s.label} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </div>

        {loading && jobs.length === 0 && <LoadingState />}
        {error && jobs.length === 0 && <ErrorState message={error} onRetry={() => load()} />}
        {!loading && !error && jobs.length === 0 && (
          <EmptyState title="No matching jobs" description="Adjust filters or enqueue maturities from the overview." />
        )}

        {jobs.length > 0 && (
          <>
            <table>
              <thead>
                <tr>
                  <th>Job ID</th>
                  <th>Status</th>
                  <th>Net payout</th>
                  <th>Retries</th>
                  <th>Created</th>
                  <th>Updated</th>
                </tr>
              </thead>
              <tbody>
                {jobs.map((job) => (
                  <tr key={job.id}>
                    <td>
                      <Link to={`/jobs/${job.id}`} className="mono">
                        {job.id.slice(0, 13)}…
                      </Link>
                    </td>
                    <td>
                      <StatusBadge status={job.status} />
                    </td>
                    <td>{formatMoney(job.net_payout_cents)}</td>
                    <td>
                      {job.retry_count} / {job.max_retries}
                    </td>
                    <td>{formatDateTime(job.created_at)}</td>
                    <td>{formatDateTime(job.updated_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {hasMore && cursor && (
              <div className="pager">
                <button
                  type="button"
                  className="btn btn-secondary"
                  disabled={loading}
                  onClick={() => load(cursor, true)}
                >
                  Load more
                </button>
              </div>
            )}
          </>
        )}
      </section>
    </>
  )
}
