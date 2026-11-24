# SFTP sources runbook (Docker)

Back up remote files (e.g. Windows OpenSSH) into Seaweed; restore writes back to the same path on the remote (`remote_root` + relative path). Architecture: [multi-source-backup-strategy.md](./multi-source-backup-strategy.md).

CosmosBDR connects with **host, port, user, SSH key, host key fingerprint, remote_root** from `.env` — no app-level tunnel or dial overrides.

**Legend:** **One-time** · **Per change** · **Each backup** (stack already up)

---

## Why direct SFTP failed at first (then worked)

Early tests showed containers **could not** reach `192.168.x.x` (ping/SSH `No route to host`), while the same IP worked from the Mac Terminal. Typical combined causes on **Docker Desktop for Mac**:

1. **macOS Local Network** — Docker needs **Settings → Privacy & Security → Local Network** allowed; without it, the Linux VM may not reach LAN hosts.
2. **Docker Desktop networking** — Host networking / network stack updates (e.g. 4.8x) can change LAN reachability; worth re-testing after upgrades.
3. **Transient LAN state** — Windows asleep, Wi‑Fi change, or wrong DHCP IP while the Mac still had an old ARP entry or route.
4. **Misdiagnosis via tunnel** — We added a Mac `ssh -L` workaround before LAN was fixed; once (1)–(3) were resolved, `docker run … nc <WIN_HOST> 22` returned **open** and backups work **without** any tunnel.

**Sanity check** (run anytime connectivity breaks):

```bash
docker run --rm alpine sh -c 'nc -zv -w 5 <WIN_HOST> 22'
```

If this fails but `ssh` from the **Mac** works, fix Docker/macOS networking (below) — do not assume the app is wrong.

---

## One-time setup

### Remote (Windows / SFTP server)

- OpenSSH / SFTP on port **22**; firewall allows private networks.
- CosmosBDR **public key** for the backup user (Windows admins: `administrators_authorized_keys` when needed).
- Data under **`remote_root`** (e.g. `D:\Karthik\cosmosbdr_data`).

### Mac — backup SSH key

```bash
ssh-keygen -t ed25519 -f ~/.ssh/cosmosbdr_win_backup -C "cosmosbdr-backup"
ssh -i ~/.ssh/cosmosbdr_win_backup <user>@<WIN_HOST>
```

### Host key fingerprint

```bash
ssh-keyscan -p 22 <WIN_HOST> 2>/dev/null | ssh-keygen -lf -
```

Use the **ED25519** `SHA256:…` value in `.env` as `<PREFIX>_HOST_KEY_FINGERPRINT` (`PREFIX` = source id in UPPER_SNAKE — see `.env.example`).

### `.env` and `sources.yaml`

1. Copy `.env.example` → `.env`.
2. Set `HOST_DATA_PATH`, Seaweed/Postgres paths, Cadence/Postgres/Seaweed credentials.
3. Per SFTP source id (e.g. `win-home-laptop` → `WIN_HOME_LAPTOP_*`): **HOST, USER, REMOTE_ROOT, SSH_KEY, HOST_KEY_FINGERPRINT**.
4. Commit **`sources.yaml`** only (uses `env:VAR`); keep real IPs/users in `.env`.
5. Optional **scheduled backups:** under the source, add `schedules` (see [multi-source-backup-strategy.md](./multi-source-backup-strategy.md) §1). Restart `cosmosbdr` after changes; crons appear in Cadence as `scheduled_backup_<schedule_id>`.

### Same remote host: files and DB (recommended)

Use **two `source_id`s** in `sources.yaml` (same `host` / `ssh` / `remote_root` via YAML anchors)—one **without** `pre_backup` for normal files, one **with** `pre_backup` for Tier 2 dump-and-backup schedules. Do not mix ad-hoc file paths on a source that has `pre_backup` (dump runs before every file job on that id). Full pattern: [multi-source-backup-strategy.md §2](./multi-source-backup-strategy.md) (“One remote host → multiple `source_id`s”).

### Docker Desktop (Mac) — one-time

- **Local Network** → **Docker** on.
- **Docker Desktop → Settings → Resources → Network** → enable **Host networking** if offered; restart Docker.
- Confirm: `nc` from an `alpine` container to `<WIN_HOST>:22` is **open**.

### First bring-up

```bash
cd /path/to/CosmosBDR
docker compose up -d --build
```

**Per change:** rebuild/restart `cosmosbdr` after Go or `sources.yaml` changes.

---

## Each backup / restore (normal path)

1. **Optional:** `docker compose up -d` if stack stopped.
2. **Optional:** `nc` from container to `<WIN_HOST>:22` if you suspect network regression.
3. **Backup:**

