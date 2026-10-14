# `goedgeinferctl`

```sh
goedgeinferctl --server <url> --apikey <key> <command> [args...]
# or:
export GOEDGEINFER_SERVER=http://localhost:8080
export GOEDGEINFER_APIKEY=yourkey
```

> **Known bug (plan.md A9):** the CLI's request paths don't match the
> server's `/api/v1` routes, and `--server`/`--apikey` flags are silently
> ignored (only env vars reach requests). The table below documents the
> **intended** contract — the server (`internal/api/router.go`) is source of
> truth. Fix the CLI before trusting this doc.

| Command | Maps to |
|---|---|
| `list-models` | `GET /api/v1/models` |
| `load-model <id> <path> [version]` | `POST /api/v1/models` |
| `unload-model <id>` | `DELETE /api/v1/models/:id` |
| `reload` | `POST /api/v1/mgmt/system/reload` |
| `list-remote` | `GET /api/v1/mgmt/remote/models` |
| `upload-remote <local> <key>` | `POST /api/v1/mgmt/remote/upload` |
| `delete-remote <key>` | `POST /api/v1/mgmt/remote/delete` |
| `cleanup-cache <dir> <keep…>` | `POST /api/v1/mgmt/remote/cleanup` |
| `batch-infer <jsonl> <model> [version] [json\|table\|quiet]` | `POST /api/v1/infer/:model` per line |
| `jwt generate <secret> <alg> <iss> <aud> <role> <scope> [expire_min]` | local JWT mint (HS256) |
| `jwt validate <token> <secret> <alg>` | local JWT check |

## Contract test (required by A9)

Boot the server, run every command above against it, assert success. This
test is the reason the CLI/server drift happened — there wasn't one.
