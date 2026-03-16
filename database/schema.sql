-- Unified Nebula Schema Snapshot
--
-- This file unifies the final database schema produced by these migrations:
-- - 006_pgcrypto.sql
-- - 000_init.sql
-- - 001_seed.sql
-- - 002_entity_types.sql
-- - 003_add_requires_approval.sql
-- - 004_approval_execution.sql
-- - 005_log_types.sql
-- - 007_schema_fixes.sql
-- - 008_api_keys.sql
-- - 009_agent_api_keys.sql
-- - 010_add_work_scope.sql
-- - 011_security_hardening.sql
-- - 012_taxonomy_lifecycle.sql
-- - 013_taxonomy_generalization.sql
-- - 014_mcp_agent_enrollment.sql
-- - 015_add_default_log_types.sql
-- - 016_jobs_privacy_scopes.sql
-- - 017_enterprise_defaults.sql
-- - 018_context_core_rename.sql
-- - 019_source_refs_and_files_uri.sql
-- - 020_requires_approval_defaults.sql
-- - 021_context_of_and_drop_metadata.sql
--
-- Generated from a clean temporary database (nebula_schema_snapshot) on 2026-02-20 13:57:40Z.
-- Source migrations dir: database/migrations/

--
-- PostgreSQL database dump
--

\restrict JHjZmUYDj96H6T96jEgpQLtKOJLZ2ha4XckyCh33fGmkMeyKaLIusqbfNm0P7EO

