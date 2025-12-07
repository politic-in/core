-- politic-core: Civic Score History
-- Audit trail of all score changes (transparency)

CREATE TYPE civic_action AS ENUM (
    'kyc_completed',            -- +10
    'booth_challenge_passed',   -- +15
    'issue_verified',           -- +5 per issue
    'verification_given',       -- +2 per verification (if issue confirmed real)
    'poll_completed',           -- +1
    'issue_fixed',              -- +10 (reporter bonus when their issue fixed)
    'account_age_30',           -- +5
    'account_age_90',           -- +5
    'account_age_180',          -- +5
    'fake_verification',        -- -10
    'fake_issue_reported',      -- -15
    'low_quality_response',     -- -5
    'inactive_60_days'          -- -10
);

-- Civic score change log
CREATE TABLE civic_score_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),

    action civic_action NOT NULL,
    points INT NOT NULL,                    -- Can be positive or negative

    score_before INT NOT NULL,
    score_after INT NOT NULL,

    -- Reference to what caused the change
    reference_type VARCHAR(50),             -- "issue", "poll", "verification"
    reference_id UUID,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_civic_log_user ON civic_score_log(user_id);
CREATE INDEX idx_civic_log_action ON civic_score_log(action);
CREATE INDEX idx_civic_log_created ON civic_score_log(created_at DESC);

-- Point values (can be adjusted - keeping in DB for transparency)
CREATE TABLE civic_score_config (
    action civic_action PRIMARY KEY,
    points INT NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default values
INSERT INTO civic_score_config (action, points, description) VALUES
    ('kyc_completed', 10, 'Completed KYC verification'),
    ('booth_challenge_passed', 15, 'Passed Polling Station Challenge'),
    ('issue_verified', 5, 'Issue verified by 2+ neighbors'),
    ('verification_given', 2, 'Verified a neighbor issue (confirmed real)'),
    ('poll_completed', 1, 'Completed a poll'),
    ('issue_fixed', 10, 'Issue you reported was fixed'),
    ('account_age_30', 5, 'Account age reached 30 days'),
    ('account_age_90', 5, 'Account age reached 90 days'),
    ('account_age_180', 5, 'Account age reached 180 days'),
    ('fake_verification', -10, 'Verified an issue that was fake'),
    ('fake_issue_reported', -15, 'Reported issue flagged as fake by 3+ users'),
    ('low_quality_response', -5, 'Poll response flagged as low quality'),
    ('inactive_60_days', -10, 'Inactive for 60+ days');
