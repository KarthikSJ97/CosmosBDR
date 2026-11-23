# CosmosBDR: Multi-Source Backup & Platform Strategy

Living design doc for phased implementation. Reference this document when building any step/phase.

**Last updated:** 2026-11-23 (Phase H DB backup strategy locked; Phase G encrypt/compress done)

---

## Current baseline (what exists today)

- **Ingress:** HTTP triggers → Cadence `BackupWorkflow` / `RestoreWorkflow` (restore is manual).
- **Source data:** Host folder bind-mounted as `/host-data` via `HOST_DATA_PATH` in `docker-compose.yml` (backup *source*, not Seaweed).
- **Pipeline:** Single file per job → `InspectFile` (size/mtime/hash, SKIP + chunk clone) → fixed 4 MB chunks → content-hash dedup → SeaweedFS (S3 API) → Postgres job/item/chunk metadata + snapshot manifest.
- **Versions:** List/restore by logical path; optional `source_job_item_id` pins a version (default latest eligible).
- **Encrypt/compress:** `chunkcodec` — plaintext SHA-256 dedup, then gzip/zstd + AES-256-GCM per chunk; legacy plaintext Seaweed objects still restore (`encrypted=false`, `compression_alg=none`).

---

## Strategic goal

Treat every backup target as a **logical item** with a stable identity, backed by a **pluggable source reader**. The chunk → encrypt/compress → Seaweed → Postgres manifest path stays the same; only ingestion, orchestration (directory, schedule, retention), and restore targets evolve.

---

## 1. Configuration model (YAML + `.env`)

### Files (suggested layout next to `.env`)

| File | Check in? | Contents |
|------|-----------|----------|
| `sources.yaml` | Yes | Source definitions, SSH connection params (refs only), **per-source retention TTL**, **per-source backup schedules (Cadence cron)** |
| `.env` | No | SSH keys/passphrases, key paths, `ENCRYPTION_KEY`, etc. |

**Single file for backup config:** connection, retention, and scheduled jobs all live under each source in `sources.yaml`—no separate `backup-jobs.json` or `backup-policies.yaml`.

### Secret resolution

- In YAML: **references only**, e.g. `ssh_secret_ref: PROD_WEB01_SSH_PRIVATE_KEY` (PEM in env) or path via env.
- Startup: load YAML → validate schema → resolve refs from env → **SourceRegistry** (immutable after load).
- **Known hosts:** prefer pinned `host_key_fingerprint` in YAML (commit-safe) over disabling host key verification.

### Validation & loading (`src/config/`)

- **Schema + structs:** JSON Schema (or equivalent) definitions, YAML → Go struct mapping, and `SourceRegistry` loader live under **`src/config/`** (alongside existing env config).
- **Fail fast:** if `sources.yaml` fails validation, required fields are missing, or mandatory secrets from `.env` cannot be resolved, the **server must not start** (log explicit errors and exit non-zero).
- **CI (recommended later):** same validation in a test or `go run` check so bad YAML never merges.

### Example source record

```yaml
version: 1
sources:
  - id: local-host-data
    type: local
    container_mount: /host-data

  - id: prod-app-01
    type: sftp
    host: prod-app-01.example.com
    port: 22
    user: cosmosbdr-backup
    domain: production
    remote_root: /var/lib/myapp/data
    ssh:
      secret_ref: PROD_APP01_SSH_KEY
      known_hosts_policy: pinned
      host_key_fingerprint: "SHA256:..."
      proxy_jump: bastion.example.com
    # Retention TTL is configured per source (see §6, §15)
    retention:
      ttl_days: 90
    # Cadence cron schedules (Phase E) — one row per target; stable `id` becomes workflow id suffix
    schedules:
      - id: prod-config-nightly
        relative_path: config/app.yml
        schedule: "0 2 * * *"
        enabled: true
      - id: prod-data-weekly
        relative_path: data
        schedule: "0 3 * * 0"
        directory:
          selection: all
          exclude:
            - type: glob
              pattern: "**/*.tmp"
```

