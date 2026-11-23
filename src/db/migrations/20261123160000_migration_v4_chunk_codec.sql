-- Phase G: chunk encryption/compression metadata
ALTER TABLE "public"."chunks"
  ADD COLUMN "compression_alg" character varying(16) NOT NULL DEFAULT 'none',
  ADD COLUMN "encrypted" boolean NOT NULL DEFAULT false;