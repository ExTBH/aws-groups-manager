# Technical Spec (Refined)

## Product Invariants
- Ctrl-only shortcuts for mutating actions.
- No custom caching and no custom application-level rate limiter.
- Errors shown in status strip and details modal.
- Dark theme only.

## Runtime Dependencies
- Go toolchain
- AWS CLI v2 on PATH (for `aws sso login`)
- AWS profile configured for Identity Center access
- IAM permissions for `identitystore`, `ssoadmin`, and optionally `organizations`

## CLI Contract
- `aws-groups-manager [--profile <name>] [--region <region>]`
- `aws-groups-manager update`
- `aws-groups-manager version`

## State Model
- selection context: region/profile/instance
- primary screens: region -> profile -> instance -> groups -> group detail
- detail tabs: users | accounts
- status line + last error details payload
- modal layer for confirmations/pickers/inputs/error details

## Organizations Fallback Decision
If `organizations:ListAccounts` is denied:

- Do not block Accounts tab with fatal error.
- Show warning in status strip.
- Keep permission set loading and assignment actions available.
- Add Assignment wizard asks for manual 12-digit account ID.

## Non-goals (v1)
- Native `ssooidc` device flow in-app
- Windows artifact target
- Background sync daemon
