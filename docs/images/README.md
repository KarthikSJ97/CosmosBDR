# Documentation images

Screenshots in this folder must be safe to publish:

- **No** `.env` values (hosts, passwords, API keys, SSH material, encryption keys).
- **No** real customer IPs or home paths — use placeholders in examples (`203.0.113.1`, `192.168.x.x`, `D:/data/backups`).
- Prefer **localhost** URLs and fictional `source_id` names (`prod-db-vm`, `win-lab-sftp`) that match `sources.yaml` / docs examples.

When capturing new shots:

1. Use Bruno **No Environment** or vars that are not secrets.
2. Redact terminal `export $(grep … .env)` — show `export SEAWEED_ACCESS_KEY="…"` placeholders instead.
3. Crop or avoid OS usernames in prompts (`user@host`).

If an image may contain secrets, do not commit it — regenerate or use the sanitized terminal pattern in [worm-runbook.md](../worm-runbook.md).