-- Dumped from database version 16.10 (Debian 16.10-1.pgdg12+1)
-- Dumped by pg_dump version 16.10 (Debian 16.10-1.pgdg12+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


--
-- Name: audit_trigger_function(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.audit_trigger_function() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    changed_fields TEXT[];
    old_json JSONB;
    new_json JSONB;
    changed_by_type TEXT;
    changed_by_id UUID;
BEGIN
    -- Get current session variables (set by application)
    BEGIN
        changed_by_type := current_setting('app.changed_by_type', TRUE);
        changed_by_id := current_setting('app.changed_by_id', TRUE)::UUID;
    EXCEPTION
        WHEN OTHERS THEN
            changed_by_type := 'system';
            changed_by_id := NULL;
    END;

    -- Convert old and new rows to JSONB
    IF TG_OP = 'DELETE' THEN
        old_json := to_jsonb(OLD);
        new_json := NULL;
    ELSIF TG_OP = 'INSERT' THEN
        old_json := NULL;
        new_json := to_jsonb(NEW);
    ELSIF TG_OP = 'UPDATE' THEN
        old_json := to_jsonb(OLD);
        new_json := to_jsonb(NEW);

        -- Determine which fields changed
        SELECT array_agg(key)
        INTO changed_fields
        FROM jsonb_each(new_json)
        WHERE new_json->key IS DISTINCT FROM old_json->key;
    END IF;

    -- Insert audit record
    INSERT INTO audit_log (
        table_name,
        record_id,
        action,
        changed_by_type,
        changed_by_id,
        old_data,
        new_data,
        changed_fields,
        changed_at
    ) VALUES (
        TG_TABLE_NAME,
        COALESCE(NEW.id::TEXT, OLD.id::TEXT),
        lower(TG_OP),
        changed_by_type,
        changed_by_id,
        old_json,
        new_json,
        changed_fields,
        NOW()
    );

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$;


--
-- Name: cascade_status_to_relationships(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cascade_status_to_relationships() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    source_table TEXT;
    status_category TEXT;
    type_name TEXT;
BEGIN
    source_table := TG_TABLE_NAME;

    type_name := CASE
        WHEN source_table = 'entities' THEN 'entity'
        WHEN source_table = 'context_items' THEN 'context'
        WHEN source_table = 'logs' THEN 'log'
        WHEN source_table = 'jobs' THEN 'job'
        WHEN source_table = 'agents' THEN 'agent'
        WHEN source_table = 'files' THEN 'file'
        WHEN source_table = 'protocols' THEN 'protocol'
    END;

    SELECT category INTO status_category
    FROM statuses
    WHERE id = NEW.status_id;

    IF status_category = 'archived' THEN
        PERFORM set_config('nebula.cascade_in_progress', 'true', true);

        UPDATE relationships
        SET status_id = NEW.status_id,
            status_changed_at = NOW()
        WHERE (
            (source_type = type_name AND source_id = NEW.id::text)
            OR
            (target_type = type_name AND target_id = NEW.id::text)
        );

        PERFORM set_config('nebula.cascade_in_progress', 'false', true);
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: generate_job_id(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.generate_job_id() RETURNS text
    LANGUAGE plpgsql
    AS $$
DECLARE
  alphabet TEXT := 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';
  suffix TEXT := '';
  i INT;
  b INT;
  yr INT := EXTRACT(YEAR FROM NOW())::INT;
  q INT := ((EXTRACT(MONTH FROM NOW())::INT - 1) / 3 + 1)::INT;
  new_id TEXT;
  max_attempts INT := 10;
  attempt INT := 0;
BEGIN
  LOOP
    suffix := '';
    FOR i IN 1..4 LOOP
      b := get_byte(gen_random_bytes(1), 0);
      suffix := suffix || SUBSTR(alphabet, (b % LENGTH(alphabet)) + 1, 1);
    END LOOP;

    new_id := yr::TEXT || 'Q' || q::TEXT || '-' || suffix;

    -- Check if ID already exists
    IF NOT EXISTS (SELECT 1 FROM jobs WHERE id = new_id) THEN
      RETURN new_id;
    END IF;

    attempt := attempt + 1;
    IF attempt >= max_attempts THEN
      RAISE EXCEPTION 'Failed to generate unique job ID after % attempts', max_attempts;
    END IF;
  END LOOP;
END;
$$;


--
-- Name: sync_symmetric_relationships(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_symmetric_relationships() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    is_sym BOOLEAN;
BEGIN
    SELECT is_symmetric INTO is_sym
    FROM relationship_types
    WHERE id = COALESCE(NEW.type_id, OLD.type_id);

    IF is_sym THEN
        IF TG_OP = 'INSERT' THEN
            INSERT INTO relationships (
                source_type, source_id,
                target_type, target_id,
                type_id, properties, embedding
            )
            VALUES (
                NEW.target_type, NEW.target_id,
                NEW.source_type, NEW.source_id,
                NEW.type_id, NEW.properties, NEW.embedding
            )
            ON CONFLICT (source_type, source_id, target_type, target_id, type_id) DO NOTHING;

        ELSIF TG_OP = 'UPDATE' THEN
            IF current_setting('nebula.cascade_in_progress', true) = 'true' THEN
                RETURN NEW;
            END IF;

            UPDATE relationships
            SET properties = NEW.properties,
                embedding = NEW.embedding,
                updated_at = NOW()
            WHERE source_type = NEW.target_type
              AND source_id = NEW.target_id
              AND target_type = NEW.source_type
              AND target_id = NEW.source_id
              AND type_id = NEW.type_id;

        ELSIF TG_OP = 'DELETE' THEN
            DELETE FROM relationships
            WHERE source_type = OLD.target_type
              AND source_id = OLD.target_id
              AND target_type = OLD.source_type
              AND target_id = OLD.source_id
              AND type_id = OLD.type_id;
            RETURN OLD;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: update_updated_at_column(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


--
-- Name: validate_relationship_references(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.validate_relationship_references() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.source_type = 'entity' THEN
        IF NOT EXISTS (SELECT 1 FROM entities WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source entity % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'context' THEN
        IF NOT EXISTS (SELECT 1 FROM context_items WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source context_item % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'log' THEN
        IF NOT EXISTS (SELECT 1 FROM logs WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source log % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'job' THEN
        IF NOT EXISTS (SELECT 1 FROM jobs WHERE id = NEW.source_id) THEN
            RAISE EXCEPTION 'source job % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'agent' THEN
        IF NOT EXISTS (SELECT 1 FROM agents WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source agent % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'file' THEN
        IF NOT EXISTS (SELECT 1 FROM files WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source file % does not exist', NEW.source_id;
        END IF;
    ELSIF NEW.source_type = 'protocol' THEN
        IF NOT EXISTS (SELECT 1 FROM protocols WHERE id::text = NEW.source_id) THEN
            RAISE EXCEPTION 'source protocol % does not exist', NEW.source_id;
        END IF;
    END IF;

    IF NEW.target_type = 'entity' THEN
        IF NOT EXISTS (SELECT 1 FROM entities WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target entity % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'context' THEN
        IF NOT EXISTS (SELECT 1 FROM context_items WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target context_item % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'log' THEN
        IF NOT EXISTS (SELECT 1 FROM logs WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target log % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'job' THEN
        IF NOT EXISTS (SELECT 1 FROM jobs WHERE id = NEW.target_id) THEN
            RAISE EXCEPTION 'target job % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'agent' THEN
        IF NOT EXISTS (SELECT 1 FROM agents WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target agent % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'file' THEN
        IF NOT EXISTS (SELECT 1 FROM files WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target file % does not exist', NEW.target_id;
        END IF;
    ELSIF NEW.target_type = 'protocol' THEN
        IF NOT EXISTS (SELECT 1 FROM protocols WHERE id::text = NEW.target_id) THEN
            RAISE EXCEPTION 'target protocol % does not exist', NEW.target_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: agent_enrollment_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_enrollment_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    agent_id uuid NOT NULL,
    approval_request_id uuid NOT NULL,
    status text DEFAULT 'pending_approval'::text NOT NULL,
    enrollment_token_hash text NOT NULL,
    requested_scope_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    granted_scope_ids uuid[] DEFAULT '{}'::uuid[],
    requested_requires_approval boolean DEFAULT true NOT NULL,
    granted_requires_approval boolean,
    rejected_reason text,
    approved_by uuid,
    approved_at timestamp with time zone,
    redeemed_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT agent_enrollment_sessions_enrollment_token_hash_check CHECK ((char_length(enrollment_token_hash) > 0)),
    CONSTRAINT agent_enrollment_sessions_status_check CHECK ((status = ANY (ARRAY['pending_approval'::text, 'approved'::text, 'rejected'::text, 'redeemed'::text, 'expired'::text])))
);


--
-- Name: agents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    system_prompt text,
    scopes uuid[] DEFAULT '{}'::uuid[],
    capabilities text[] DEFAULT '{}'::text[],
    status_id uuid,
    metadata jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    requires_approval boolean DEFAULT true NOT NULL,
    CONSTRAINT metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text))
);


--
-- Name: api_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_keys (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    entity_id uuid,
    key_hash text NOT NULL,
    key_prefix text NOT NULL,
    name text NOT NULL,
    scopes uuid[] DEFAULT '{}'::uuid[],
    last_used_at timestamp with time zone,
    expires_at timestamp with time zone,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    agent_id uuid,
    CONSTRAINT api_keys_owner_check CHECK ((((entity_id IS NOT NULL) AND (agent_id IS NULL)) OR ((entity_id IS NULL) AND (agent_id IS NOT NULL))))
);


--
-- Name: approval_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.approval_requests (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    job_id text,
    request_type text NOT NULL,
    requested_by uuid,
    change_details jsonb DEFAULT '{}'::jsonb,
    status text DEFAULT 'pending'::text,
    reviewed_by uuid,
    reviewed_at timestamp with time zone,
    review_notes text,
    created_at timestamp with time zone DEFAULT now(),
    execution_error text,
    review_details jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT approval_requests_review_details_is_object CHECK ((jsonb_typeof(review_details) = 'object'::text)),
    CONSTRAINT approval_requests_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text, 'approved-failed'::text]))),
    CONSTRAINT change_details_is_object CHECK ((jsonb_typeof(change_details) = 'object'::text))
);


--
-- Name: COLUMN approval_requests.execution_error; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_requests.execution_error IS 'Error message if execution failed after approval. Used for agent feedback when status is approved-failed.';


--
-- Name: COLUMN approval_requests.review_details; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.approval_requests.review_details IS 'Structured reviewer decisions for approvals, including scope/trust grants.';


--
-- Name: audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_log (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    table_name text NOT NULL,
    record_id text NOT NULL,
    action text NOT NULL,
    changed_by_type text,
    changed_by_id uuid,
    old_data jsonb,
    new_data jsonb,
    changed_fields text[],
    change_reason text,
    changed_at timestamp with time zone DEFAULT now(),
    metadata jsonb DEFAULT '{}'::jsonb,
    CONSTRAINT audit_log_action_check CHECK ((action = ANY (ARRAY['insert'::text, 'update'::text, 'delete'::text]))),
    CONSTRAINT audit_log_changed_by_type_check CHECK ((changed_by_type = ANY (ARRAY['agent'::text, 'entity'::text, 'system'::text])))
);


--
-- Name: context_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.context_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    privacy_scope_ids uuid[] DEFAULT '{}'::uuid[],
    title text NOT NULL,
    url text,
    source_type text,
    content text,
    status_id uuid,
    status_changed_at timestamp with time zone,
    status_reason text,
    tags text[] DEFAULT '{}'::text[],
    source_path text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT context_items_tags_limit CHECK ((COALESCE(array_length(tags, 1), 0) <= 50))
);


--
-- Name: entities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.entities (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    privacy_scope_ids uuid[] DEFAULT '{}'::uuid[],
    name text NOT NULL,
    status_id uuid,
    status_changed_at timestamp with time zone,
    status_reason text,
    tags text[] DEFAULT '{}'::text[],
    embedding public.vector(1536),
    source_path text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    type_id uuid NOT NULL,
    CONSTRAINT entities_tags_limit CHECK ((COALESCE(array_length(tags, 1), 0) <= 50))
);


--
-- Name: entity_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.entity_types (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    is_builtin boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT entity_types_metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text))
);


--
-- Name: external_refs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.external_refs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    node_type text NOT NULL,
    node_id text NOT NULL,
    system text NOT NULL,
    external_id text NOT NULL,
    url text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT external_refs_metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text)),
    CONSTRAINT external_refs_node_type_check CHECK ((node_type = ANY (ARRAY['entity'::text, 'context'::text, 'log'::text, 'job'::text, 'agent'::text, 'file'::text, 'protocol'::text])))
);


--
-- Name: files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.files (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    filename text NOT NULL,
    file_path text NOT NULL,
    mime_type text,
    size_bytes bigint,
    checksum text,
    status_id uuid,
    status_changed_at timestamp with time zone,
    status_reason text,
    tags text[] DEFAULT '{}'::text[],
    metadata jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    uri text,
    CONSTRAINT files_tags_limit CHECK ((COALESCE(array_length(tags, 1), 0) <= 50)),
    CONSTRAINT metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text))
);


--
-- Name: jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobs (
    id text DEFAULT public.generate_job_id() NOT NULL,
    title text NOT NULL,
    description text,
    job_type text,
    assigned_to uuid,
    agent_id uuid,
    status_id uuid,
    priority text,
    parent_job_id text,
    due_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    status_reason text,
    status_changed_at timestamp with time zone,
    privacy_scope_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    CONSTRAINT jobs_priority_check CHECK ((priority = ANY (ARRAY['low'::text, 'medium'::text, 'high'::text, 'critical'::text])))
);


--
-- Name: log_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.log_types (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    value_schema jsonb,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    is_builtin boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL
);


--
-- Name: logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    "timestamp" timestamp with time zone NOT NULL,
    value jsonb DEFAULT '{}'::jsonb,
    status_id uuid,
    status_changed_at timestamp with time zone,
    status_reason text,
    tags text[] DEFAULT '{}'::text[],
    metadata jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    log_type_id uuid NOT NULL,
    CONSTRAINT logs_tags_limit CHECK ((COALESCE(array_length(tags, 1), 0) <= 50)),
    CONSTRAINT metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text)),
    CONSTRAINT value_is_object CHECK ((jsonb_typeof(value) = 'object'::text))
);


