-- Add stable logical identity for backup version history (Phase A)
ALTER TABLE "public"."job_items" ADD COLUMN "backup_key" text NULL;
CREATE INDEX "idx_job_items_backup_key" ON "public"."job_items" ("backup_key");