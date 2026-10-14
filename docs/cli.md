# `goedgeinferctl`

```sh
goedgeinferctl --server <url> --apikey <key> <command> [args...]
# or:
export GOEDGEINFER_SERVER=http://localhost:8080
export GOEDGEINFER_APIKEY=yourkey
```

> Routes and `--server`/`--apikey` flags are verified against a live server
> by `cmd/goedgeinferctl/contract_test.go` (plan.md A9). The server
> (`internal/api/router.go`) is the source of truth.

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

## Contract test (A9)

`go test ./cmd/goedgeinferctl/ -run TestCLI_` boots the real router and runs
every command against it, asserting success. This test is why the old
CLI/server route drift cannot come back.
