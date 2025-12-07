-- politic-core: Users Schema
-- Participants (citizens), Customers (buyers), Fixers (govt/NGO)

-- User types enum
CREATE TYPE user_type AS ENUM ('participant', 'customer', 'fixer');
CREATE TYPE kyc_status AS ENUM ('none', 'pending', 'verified', 'rejected');
CREATE TYPE fixer_type AS ENUM ('elected_rep', 'govt_dept', 'ngo', 'rwa', 'business', 'volunteer');

-- Base users table (Identity Service - separate DB in production)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type user_type NOT NULL DEFAULT 'participant',
    phone_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA256 of phone (never store plain)
    phone_encrypted BYTEA,                   -- AES encrypted (for OTP resend only)
    email_encrypted BYTEA,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    last_active_at TIMESTAMPTZ
);

CREATE INDEX idx_users_type ON users(type);
CREATE INDEX idx_users_created ON users(created_at);

-- Participant profiles (citizens who report/respond)
CREATE TABLE participants (
    user_id UUID PRIMARY KEY REFERENCES users(id),

    -- Verification status
    kyc_status kyc_status DEFAULT 'none',
    kyc_doc_type VARCHAR(20),               -- "aadhaar", "voter_id", "pan"
    kyc_verified_at TIMESTAMPTZ,

    -- Polling Station Challenge
    booth_challenge_passed BOOLEAN DEFAULT FALSE,
    booth_id INT REFERENCES polling_booths(id),
    booth_verified_at TIMESTAMPTZ,

    -- Location (from GPS, not KYC)
    primary_hexagon_id VARCHAR(20) REFERENCES hexagons(id),

    -- Trust metrics
    civic_score INT DEFAULT 20 CHECK (civic_score >= 0 AND civic_score <= 100),
    account_age_days INT GENERATED ALWAYS AS (
        EXTRACT(DAY FROM NOW() - created_at)
    ) STORED,

    -- Activity counters (denormalized for performance)
    issues_reported INT DEFAULT 0,
    issues_verified INT DEFAULT 0,
    polls_completed INT DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_participant_hexagon ON participants(primary_hexagon_id);
CREATE INDEX idx_participant_civic_score ON participants(civic_score);

-- Customer profiles (politicians, businesses who buy data)
CREATE TABLE customers (
    user_id UUID PRIMARY KEY REFERENCES users(id),

    org_name VARCHAR(255) NOT NULL,
    org_type VARCHAR(50) NOT NULL,          -- "political_party", "mla_office", "business", "ngo"

    -- Subscription
    subscription_tier VARCHAR(20) DEFAULT 'free',  -- "free", "paid"
    subscription_expires_at TIMESTAMPTZ,

    -- Limits
    polls_remaining_micro INT DEFAULT 0,
    polls_remaining_detailed INT DEFAULT 0,

    -- Billing
    razorpay_customer_id VARCHAR(50),
    gst_number VARCHAR(20),

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Fixer profiles (those who claim fixes)
CREATE TABLE fixers (
    user_id UUID PRIMARY KEY REFERENCES users(id),

    fixer_type fixer_type NOT NULL,
    org_name VARCHAR(255) NOT NULL,

    -- Verification
    verified BOOLEAN DEFAULT FALSE,
    verified_by UUID REFERENCES users(id),
    verified_at TIMESTAMPTZ,

    -- Scope (which ACs can they claim fixes in)
    ac_ids INT[],                           -- Array of AC IDs

    -- Stats
    fixes_claimed INT DEFAULT 0,
    fixes_verified INT DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_fixer_type ON fixers(fixer_type);