--
-- Name: privacy_scopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.privacy_scopes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    is_builtin boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT privacy_scopes_metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text))
);


--
-- Name: protocols; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.protocols (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    title text NOT NULL,
    version text,
    content text NOT NULL,
    protocol_type text,
    applies_to text[] DEFAULT '{}'::text[],
    status_id uuid,
    tags text[] DEFAULT '{}'::text[],
    metadata jsonb DEFAULT '{}'::jsonb,
    source_path text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    trusted boolean DEFAULT false,
    CONSTRAINT metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text)),
    CONSTRAINT protocols_tags_limit CHECK ((COALESCE(array_length(tags, 1), 0) <= 50))
);


--
-- Name: relationship_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.relationship_types (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    is_symmetric boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    is_builtin boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT relationship_types_metadata_is_object CHECK ((jsonb_typeof(metadata) = 'object'::text))
);


--
-- Name: relationships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.relationships (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    source_type text NOT NULL,
    source_id text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    type_id uuid NOT NULL,
    status_id uuid,
    status_changed_at timestamp with time zone,
    properties jsonb DEFAULT '{}'::jsonb,
    embedding public.vector(1536),
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT relationships_source_type_check CHECK ((source_type = ANY (ARRAY['entity'::text, 'context'::text, 'log'::text, 'job'::text, 'agent'::text, 'file'::text, 'protocol'::text]))),
    CONSTRAINT relationships_target_type_check CHECK ((target_type = ANY (ARRAY['entity'::text, 'context'::text, 'log'::text, 'job'::text, 'agent'::text, 'file'::text, 'protocol'::text])))
);


