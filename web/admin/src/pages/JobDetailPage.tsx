import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { settlementApi } from '../api'
import type { PayoutAttempt, SettlementJob } from '../api/types'
import { EmptyState } from '../components/EmptyState'
import { ErrorState } from '../components/ErrorState'
import { LoadingState } from '../components/LoadingState'
import { StatusBadge } from '../components/StatusBadge'
import { formatDateTime, formatMoney } from '../lib/format'

export function JobDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [job, setJob] = useState<SettlementJob | null>(null)
  const [attempts, setAttempts] = useState<PayoutAttempt[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionMsg, setActionMsg] = useState<string | null>(null)
  const [actionBusy, setActionBusy] = useState(false)

  const load = useCallback(async () => {
    if (!id) return
    setLoading(true)
    setError(null)
    try {
      const [jobRes, attemptsRes] = await Promise.all([
        settlementApi.getJob(id),
        settlementApi.listAttempts(id),
      ])
      setJob(jobRes)
      setAttempts(attemptsRes.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load job')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    load()
    const timer = window.setInterval(load, 8000)
    return () => window.clearInterval(timer)
  }, [load])

  async function handleReplay() {
    if (!id) return
    setActionBusy(true)
    setActionMsg(null)
    try {
      const res = await settlementApi.replay(id)
      setActionMsg(`Job moved to ${res.job.status.replace(/_/g, ' ')}.`)
      await load()
    } catch (e) {
      setActionMsg(e instanceof Error ? e.message : 'Replay failed')
    } finally {
      setActionBusy(false)
    }
  }

  async function handleRequeue() {
    if (!id) return
    setActionBusy(true)
    setActionMsg(null)
    try {
      const res = await settlementApi.requeue(id)
      setActionMsg(`Job moved to ${res.job.status.replace(/_/g, ' ')}.`)
      await load()
    } catch (e) {
      setActionMsg(e instanceof Error ? e.message : 'Requeue failed')
    } finally {
      setActionBusy(false)
    }
  }

  if (!id) {
    return <ErrorState message="Missing job identifier" />
  }

  if (loading && !job) {
    return <LoadingState message="Loading job detail…" />
  }

  if (error && !job) {
    return <ErrorState message={error} onRetry={load} />
  }

  if (!job) {
    return <EmptyState title="Job not found" description="This settlement job does not exist." />
  }

  return (
    <>
      <header className="page-header">
        <div>
          <p style={{ margin: '0 0 0.35rem', fontSize: 13 }}>
            <Link to="/jobs">Settlement jobs</Link>
          </p>
          <h2>
            Job <span className="mono">{job.id.slice(0, 8)}…</span>
          </h2>
          <p>
            <StatusBadge status={job.status} /> · Updated {formatDateTime(job.updated_at)}
          </p>
        </div>
        <div className="btn-group">
          {job.status === 'dead_letter' && (
            <button type="button" className="btn btn-primary" disabled={actionBusy} onClick={handleReplay}>
              Replay from dead letter
            </button>
          )}
          {job.status === 'failed' && (
            <button type="button" className="btn btn-primary" disabled={actionBusy} onClick={handleRequeue}>
              Requeue for retry
            </button>
          )}
          <Link to={`/ledger?job=${job.id}`} className="btn btn-secondary" style={{ textDecoration: 'none' }}>
            View ledger
          </Link>
        </div>
      </header>

      {actionMsg && (
        <div className="panel" style={{ marginBottom: '1rem', padding: '0.85rem 1.25rem' }}>
          <p style={{ margin: 0, fontSize: 13 }}>{actionMsg}</p>
        </div>
      )}

      <section className="panel" style={{ marginBottom: '1.5rem' }}>
        <div className="panel-header">
          <h3>Settlement amounts</h3>
        </div>
        <div className="panel-body">
          <div className="detail-grid">
            <DetailItem label="Principal" value={formatMoney(job.principal_cents)} money />
            <DetailItem label="Gross return" value={formatMoney(job.gross_return_cents)} money />
            <DetailItem label="Platform fee" value={formatMoney(job.platform_fee_cents)} money />
            <DetailItem label="Withholding tax" value={formatMoney(job.withholding_tax_cents)} money />
            <DetailItem label="Net payout" value={formatMoney(job.net_payout_cents)} money />
            <DetailItem label="Investment" value={job.investment_id} />
            <DetailItem label="Maturity" value={job.maturity_schedule_id} />
            {job.payout_reference && <DetailItem label="Payout reference" value={job.payout_reference} />}
            {job.last_error && <DetailItem label="Last error" value={job.last_error} />}
            {job.dead_letter_reason && <DetailItem label="Dead letter reason" value={job.dead_letter_reason} />}
          </div>
        </div>
      </section>

      <section className="panel">
        <div className="panel-header">
          <h3>Payout attempts</h3>
          <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{attempts.length} attempt(s)</span>
        </div>
        <div className="panel-body flush">
          {attempts.length === 0 ? (
            <EmptyState title="No attempts recorded" description="Attempts appear when a worker processes this job." />
          ) : (
            <table>
              <thead>
                <tr>
                  <th>#</th>
                  <th>Status</th>
                  <th>Started</th>
                  <th>Finished</th>
                  <th>Error</th>
                </tr>
              </thead>
              <tbody>
                {attempts.map((a) => (
                  <tr key={a.id}>
                    <td>{a.attempt_number}</td>
                    <td>
                      <StatusBadge status={a.status} />
                    </td>
                    <td>{formatDateTime(a.started_at)}</td>
                    <td>{a.finished_at ? formatDateTime(a.finished_at) : '—'}</td>
                    <td style={{ fontSize: 12 }}>{a.error_message ?? '—'}</td>
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

function DetailItem({
  label,
  value,
  money,
}: {
  label: string
  value: string
  money?: boolean
}) {
  return (
    <div className="detail-item">
      <div className="label">{label}</div>
      <div className={`value${money ? ' money' : ''} mono`}>{value}</div>
    </div>
  )
}
