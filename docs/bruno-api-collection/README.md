# Bruno API collection

Import the whole collection in one step:

1. Download or open [`CosmosBDR.zip`](CosmosBDR.zip) in this folder.
2. In [Bruno](https://www.usebruno.com/), **Import collection** → choose the zip (or extract and open the folder).
3. Set **No Environment** or point vars at `http://localhost:8080` — do not commit real `.env` values into Bruno env files.

The archive includes requests for health, sources, backups, restores, schedules, and optional Seaweed/Temporal probes. Example payloads use fictional `source_id` values (e.g. `prod-db-vm`, `local-host-data`) aligned with [configuration.md](../configuration.md).

To refresh the zip after editing requests locally: export from Bruno or re-zip the collection folder and replace `CosmosBDR.zip`.

HTTP shapes and response notes: [API reference](../api-reference.md).