--
-- Name: semantic_search; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.semantic_search (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    source_type text NOT NULL,
    source_id text NOT NULL,
    segment_index integer,
    embedding public.vector(1536) NOT NULL,
    scopes uuid[] DEFAULT '{}'::uuid[],
    metadata jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT semantic_search_source_type_check CHECK ((source_type = ANY (ARRAY['entity'::text, 'context'::text, 'log'::text, 'job'::text, 'agent'::text, 'file'::text, 'protocol'::text])))
);


--
-- Name: statuses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.statuses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    category text NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT statuses_category_check CHECK ((category = ANY (ARRAY['active'::text, 'archived'::text])))
);


--
-- Name: agent_enrollment_sessions agent_enrollment_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_enrollment_sessions
    ADD CONSTRAINT agent_enrollment_sessions_pkey PRIMARY KEY (id);


--
-- Name: agents agents_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agents
    ADD CONSTRAINT agents_name_key UNIQUE (name);


--
-- Name: agents agents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agents
    ADD CONSTRAINT agents_pkey PRIMARY KEY (id);


--
-- Name: api_keys api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);


--
-- Name: approval_requests approval_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_pkey PRIMARY KEY (id);


--
-- Name: audit_log audit_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log
    ADD CONSTRAINT audit_log_pkey PRIMARY KEY (id);


--
-- Name: entities entities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entities
    ADD CONSTRAINT entities_pkey PRIMARY KEY (id);


