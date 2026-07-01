package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// SettlementJobResponse is the API view of a settlement job.
type SettlementJobResponse struct {
	ID                  string  `json:"id"`
	MaturityScheduleID  string  `json:"maturity_schedule_id"`
	InvestmentID        string  `json:"investment_id"`
	IdempotencyKey      string  `json:"idempotency_key"`
	Status              string  `json:"status"`
	PrincipalCents      int64   `json:"principal_cents"`
	GrossReturnCents    int64   `json:"gross_return_cents"`
	PlatformFeeCents    int64   `json:"platform_fee_cents"`
	WithholdingTaxCents int64   `json:"withholding_tax_cents"`
	NetPayoutCents      int64   `json:"net_payout_cents"`
	PayoutReference     *string `json:"payout_reference,omitempty"`
	RetryCount          int32   `json:"retry_count"`
	MaxRetries          int32   `json:"max_retries"`
	NextRetryAt         *string `json:"next_retry_at,omitempty"`
	ProcessingOwner     *string `json:"processing_owner,omitempty"`
	LastError           *string `json:"last_error,omitempty"`
	ErrorClass          *string `json:"error_class,omitempty"`
	DeadLetterReason    *string `json:"dead_letter_reason,omitempty"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	CompletedAt         *string `json:"completed_at,omitempty"`
}

// SettlementJobListResponse wraps a paginated job list.
type SettlementJobListResponse struct {
	Items []SettlementJobResponse `json:"items"`
	Page  PageMeta                `json:"page"`
}

// PayoutAttemptResponse is the API view of a payout attempt.
type PayoutAttemptResponse struct {
	ID              string  `json:"id"`
	SettlementJobID string  `json:"settlement_job_id"`
	AttemptNumber   int32   `json:"attempt_number"`
	Status          string  `json:"status"`
	PayoutReference *string `json:"payout_reference,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	ErrorClass      *string `json:"error_class,omitempty"`
	StartedAt       string  `json:"started_at"`
	FinishedAt      *string `json:"finished_at,omitempty"`
}

// PayoutAttemptListResponse wraps payout attempts for a job.
type PayoutAttemptListResponse struct {
	Items []PayoutAttemptResponse `json:"items"`
}

// LedgerEntryResponse is the API view of a ledger line.
type LedgerEntryResponse struct {
	ID              string          `json:"id"`
	EntryGroupID    string          `json:"entry_group_id"`
	SettlementJobID string          `json:"settlement_job_id"`
	AccountID       string          `json:"account_id"`
	Side            string          `json:"side"`
	AmountCents     int64           `json:"amount_cents"`
	Currency        string          `json:"currency"`
	Description     string          `json:"description"`
	PostedAt        string          `json:"posted_at"`
	Metadata        json.RawMessage `json:"metadata"`
}

// LedgerEntryListResponse wraps a paginated ledger entry list.
type LedgerEntryListResponse struct {
	Items []LedgerEntryResponse `json:"items"`
	Page  PageMeta              `json:"page"`
}

// LedgerAccountResponse is the API view of a ledger account.
type LedgerAccountResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	AccountType string `json:"account_type"`
	CreatedAt   string `json:"created_at"`
}

// LedgerAccountListResponse wraps ledger accounts.
type LedgerAccountListResponse struct {
	Items []LedgerAccountResponse `json:"items"`
}

// ReconciliationSummary holds aggregate reconciliation totals.
type ReconciliationSummary struct {
	ExpectedTotalCents  int64          `json:"expected_total_cents"`
	SucceededTotalCents int64          `json:"succeeded_total_cents"`
	DiscrepancyCents    int64          `json:"discrepancy_cents"`
	ByStatus            map[string]int `json:"by_status"`
}

// ReconciliationResponse is the API view of a reconciliation snapshot.
type ReconciliationResponse struct {
	ID         string                `json:"id"`
	SnapshotAt string                `json:"snapshot_at"`
	Summary    ReconciliationSummary `json:"summary"`
	Flags      []string              `json:"flags"`
}

// ReconciliationListResponse wraps historical snapshots.
type ReconciliationListResponse struct {
	Items []ReconciliationResponse `json:"items"`
	Page  PageMeta                 `json:"page"`
}

