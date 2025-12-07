-- politic-core: Election Blackout Schema
-- Section 126 RP Act compliance - 48hr blackout before polling

CREATE TYPE election_type AS ENUM ('general', 'assembly', 'by_election', 'local_body');
CREATE TYPE blackout_status AS ENUM ('scheduled', 'active', 'completed');

-- Elections calendar
CREATE TABLE elections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    type election_type NOT NULL,
    name VARCHAR(255) NOT NULL,             -- "Karnataka Assembly 2028"

    -- Scope
    state_id INT REFERENCES states(id),     -- NULL for general elections
    ac_ids INT[],                           -- Affected ACs (NULL = all in state)

    -- Dates
    polling_date DATE NOT NULL,
    polling_start_time TIME DEFAULT '07:00',
    polling_end_time TIME DEFAULT '18:00',

    -- Blackout period (auto-calculated: 48hrs before poll close)
    blackout_starts_at TIMESTAMPTZ NOT NULL,
    blackout_ends_at TIMESTAMPTZ NOT NULL,

    status blackout_status DEFAULT 'scheduled',

    -- Audit
    source_url VARCHAR(500),                -- ECI notification link
    verified_by VARCHAR(100),               -- Who verified this entry
    verified_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_election_blackout ON elections(blackout_starts_at, blackout_ends_at);
CREATE INDEX idx_election_status ON elections(status);

-- Blackout enforcement log (audit trail)
CREATE TABLE blackout_enforcement_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    election_id UUID NOT NULL REFERENCES elections(id),
    ac_id INT NOT NULL REFERENCES assembly_constituencies(id),

    -- What was blocked
    action_blocked VARCHAR(50) NOT NULL,    -- "poll_create", "results_view", "analytics"
    user_id UUID,                           -- Who tried to access (if logged in)
    ip_address INET,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_blackout_log_election ON blackout_enforcement_log(election_id);
CREATE INDEX idx_blackout_log_created ON blackout_enforcement_log(created_at);

-- Manual override requests (requires 2 founders + legal)
CREATE TABLE blackout_override_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    election_id UUID NOT NULL REFERENCES elections(id),
    ac_ids INT[] NOT NULL,

    reason TEXT NOT NULL,
    requested_by VARCHAR(100) NOT NULL,

    -- Approvals
    approval_1_by VARCHAR(100),
    approval_1_at TIMESTAMPTZ,
    approval_2_by VARCHAR(100),
    approval_2_at TIMESTAMPTZ,
    legal_approval_by VARCHAR(100),
    legal_approval_at TIMESTAMPTZ,

    -- Status
    approved BOOLEAN DEFAULT FALSE,
    override_starts_at TIMESTAMPTZ,
    override_ends_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW()
);