--
-- Name: entity_types entity_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity_types
    ADD CONSTRAINT entity_types_name_key UNIQUE (name);


--
-- Name: entity_types entity_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entity_types
    ADD CONSTRAINT entity_types_pkey PRIMARY KEY (id);


--
-- Name: external_refs external_refs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.external_refs
    ADD CONSTRAINT external_refs_pkey PRIMARY KEY (id);


--
-- Name: files files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.files
    ADD CONSTRAINT files_pkey PRIMARY KEY (id);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- Name: context_items knowledge_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.context_items
    ADD CONSTRAINT knowledge_items_pkey PRIMARY KEY (id);


--
-- Name: log_types log_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.log_types
    ADD CONSTRAINT log_types_name_key UNIQUE (name);


--
-- Name: log_types log_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.log_types
    ADD CONSTRAINT log_types_pkey PRIMARY KEY (id);


--
-- Name: logs logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.logs
    ADD CONSTRAINT logs_pkey PRIMARY KEY (id);


--
-- Name: privacy_scopes privacy_scopes_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.privacy_scopes
    ADD CONSTRAINT privacy_scopes_name_key UNIQUE (name);


--
-- Name: privacy_scopes privacy_scopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.privacy_scopes
    ADD CONSTRAINT privacy_scopes_pkey PRIMARY KEY (id);


--
-- Name: protocols protocols_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.protocols
    ADD CONSTRAINT protocols_name_key UNIQUE (name);


--
-- Name: protocols protocols_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.protocols
    ADD CONSTRAINT protocols_pkey PRIMARY KEY (id);


--
-- Name: relationship_types relationship_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.relationship_types
    ADD CONSTRAINT relationship_types_name_key UNIQUE (name);


--
-- Name: relationship_types relationship_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.relationship_types
    ADD CONSTRAINT relationship_types_pkey PRIMARY KEY (id);


--
-- Name: relationships relationships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.relationships
    ADD CONSTRAINT relationships_pkey PRIMARY KEY (id);


--
-- Name: relationships relationships_source_type_source_id_target_type_target_id_t_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.relationships
    ADD CONSTRAINT relationships_source_type_source_id_target_type_target_id_t_key UNIQUE (source_type, source_id, target_type, target_id, type_id);


--
-- Name: semantic_search semantic_search_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.semantic_search
    ADD CONSTRAINT semantic_search_pkey PRIMARY KEY (id);


--
-- Name: statuses statuses_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.statuses
    ADD CONSTRAINT statuses_name_key UNIQUE (name);


--
-- Name: statuses statuses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.statuses
    ADD CONSTRAINT statuses_pkey PRIMARY KEY (id);


--
-- Name: idx_agent_enroll_approval_request; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_agent_enroll_approval_request ON public.agent_enrollment_sessions USING btree (approval_request_id);


--
-- Name: idx_agent_enroll_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_enroll_expires_at ON public.agent_enrollment_sessions USING btree (expires_at);


--
-- Name: idx_agent_enroll_pending_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_agent_enroll_pending_agent ON public.agent_enrollment_sessions USING btree (agent_id) WHERE (status = 'pending_approval'::text);


--
-- Name: idx_agent_enroll_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_enroll_status ON public.agent_enrollment_sessions USING btree (status);


--
-- Name: idx_agents_capabilities; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agents_capabilities ON public.agents USING gin (capabilities);


--
-- Name: idx_agents_requires_approval; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agents_requires_approval ON public.agents USING btree (requires_approval) WHERE (requires_approval = true);


--
-- Name: idx_agents_scopes; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agents_scopes ON public.agents USING gin (scopes);


--
-- Name: idx_agents_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agents_status ON public.agents USING btree (status_id);


--
-- Name: idx_api_keys_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_keys_agent ON public.api_keys USING btree (agent_id);


--
-- Name: idx_api_keys_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_keys_entity ON public.api_keys USING btree (entity_id);


--
-- Name: idx_api_keys_prefix; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_keys_prefix ON public.api_keys USING btree (key_prefix);


--
-- Name: idx_approval_job; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_approval_job ON public.approval_requests USING btree (job_id);


--
-- Name: idx_approval_requested_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_approval_requested_by ON public.approval_requests USING btree (requested_by);


--
-- Name: idx_approval_reviewed_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_approval_reviewed_by ON public.approval_requests USING btree (reviewed_by);


--
-- Name: idx_approval_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_approval_status ON public.approval_requests USING btree (status);


--
-- Name: idx_audit_action; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_action ON public.audit_log USING btree (action);


--
-- Name: idx_audit_changed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_changed_at ON public.audit_log USING btree (changed_at DESC);


--
-- Name: idx_audit_changed_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_changed_by ON public.audit_log USING btree (changed_by_type, changed_by_id);


