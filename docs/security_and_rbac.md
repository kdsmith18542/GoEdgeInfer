# Security, RBAC, and Audit Logging in GoEdgeInfer

## JWT & API Key Authentication
- All management endpoints under `/mgmt` require JWT authentication.
- JWT claims (role, scope) are validated for RBAC.
- Model and prediction endpoints require API key.

## RBAC (Role-Based Access Control)
- Management endpoints require `role` claim (e.g., `admin`, `ops`).
- Claims are checked in middleware and handlers.
- Example: Only `admin` can access `/mgmt/security_config`.

## Audit Logging
- All management actions are audit-logged with user, action, and details.
- Logs are written via the `logging` package.

## Live Security Config Endpoint
- `/mgmt/security_config` (admin only) returns current JWT and API key config.
- Use for runtime inspection and troubleshooting.

## Example JWT Claims
```json
{
  "iss": "myorg",
  "aud": "goedgeinfer",
  "role": "admin",
  "scope": "manage",
  "exp": 1735689600
}
```

---

# CLI Scripting & Batch Inference

## Batch Inference
- Use `goedgeinferctl batch-infer <input_jsonl_file> <model_id> [version] [output_format]`.
- Output formats: `json`, `table`, `quiet`.

## JWT Management
- Generate: `goedgeinferctl jwt generate <secret> <algorithm> <issuer> <audience> <role> <scope> <expire_minutes>`
- Validate: `goedgeinferctl jwt validate <token> <secret> <algorithm>`

## Example Usage
```sh
goedgeinferctl batch-infer data.jsonl mnist_model latest table
goedgeinferctl jwt generate mysecret HS256 myorg goedgeinfer admin manage 60
```
