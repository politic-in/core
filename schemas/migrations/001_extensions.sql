-- politic-core: Database Extensions
-- Required PostgreSQL extensions for Politic

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";      -- UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";       -- Encryption functions
CREATE EXTENSION IF NOT EXISTS "pg_trgm";        -- Fuzzy text matching (booth names)
CREATE EXTENSION IF NOT EXISTS "vector";         -- pgvector for embeddings
CREATE EXTENSION IF NOT EXISTS "h3";             -- H3 hexagon functions
CREATE EXTENSION IF NOT EXISTS "postgis";        -- GeoJSON boundaries
