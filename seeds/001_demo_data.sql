-- Idempotent demo seed for Arrakin local development.
-- Scenarios:
--   1. INV-DEMO-001  simulation_profile = success
--   2. INV-DEMO-002  simulation_profile = transient_then_success
--   3. INV-DEMO-003  simulation_profile = terminal_failure
--   4. INV-DEMO-004  simulation_profile = success (additional volume)
--   5. INV-DEMO-005  simulation_profile = NULL (production-style, no injection)
--
-- All maturities are in the past so the scheduler can pick them up immediately.
-- Re-running this script is safe (ON CONFLICT DO NOTHING).

BEGIN;

-- Fixed UUIDs keep demo references stable across docs, tests, and admin UI.
INSERT INTO investors (id, external_ref, display_name)
VALUES
    ('a1000001-0001-4001-8001-000000000001', 'INVSTR-ALICE', 'Alice Chen'),
    ('a1000001-0001-4001-8001-000000000002', 'INVSTR-BOB', 'Bob Martinez'),
    ('a1000001-0001-4001-8001-000000000003', 'INVSTR-CARA', 'Cara Okonkwo')
ON CONFLICT (external_ref) DO NOTHING;

INSERT INTO investments (
    id,
    investor_id,
    principal_cents,
    annual_rate_bps,
    term_days,
    status,
    currency,
    simulation_profile
)
VALUES
    (
        'b2000001-0002-4002-8002-000000000001',
        'a1000001-0001-4001-8001-000000000001',
        1000000,
        800,
        365,
        'active',
        'USD',
        'success'
    ),
    (
        'b2000001-0002-4002-8002-000000000002',
        'a1000001-0001-4001-8001-000000000002',
        2500000,
        750,
        180,
        'active',
        'USD',
        'transient_then_success'
    ),
    (
        'b2000001-0002-4002-8002-000000000003',
        'a1000001-0001-4001-8001-000000000003',
        500000,
        900,
        90,
        'active',
        'USD',
        'terminal_failure'
    ),
    (
        'b2000001-0002-4002-8002-000000000004',
        'a1000001-0001-4001-8001-000000000001',
        1500000,
        650,
        270,
        'active',
        'USD',
        'success'
    ),
    (
        'b2000001-0002-4002-8002-000000000005',
        'a1000001-0001-4001-8001-000000000002',
        800000,
        700,
        120,
        'active',
        'USD',
        NULL
    )
ON CONFLICT (id) DO NOTHING;

INSERT INTO maturity_schedules (id, investment_id, matures_at, status)
VALUES
    (
        'c3000001-0003-4003-8003-000000000001',
        'b2000001-0002-4002-8002-000000000001',
        now() - interval '2 days',
        'pending'
    ),
    (
        'c3000001-0003-4003-8003-000000000002',
        'b2000001-0002-4002-8002-000000000002',
        now() - interval '1 day',
        'pending'
    ),
    (
        'c3000001-0003-4003-8003-000000000003',
        'b2000001-0002-4002-8002-000000000003',
        now() - interval '3 hours',
        'pending'
    ),
    (
        'c3000001-0003-4003-8003-000000000004',
        'b2000001-0002-4002-8002-000000000004',
        now() - interval '6 hours',
        'pending'
    ),
    (
        'c3000001-0003-4003-8003-000000000005',
        'b2000001-0002-4002-8002-000000000005',
        now() - interval '12 hours',
        'pending'
    )
ON CONFLICT (id) DO NOTHING;

COMMIT;