Each schedule row is **`relative_path` + `schedule`** under its source. **Single-file:** omit `directory`. **Directory tree (Phase D):** set `directory` (same shape as `POST /backups`). Schedule `id` values must be **unique across the whole file** (not just within one source). `enabled: false` skips Cadence registration but still validates at load.

Manual HTTP triggers still use **`source_id` + `relative_path`** (or legacy `source_path` + `file_name`). No `policy_id` layer.

---

## 2. Source types & DB dumps

### Remote files: SFTP (not SCP as primary)

| Concern | SFTP | SCP |
|---------|------|-----|
| Stat / mtime / size | Yes | Awkward |
| Parallel `ReadAt(offset, len)` for 4 MB chunks | Yes | Whole-file copy |
| Fits `InspectFile` + `ProcessFileChunk` | Yes | Poor |

**SCP fallback (optional):** download entire file to temp, then run existing local pipeline—OK for small files / MVP only.

**Remote file flow:**

1. Open SSH/SFTP session (pool or per-workflow with retries).
2. Stat remote path → same decision ladder as today.
3. `ReadAt` per chunk activity instead of `os.Seek`/`Read`.

### DB dumps (locked strategy)

CosmosBDR protects **files** in Seaweed (chunked, encrypted). Database DR is: **produce a dump file → back it up like any other file**. Restore v1 is **write the dump file back to disk** (local or SFTP)—not import into PostgreSQL/MySQL/etc. (`psql` / `pg_restore` over SSH is a later, explicit feature).

Most engines ship **dump tools**, not **scheduled dumps**. Production teams use **cron**, SQL Server Agent, Ansible, or **cloud snapshot/PITR** (RDS, Cloud SQL). Cosmos fits **self-managed VMs** and **logical exports to a path**; it does **not** replace managed-cloud control-plane backups.

#### Tier 1 — **Default (no Phase H code; works today)**

| Who | What |
|-----|------|
| **Customer** | Scheduled dump on the DB host (`pg_dump`, `mysqldump`, custom script, SQL Agent) → stable path under `remote_root` (e.g. `backups/postgres/latest.dump`, often a `latest` symlink). |
| **CosmosBDR** | **`type: sftp`** + schedule or API on `relative_path` → `InspectFile` → chunks → Seaweed. |

**Operational rule:** run dump **before** Cosmos schedule (e.g. dump 01:00, backup 02:00). Ship **reference scripts** in-repo (`scripts/dump/*`) as copy/install helpers—Cosmos does not execute them in Tier 1.

**Risk — stale file + SKIP:** if tonight’s dump fails and the file is unchanged, `InspectFile` may **SKIP** on mtime/hash and yesterday’s version remains “current”. Mitigation (Phase H.1 / small follow-up): optional **`pre_backup.max_dump_age`** or **require mtime after job start** for paths tagged as dump artifacts.

#### Tier 2 — **Phase H: optional `pre_backup` on `sftp` sources**

When a team wants **one control plane** (only `sources.yaml`) or **fail-closed** “no dump ⇒ no backup”, add an optional **`pre_backup`** block on an existing **`sftp`** source—**not** a separate `postgres` / `mysql` source `type`.

```yaml
sources:
  - id: prod-db-vm
    type: sftp
    host: db.internal
    remote_root: /var/backups
    ssh: { secret_ref: ..., host_key_fingerprint: "SHA256:..." }
    pre_backup:
      # Preset — Cosmos expands to engine dump + env refs (v1: postgres first)
      engine: postgres
      database_ref: PROD_PG_DATABASE
      password_ref: PROD_PG_PASSWORD
      output:
        relative_path: cosmosbdr/nightly.dump   # under remote_root; same path backup reads
        cleanup_after: true                     # optional remote delete after successful upload
      # Alternative — long tail / RMAN / bespoke tools
      # command: ["/opt/cosmosbdr/dump.sh"]
      # env_refs:
      #   FOO: PROD_FOO
    schedules:
      - id: pg-nightly
        relative_path: cosmosbdr/nightly.dump
        schedule: "0 2 * * *"
        enabled: true
```

