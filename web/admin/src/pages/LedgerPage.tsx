import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ledgerApi } from '../api'
import type { LedgerEntry } from '../api/types'
import { EmptyState } from '../components/EmptyState'
import { ErrorState } from '../components/ErrorState'
import { LoadingState } from '../components/LoadingState'
import { formatDateTime, formatMoney } from '../lib/format'

export function LedgerPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const jobFilter = searchParams.get('job') ?? ''
  const [accountFilter, setAccountFilter] = useState('')
  const [entries, setEntries] = useState<LedgerEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await ledgerApi.listEntries({
        settlement_job_id: jobFilter || undefined,
        account_code: accountFilter || undefined,
        limit: 50,
      })
      setEntries(res.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load ledger')
    } finally {
      setLoading(false)
    }
  }, [jobFilter, accountFilter])

  useEffect(() => {
    load()
  }, [load])

  function applyJobFilter(value: string) {
    if (value) {
      setSearchParams({ job: value })
    } else {
      setSearchParams({})
    }
  }

  return (
    <>
      <header className="page-header">
        <div>
          <h2>Ledger entries</h2>
          <p>Immutable double entry postings linked to settlements</p>
        </div>
      </header>

      <section className="panel">
        <div className="filter-bar">
          <label htmlFor="job-filter">Settlement job</label>
          <input
            id="job-filter"
            type="text"
            placeholder="UUID"
            value={jobFilter}
            onChange={(e) => applyJobFilter(e.target.value)}
            style={{ minWidth: 280 }}
          />
          <label htmlFor="account-filter">Account code</label>
          <input
            id="account-filter"
            type="text"
            placeholder="e.g. PLATFORM_FEE_REVENUE"
            value={accountFilter}
            onChange={(e) => setAccountFilter(e.target.value)}
          />
          <button type="button" className="btn btn-secondary" onClick={load}>
            Apply filters
          </button>
        </div>

        {loading && entries.length === 0 && <LoadingState />}
        {error && entries.length === 0 && <ErrorState message={error} onRetry={load} />}
        {!loading && !error && entries.length === 0 && (
          <EmptyState title="No ledger lines" description="Successful settlements produce four balanced entries per job." />
        )}

        {entries.length > 0 && (
          <table>
            <thead>
              <tr>
                <th>Posted</th>
                <th>Side</th>
                <th>Amount</th>
                <th>Description</th>
                <th>Job</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => (
                <tr key={e.id}>
                  <td>{formatDateTime(e.posted_at)}</td>
                  <td className={e.side === 'D' ? 'amount-debit' : 'amount-credit'}>
                    {e.side === 'D' ? 'Debit' : 'Credit'}
                  </td>
                  <td>{formatMoney(e.amount_cents, e.currency)}</td>
                  <td>{e.description}</td>
                  <td>
                    <Link to={`/jobs/${e.settlement_job_id}`} className="mono">
                      {e.settlement_job_id.slice(0, 8)}…
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </>
  )
}
