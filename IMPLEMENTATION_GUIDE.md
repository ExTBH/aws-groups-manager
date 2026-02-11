# aws-groups-manager Implementation Guide (Checklist)

## 0) Project Bootstrap
- [x] Initialize Go module `aws-groups-manager`
- [x] Add Cobra root command and subcommands (`tui` behavior on root, `update`, `version`)
- [x] Add Bubble Tea app shell (`header/body/status/footer`)
- [x] Add dark theme primitives (Lip Gloss styles + constants)
- [x] Add keymap foundation with Ctrl-only action bindings
- [x] Add project README skeleton

## 1) Phase 1 — Skeleton + Dark Theme + Shortcuts
- [x] Implement placeholder-capable screens: Region, Profile, Instance, Groups, Group Detail
- [x] Implement screen router/state model
- [x] Implement footer rendering for current screen shortcuts
- [x] Ensure mutating actions are Ctrl-based only
- [x] Implement status strip and modal shells

### Acceptance
- [x] TUI runs
- [x] Navigation paths implemented (arrows, enter, esc, tab)
- [x] Ctrl shortcuts dispatch for refresh/help/search/actions

## 2) Phase 2 — Region + Profile Selection
- [x] Add static region list picker when `--region` absent
- [x] Parse local AWS profiles from config and credentials files
- [x] De-duplicate/sort profiles
- [x] Wire selected values into app context

### Acceptance
- [x] Region/profile selection proceeds to auth flow

## 3) Phase 3 — SSO Ensure + Instance Select
- [x] Load AWS config with profile/region
- [x] Call `ssoadmin.ListInstances`
- [x] On missing/expired session: run `aws sso login --profile` and retry once
- [x] If multiple instances, show picker and persist `InstanceArn` + `IdentityStoreId`

### Acceptance
- [x] Flow reaches Groups screen for valid profiles

## 4) Phase 4 — Groups Home CRUD
- [x] Fetch and list groups (`identitystore.ListGroups`)
- [x] Fetch selected group membership count (no cache)
- [x] Create group modal (`Ctrl+N`)
- [x] Delete group confirm (`Ctrl+D`)
- [x] Refresh (`Ctrl+R`) re-fetches

### Acceptance
- [x] Create/delete flow wired with status and error visibility

## 5) Phase 5 — Group Detail / Users Tab
- [x] List memberships and resolve users
- [x] Add user picker via `ListUsers` + `CreateGroupMembership` (`Ctrl+A`)
- [x] Remove membership via `DeleteGroupMembership` (`Ctrl+X`)
- [x] Refresh path and status messaging

### Acceptance
- [x] Membership mutation flow implemented

## 6) Phase 6 — Group Detail / Accounts Tab
- [x] List accounts (`organizations.ListAccounts`, paginated)
- [x] List permission sets
- [x] Discover assignments across account x permission set
- [x] Esc cancellation supported for discovery
- [x] Add/remove assignment with status polling
- [x] Organizations denied fallback implemented: manual account ID input, no blocking error

### Acceptance
- [x] Assignment create/delete flow implemented
- [x] Accounts tab remains usable without Organizations permissions

## 7) Phase 7 — Installer + Self-Update
- [x] Implement `install.sh` OS/arch detection and release download
- [x] Install path logic `/usr/local/bin` fallback `~/.local/bin`
- [x] Implement `update` command download/extract/atomic replace
- [x] Implement `version` output metadata plumbing

### Acceptance
- [x] Installer and updater code paths implemented
- [ ] Validate installs/updates on macOS and Ubuntu from real release assets

## Definition of Done (Global)
- [x] Dark theme only
- [x] Mutating actions are Ctrl-based
- [x] No custom caching layer
- [x] No custom rate limiter
- [x] Error visibility + details panel shortcut
- [x] Release workflow scaffolded
- [ ] Full AWS end-to-end validation in a live account