| Mechanism | Role |
|-----------|------|
| **`pre_backup.command` + `env_refs`** | Generic SSH exec (allowlisted argv from YAML only—never from HTTP API). |
| **`pre_backup.engine`** | Preset sugar (`postgres`, later `mysql`, …)—same workflow, config expansion only. |
| **Output path** | Dump must land under **`remote_root` + `relative_path`** (or explicit `output.relative_path`) so the existing **`SFTPReader`** path is unchanged. |

**Workflow (Phase H):**

```text
BackupWorkflow
  → Init → [optional] RunPreBackupActivity (SSH exec)
  → InspectFile → chunk fan-out → manifest → finalize
  → [optional] cleanup remote staging
```

**v1 staging default:** remote file on the SSH host, then **SFTP ranged reads** (no worker streaming / `StagedFileReader` unless a later phase requires it).

**Security:** commands and secret refs come **only** from `sources.yaml`; DB creds resolve from `.env` like SSH keys.

#### When to use which tier

| Situation | Use |
|-----------|-----|
| Dump already exists on a schedule | **Tier 1** — `sftp` only |
| Separate team owns DB; Cosmos only off-sites files | **Tier 1** |
| Single Cadence schedule; backup must fail if dump fails | **Tier 2** — `pre_backup` |
| Non-standard engine (Oracle RMAN, etc.) | **Tier 2** — `pre_backup.command` or customer script in **Tier 1** |
| RDS / Cloud SQL as primary DR | Vendor snapshot/PITR; Cosmos for **exports** or files on a bastion |

#### Phase H implementation order (locked)

1. Docs + reference dump scripts (**Tier 1**).
2. `RunPreBackupActivity`: SSH exec + `command` / `env_refs`.
3. **`engine: postgres`** preset + YAML validation.
4. Optional **`max_dump_age`** / freshness for dump `relative_path`s (valuable for Tier 1 too).
5. Additional engine presets only when a real deployment needs them.

**Defer:** separate `db_dump` source `type`; per-DB source types in `SourceRegistry`; worker-local streaming staging; restore-into-DB orchestration.

---

## 3. Stable identity: `backup_key`

Do not key version history only on container-local paths.

```text
backup_key = "{source_id}:{normalized_relative_path}"
# e.g. prod-app-01:config/app.yml
```

Persist: `source_id`, `source_type`, logical path, optional display URI.

List/restore APIs accept `backup_key` or `(source_id, relative_path)`.

---

## 4. Source Reader abstraction

```text
SourceReader:
  Stat(ctx) → size, mtime, isDir
  ContentSHA256(ctx) → when tier-3 hash needed
  ReadAt(ctx, offset, length) → []byte
  List(ctx, prefix) → []Entry   # directories
```

| Implementation | Role |
|----------------|------|
| `LocalReader` | Current `os` under `/host-data` |
| `SFTPReader` | Remote ranged reads |
| `StagedFileReader` | *(deferred)* SCP fallback or worker-side staging—Tier 2 default uses remote path + `SFTPReader` |

### Activities (conceptual)

| Activity | Role |
|----------|------|
| `ResolveSourceActivity` | Request/policy → reader + paths |
| `InspectFileActivity` | Reader instead of `os` |
| `ProcessFileChunkActivity` | `ReadAt` |
| `ListDirectoryActivity` | Directory policies |
| `RunPreBackupActivity` (Phase H) | Optional SSH exec before inspect (`pre_backup` on `sftp`) |
| `RestoreTargetActivity` (later) | Local or SFTP write |

Remote workloads: longer activity timeouts, tuned parallel chunk count.

---

## 5. Workflows

### Single file

Graph: Init → **[optional] RunPreBackup** (Phase H, `pre_backup` on source) → Inspect → chunk fan-out → manifest → finalize. Request carries `source_id` + path. Without `pre_backup`, identical to today.

### Directory backup

**`DirectoryBackupWorkflow` (parent):**

1. `ListDirectoryActivity`: walk under `relative_path`, apply **selection** rules below.
2. Child `BackupWorkflow` per matched file; **`CADENCE_MAX_CONCURRENT_WORKFLOWS`** for child batches. All activities share **`CADENCE_WORKER_MAX_CONCURRENT_ACTIVITIES`** (worker pool + per-wave schedule in chunk workflows).
3. Parent tracks totals/failures; **best-effort** (continue other files if a child fails).

