import { useCallback, useEffect, useState } from 'react'
import { reconciliationApi } from '../api'
import type { ReconciliationSnapshot } from '../api/types'
import { ApiClientError } from '../api/client'
import { EmptyState } from '../components/EmptyState'
import { ErrorState } from '../components/ErrorState'
import { LoadingState } from '../components/LoadingState'
import { formatDateTime, formatMoney } from '../lib/format'

export function ReconciliationPage() {
  const [latest, setLatest] = useState<ReconciliationSnapshot | null>(null)
  const [history, setHistory] = useState<ReconciliationSnapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [runBusy, setRunBusy] = useState(false)
  const [runMsg, setRunMsg] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [snap, list] = await Promise.all([
        reconciliationApi.latest().catch((e: unknown) => {
          if (e instanceof ApiClientError && e.status === 404) return null
          throw e
        }),
        reconciliationApi.listSnapshots({ limit: 10 }),
      ])
      setLatest(snap)
      setHistory(list.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load reconciliation')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function handleRun() {
    setRunBusy(true)
    setRunMsg(null)
    try {
      const snap = await reconciliationApi.run()
      setLatest(snap)
      setRunMsg(`Snapshot recorded at ${formatDateTime(snap.snapshot_at)}`)
      await load()
    } catch (e) {
      setRunMsg(e instanceof Error ? e.message : 'Snapshot failed')
    } finally {
      setRunBusy(false)
    }
  }

  if (loading && !latest && history.length === 0) {
    return <LoadingState message="Loading reconciliation…" />
  }

  if (error && !latest && history.length === 0) {
    return <ErrorState message={error} onRetry={load} />
  }

  const summary = latest?.summary

  return (
    <>
      <header className="page-header">
        <div>
          <h2>Reconciliation</h2>
          <p>Expected settlement totals versus processed outcomes</p>
        </div>
        <button type="button" className="btn btn-primary" disabled={runBusy} onClick={handleRun}>
          {runBusy ? 'Running…' : 'Run snapshot now'}
        </button>
      </header>

      {runMsg && (
        <div className="panel" style={{ marginBottom: '1rem', padding: '0.85rem 1.25rem' }}>
          <p style={{ margin: 0, fontSize: 13 }}>{runMsg}</p>
        </div>
      )}

      {!latest ? (
        <div className="panel" style={{ marginBottom: '1.5rem' }}>
          <EmptyState title="No snapshot yet" description="Run a reconciliation snapshot to compare expected and settled totals." />
        </div>
      ) : (
        <>
          <div className="stat-grid">
            <div className="stat-card">
              <div className="label">Expected total</div>
              <div className="value">{formatMoney(summary!.expected_total_cents)}</div>
            </div>
            <div className="stat-card">
              <div className="label">Succeeded total</div>
              <div className="value">{formatMoney(summary!.succeeded_total_cents)}</div>
            </div>
            <div className="stat-card">
              <div className="label">Discrepancy</div>
              <div
                className="value"
                style={{
                  color: summary!.discrepancy_cents > 0 ? 'var(--status-dead)' : 'var(--status-succeeded)',
                }}
              >
                {formatMoney(summary!.discrepancy_cents)}
              </div>
            </div>
            <div className="stat-card">
              <div className="label">Snapshot time</div>
              <div className="value" style={{ fontSize: '1rem' }}>
                {formatDateTime(latest.snapshot_at)}
              </div>
            </div>
          </div>

          {latest.flags.length > 0 && (
            <div className="panel" style={{ marginBottom: '1.5rem', padding: '1rem 1.25rem' }}>
              <div className="label" style={{ marginBottom: '0.5rem' }}>
                Discrepancy flags
              </div>
              <div className="flags">
                {latest.flags.map((f) => (
                  <span key={f} className="flag">
                    {f.replace(/_/g, ' ')}
                  </span>
                ))}
              </div>
            </div>
          )}

          <section className="panel" style={{ marginBottom: '1.5rem' }}>
            <div className="panel-header">
              <h3>Status breakdown</h3>
            </div>
            <div className="panel-body flush">
              <table>
                <thead>
                  <tr>
                    <th>Status</th>
                    <th>Count</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(summary!.by_status).map(([status, count]) => (
                    <tr key={status}>
                      <td style={{ textTransform: 'capitalize' }}>{status.replace(/_/g, ' ')}</td>
                      <td>{count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        </>
      )}

      <section className="panel">
        <div className="panel-header">
          <h3>Snapshot history</h3>
        </div>
        <div className="panel-body flush">
          {history.length === 0 ? (
            <EmptyState title="No history" description="Previous snapshots will appear here after you run reconciliation." />
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Expected</th>
                  <th>Succeeded</th>
                  <th>Discrepancy</th>
                  <th>Flags</th>
                </tr>
              </thead>
              <tbody>
                {history.map((row) => (
                  <tr key={row.id}>
                    <td>{formatDateTime(row.snapshot_at)}</td>
                    <td>{formatMoney(row.summary.expected_total_cents)}</td>
                    <td>{formatMoney(row.summary.succeeded_total_cents)}</td>
                    <td>{formatMoney(row.summary.discrepancy_cents)}</td>
                    <td>{row.flags.length ? row.flags.join(', ') : '—'}</td>
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