// AuditEventResponse is the API view of an audit event.
type AuditEventResponse struct {
	ID            string          `json:"id"`
	OccurredAt    string          `json:"occurred_at"`
	ActorType     string          `json:"actor_type"`
	ActorID       string          `json:"actor_id"`
	Action        string          `json:"action"`
	EntityType    string          `json:"entity_type"`
	EntityID      string          `json:"entity_id"`
	Payload       json.RawMessage `json:"payload"`
	CorrelationID string          `json:"correlation_id"`
}

// AuditEventListResponse wraps a paginated audit event list.
type AuditEventListResponse struct {
	Items []AuditEventResponse `json:"items"`
	Page  PageMeta             `json:"page"`
}

// SchedulerTickResponse is returned by admin scheduler trigger.
type SchedulerTickResponse struct {
	JobsCreated int `json:"jobs_created"`
}

// JobActionResponse is returned by replay/requeue endpoints.
type JobActionResponse struct {
	Job SettlementJobResponse `json:"job"`
}

// MapSettlementJob converts a store row to API DTO.
func MapSettlementJob(job sqlc.SettlementJob) (SettlementJobResponse, error) {
	id, err := store.PgtypeToUUID(job.ID)
	if err != nil {
		return SettlementJobResponse{}, err
	}
	maturityID, err := store.PgtypeToUUID(job.MaturityScheduleID)
	if err != nil {
		return SettlementJobResponse{}, err
	}
	investmentID, err := store.PgtypeToUUID(job.InvestmentID)
	if err != nil {
		return SettlementJobResponse{}, err
	}

	resp := SettlementJobResponse{
		ID:                  id.String(),
		MaturityScheduleID:  maturityID.String(),
		InvestmentID:        investmentID.String(),
		IdempotencyKey:      job.IdempotencyKey,
		Status:              string(job.Status),
		PrincipalCents:      job.PrincipalCents,
		GrossReturnCents:    job.GrossReturnCents,
		PlatformFeeCents:    job.PlatformFeeCents,
		WithholdingTaxCents: job.WithholdingTaxCents,
		NetPayoutCents:      job.NetPayoutCents,
		PayoutReference:     job.PayoutReference,
		RetryCount:          job.RetryCount,
		MaxRetries:          job.MaxRetries,
		NextRetryAt:         formatPgTimePtr(job.NextRetryAt),
		ProcessingOwner:     job.ProcessingOwner,
		LastError:           job.LastError,
		DeadLetterReason:    job.DeadLetterReason,
		CreatedAt:           formatPgTime(job.CreatedAt),
		UpdatedAt:           formatPgTime(job.UpdatedAt),
		CompletedAt:         formatPgTimePtr(job.CompletedAt),
	}
	if job.ErrorClass != nil {
		s := string(*job.ErrorClass)
		resp.ErrorClass = &s
	}
	return resp, nil
}

// MapPayoutAttempt converts a store row to API DTO.
func MapPayoutAttempt(attempt sqlc.PayoutAttempt) (PayoutAttemptResponse, error) {
	id, err := store.PgtypeToUUID(attempt.ID)
	if err != nil {
		return PayoutAttemptResponse{}, err
	}
	jobID, err := store.PgtypeToUUID(attempt.SettlementJobID)
	if err != nil {
		return PayoutAttemptResponse{}, err
	}
	resp := PayoutAttemptResponse{
		ID:              id.String(),
		SettlementJobID: jobID.String(),
		AttemptNumber:   attempt.AttemptNumber,
		Status:          string(attempt.Status),
		PayoutReference: attempt.PayoutReference,
		ErrorMessage:    attempt.ErrorMessage,
		StartedAt:       formatPgTime(attempt.StartedAt),
		FinishedAt:      formatPgTimePtr(attempt.FinishedAt),
	}
	if attempt.ErrorClass != nil {
		s := string(*attempt.ErrorClass)
		resp.ErrorClass = &s
	}
	return resp, nil
}

// MapLedgerEntry converts a store row to API DTO.
func MapLedgerEntry(entry sqlc.LedgerEntry) (LedgerEntryResponse, error) {
	id, err := store.PgtypeToUUID(entry.ID)
	if err != nil {
		return LedgerEntryResponse{}, err
	}
	groupID, err := store.PgtypeToUUID(entry.EntryGroupID)
	if err != nil {
		return LedgerEntryResponse{}, err
	}
	jobID, err := store.PgtypeToUUID(entry.SettlementJobID)
	if err != nil {
		return LedgerEntryResponse{}, err
	}
	accountID, err := store.PgtypeToUUID(entry.AccountID)
	if err != nil {
		return LedgerEntryResponse{}, err
	}
	return LedgerEntryResponse{
		ID:              id.String(),
		EntryGroupID:    groupID.String(),
		SettlementJobID: jobID.String(),
		AccountID:       accountID.String(),
		Side:            entry.Side,
		AmountCents:     entry.AmountCents,
		Currency:        entry.Currency,
		Description:     entry.Description,
		PostedAt:        formatPgTime(entry.PostedAt),
		Metadata:        entry.Metadata,
	}, nil
}