**Failure handling (v1):** Activities use a default retry policy (5 attempts, exponential backoff). After permanent activity/workflow failure: `FailJobExecutionActivity` marks `PENDING` job items **`FAILED`**, then job **`FAILED`** + `error_message`. Partial Seaweed chunk blobs from aborted runs are **not** deleted here — **Phase F** refcount GC.

Schema: optional `parent_job_id`, optional schedule job `id` on job payload for traceability. Parent job **payload stores the resolved directory filter** (selection, patterns, limits) for audit / DR drills.

**Trigger (conceptual):** `source_id` + directory `relative_path` + optional `directory` block. Single-file jobs unchanged (`relative_path` + `file_name` or path to one file).

#### Selection modes (v1 — locked)

Two explicit modes; default **`all`** (DR-friendly: tree as-is under the root).

| Mode | Meaning |
|------|---------|
| **`all`** | Every **file** under `relative_path` is a candidate, then **exclude** patterns (and size limits) apply. **`include` is ignored** (must be empty or omitted). |
| **`matched`** | **`include` required** (≥1 pattern). A file must match **at least one** include (glob or regex), then **exclude** removes matches. |

#### Patterns: glob and regex (v1 — locked)

Each include/exclude entry is a small object (not a bare string):

```json
{ "type": "glob", "pattern": "**/*.yml" }
{ "type": "regex", "pattern": "\\.(sql\\.gz|dump)$" }
```

- **Glob:** `path/filepath`-style matching on the file path **relative to the directory root** (`relative_path`), `/` separators, case-sensitive (document if we add fold-case later).
- **Regex:** Go `regexp` syntax; match against the same **relative path** string (not basename-only unless the pattern says so).
- Invalid regex → **fail the list activity** (parent job fails fast); do not silently skip.

#### Precedence (no contradiction solver)

Fixed order; predictable, same as common backup tools:

1. Walk files under `relative_path` (directories traversed, never backed up as items).
2. Apply **`max_depth`** if set (omit or `null` = **unlimited** — default for DR).
3. Apply **`selection` mode** (`all` vs `matched` + include).
4. Apply **`exclude`** (glob or regex); **exclude wins** if both include and exclude match.
5. Optional **`min_file_bytes`** / **`max_file_bytes`**.

#### Example (`matched` + glob and regex)

```json
{
  "flow_type": "backup",
  "source_id": "prod-app-01",
  "relative_path": "data",
  "directory": {
    "selection": "matched",
    "include": [
      { "type": "glob", "pattern": "**/*.json" },
      { "type": "regex", "pattern": "^config/.*\\.ya?ml$" }
    ],
    "exclude": [
      { "type": "glob", "pattern": "**/staging/**" },
      { "type": "regex", "pattern": "\\.swp$" }
    ],
    "max_depth": null,
    "max_file_bytes": null
  }
}
```

#### Example (full tree for DR, junk excluded)

```json
{
  "flow_type": "backup",
  "source_id": "local-host-data",
  "relative_path": ".",
  "directory": {
    "selection": "all",
    "exclude": [
      { "type": "glob", "pattern": "**/.cache/**" },
      { "type": "glob", "pattern": "**/*.tmp" }
    ]
  }
}
```

### Schedule vs restore (Phase E — implemented)

- **Backup schedules:** each enabled `sources.yaml` `schedules[]` row → Cadence cron at process startup (`workflow_id` = `scheduled_backup_<id>`). Each tick runs `BackupWorkflow` or `DirectoryBackupWorkflow` with the same resolved request as `POST /backups`. Job payload includes `schedule_job_id` when cron-fired.
- **Startup:** worker registers crons after workflows are registered. If the workflow id already exists, startup logs and continues (change cron expression → terminate the old workflow in Cadence UI, then restart CosmosBDR).
- **Restore:** manual HTTP only (no cron).

---

## 6. Retention TTL & chunk GC

Two lifecycles:

