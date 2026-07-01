import { apiGet, apiPost, newIdempotencyKey } from './client'
import type {
  AuditEventList,
  JobActionResult,
  JobStatus,
  LedgerEntryList,
  PayoutAttemptList,
  ReconciliationList,
  ReconciliationSnapshot,
  SchedulerTickResult,
  SettlementJob,
  SettlementJobList,
} from './types'

export const settlementApi = {
  listJobs(params: { status?: JobStatus; limit?: number; cursor?: string }) {
    return apiGet<SettlementJobList>('/api/v1/settlement-jobs', params)
  },
  getJob(id: string) {
    return apiGet<SettlementJob>(`/api/v1/settlement-jobs/${id}`)
  },
  listAttempts(jobId: string) {
    return apiGet<PayoutAttemptList>(`/api/v1/settlement-jobs/${jobId}/attempts`)
  },
  replay(jobId: string) {
    return apiPost<JobActionResult>(
      `/api/v1/settlement-jobs/${jobId}/replay`,
      newIdempotencyKey('replay'),
    )
  },
  requeue(jobId: string) {
    return apiPost<JobActionResult>(
      `/api/v1/settlement-jobs/${jobId}/requeue`,
      newIdempotencyKey('requeue'),
    )
  },
}

export const ledgerApi = {
  listEntries(params: {
    settlement_job_id?: string
    account_code?: string
    limit?: number
    cursor?: string
  }) {
    return apiGet<LedgerEntryList>('/api/v1/ledger/entries', params)
  },
}

export const reconciliationApi = {
  latest() {
    return apiGet<ReconciliationSnapshot>('/api/v1/reconciliation/latest')
  },
  listSnapshots(params?: { limit?: number; cursor?: string }) {
    return apiGet<ReconciliationList>('/api/v1/reconciliation/snapshots', params)
  },
  run() {
    return apiPost<ReconciliationSnapshot>(
      '/api/v1/reconciliation/run',
      newIdempotencyKey('recon'),
    )
  },
}

export const auditApi = {
  listEvents(params?: {
    entity_type?: string
    entity_id?: string
    action?: string
    limit?: number
    cursor?: string
  }) {
    return apiGet<AuditEventList>('/api/v1/audit/events', params)
  },
}

export const adminApi = {
  schedulerTick() {
    return apiPost<SchedulerTickResult>(
      '/api/v1/admin/scheduler/tick',
      newIdempotencyKey('tick'),
    )
  },
}
