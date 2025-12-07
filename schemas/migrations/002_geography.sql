-- politic-core: Geography Schema
-- Dual-layer geography: Hexagons (atomic) + Assembly Constituencies (political)

-- States
CREATE TABLE states (
    id SERIAL PRIMARY KEY,
    code VARCHAR(3) NOT NULL UNIQUE,        -- "KA", "TN", "MH"
    name VARCHAR(100) NOT NULL,
    name_local VARCHAR(100),                -- Regional language name
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Districts
CREATE TABLE districts (
    id SERIAL PRIMARY KEY,
    state_id INT NOT NULL REFERENCES states(id),
    code VARCHAR(10) NOT NULL UNIQUE,       -- "KA-BLR-U"
    name VARCHAR(100) NOT NULL,
    name_local VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Assembly Constituencies (Political Layer)
CREATE TABLE assembly_constituencies (
    id SERIAL PRIMARY KEY,
    district_id INT NOT NULL REFERENCES districts(id),
    code VARCHAR(20) NOT NULL UNIQUE,       -- "KA-176" (Jayanagar)
    name VARCHAR(100) NOT NULL,
    name_local VARCHAR(100),
    boundary GEOMETRY(MULTIPOLYGON, 4326),  -- GeoJSON boundary
    total_voters INT,
    reserved_category VARCHAR(10),          -- "GEN", "SC", "ST"
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ac_boundary ON assembly_constituencies USING GIST(boundary);
CREATE INDEX idx_ac_district ON assembly_constituencies(district_id);

-- Hexagons (Atomic Unit - H3 Resolution 9)
CREATE TABLE hexagons (
    id VARCHAR(20) PRIMARY KEY,             -- H3 index: "89283082837ffff"
    ac_id INT REFERENCES assembly_constituencies(id),
    center_lat DOUBLE PRECISION NOT NULL,
    center_lng DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_hex_ac ON hexagons(ac_id);

-- Polling Booths (Text only - for verification, not geographic)
CREATE TABLE polling_booths (
    id SERIAL PRIMARY KEY,
    ac_id INT NOT NULL REFERENCES assembly_constituencies(id),
    booth_number VARCHAR(10) NOT NULL,
    name VARCHAR(255) NOT NULL,             -- "Govt Primary School, 5th Block"
    name_normalized VARCHAR(255) NOT NULL,  -- Lowercase, no punctuation
    name_trigrams TEXT,                     -- For pg_trgm fuzzy matching
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_booth_ac ON polling_booths(ac_id);
CREATE INDEX idx_booth_name_trgm ON polling_booths USING GIN(name_normalized gin_trgm_ops);

-- Hexagon-to-AC mapping (precomputed for performance)
CREATE TABLE hexagon_ac_mapping (
    hexagon_id VARCHAR(20) PRIMARY KEY REFERENCES hexagons(id),
    ac_id INT NOT NULL REFERENCES assembly_constituencies(id),
    coverage_pct DECIMAL(5,2) DEFAULT 100.0  -- % of hexagon in this AC
);