1. **Versions:** `job_items` + manifests (including SKIPPED) in Postgres.
2. **Blobs:** Seaweed chunk objects on the **CosmosBDR host** (bind-mounted `SEAWEED_DATA_PATH`; see §13–§14).

**TTL (locked decision):**

- **`ttl_days` is set on each entry in `sources.yaml`** (source-level default for all `backup_key`s under that source). **Omitted `retention` or `ttl_days: 0` disables expiry GC for that source** (orphan chunk sweep still runs globally).
- When a backup version’s age **exceeds the source TTL**, expire that version’s **metadata** (job_items, manifests, item_chunk links for that version only).

**Physical delete (not WORM):**

- We do **not** rely on Seaweed WORM/object-lock for v1. After TTL, eligible data is **removed for real**.
- **Chunk objects** in Seaweed are deleted only when **no remaining `item_chunk` row references them** (global refcount). Shared chunks from dedup must survive while any **in-TTL or newer** version still points at them.
- Order: expire version metadata → remove that version’s chunk links → **GC pass** deletes unreferenced chunks from Seaweed and orphan `chunks` rows if applicable.

**SKIPPED** versions count toward history and TTL like any other completed snapshot.

**`RetentionGCWorkflow`:** daily Cadence cron, separate from backup; applies per-source TTL from config loaded at startup.

---

## 7. Encrypt & compress

**Order:**

```text
plaintext segment → SHA-256 (dedup key)
                 → compress (gzip/zstd)
                 → encrypt (AES-256-GCM, per-object nonce)
                 → Seaweed upload
```

- Dedup on **plaintext hash** (app-level); skip upload when hash exists.
- Store: `compression_alg`, encrypted flag, `compressed_size_bytes`.
- Restore: download → decrypt → decompress → write; verify hash.

Shared `chunkcodec` for backup and restore.

---

## 8. Docker & SSH

- Keys: env PEM or read-only secret mount—never in image.
- Pinned host key fingerprint in YAML.
- App must reach `host:22` (or `proxy_jump`).
- SFTP connection/channel limits → lower parallelism for remote policies.
- **Local source:** `HOST_DATA_PATH` → `/host-data`.

---

## 9. API evolution (minimal)

**Backup store vs restore target (locked):**

- **All backup bytes** live in **SeaweedFS** on the machine running CosmosBDR, with chunk data on a **host bind-mounted path** (`SEAWEED_DATA_PATH`). Remote SFTP/SSH paths are **sources only**—never the durability tier.
- **Restore (default):** read chunks from Seaweed → decrypt/decompress → write back to the **same path as the backup source** (local `container_mount` or SFTP `remote_root` + relative path). Existing files are overwritten (create if missing; parent dirs created on write). Optional **`target_path`** on the API restores to a local path on the CosmosBDR host instead (e.g. DR staging under `/host-data`).

**Endpoints (conceptual):**

- **Backup:** `{ "source_id", "relative_path" }` in payload (see runbook for shape).
- **List:** by `backup_key` or `source_id` + path.
- **Restore:** `{ "backup_key", "source_job_item_id?", "target_path"?: "/host-data/..." }` — without `target_path`, default is the original source location from `backup_key`; `target_path` overrides to a local path on the host.

---

## 10. Phased delivery

| Phase | Deliverable |
|-------|-------------|
| **A** | YAML in `src/config/` (schema, structs, loader), fail-fast startup, `SourceRegistry`, `backup_key` in DB/API |
| **B** | `SFTPReader` + refactor inspect/chunk to `SourceReader` (single file) |
| **C** | *(skipped)* Named policies / `policy_id` — not used; explicit `source_id` + `relative_path` instead |
| **D** | Directory parent workflow + selection (`all` / `matched`), glob + regex, limits |
| **E** | Cadence cron registration at startup from `sources.yaml` `schedules` (per source); stable workflow id per schedule `id` |
| **F** | Retention + chunk GC workflow |
| **G** | Encrypt/compress in chunk pipeline + restore |
| **H** | Optional `pre_backup` on **`sftp`** (SSH exec + presets); Tier 1 dump-via-cron unchanged |
| **H.1** | Dump freshness / `max_dump_age` (Tier 1 + Tier 2 safety) |

