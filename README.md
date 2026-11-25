# CosmosBDR

**CosmosBDR** is a self-hosted backup and disaster-recovery engine: versioned, encrypted file protection across **local paths**, **remote SFTP hosts**, and **scheduled database dumps**—with immutable object storage and workflow observability.

<p align="center">
  <img src="docs/images/cadence-workflows-completed.png" alt="Cadence UI — backup, restore, and retention GC workflows" width="720" />
</p>

---

## Why CosmosBDR exists

Traditional backup tools often fall into one of two camps: **opaque appliances** you cannot audit, or **ad-hoc rsync/cron scripts** that do not give you versioning, deduplication, encryption, or provable retention.

CosmosBDR targets teams that need:

| Need | How CosmosBDR addresses it |
|------|----------------------------|
| **Multi-origin data** | One control plane for laptop folders, Windows SFTP shares, and Linux VMs |
| **Version history** | Every backup is a snapshot you can list and pin for restore |
| **Efficiency** | Content-hashed 4 MB chunks with skip-on-unchanged files |
| **Defense in depth** | AES-256-GCM + compression at rest; optional Seaweed S3 **COMPLIANCE** object lock |
| **Operational clarity** | Cadence workflows you can trace in the UI; schedules declared in Git-friendly YAML |
| **Database DR without vendor lock-in** | Back up **dump files** (Postgres, MySQL, anything that writes to disk)—not a proprietary agent |

CosmosBDR is **not** a managed-cloud PITR replacement (RDS, Cloud SQL). It excels when **you own the VMs and paths** and want logical exports and file trees off-site with policy you control.

---

## Features

- **Single-file backup** from `local` (`HOST_DATA_PATH` → `/host-data`) or `sftp` (`remote_root` + `relative_path`)
- **Directory backup** with `all` or `matched` selection, **glob** and **regex** include/exclude, depth and size filters
- **Declarative schedules** — Cadence cron per row in `sources.yaml` (`enabled: true` registers on startup)
- **`pre_backup` hooks** (SFTP only) — SSH exec once per job for `pg_dump`, `mysqldump`, unzip, tar extract, custom scripts (defined entirely in YAML)
- **`dump_freshness`** — fail the job if a dump artifact is older than `max_age_hours` (avoids silent SKIP on stale dumps)
- **Restore** — single file or directory tree to local mount or remote SFTP; pin version via `source_job_item_id` or `backup_key`
- **Retention GC** — nightly Cadence cron expires snapshots past per-source `ttl_days` and reclaims unreferenced chunks (respects WORM)
- **HTTP API** — trigger backup/restore, list versions, reconcile schedules ([API reference](docs/api-reference.md))

---

## Architecture (how it works)

```mermaid
flowchart LR
  subgraph ingress [Ingress]
    API[Fiber HTTP API]
    CRON[Cadence cron schedules]
  end
  subgraph orchestration [Orchestration]
    CW[Cadence worker]
    BW[BackupWorkflow]
    DBW[DirectoryBackupWorkflow]
    RW[RestoreWorkflow]
    GC[RetentionGCWorkflow]
  end
  subgraph data [Data plane]
    SRC[Source readers local / SFTP]
    CH[Chunk codec gzip or zstd + AES-256-GCM]
    SW[(SeaweedFS S3)]
    PG[(Postgres metadata)]
  end
  API --> CW
  CRON --> CW
  CW --> BW & DBW & RW & GC
  BW --> SRC --> CH --> SW
  BW --> PG
  GC --> SW
  GC --> PG
```

<p align="center">
  <img src="docs/images/architecture-diagram.png" alt="CosmosBDR architecture — ingress, Cadence workflows, chunk pipeline, Postgres and SeaweedFS" width="720" />
</p>

**Single-file backup path**

