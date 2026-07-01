export type JobStatus =
  | 'pending'
  | 'processing'
  | 'succeeded'
  | 'failed'
  | 'dead_letter'

export interface PageMeta {
  limit: number
  next_cursor?: string
  has_more: boolean
}

export interface SettlementJob {
  id: string
  maturity_schedule_id: string
  investment_id: string
  idempotency_key: string
  status: JobStatus
  principal_cents: number
  gross_return_cents: number
  platform_fee_cents: number
  withholding_tax_cents: number
  net_payout_cents: number
  payout_reference?: string
  retry_count: number
  max_retries: number
  next_retry_at?: string
  processing_owner?: string
  last_error?: string
  error_class?: string
  dead_letter_reason?: string
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface SettlementJobList {
  items: SettlementJob[]
  page: PageMeta
}

export interface PayoutAttempt {
  id: string
  settlement_job_id: string
  attempt_number: number
  status: string
  payout_reference?: string
  error_message?: string
  error_class?: string
  started_at: string
  finished_at?: string
}

export interface PayoutAttemptList {
  items: PayoutAttempt[]
}

export interface LedgerEntry {
  id: string
  entry_group_id: string
  settlement_job_id: string
  account_id: string
  side: string
  amount_cents: number
  currency: string
  description: string
  posted_at: string
  metadata: Record<string, unknown>
}

export interface LedgerEntryList {
  items: LedgerEntry[]
  page: PageMeta
}

export interface ReconciliationSummary {
  expected_total_cents: number
  succeeded_total_cents: number
  discrepancy_cents: number
  by_status: Record<string, number>
}

export interface ReconciliationSnapshot {
  id: string
  snapshot_at: string
  summary: ReconciliationSummary
  flags: string[]
}

export interface ReconciliationList {
  items: ReconciliationSnapshot[]
  page: PageMeta
}

export interface AuditEvent {
  id: string
  occurred_at: string
  actor_type: string
  actor_id: string
  action: string
  entity_type: string
  entity_id: string
  payload: Record<string, unknown>
  correlation_id: string
}

export interface AuditEventList {
  items: AuditEvent[]
  page: PageMeta
}

export interface SchedulerTickResult {
  jobs_created: number
}

export interface JobActionResult {
  job: SettlementJob
}

export interface ApiError {
  error: string
  code?: string
  details?: string
}