---

## 11. Decisions to lock early

1. `backup_key = source_id:relative_path`
2. SFTP primary; SCP staging fallback only
3. **DB (locked):** default **Tier 1** — customer dump on host + **`sftp`** file backup; optional **Tier 2** — `pre_backup` on **`sftp`** (generic `command` + `engine` presets). No per-engine source `types`.
4. Cadence cron for backup; restore API-only
5. Plaintext-hash dedup, encrypted blobs in Seaweed
6. **Backup durability:** always Seaweed on CosmosBDR host (bind mount); remotes are ingest sources only
7. **Restore default:** original backup source path (local mount or SFTP remote); optional `target_path` for local staging on CosmosBDR host
8. **Retention:** `ttl_days` per **source**; physical delete of expired metadata + unreferenced chunks (no WORM v1)
9. **Config:** validate `sources.yaml` in `src/config/`; **abort server start** on any validation or mandatory env resolution failure
10. **Directory selection (Phase D):** modes **`all`** (default) and **`matched`**; **glob + regex** pattern objects; precedence include (matched only) → exclude wins; **`max_depth`** optional, default unlimited
11. **Schedules (Phase E):** nested under each source in **`sources.yaml`** only; global unique schedule `id`; Cadence cron per enabled row → `BackupWorkflow` or `DirectoryBackupWorkflow`
12. **DB backups (Phase H):** restore = **dump file write-back** only; import into DB engines is out of scope v1. `pre_backup` commands are YAML-only (not API-driven).

---

## 12. Architecture diagram

```mermaid
flowchart LR
  subgraph config
    YAML[sources.yaml]
    ENV[.env secrets]
    YAML --> Registry[SourceRegistry]
    ENV --> Registry
  end

  subgraph trigger
    Cron[Cadence cron]
    API[Manual API]
    Cron --> DirWF[DirectoryBackupWorkflow]
    API --> DirWF
    API --> BkWF[BackupWorkflow]
    DirWF --> BkWF
  end

  Registry --> Resolve[ResolveSourceActivity]
  Resolve --> Reader[SourceReader local / sftp / staged]
  BkWF --> PreBk[RunPreBackupActivity optional]
  PreBk --> Inspect[InspectFileActivity]
  Inspect --> Reader
  BkWF --> Chunk[ProcessFileChunkActivity]
  Chunk --> Reader
  Chunk --> Codec[compress + encrypt]
  Codec --> Seaweed[(SeaweedFS)]
  Chunk --> PG[(Postgres manifests + refs)]

  Retention[RetentionGCWorkflow] --> PG
  Retention --> Seaweed
```

---

## 13. SeaweedFS storage & host bind mounts

### How it works today (`docker-compose.yml`)

| Mount | Purpose |
|-------|---------|
| `${HOST_DATA_PATH}:/host-data` | **Backup source** data (files you protect)—explicit host path |
| `seaweeddata:/data` (Docker **named volume**) | **SeaweedFS backend**—not a bind mount to a chosen host folder |

Named volume data lives in Docker’s volume store (on Docker Desktop for Mac, inside the Linux VM disk image), not as an obvious folder like `~/Downloads/...`.

Postgres uses the same pattern: `pgdata` named volume.

### Docker Desktop (Mac): limits & crash recovery

| | **Named volume** (`seaweeddata`, `pgdata`) | **Bind mount** (host path) |
|---|--------------------------------------------|----------------------------|
| **Where data lives** | Inside the Linux VM **virtual disk** | On the Mac filesystem (path you choose) |
| **Effective size cap** | Docker **Settings → Resources → Virtual disk limit**, shared with images, other volumes, build cache | Mac disk free space |
| **Docker stop / restart** | Usually persists | Persists |
| **VM corrupt, factory reset, “delete all data”** | **High risk of loss** unless exported | Folder **survives** on the host |
| **Backup / inspect** | Opaque (`docker volume` tooling) | Finder, Time Machine, `rsync`, normal paths |

The concern is usually **not** a per-volume quota, but **all chunk data competing for space inside one capped VM disk**—backups grow, the VM disk fills, and every service suffers.