--
-- Name: idx_audit_log_metadata_approval; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_log_metadata_approval ON public.audit_log USING gin (metadata);


--
-- Name: idx_audit_table_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_table_name ON public.audit_log USING btree (table_name);


--
-- Name: idx_audit_table_record; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_table_record ON public.audit_log USING btree (table_name, record_id);


--
-- Name: idx_context_privacy; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_context_privacy ON public.context_items USING gin (privacy_scope_ids);


--
-- Name: idx_context_search; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_context_search ON public.context_items USING gin (to_tsvector('english'::regconfig, ((title || ' '::text) || COALESCE(content, ''::text))));


--
-- Name: idx_context_source_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_context_source_type ON public.context_items USING btree (source_type);


--
-- Name: idx_context_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_context_status ON public.context_items USING btree (status_id);


--
-- Name: idx_context_tags; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_context_tags ON public.context_items USING gin (tags);


--
-- Name: idx_entities_embedding; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entities_embedding ON public.entities USING ivfflat (embedding public.vector_cosine_ops) WITH (lists='100');


--
--
-- Name: idx_entities_privacy; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entities_privacy ON public.entities USING gin (privacy_scope_ids);


--
-- Name: idx_entities_search; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entities_search ON public.entities USING gin (to_tsvector('english'::regconfig, name));


--
-- Name: idx_entities_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entities_status ON public.entities USING btree (status_id);


--
-- Name: idx_entities_tags; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entities_tags ON public.entities USING gin (tags);


--
-- Name: idx_entities_type_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entities_type_id ON public.entities USING btree (type_id);


--
-- Name: idx_entity_types_active_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entity_types_active_name ON public.entity_types USING btree (is_active, name);


--
-- Name: idx_entity_types_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_entity_types_name ON public.entity_types USING btree (name);


--
-- Name: idx_external_refs_node; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_external_refs_node ON public.external_refs USING btree (node_type, node_id);


--
-- Name: idx_external_refs_system; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_external_refs_system ON public.external_refs USING btree (system);


--
-- Name: idx_files_checksum; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_files_checksum ON public.files USING btree (checksum);


--
-- Name: idx_files_mime_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_files_mime_type ON public.files USING btree (mime_type);


--
-- Name: idx_files_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_files_status ON public.files USING btree (status_id);


--
-- Name: idx_files_tags; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_files_tags ON public.files USING gin (tags);


--
-- Name: idx_files_uri; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_files_uri ON public.files USING btree (uri);


--
-- Name: idx_jobs_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_agent ON public.jobs USING btree (agent_id);


--
-- Name: idx_jobs_assigned_to; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_assigned_to ON public.jobs USING btree (assigned_to);


--
-- Name: idx_jobs_due_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_due_at ON public.jobs USING btree (due_at);


--
-- Name: idx_jobs_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_parent ON public.jobs USING btree (parent_job_id);


--
-- Name: idx_jobs_priority; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_priority ON public.jobs USING btree (priority);


--
-- Name: idx_jobs_privacy; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_privacy ON public.jobs USING gin (privacy_scope_ids);


--
-- Name: idx_jobs_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_status ON public.jobs USING btree (status_id);


--
-- Name: idx_jobs_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_type ON public.jobs USING btree (job_type);


--
-- Name: idx_log_types_active_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_log_types_active_name ON public.log_types USING btree (is_active, name);


--
-- Name: idx_log_types_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_log_types_name ON public.log_types USING btree (name);


--
-- Name: idx_logs_metadata; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_logs_metadata ON public.logs USING gin (metadata);


--
-- Name: idx_logs_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_logs_status ON public.logs USING btree (status_id);


--
-- Name: idx_logs_tags; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_logs_tags ON public.logs USING gin (tags);


--
-- Name: idx_logs_timestamp; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_logs_timestamp ON public.logs USING btree ("timestamp" DESC);


--
-- Name: idx_logs_type_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_logs_type_id ON public.logs USING btree (log_type_id);


--
-- Name: idx_logs_type_id_timestamp; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_logs_type_id_timestamp ON public.logs USING btree (log_type_id, "timestamp" DESC);


--
-- Name: idx_privacy_scopes_active_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_privacy_scopes_active_name ON public.privacy_scopes USING btree (is_active, name);


--
-- Name: idx_protocols_applies_to; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_protocols_applies_to ON public.protocols USING gin (applies_to);


--
-- Name: idx_protocols_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_protocols_name ON public.protocols USING btree (name);


--
-- Name: idx_protocols_search; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_protocols_search ON public.protocols USING gin (to_tsvector('english'::regconfig, ((((title || ' '::text) || content) || ' '::text) || COALESCE((metadata)::text, ''::text))));


--
-- Name: idx_protocols_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_protocols_status ON public.protocols USING btree (status_id);


--
-- Name: idx_protocols_tags; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_protocols_tags ON public.protocols USING gin (tags);