// MapLedgerAccount converts a store row to API DTO.
func MapLedgerAccount(account sqlc.LedgerAccount) (LedgerAccountResponse, error) {
	id, err := store.PgtypeToUUID(account.ID)
	if err != nil {
		return LedgerAccountResponse{}, err
	}
	return LedgerAccountResponse{
		ID:          id.String(),
		Code:        account.Code,
		Name:        account.Name,
		AccountType: account.AccountType,
		CreatedAt:   formatPgTime(account.CreatedAt),
	}, nil
}

// MapReconciliationSnapshot builds a reconciliation API response with flags.
func MapReconciliationSnapshot(snapshot sqlc.ReconciliationSnapshot, flags []string) (ReconciliationResponse, error) {
	id, err := store.PgtypeToUUID(snapshot.ID)
	if err != nil {
		return ReconciliationResponse{}, err
	}

	byStatus := map[string]int{
		"pending":     int(snapshot.PendingCount),
		"failed":      int(snapshot.FailedCount),
		"dead_letter": int(snapshot.DeadLetterCount),
		"succeeded":   int(snapshot.SucceededCount),
	}
	var details map[string]any
	if len(snapshot.Details) > 0 {
		_ = json.Unmarshal(snapshot.Details, &details)
		if v, ok := details["processing_count"].(float64); ok {
			byStatus["processing"] = int(v)
		}
	}

	if flags == nil {
		flags = []string{}
	}

	return ReconciliationResponse{
		ID:         id.String(),
		SnapshotAt: formatTime(snapshot.SnapshotAt),
		Summary: ReconciliationSummary{
			ExpectedTotalCents:  snapshot.ExpectedTotalCents,
			SucceededTotalCents: snapshot.SucceededTotalCents,
			DiscrepancyCents:    snapshot.DiscrepancyCents,
			ByStatus:            byStatus,
		},
		Flags: flags,
	}, nil
}

// MapAuditEvent converts a store row to API DTO.
func MapAuditEvent(event sqlc.AuditEvent) (AuditEventResponse, error) {
	id, err := store.PgtypeToUUID(event.ID)
	if err != nil {
		return AuditEventResponse{}, err
	}
	entityID, err := store.PgtypeToUUID(event.EntityID)
	if err != nil {
		return AuditEventResponse{}, err
	}
	return AuditEventResponse{
		ID:            id.String(),
		OccurredAt:    formatTime(event.OccurredAt),
		ActorType:     string(event.ActorType),
		ActorID:       event.ActorID,
		Action:        event.Action,
		EntityType:    event.EntityType,
		EntityID:      entityID.String(),
		Payload:       event.Payload,
		CorrelationID: event.CorrelationID,
	}, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

// JobCursorFromSettlementJob returns pagination cursor fields for a job.
func JobCursorFromSettlementJob(job sqlc.SettlementJob) (time.Time, uuid.UUID, error) {
	id, err := store.PgtypeToUUID(job.ID)
	return job.CreatedAt, id, err
}

// AuditCursorFromEvent returns pagination cursor fields for an audit event.
func AuditCursorFromEvent(event sqlc.AuditEvent) (time.Time, uuid.UUID, error) {
	id, err := store.PgtypeToUUID(event.ID)
	return event.OccurredAt, id, err
}

// LedgerCursorFromEntry returns pagination cursor fields for a ledger entry.
func LedgerCursorFromEntry(entry sqlc.LedgerEntry) (time.Time, uuid.UUID, error) {
	id, err := store.PgtypeToUUID(entry.ID)
	return entry.PostedAt, id, err
}

// ReconciliationCursorFromSnapshot returns pagination cursor fields.
func ReconciliationCursorFromSnapshot(snapshot sqlc.ReconciliationSnapshot) (time.Time, uuid.UUID, error) {
	id, err := store.PgtypeToUUID(snapshot.ID)
	return snapshot.SnapshotAt, id, err
}
