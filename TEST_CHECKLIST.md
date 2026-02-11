# Test Checklist

## Build Validation
- [x] `go build ./...` passes

## Unit/Logic Validation Targets
- [ ] Key routing enforces Ctrl-only mutation commands
- [ ] Region/profile selection transition coverage
- [ ] Modal workflows (input, confirm, picker)
- [ ] Error details payload formatting
- [ ] Organizations denied fallback in Accounts tab

## Manual E2E (Live AWS)
- [ ] Region/profile/instance flow
- [ ] Group create/delete
- [ ] Group user add/remove
- [ ] Assignment add/remove with polling
- [ ] Discovery cancellation via Esc
- [ ] Error details panel (`Ctrl+E`) context accuracy

## Regression Gates
- [x] No custom cache layer added
- [x] No custom app-level rate limiter added
- [x] Mutating actions wired to Ctrl combos