--
-- Name: idx_protocols_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_protocols_type ON public.protocols USING btree (protocol_type);


--
-- Name: idx_relationship_types_active_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_relationship_types_active_name ON public.relationship_types USING btree (is_active, name);


--
-- Name: idx_relationships_full; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_relationships_full ON public.relationships USING btree (source_type, source_id, target_type, target_id, type_id);


--
-- Name: idx_relationships_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_relationships_source ON public.relationships USING btree (source_type, source_id);


--
-- Name: idx_relationships_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_relationships_status ON public.relationships USING btree (status_id);


--
-- Name: idx_relationships_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_relationships_target ON public.relationships USING btree (target_type, target_id);


--
-- Name: idx_relationships_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_relationships_type ON public.relationships USING btree (type_id);


--
-- Name: idx_semantic_search_scopes; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_semantic_search_scopes ON public.semantic_search USING gin (scopes);


--
-- Name: idx_semantic_search_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_semantic_search_source ON public.semantic_search USING btree (source_type, source_id);


--
-- Name: idx_semantic_search_vector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_semantic_search_vector ON public.semantic_search USING ivfflat (embedding public.vector_cosine_ops) WITH (lists='100');


--
-- Name: uq_entity_types_name_ci; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_entity_types_name_ci ON public.entity_types USING btree (lower(name));


--
-- Name: uq_external_refs_system_external_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_external_refs_system_external_id ON public.external_refs USING btree (system, external_id);


--
-- Name: uq_log_types_name_ci; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_log_types_name_ci ON public.log_types USING btree (lower(name));


--
-- Name: uq_privacy_scopes_name_ci; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_privacy_scopes_name_ci ON public.privacy_scopes USING btree (lower(name));


--
-- Name: uq_relationship_types_name_ci; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_relationship_types_name_ci ON public.relationship_types USING btree (lower(name));


--
-- Name: agent_enrollment_sessions audit_agent_enrollment_sessions_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_agent_enrollment_sessions_trigger AFTER INSERT OR DELETE OR UPDATE ON public.agent_enrollment_sessions FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: agents audit_agents_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_agents_trigger AFTER INSERT OR DELETE OR UPDATE ON public.agents FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: approval_requests audit_approval_requests_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_approval_requests_trigger AFTER INSERT OR DELETE OR UPDATE ON public.approval_requests FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: context_items audit_context_items_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_context_items_trigger AFTER INSERT OR DELETE OR UPDATE ON public.context_items FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: entities audit_entities_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_entities_trigger AFTER INSERT OR DELETE OR UPDATE ON public.entities FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: jobs audit_jobs_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_jobs_trigger AFTER INSERT OR DELETE OR UPDATE ON public.jobs FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: protocols audit_protocols_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_protocols_trigger AFTER INSERT OR DELETE OR UPDATE ON public.protocols FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: relationships audit_relationships_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_relationships_trigger AFTER INSERT OR DELETE OR UPDATE ON public.relationships FOR EACH ROW EXECUTE FUNCTION public.audit_trigger_function();


--
-- Name: agents cascade_agent_status_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cascade_agent_status_trigger AFTER UPDATE OF status_id ON public.agents FOR EACH ROW EXECUTE FUNCTION public.cascade_status_to_relationships();


--
-- Name: context_items cascade_context_status_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cascade_context_status_trigger AFTER UPDATE OF status_id ON public.context_items FOR EACH ROW EXECUTE FUNCTION public.cascade_status_to_relationships();


--
-- Name: entities cascade_entity_status_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cascade_entity_status_trigger AFTER UPDATE OF status_id ON public.entities FOR EACH ROW EXECUTE FUNCTION public.cascade_status_to_relationships();


--
-- Name: files cascade_file_status_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cascade_file_status_trigger AFTER UPDATE OF status_id ON public.files FOR EACH ROW EXECUTE FUNCTION public.cascade_status_to_relationships();


--
-- Name: jobs cascade_job_status_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cascade_job_status_trigger AFTER UPDATE OF status_id ON public.jobs FOR EACH ROW EXECUTE FUNCTION public.cascade_status_to_relationships();


--
-- Name: logs cascade_log_status_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cascade_log_status_trigger AFTER UPDATE OF status_id ON public.logs FOR EACH ROW EXECUTE FUNCTION public.cascade_status_to_relationships();


--
-- Name: protocols cascade_protocol_status_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cascade_protocol_status_trigger AFTER UPDATE OF status_id ON public.protocols FOR EACH ROW EXECUTE FUNCTION public.cascade_status_to_relationships();


--
-- Name: relationships sync_symmetric_relationships_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sync_symmetric_relationships_trigger AFTER INSERT OR DELETE OR UPDATE ON public.relationships FOR EACH ROW EXECUTE FUNCTION public.sync_symmetric_relationships();