```bash
curl -sS -X POST http://localhost:8080/api/v1/backups \
  -H 'Content-Type: application/json' \
  -d '{
    "flow_type": "BACKUP",
    "payload": {
      "source_id": "win-home-laptop",
      "relative_path": "My Memes.pdf"
    }
  }'
```

`flow_type` must be **`BACKUP`**.

**Directory backup (Phase D):** include a `directory` block; `relative_path` is the folder under `remote_root` (use `"."` for all of `remote_root`).

```bash
curl -sS -X POST http://localhost:8080/api/v1/backups \
  -H 'Content-Type: application/json' \
  -d '{
    "flow_type": "BACKUP",
    "payload": {
      "source_id": "win-home-laptop",
      "relative_path": ".",
      "directory": {
        "selection": "matched",
        "include": [
          { "type": "glob", "pattern": "**/*.pdf" }
        ],
        "exclude": [
          { "type": "glob", "pattern": "**/.cache/**" }
        ]
      }
    }
  }'
```

Each matched file runs a child file backup; list versions per file with `backup_key=<source_id>:<relative_path>`.

4. **List:** `curl -sS 'http://localhost:8080/api/v1/backups?backup_key=win-home-laptop:My%20Memes.pdf'`
5. **Restore (default → same path on remote):**

```bash
curl -sS -X POST http://localhost:8080/api/v1/restores \
  -H 'Content-Type: application/json' \
  -d '{
    "flow_type": "RESTORE",
    "payload": {
      "source_id": "win-home-laptop",
      "relative_path": "My Memes.pdf"
    }
  }'
```

Output: `<remote_root>/<relative_path>` on the Windows host (overwrites if the file already exists).

Optional local staging only: set top-level `"target_path": "/host-data/My Memes.pdf"` to write on the Mac instead.

**Directory restore** — restores every eligible file under a folder from backup metadata (not a live disk walk). Each file uses the same chunk pipeline as single-file restore.

Pin a specific directory backup run (recommended for DR drills):

```bash
curl -sS -X POST http://localhost:8080/api/v1/restores \
  -H 'Content-Type: application/json' \
  -d '{
    "flow_type": "RESTORE",
    "payload": {
      "source_id": "win-home-laptop",
      "relative_path": "dir-backup-test",
      "source_directory_job_id": "<parent_job_id from directory backup>"
    }
  }'
```

Or restore **latest** snapshot per `backup_key` under the folder (omit `source_directory_job_id`):

```bash
curl -sS -X POST http://localhost:8080/api/v1/restores \
  -H 'Content-Type: application/json' \
  -d '{
    "flow_type": "RESTORE",
    "payload": {
      "source_id": "win-home-laptop",
      "relative_path": "dir-backup-test",
      "directory": { "selection": "all" }
    }
  }'
```

Optional `directory` include/exclude filters the **backup keys** in that set (same rules as directory backup), not files currently on disk.

No Mac `ssh -L`, no extra env vars beyond the source block.

---

## Cheat sheet

| Step | When |
|------|------|
| Windows SSH + keys + `remote_root` | **One-time** |
| `.env` + `ssh-keyscan` fingerprint | **One-time** (update if host keys rotate) |
| Docker Local Network + network settings | **One-time** (re-check after OS/Docker upgrades) |
| `docker compose up -d --build` | **Per change** |
| Container `nc <WIN_HOST> 22` | When connectivity is unclear |
| `POST /backups` / `POST /restores` | **Each operation** |

---

## Linux app server on same LAN

Same `.env` pattern; container should reach `<WIN_HOST>:22` directly. No Mac-specific steps.

```bash
docker compose exec cosmosbdr sh -c 'nc -zv -w 3 <WIN_HOST> 22'   # if nc available
```

---

## Troubleshooting

| Symptom | Likely fix |
|---------|------------|
| Container `nc` to LAN fails; Mac `ssh` works | Local Network for Docker; restart Docker; Host networking toggle |
| `No route to host` from Mac | Windows off/wrong IP; same Wi‑Fi; verify `ipconfig` |
| `ssh host key mismatch` | Re-scan fingerprint; update `.env`; rebuild `cosmosbdr` |
| Empty `GET /backups` | Use `"flow_type": "BACKUP"` on trigger |
| App won’t start | Missing required `env:VAR` in `.env` — read startup error |

---

## If container LAN access never works

Run CosmosBDR on a **Linux host on the LAN** (same Compose stack), or fix Docker Desktop until `nc` succeeds. The application only opens **TCP to `host:port`** from the worker container — it does not implement SSH port forwarding.