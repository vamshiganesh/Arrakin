import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { auditApi } from '../api'
import type { AuditEvent } from '../api/types'
import { EmptyState } from '../components/EmptyState'
import { ErrorState } from '../components/ErrorState'
import { LoadingState } from '../components/LoadingState'
import { formatDateTime } from '../lib/format'

export function AuditPage() {
  const [actionFilter, setActionFilter] = useState('')
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await auditApi.listEvents({
        action: actionFilter || undefined,
        limit: 50,
      })
      setEvents(res.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load audit log')
    } finally {
      setLoading(false)
    }
  }, [actionFilter])

  useEffect(() => {
    load()
    const id = window.setInterval(load, 15000)
    return () => window.clearInterval(id)
  }, [load])

  return (
    <>
      <header className="page-header">
        <div>
          <h2>Audit log</h2>
          <p>Settlement lifecycle and administrative actions</p>
        </div>
      </header>

      <section className="panel">
        <div className="filter-bar">
          <label htmlFor="action-filter">Action contains</label>
          <input
            id="action-filter"
            type="text"
            placeholder="settlement_job"
            value={actionFilter}
            onChange={(e) => setActionFilter(e.target.value)}
          />
          <button type="button" className="btn btn-secondary" onClick={load}>
            Refresh
          </button>
        </div>

        {loading && events.length === 0 && <LoadingState />}
        {error && events.length === 0 && <ErrorState message={error} onRetry={load} />}
        {!loading && !error && events.length === 0 && (
          <EmptyState title="No audit events" description="Events are recorded when jobs transition or admins take action." />
        )}

        {events.length > 0 && (
          <ul className="timeline" style={{ padding: '1.25rem 1.5rem' }}>
            {events.map((ev) => (
              <li key={ev.id}>
                <div className="time">{formatDateTime(ev.occurred_at)}</div>
                <div className="action">{ev.action}</div>
                <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 4 }}>
                  {ev.actor_type} · {ev.actor_id || 'system'}
                  {ev.entity_type === 'settlement_job' && (
                    <>
                      {' · '}
                      <Link to={`/jobs/${ev.entity_id}`}>View job</Link>
                    </>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  )
}