### Should SeaweedFS be bind-mounted to the local host?

**Dev / laptop (this project):** **Prefer bind mounts** for Seaweed (and consider the same for Postgres metadata) so chunk/job data is not trapped inside the Docker VM. Named volumes are acceptable for throwaway experiments only.

**Production / DR seriousness:** **Prefer explicit durable storage**, not an anonymous named volume on a single laptop:

| Approach | When |
|----------|------|
| **Bind mount** e.g. `./data/seaweed:/data` or `/var/lib/cosmosbdr/seaweed:/data` | Single-node prod, clear backup-of-backup, rsync snapshots |
| **Dedicated disk/volume** (EBS, PVC in K8s, managed NFS) | Prod default |
| **Remote object store** (Seaweed alongside or migration to S3-compatible external bucket) | Multi-AZ, avoid single Docker host as SOLE copy of chunks |

**Important separation:**

- **`HOST_DATA_PATH`** = sources being backed up (may be remote via SFTP later; mount is for *local* sources).
- **Seaweed volume** = immutable chunk store (the DR artifact). Losing the host source without Seaweed loses nothing; losing Seaweed loses all backups.

### Recommendations

1. **Do not** bind-mount Seaweed to the same tree as `HOST_DATA_PATH` (avoid backing up the backup store accidentally without policy).
2. **Do** document a host path or cloud volume for Seaweed in prod; add bind mount or PVC when you leave pure local dev.
3. **Do** plan 3-2-1: Postgres metadata + Seaweed blobs replicated or snapshotted off the app host.
4. Encrypt/compress at chunk layer (Phase G) complements—not replaces—storage redundancy.

### Target compose pattern (bind mount)

```yaml
# seaweedfs service — SEAWEED_DATA_PATH must be set in .env (no compose default)
volumes:
  - ${SEAWEED_DATA_PATH}:/data

# postgres service — POSTGRES_DATA_PATH must be set in .env
volumes:
  - ${POSTGRES_DATA_PATH}:/var/lib/postgresql/data
```

---

## 14. What we'd do in this setup (ordered checklist)

Track infrastructure hygiene **before** multi-source phases (Section 10). Do these in order; check off as implemented.

### Prerequisite: durable Docker data on the Mac

- [x] **1. Pick host paths** (keep separate from backup *sources* and from each other):
  - Sources (already): `HOST_DATA_PATH` → `/host-data` (e.g. test/customer files).
  - Chunk store: `SEAWEED_DATA_PATH` (e.g. sibling `cosmosbdr_data/seaweed` — absolute path in `.env`).
  - Job DB: `POSTGRES_DATA_PATH` (e.g. `cosmosbdr_data/postgres`).
  - **Do not** nest Seaweed under `HOST_DATA_PATH` (avoid sweeping the backup store into a source directory policy later).

- [x] **2. Create directories on the host** before first `docker compose up` with new mounts.

- [x] **3. Add env vars** to `.env` and `.env.example`:
  - `SEAWEED_DATA_PATH`
  - `POSTGRES_DATA_PATH` (if binding Postgres)

- [x] **4. Update `docker-compose.yml`**: replace named volumes `seaweeddata` and (if desired) `pgdata` with the bind mounts above; remove unused entries under top-level `volumes:` if nothing else needs them.

- [x] **5. Data layout**: durable paths live outside the repo (e.g. sibling `cosmosbdr_data/`); no in-repo `data/` tree required. Add `.gitignore` entries only if you use repo-relative mount dirs again.

- [ ] **6. Migrate or reset data** (one-time, when switching from named volumes):
  - **Fresh start:** remove old named volumes (`docker volume rm ...`), empty new host dirs, re-run backups.
  - **Keep existing chunks:** copy from old volume to host path (e.g. temp container mount) *before* cutting over compose—otherwise history in Postgres may not match files on disk.

- [ ] **7. Document / verify:** after cutover, confirm Seaweed objects land under `SEAWEED_DATA_PATH` on the Mac and Postgres files under `POSTGRES_DATA_PATH` if used.

