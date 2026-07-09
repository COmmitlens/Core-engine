-- CommitLens PostgreSQL initialisation
-- This script runs once when the postgres container is first created.

-- Enable the pgvector extension (required for commit embedding search)
CREATE EXTENSION IF NOT EXISTS vector;

-- Enable UUID generation (commonly useful)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
