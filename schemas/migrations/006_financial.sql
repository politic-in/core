-- politic-core: Financial Schema
-- Wallets, transactions, subscriptions

CREATE TYPE transaction_type AS ENUM (
    'poll_earning',         -- Citizen earned from poll
    'verification_earning', -- Citizen earned from verifying
    'referral_earning',     -- Citizen earned from referral
    'withdrawal',           -- Citizen withdrew to UPI/voucher
    'subscription_payment', -- Customer paid subscription
    'poll_purchase',        -- Customer paid for poll add-on
    'refund'
);

CREATE TYPE payout_method AS ENUM ('upi', 'amazon_voucher', 'flipkart_voucher', 'erupi');
CREATE TYPE subscription_tier AS ENUM ('free', 'paid');

-- Wallets (for citizens - earning balance)
CREATE TABLE wallets (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    balance_inr DECIMAL(10,2) DEFAULT 0.00,
    lifetime_earnings_inr DECIMAL(10,2) DEFAULT 0.00,
    pending_withdrawal_inr DECIMAL(10,2) DEFAULT 0.00,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Transactions
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),

    type transaction_type NOT NULL,
    amount_inr DECIMAL(10,2) NOT NULL,
    description TEXT,

    -- References
    poll_id UUID,                           -- For poll-related transactions
    issue_id UUID,                          -- For verification earnings

    -- For withdrawals
    payout_method payout_method,
    payout_reference VARCHAR(255),          -- UPI ref / voucher code

    -- Status
    status VARCHAR(20) DEFAULT 'completed', -- "pending", "completed", "failed"

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_txn_user ON transactions(user_id);
CREATE INDEX idx_txn_type ON transactions(type);
CREATE INDEX idx_txn_created ON transactions(created_at DESC);

-- Subscriptions
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES users(id),

    tier subscription_tier NOT NULL,
    price_inr DECIMAL(10,2) NOT NULL,

    -- Razorpay
    razorpay_subscription_id VARCHAR(50),
    razorpay_plan_id VARCHAR(50),

    -- Period
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    cancelled_at TIMESTAMPTZ,

    -- Included quotas
    polls_micro_included INT DEFAULT 10,
    polls_detailed_included INT DEFAULT 2,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_sub_customer ON subscriptions(customer_id);
CREATE INDEX idx_sub_ends ON subscriptions(ends_at);

-- Referrals
CREATE TABLE referrals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    referrer_id UUID NOT NULL REFERENCES users(id),
    referred_id UUID NOT NULL REFERENCES users(id),

    -- Bonus paid when referred user completes first poll
    bonus_paid BOOLEAN DEFAULT FALSE,
    bonus_amount_inr DECIMAL(10,2) DEFAULT 25.00,
    bonus_paid_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(referred_id)                     -- Each user can only be referred once
);

CREATE INDEX idx_referral_referrer ON referrals(referrer_id);
