import type { JobStatus } from '../api/types'
import { statusLabel } from '../lib/format'

const classMap: Record<JobStatus, string> = {
  pending: 'badge-pending',
  processing: 'badge-processing',
  succeeded: 'badge-succeeded',
  failed: 'badge-failed',
  dead_letter: 'badge-dead_letter',
}

export function StatusBadge({ status }: { status: string }) {
  const key = status as JobStatus
  const cls = classMap[key] ?? 'badge-pending'
  return <span className={`badge ${cls}`}>{statusLabel(status)}</span>
}