- [ ] **8. Off-host copy (when serious about DR):** Time Machine, rsync, or second location for **both** Seaweed data and Postgres data (3-2-1). Bind mount on laptop is not a substitute for a second copy.

### Then: multi-source & product phases (Section 10)

Only start **Phase A** (YAML, `SourceRegistry`, `backup_key`) after **1–7** are done—or explicitly accept named volumes for a short-lived dev spike.

| Order | Track | Section |
|-------|-------|---------|
| Infra | Prerequisite checklist above | §14 |
| A | YAML schema, loader, `SourceRegistry`, `backup_key` | §10 |
| B | `SFTPReader` + `SourceReader` refactor | §10 |
| C | *(skipped)* — no `policy_id` | §10 |
| D | Directory workflow | §10 |
| E | Cadence cron from `sources.yaml` schedules | §10 |
| F | Retention + chunk GC | §10 |
| G | Encrypt/compress pipeline | §10 |
| H | `pre_backup` on `sftp` + postgres preset | §2, §10 |
| H.1 | Dump freshness / `max_dump_age` | §2 |

### Status log (fill in as you go)

| Item | Status | Notes |
|------|--------|-------|
| Seaweed bind mount | done | `cosmosbdr_data/seaweed` via `.env` |
| Postgres bind mount | done | `cosmosbdr_data/postgres` via `.env` |
| Volume migration | | |
| Phase A | done | YAML, registry, backup_key, APIs |
| Phase B | done | SFTP `SourceReader`, inspect/chunk via `sourceread` |
| Phase C | skipped | No `policy_id`; schedules live under each source in `sources.yaml` |
| SFTP ops runbook | done | [sftp-sources-runbook.md](./sftp-sources-runbook.md) — Docker LAN, `.env` per source, backup/restore |
| Phase D | done | `DirectoryBackupWorkflow`, `ListDirectoryActivity`, `POST /backups` with `directory` block |
| Phase E | done | `sources.yaml` `schedules` → Cadence cron at startup (`scheduled_backup_<id>`) |
| Phase F | done | `RetentionGCWorkflow` daily cron; per-source TTL when `ttl_days > 0`; hard delete metadata + Seaweed; refcount via `item_chunks` |
| Phase G | done | `src/chunkcodec`; backup compress→encrypt, restore decrypt→decompress; `chunks.compression_alg` + `chunks.encrypted` |
| DB backup strategy | locked | Tier 1 = cron dump + `sftp`; Tier 2 = `pre_backup` on `sftp` (§2); no `db_dump` source type |
| Phase H | planned | `RunPreBackupActivity`, YAML `pre_backup`, `engine: postgres` preset |
| Phase H.1 | planned | `max_dump_age` / freshness for dump paths |

---

## 15. Locked product decisions (formerly open questions)

| Topic | Decision | Implication |
|-------|----------|-------------|
| **Where backups live** | All chunk/manifest storage in **Seaweed** on the **local host** running the server (`SEAWEED_DATA_PATH` bind mount). | Ingest from local mount or remote SFTP; durability is always central Seaweed + Postgres on this host. |
| **Restore default** | **Original source path** from `backup_key` (local or SFTP write-back). Optional `target_path` for local-only staging on the CosmosBDR host. | `PrepareRestoreTargetActivity` + chunk write use local FS or SFTP based on source type. |
| **WORM vs TTL** | **No WORM** for v1. **TTL per source** in `sources.yaml`. After TTL, delete **version metadata** and **physically remove** Seaweed objects / chunk rows that are **no longer referenced**. | Phase F implements refcount-safe GC; deduped chunks shared with newer backups are never deleted early. |
| **YAML validation** | Schemas, struct mapping, and loader in **`src/config/`**. | Missing/invalid YAML or unresolved mandatory secrets → **process exits**; no degraded “empty registry” mode. |
| **Database backups** | **Tier 1 (default):** host/cron produces dump → **`sftp`** backs up `relative_path`. **Tier 2 (optional):** `pre_backup` on same **`sftp`** source (generic `command` or `engine` preset). | No separate DB source types; chunk pipeline unchanged; restore = file only v1. |

_Add new rows here when scope changes._