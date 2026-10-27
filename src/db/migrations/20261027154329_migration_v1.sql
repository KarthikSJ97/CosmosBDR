-- Add new schema named "atlas_schema_revisions"
CREATE SCHEMA "atlas_schema_revisions";
-- Create "chunks" table
CREATE TABLE "public"."chunks" (
  "id" uuid NOT NULL DEFAULT public.uuid_generate_v4(),
  "chunk_hash" character varying(64) NOT NULL,
  "seaweed_key" text NOT NULL,
  "size_bytes" bigint NOT NULL,
  "compressed_size_bytes" bigint NOT NULL,
  "reference_count" bigint NULL DEFAULT 1,
  "created_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_chunks_chunk_hash" to table: "chunks"
CREATE UNIQUE INDEX "idx_chunks_chunk_hash" ON "public"."chunks" ("chunk_hash");
-- Create "atlas_schema_revisions" table
CREATE TABLE "atlas_schema_revisions"."atlas_schema_revisions" (
  "version" character varying NOT NULL,
  "description" character varying NOT NULL,
  "type" bigint NOT NULL DEFAULT 2,
  "applied" bigint NOT NULL DEFAULT 0,
  "total" bigint NOT NULL DEFAULT 0,
  "executed_at" timestamptz NOT NULL,
  "execution_time" bigint NOT NULL,
  "error" text NULL,
  "error_stmt" text NULL,
  "hash" character varying NOT NULL,
  "partial_hashes" jsonb NULL,
  "operator_version" character varying NOT NULL,
  PRIMARY KEY ("version")
);
-- Create "jobs" table
CREATE TABLE "public"."jobs" (
  "id" uuid NOT NULL DEFAULT public.uuid_generate_v4(),
  "flow_type" character varying(50) NOT NULL,
  "status" character varying(50) NOT NULL DEFAULT 'IN_PROGRESS',
  "source_path" text NOT NULL,
  "target_path" text NULL,
  "total_items" bigint NULL DEFAULT 0,
  "total_bytes" bigint NULL DEFAULT 0,
  "error_message" text NULL,
  "payload" jsonb NULL DEFAULT '{}',
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_jobs_deleted_at" to table: "jobs"
CREATE INDEX "idx_jobs_deleted_at" ON "public"."jobs" ("deleted_at");
-- Create index "idx_jobs_flow_status" to table: "jobs"
CREATE INDEX "idx_jobs_flow_status" ON "public"."jobs" ("flow_type", "status");
-- Create "job_items" table
CREATE TABLE "public"."job_items" (
  "id" uuid NOT NULL DEFAULT public.uuid_generate_v4(),
  "job_id" uuid NOT NULL,
  "item_path" text NOT NULL,
  "original_size" bigint NOT NULL,
  "processed_size" bigint NOT NULL,
  "checksum" character varying(64) NOT NULL,
  "status" character varying(50) NOT NULL DEFAULT 'PENDING',
  "item_metadata" jsonb NULL DEFAULT '{}',
  "created_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_jobs_job_items" FOREIGN KEY ("job_id") REFERENCES "public"."jobs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_job_items_job_id" to table: "job_items"
CREATE INDEX "idx_job_items_job_id" ON "public"."job_items" ("job_id");
-- Create "item_chunks" table
CREATE TABLE "public"."item_chunks" (
  "item_id" uuid NOT NULL,
  "chunk_id" uuid NOT NULL,
  "chunk_index" bigint NOT NULL,
  "offset_bytes" bigint NOT NULL,
  "length_bytes" bigint NOT NULL,
  PRIMARY KEY ("item_id", "chunk_index"),
  CONSTRAINT "fk_chunks_item_chunks" FOREIGN KEY ("chunk_id") REFERENCES "public"."chunks" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_job_items_item_chunks" FOREIGN KEY ("item_id") REFERENCES "public"."job_items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_item_chunks_chunk_id" to table: "item_chunks"
CREATE INDEX "idx_item_chunks_chunk_id" ON "public"."item_chunks" ("chunk_id");
