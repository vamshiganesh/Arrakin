import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { adminApi, reconciliationApi, settlementApi } from '../api'
import type { ReconciliationSnapshot, SettlementJob } from '../api/types'
import { ApiClientError } from '../api/client'
import { EmptyState } from '../components/EmptyState'
import { ErrorState } from '../components/ErrorState'
import { LoadingState } from '../components/LoadingState'
import { StatusBadge } from '../components/StatusBadge'
import { formatDateTime, formatMoney } from '../lib/format'

export function DashboardPage() {
  const [recon, setRecon] = useState<ReconciliationSnapshot | null>(null)
  const [recentJobs, setRecentJobs] = useState<SettlementJob[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [tickBusy, setTickBusy] = useState(false)
  const [tickResult, setTickResult] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [latest, jobs] = await Promise.all([
        reconciliationApi.latest().catch((e: unknown) => {
          if (e instanceof ApiClientError && e.status === 404) return null
          throw e
        }),
        settlementApi.listJobs({ limit: 8 }),
      ])
      setRecon(latest)
      setRecentJobs(jobs.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load overview')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const id = window.setInterval(load, 10000)
    return () => window.clearInterval(id)
  }, [load])

  async function handleTick() {
    setTickBusy(true)
    setTickResult(null)
    try {
      const res = await adminApi.schedulerTick()
      setTickResult(`Scheduler scan complete. ${res.jobs_created} new job(s) enqueued.`)
      await load()
    } catch (e) {
      setTickResult(e instanceof Error ? e.message : 'Scheduler tick failed')
    } finally {
      setTickBusy(false)
    }
  }

  if (loading && !recentJobs.length) {
    return <LoadingState message="Loading operations overview…" />
  }

  if (error && !recentJobs.length) {
    return <ErrorState message={error} onRetry={load} />
  }

  const byStatus = recon?.summary.by_status ?? {}

  return (
    <>
      <header className="page-header">
        <div>
          <h2>Operations overview</h2>
          <p>Settlement pipeline health and recent activity</p>
        </div>
        <div className="btn-group">
          <button type="button" className="btn btn-primary" disabled={tickBusy} onClick={handleTick}>
            {tickBusy ? 'Running scan…' : 'Run scheduler scan'}
          </button>
        </div>
      </header>

      {tickResult && (
        <div className="panel" style={{ marginBottom: '1rem', padding: '0.85rem 1.25rem' }}>
          <p style={{ margin: 0, fontSize: 13 }}>{tickResult}</p>
        </div>
      )}

      <div className="stat-grid">
        <div className="stat-card">
          <div className="label">Succeeded volume</div>
          <div className="value">
            {recon ? formatMoney(recon.summary.succeeded_total_cents) : 'n/a'}
          </div>
          <div className="sub">{byStatus.succeeded ?? 0} completed jobs</div>
        </div>
        <div className="stat-card">
          <div className="label">Discrepancy</div>
          <div className="value" style={{ color: recon && recon.summary.discrepancy_cents > 0 ? 'var(--status-dead)' : undefined }}>
            {recon ? formatMoney(recon.summary.discrepancy_cents) : 'n/a'}
          </div>
          <div className="sub">Expected minus settled</div>
        </div>
        <div className="stat-card">
          <div className="label">Pending</div>
          <div className="value">{byStatus.pending ?? 0}</div>
        </div>
        <div className="stat-card">
          <div className="label">Dead letter</div>
          <div className="value">{byStatus.dead_letter ?? 0}</div>
        </div>
      </div>

      <section className="panel">
        <div className="panel-header">
          <h3>Recent settlement jobs</h3>
          <Link to="/jobs">View all</Link>
        </div>
        <div className="panel-body flush">
          {recentJobs.length === 0 ? (
            <EmptyState title="No jobs yet" description="Run a scheduler scan to enqueue due maturities." />
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Job</th>
                  <th>Status</th>
                  <th>Net payout</th>
                  <th>Updated</th>
                </tr>
              </thead>
              <tbody>
                {recentJobs.map((job) => (
                  <tr key={job.id}>
                    <td>
                      <Link to={`/jobs/${job.id}`} className="mono">
                        {job.id.slice(0, 8)}…
                      </Link>
                    </td>
                    <td>
                      <StatusBadge status={job.status} />
                    </td>
                    <td>{formatMoney(job.net_payout_cents)}</td>
                    <td>{formatDateTime(job.updated_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>
    </>
  )
}