1. **Init job** in Postgres (flow, paths, `backup_key`).
2. Optional **`RunPreBackupActivity`** — one remote SSH exec when `pre_backup` is configured on the source.
3. **`InspectFile`** — size → mtime → full-file SHA-256; matching prior snapshot → **SKIP** (clone chunk refs, no re-upload). See [skip rules](docs/backup-workflow.md#skip-vs-complete-unchanged-files).
4. **`ProcessFileChunk`** — 4 MB segments, dedupe by plaintext hash, encrypt/compress, put to Seaweed.
5. **`PublishSnapshotManifest`** — version metadata for list/restore APIs.
6. Optional **remote cleanup** of staging file when `output.cleanup_after: true`.

**Directory jobs** run `ListDirectory` first, then fan out child `BackupWorkflow` instances per selected file (with Cadence concurrency limits from `.env`).

**Restore** resolves the target version (latest eligible or pinned ID), reads chunks from Seaweed, decrypts, and writes to `target_path` (local or SFTP).

See [Backup workflow](docs/backup-workflow.md), [Restore](docs/restore.md), and [Garbage collection](docs/garbage-collection.md) for depth. Cadence UI examples are in [Screenshots](#screenshots) below.

---

## Technology stack

| Component | Role | Why this choice |
|-----------|------|-----------------|
| **Go** | API, workers, chunk pipeline | Single static binary, strong concurrency, fits SFTP `ReadAt` chunk I/O |
| **Fiber** | HTTP API | Fast, low ceremony for a small control-plane surface |
| **Uber Cadence** | Durable workflows + cron | Retries, visibility, schedule overlap policy; long backup jobs need orchestration, not fire-and-forget goroutines |
| **PostgreSQL** | Jobs, items, chunk index | Relational queries for version lists, retention, and dedup metadata |
| **SeaweedFS (S3 API)** | Blob store | S3-compatible, self-hosted, supports **Object Lock** for WORM compliance mode |
| **Atlas** | SQL migrations | Applied via `docker-compose` before the app starts |
| **AWS SDK v2 (S3)** | Seaweed client | Mature multipart + object-lock headers |
| **`pkg/sftp`** | Remote sources | Stat, mtime, ranged reads—required for chunked backup without full-file staging |
| **`chunkcodec`** | gzip/zstd + AES-256-GCM | Dedup on plaintext hash; encrypt after dedup decision |

---

## Quick start (Docker Compose)

### Prerequisites

- Docker Desktop (or Docker Engine) with Compose
- Absolute host paths for Postgres, Seaweed, and backup source data

### 1. Configure environment

```bash
cp .env.example .env
```

Edit `.env`:

- `HOST_DATA_PATH` — folder on the host mounted at `/host-data` in the app container
- `SEAWEED_DATA_PATH`, `POSTGRES_DATA_PATH` — persistent data directories (**do not nest** under `HOST_DATA_PATH`)
- `ENCRYPTION_KEY` — 32-byte hex (or 32-char dev string)
- `SEAWEED_ACCESS_KEY` / `SEAWEED_SECRET_KEY` — S3 credentials for Seaweed
- Per-source `SOURCE_ID_*` variables (see [Configuration](docs/configuration.md))

Review and adjust `sources.yaml` (committed template with local + example SFTP sources). Set `enabled: true` on schedules only when you intend to run crons.

Field-by-field reference: **[Configuration guide](docs/configuration.md)**.

### 2. Start the stack

```bash
docker compose up -d --build
docker compose ps   # all services Up; postgres + seaweedfs (healthy)
```

<p align="center">
  <img src="docs/images/docker-compose-ps.png" alt="docker compose ps — cosmosbdr, cadence, postgres, seaweedfs" width="720" />
</p>

Services:

| Service | Port | Purpose |
|---------|------|---------|
| `cosmosbdr` | `8080` | HTTP API + Cadence worker |
| `cadence-web` | `8088` | Workflow UI |
| `seaweedfs` | `8333` (S3), `8888` (filer) | Object storage |
| `postgres` | `5432` | Metadata (+ Cadence DBs) |

### 3. Verify health

```bash
curl -s http://localhost:8080/health | jq .
curl -s http://localhost:8080/api/v1/sources | jq .
```

### 4. Trigger a backup (local file)

Place a test file under `HOST_DATA_PATH`, then:

```bash
curl -s -X POST http://localhost:8080/api/v1/backups \
  -H 'Content-Type: application/json' \
  -d '{
    "flow_type": "BACKUP",
    "payload": {
      "source_id": "local-host-data",
      "relative_path": "e2e_test_asset_v2.dat"
    }
  }' | jq .
```

Watch execution in **Cadence Web** at `http://localhost:8088` (domain `samples-domain`).

More examples: import [`docs/bruno-api-collection/CosmosBDR.zip`](docs/bruno-api-collection/CosmosBDR.zip) into [Bruno](https://www.usebruno.com/) (see [collection README](docs/bruno-api-collection/README.md)) or read [API reference](docs/api-reference.md).

### 5. List backup versions

```bash
curl -s 'http://localhost:8080/api/v1/backups?backup_key=local-host-data:e2e_test_asset_v2.dat' | jq .
```

---

## Screenshots

| Topic | |
|-------|---|
| Stack health (`docker compose ps`) | ![Compose ps](docs/images/docker-compose-ps.png) |
| API — health check | ![Health check](docs/images/bruno-health-check.png) |
| API — single-file backup (`202`) | ![Bruno backup](docs/images/bruno-backup-single-file-202.png) |
| API — DB dump backup (`prod-db-vm`) | See [backup workflow](docs/backup-workflow.md#database-dumps-postgres-mysql-and-more) |
| API — list versions (`GET /backups`) | ![List backups](docs/images/bruno-list-backups-versions.png) |
| API — directory backup (`matched` scope) | ![Directory backup](docs/images/bruno-directory-backup-202.png) |
| API — restore (`202`) | ![Restore API](docs/images/bruno-restore-api.png) |
| API — directory restore (pinned parent job) | ![Directory restore](docs/images/bruno-directory-restore-202.png) · ![3/3 files](docs/images/cadence-directory-restore-completed.png) |
| Cadence — directory parent + child file jobs | ![Directory children](docs/images/cadence-directory-children-completed.png) · ![Directory timeline](docs/images/cadence-directory-backup-timeline.png) |
| Windows SFTP directory layout | See [backup workflow — directory](docs/backup-workflow.md#directory-backup) |
| Schedules — list / register / reconcile | ![List schedules](docs/images/bruno-list-schedules.png) · [API reference](docs/api-reference.md#schedules) |
| Cadence — dedup **SKIP** (`InspectFile`) | ![Skip dedup](docs/images/cadence-inspect-skip-dedup.png) |
| Cadence — `RunPreBackupActivity` (remote hook) | See [backup workflow](docs/backup-workflow.md#database-dumps-postgres-mysql-and-more) |
| Cadence — restore chunks + completed workflow | ![Restore chunks](docs/images/cadence-restore-chunk-timeline.png) · ![Restore completed](docs/images/cadence-restore-completed.png) |
| Integrity — `shasum` matches API `checksum` | See [restore.md — verify dump](docs/restore.md#verify-restored-dump-checksum) |
| Cadence — scheduled / GC workflows | ![Scheduled backups](docs/images/cadence-scheduled-backups.png) · ![Retention GC](docs/images/cadence-retention-gc-cron.png) |
| Seaweed filer — logical snapshot paths | ![Seaweed filer](docs/images/seaweedfs-filer-snapshot.png) |

---

## Documentation

| Guide | Description |
|-------|-------------|
| **[Configuration](docs/configuration.md)** | Every `sources.yaml` field and `.env` variable |
| **[Backup workflow](docs/backup-workflow.md)** | Single/directory jobs, schedules, globs/regex, `pre_backup`, DB dumps |
| **[Restore](docs/restore.md)** | Version pinning, directory restore, DB dump recovery |
| **[Garbage collection](docs/garbage-collection.md)** | TTL, retention cron, orphaned chunks, WORM interaction |
| **[API reference](docs/api-reference.md)** | Endpoints, payloads, example responses |
| **[WORM / Object Lock runbook](docs/worm-runbook.md)** | S3 COMPLIANCE mode, testing, filer hygiene |
| **[SFTP sources runbook](docs/sftp-sources-runbook.md)** | Windows/Linux SSH, Docker networking |
| **[Architecture reference](docs/architecture.md)** | Design principles, capability map, storage layout |
| **[Bruno collection](docs/bruno-api-collection/README.md)** | [`CosmosBDR.zip`](docs/bruno-api-collection/CosmosBDR.zip) — import into Bruno |

---

**More detail not on this page:** WORM curls and compliance screenshots — [worm-runbook.md](docs/worm-runbook.md). Unchanged-file backup (`unchanged: true`) — [backup workflow](docs/backup-workflow.md#skip-vs-complete-unchanged-files). Unsigned `GET` to the Seaweed S3 endpoint returns **403** (use signed access) — [API reference](docs/api-reference.md#seaweedfs-not-a-public-http-get).

---

## Development

```bash
# Run unit tests
go test ./...

# WORM integration test (Seaweed on :8333)
export SEAWEED_INTEGRATION=1
go test -tags=integration -run TestWORM ./src/storage/
```

---

## License

See [LICENSE](LICENSE).