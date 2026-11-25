-- Phase I: Seaweed S3 object-lock metadata on chunk rows
ALTER TABLE "public"."chunks"
  ADD COLUMN "seaweed_version_id" text,
  ADD COLUMN "retain_until" timestamptz;