-- politic-core: Issues Schema
-- Issue lifecycle: OPEN → VERIFIED → ROUTING → FIXED

CREATE TYPE issue_status AS ENUM ('open', 'verified', 'rejected', 'in_progress', 'fixed');
CREATE TYPE issue_category AS ENUM (
    'roads_transport',
    'water_sanitation',
    'electricity',
    'public_spaces',
    'safety',
    'other'
);

-- Issues
CREATE TABLE issues (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Reporter (nullable for anonymity - stored in Identity DB only)
    reporter_id UUID,                       -- Only in Identity DB, NULL in Response DB

    -- Location
    hexagon_id VARCHAR(20) NOT NULL REFERENCES hexagons(id),
    ac_id INT NOT NULL REFERENCES assembly_constituencies(id),
    lat DOUBLE PRECISION NOT NULL,
    lng DOUBLE PRECISION NOT NULL,
    address_text VARCHAR(500),

    -- Content
    category issue_category NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    suggested_solution TEXT,                -- Citizen-suggested fix

    -- Photos
    photo_urls TEXT[] NOT NULL,             -- Array of GCS URLs
    photo_verified BOOLEAN DEFAULT FALSE,   -- EXIF/GPS/AI check passed

    -- Status
    status issue_status DEFAULT 'open',

    -- Verification counters
    verify_count INT DEFAULT 0,             -- Neighbors who confirmed
    reject_count INT DEFAULT 0,             -- Neighbors who rejected

    -- Resolution
    fixed_at TIMESTAMPTZ,
    fixed_by UUID REFERENCES users(id),     -- Fixer who claimed
    fix_photo_url VARCHAR(500),
    fix_verified_count INT DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    verified_at TIMESTAMPTZ,

    -- Embedding for RAG search (computed async)
    embedding vector(1536)
);

CREATE INDEX idx_issue_hexagon ON issues(hexagon_id);
CREATE INDEX idx_issue_ac ON issues(ac_id);
CREATE INDEX idx_issue_status ON issues(status);
CREATE INDEX idx_issue_category ON issues(category);
CREATE INDEX idx_issue_created ON issues(created_at DESC);
CREATE INDEX idx_issue_embedding ON issues USING ivfflat (embedding vector_cosine_ops);

-- Issue verifications (neighbor confirmations)
CREATE TABLE issue_verifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    issue_id UUID NOT NULL REFERENCES issues(id),
    verifier_id UUID NOT NULL,              -- In Identity DB only

    is_valid BOOLEAN NOT NULL,              -- true = confirmed, false = rejected
    comment TEXT,

    -- Verifier location at time of verification
    verifier_lat DOUBLE PRECISION,
    verifier_lng DOUBLE PRECISION,
    distance_to_issue_m INT,                -- Must be within 500m

    created_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(issue_id, verifier_id)           -- One verification per user per issue
);

CREATE INDEX idx_verification_issue ON issue_verifications(issue_id);

-- Issue routing (where issue was sent)
CREATE TABLE issue_routes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    issue_id UUID NOT NULL REFERENCES issues(id),

    route_type VARCHAR(50) NOT NULL,        -- "elected_rep", "govt_portal", "ngo"
    route_target VARCHAR(255) NOT NULL,     -- MLA name, portal name, NGO name

    -- For govt portals
    external_ticket_id VARCHAR(100),
    external_portal_url VARCHAR(500),

    -- Status
    sent_at TIMESTAMPTZ DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_route_issue ON issue_routes(issue_id);

-- Fix claims
CREATE TABLE fix_claims (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    issue_id UUID NOT NULL REFERENCES issues(id),
    fixer_id UUID NOT NULL REFERENCES users(id),

    -- Evidence
    after_photo_url VARCHAR(500) NOT NULL,
    description TEXT,

    -- Verification
    verify_count INT DEFAULT 0,
    verified BOOLEAN DEFAULT FALSE,
    verified_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(issue_id, fixer_id)
);

CREATE INDEX idx_fix_claim_issue ON fix_claims(issue_id);
