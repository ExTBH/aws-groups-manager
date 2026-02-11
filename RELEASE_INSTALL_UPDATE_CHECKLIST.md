# Release / Install / Update Checklist

## GitHub Release Workflow
- [x] Trigger on tag push `v*`
- [x] Build matrix for darwin/linux and amd64/arm64
- [x] Inject ldflags for version/commit/date and updater repo vars
- [x] Package per-platform tar.gz with binary only
- [x] Upload assets to GitHub release
- [x] Generate checksums

## install.sh
- [x] Detect OS/ARCH
- [x] Resolve latest release metadata
- [x] Download and extract matching asset
- [x] Install to `/usr/local/bin` or fallback `~/.local/bin`
- [x] Print verification hints

## update Command
- [x] Resolve latest release
- [x] Select matching OS/ARCH asset
- [x] Download + extract binary
- [x] Atomic replace strategy (temp file + rename)
- [x] Report updated/already-up-to-date

## Remaining Manual Verification
- [ ] Run installer from one-liner on macOS
- [ ] Run installer from one-liner on Ubuntu
- [ ] Validate updater from older released binary on both
