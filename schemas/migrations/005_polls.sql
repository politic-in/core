-- politic-core: Polls Schema
-- IMPORTANT: Responses stored in SEPARATE database (no FK to users)

CREATE TYPE poll_type AS ENUM ('micro', 'detailed');
CREATE TYPE poll_status AS ENUM ('draft', 'active', 'paused', 'completed', 'cancelled');
CREATE TYPE question_type AS ENUM ('single_choice', 'multi_choice', 'rating', 'text', 'audio');

-- Polls (in Identity DB - knows who created it)
CREATE TABLE polls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES users(id),

    type poll_type NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,

    -- Targeting
    target_hexagon_ids VARCHAR(20)[],       -- Specific hexagons
    target_ac_ids INT[],                    -- Or entire ACs
    target_min_civic_score INT DEFAULT 0,   -- Minimum score required

    -- For detailed polls
    require_top_responder BOOLEAN DEFAULT FALSE,

    -- Limits
    max_responses INT,
    budget_inr DECIMAL(10,2),

    -- Status
    status poll_status DEFAULT 'draft',

    -- Timestamps
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_poll_customer ON polls(customer_id);
CREATE INDEX idx_poll_status ON polls(status);

-- Poll questions
CREATE TABLE poll_questions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,

    question_type question_type NOT NULL,
    question_text TEXT NOT NULL,
    question_order INT NOT NULL,

    -- Options for choice questions
    options JSONB,                          -- ["Option A", "Option B", ...]

    -- Branching logic (for detailed polls)
    branch_logic JSONB,                     -- {"option_1": "question_3", ...}

    -- Validation
    required BOOLEAN DEFAULT TRUE,
    min_length INT,
    max_length INT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_question_poll ON poll_questions(poll_id);

-- ============================================================
-- RESPONSES TABLE - IN SEPARATE DATABASE (Response Service)
-- NO FOREIGN KEY TO USERS - This is the anonymity guarantee
-- ============================================================

-- Poll responses (in Response DB - NO user reference)
CREATE TABLE poll_responses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    poll_id UUID NOT NULL,                  -- References polls.id but NO FK (separate DB)
    hexagon_id VARCHAR(20) NOT NULL,        -- Location of response (for aggregation)

    -- NO user_id - this is intentional for anonymity

    -- Response data
    answers JSONB NOT NULL,                 -- {"q1": "option_a", "q2": 4, ...}

    -- Metadata (for fraud detection, not identification)
    response_time_seconds INT,              -- How long to complete
    device_fingerprint_hash VARCHAR(64),    -- For duplicate detection only

    -- Payout tracking (one-way hash, not user_id)
    payout_token_hash VARCHAR(64),          -- Hash that links to payout without revealing user

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_response_poll ON poll_responses(poll_id);
CREATE INDEX idx_response_hexagon ON poll_responses(hexagon_id);
CREATE INDEX idx_response_created ON poll_responses(created_at);

-- Poll analytics (aggregated, never individual)
CREATE TABLE poll_analytics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    poll_id UUID NOT NULL REFERENCES polls(id),

    -- Aggregation level
    hexagon_id VARCHAR(20),                 -- NULL = poll-level aggregate
    ac_id INT,

    -- Counts (must be >= 10 for k-anonymity)
    response_count INT NOT NULL,

    -- Aggregated results (with differential privacy noise if count < 50)
    results JSONB NOT NULL,                 -- {"q1": {"option_a": 45, "option_b": 55}, ...}

    computed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_analytics_poll ON poll_analytics(poll_id);