--
-- Name: agent_enrollment_sessions update_agent_enrollment_sessions_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_agent_enrollment_sessions_updated_at BEFORE UPDATE ON public.agent_enrollment_sessions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: agents update_agents_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON public.agents FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: context_items update_context_items_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_context_items_updated_at BEFORE UPDATE ON public.context_items FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: entities update_entities_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_entities_updated_at BEFORE UPDATE ON public.entities FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: entity_types update_entity_types_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_entity_types_updated_at BEFORE UPDATE ON public.entity_types FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: external_refs update_external_refs_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_external_refs_updated_at BEFORE UPDATE ON public.external_refs FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: files update_files_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_files_updated_at BEFORE UPDATE ON public.files FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: jobs update_jobs_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_jobs_updated_at BEFORE UPDATE ON public.jobs FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: log_types update_log_types_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_log_types_updated_at BEFORE UPDATE ON public.log_types FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: logs update_logs_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_logs_updated_at BEFORE UPDATE ON public.logs FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: privacy_scopes update_privacy_scopes_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_privacy_scopes_updated_at BEFORE UPDATE ON public.privacy_scopes FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: protocols update_protocols_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_protocols_updated_at BEFORE UPDATE ON public.protocols FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: relationship_types update_relationship_types_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_relationship_types_updated_at BEFORE UPDATE ON public.relationship_types FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: relationships update_relationships_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_relationships_updated_at BEFORE UPDATE ON public.relationships FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: statuses update_statuses_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_statuses_updated_at BEFORE UPDATE ON public.statuses FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: relationships validate_relationships_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER validate_relationships_trigger BEFORE INSERT OR UPDATE ON public.relationships FOR EACH ROW EXECUTE FUNCTION public.validate_relationship_references();


--
-- Name: agent_enrollment_sessions agent_enrollment_sessions_agent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_enrollment_sessions
    ADD CONSTRAINT agent_enrollment_sessions_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES public.agents(id) ON DELETE CASCADE;


--
-- Name: agent_enrollment_sessions agent_enrollment_sessions_approval_request_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_enrollment_sessions
    ADD CONSTRAINT agent_enrollment_sessions_approval_request_id_fkey FOREIGN KEY (approval_request_id) REFERENCES public.approval_requests(id) ON DELETE CASCADE;


--
-- Name: agent_enrollment_sessions agent_enrollment_sessions_approved_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_enrollment_sessions
    ADD CONSTRAINT agent_enrollment_sessions_approved_by_fkey FOREIGN KEY (approved_by) REFERENCES public.entities(id);


--
-- Name: agents agents_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agents
    ADD CONSTRAINT agents_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: api_keys api_keys_agent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES public.agents(id);


--
-- Name: api_keys api_keys_entity_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_entity_id_fkey FOREIGN KEY (entity_id) REFERENCES public.entities(id);


--
-- Name: approval_requests approval_requests_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id);


--
-- Name: approval_requests approval_requests_requested_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_requested_by_fkey FOREIGN KEY (requested_by) REFERENCES public.agents(id);


--
-- Name: approval_requests approval_requests_reviewed_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES public.entities(id);


--
-- Name: entities entities_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entities
    ADD CONSTRAINT entities_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: entities entities_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.entities
    ADD CONSTRAINT entities_type_id_fkey FOREIGN KEY (type_id) REFERENCES public.entity_types(id);


--
-- Name: files files_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.files
    ADD CONSTRAINT files_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: jobs jobs_agent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES public.agents(id);


--
-- Name: jobs jobs_assigned_to_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_assigned_to_fkey FOREIGN KEY (assigned_to) REFERENCES public.entities(id);


--
-- Name: jobs jobs_parent_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_parent_job_id_fkey FOREIGN KEY (parent_job_id) REFERENCES public.jobs(id);


--
-- Name: jobs jobs_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: context_items knowledge_items_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.context_items
    ADD CONSTRAINT knowledge_items_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: logs logs_log_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.logs
    ADD CONSTRAINT logs_log_type_id_fkey FOREIGN KEY (log_type_id) REFERENCES public.log_types(id);


--
-- Name: logs logs_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.logs
    ADD CONSTRAINT logs_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: protocols protocols_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.protocols
    ADD CONSTRAINT protocols_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: relationships relationships_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.relationships
    ADD CONSTRAINT relationships_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.statuses(id);


--
-- Name: relationships relationships_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.relationships
    ADD CONSTRAINT relationships_type_id_fkey FOREIGN KEY (type_id) REFERENCES public.relationship_types(id) ON DELETE RESTRICT;


--
-- PostgreSQL database dump complete
--

\unrestrict JHjZmUYDj96H6T96jEgpQLtKOJLZ2ha4XckyCh33fGmkMeyKaLIusqbfNm0P7EO
