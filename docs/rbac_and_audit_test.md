# Integration & Coverage Tests: RBAC, Audit, Security

## Management Endpoint RBAC
- Test: Only JWT with `role: admin` or `role: ops` can access `/mgmt` endpoints.
- Test: `/mgmt/security_config` only accessible by `admin`.
- Test: Invalid/missing JWT or wrong role returns 403.

## Audit Logging
- Test: All management actions log user, action, and details.
- Test: Logs contain correct claims and action info.

## Batch Inference & CLI
- Test: `batch-infer` works with valid input, returns correct output format.
- Test: JWT CLI commands generate and validate tokens as expected.

## Security Config Endpoint
- Test: `/mgmt/security_config` returns current config for admin, 403 for others.

## Coverage
- All new/modified middleware and handlers are covered by tests.
- Use integration tests in `handlers_test.go` and new tests in `cmd/goedgeinferctl/` for CLI.
