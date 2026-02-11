# aws-groups-manager

Terminal TUI for managing IAM Identity Center (AWS SSO) groups, memberships, and account assignments.

## Status

Initial implementation is in progress and already includes:

- Cobra CLI with `version` and `update` commands
- Bubble Tea TUI shell with dark theme, footer shortcuts, and modal framework
- Region/profile selection flow
- SSO session ensure flow (`aws sso login --profile ...` fallback + retry)
- Instance selection and groups/users/accounts data operations through AWS SDK v2
- Organizations fallback behavior:
  - if `organizations:ListAccounts` is denied, Accounts tab remains usable
  - Add Assignment supports manual account ID entry (no blocking error)

## Commands

```bash
aws-groups-manager [--profile <name>] [--region <region>]
aws-groups-manager update
aws-groups-manager version
```

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/<ORG>/<REPO>/main/install.sh | bash
```

or

```bash
wget -qO- https://raw.githubusercontent.com/<ORG>/<REPO>/main/install.sh | bash
```

Before publishing, set real owner/repo defaults in `install.sh`, or pass env vars:

```bash
AGM_GITHUB_OWNER=<org> AGM_GITHUB_REPO=<repo> ./install.sh
```

## Build

```bash
go build ./...
```

## Release

Tag with semantic version:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions workflow creates tarball assets per target in `.github/workflows/release.yml`.
