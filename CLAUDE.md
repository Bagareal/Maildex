# Maildex

Self-hosted catalogue of "which email is used for which service". Go backend, plain-JS PWA front end, installed in a Proxmox LXC. Repo: https://github.com/Bagareal/Maildex (owner Bagareal). It deliberately mirrors the install/update pattern of the sibling project https://github.com/Bagareal/homie.

## Layout

- `cmd/maildex/main.go`: entry point. Subcommands: `serve` (default), `version`, `reset-password`. Env: `MAILDEX_ADDR`, `MAILDEX_DATA`, `MAILDEX_TLS_CERT/KEY`. `version` is injected with `-ldflags "-X main.version=..."`.
- `internal/store`: in-memory state persisted atomically to `data.json` (emails, services, settings) and `auth.json` (password hash, passkeys, sessions). Access only via `View` / `Update`.
- `internal/server`: HTTP API + static hosting. `auth.go` (sessions, password, idle lock), `passkeys.go` (WebAuthn endpoints), `data.go` (service/email CRUD + validation), `folders.go` (folders CRUD and the bulk-action endpoint), `transfer.go` (JSON import/export), `update.go` (GitHub latest-release check).
- `internal/webauthn`: minimal WebAuthn relying party (ES256 + RS256, "none" attestation, CBOR decoder). Has a software-authenticator test.
- `web/static`: the whole UI (`index.html`, `app.js`, `providers.js`, `styles.css`, `sw.js`, `manifest.webmanifest`, `icons/`), embedded via `web/embed.go`. **No build step, no npm.**
- `install.sh`: run on the Proxmox host. A whiptail step-by-step wizard modelled on homie's installer (defaults or advanced: ID, hostname, cores, RAM, disk, storage, bridge, DHCP/static IP, **DNS server + search domain**, privileged/unprivileged, root SSH, verbose, confirm; Cancel = back). Checks `lxcfs`, picks Debian 13 then 12 templates filtered by CPU arch, and if a container will not start it saves the LXC debug log to `/tmp/maildex-lxc-<ID>.log`, destroys it and retries with the next template (`MAILDEX_DEBIAN=12` skips 13). It pipes `scripts/setup.sh` into `pct exec`, so setup.sh must never read stdin (apt-get calls use `</dev/null`).
- `scripts/setup.sh`: runs inside the container, installs/updates the binary and systemd unit, and creates `update`, `maildex-update` and `maildex-reset-password` in `/usr/local/bin`.
- `tools/genicons`: regenerates the PNG icons (`make icons`).

## Conventions and decisions

- Go stdlib only (no third-party modules). Keep it that way unless there is a strong reason.
- Front end uses a tiny `h()` DOM helper. Never build HTML from user data with `innerHTML` (only static icon strings use it).
- No passwords for third-party services are ever stored, imported or even uploaded. Import files are parsed in the browser (`parseImport` in `app.js`) and the upload is built from an explicit whitelist of harmless fields; Bitwarden/Vaultwarden JSON and CSV are converted client-side (TOTP/passkey presence becomes a 2FA flag, the secret is dropped; notes are dropped for password-bearing files). Never POST a parsed user file as-is. v1.1.0 did that for JSON and would have sent a Bitwarden export, passwords included, to the server in clear text over HTTP.
- Every state-changing API request must carry the `X-Maildex` header (CSRF guard, see `Server.api`). The front end's `api()` adds it.
- Sessions are 90-day cookies. The idle lock (`Settings.LockMinutes`, default 5) only applies once a passkey is enrolled; locked sessions get HTTP 423 on data routes. The client calls `/api/ping` on activity so typing in search counts as use.
- Passkeys require a secure origin and a hostname (not an IP). `expectFor` derives RP ID/origin from `Host` / `X-Forwarded-*`, so reverse proxies must forward those.
- Export/import files reference emails by address, not ID, so they can be edited by hand. Import is idempotent.
- README is in English (user requirement). Chat with the user is in Italian.
- `web/static/providers.js` holds the email provider catalogue (id, name, brand colour, Simple Icons SVG path, domains for auto-detection). Logos are bundled on purpose: no external requests. `Email.Provider` stores a provider id (e.g. `gmail`), `Own domain`, or free text; `providerFor()` also resolves legacy names. Providers without a logo render as a coloured letter tile.
- Folders are flat and many-to-many: `Service.Folders` holds folder IDs, `Data.Folders` holds `{id,name,color}`. The pre-1.2 single `Service.Category` is migrated into a folder on load (`Data.normalise`, case-insensitive merge) and the untouched original is kept once as `data.pre-folders.json`. Deleting a folder never deletes services. Import/export refer to folders by name (and still accept `category`). `POST /api/services/bulk` takes `{ids, action, ...}` (`addFolder`, `removeFolder`, `addTwoFactor`, `removeTwoFactor`, `setStatus`, `delete`), validates before mutating, and 2FA actions add/remove only the named methods.
- UI selection mode: `S.selMode` / `S.sel`, a `selecting` class on `<body>`, an action bar in `#selbar`. Cards toggle selection in that mode; long-press (touch only) starts it. `refreshSelection()` updates in place, so do not re-render the list on every toggle.
- `Service.TwoFactor` is a list of ids (`totp`, `passkey`, `security-key`, `sms`, `email`, `push`, `backup`); the server whitelists and orders them (`cleanTwoFactor`). Empty means none recorded.

## Working with the user

- Talk in Italian; keep README and code comments in English.
- A question is only a question: answer it, do not edit code, commit, push or release (this went wrong once and they were angry). When they ask to build something, or approve a plan I proposed ("vai"), do it end to end: implement, test in a real browser, commit, push and release, then summarise plainly (what changed, what was and was not verified). For bigger features they like to talk the design through first; play the plan back and wait for the go-ahead.
- "Like homie" means the same behaviour, not just the same idea. Read `https://github.com/Bagareal/homie` (`curl` the raw files; WebFetch only summarises) before building something homie already does.
- Never put CLAUDE.md or anything about the AI conversation in `.gitignore`; CLAUDE.md is committed on purpose.
- Be upfront about what was not tested on their real Proxmox host. Their live container is CT 106 (`192.168.1.51`).

## Environment notes (Windows dev machine)

- Drive E: does not record ownership, so git refuses the folder. Use `git -c safe.directory=E:/DOWNLOADS/myemails ...` per command (do not change global config) and `go build -buildvcs=false`.
- No `gh`, no `make`, no Python. Node 24 and headless Edge (`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`) are available; a throwaway DevTools-protocol script (launch Edge with `--headless=new --remote-debugging-port`, use real `Input.dispatchMouseEvent` clicks, scroll elements into view, report `Runtime.exceptionThrown`) is how the UI was verified. Keep such scripts in the scratchpad, not the repo.
- `git push` works over HTTPS with the stored credentials; set `GIT_TERMINAL_PROMPT=0` so it fails instead of hanging.

## Status

- Latest release: **v1.2.0** (v1.0.0 initial, v1.0.1 fixed blank pages after login, v1.1.0 provider logos + per-service 2FA, v1.1.1 safe Bitwarden/Vaultwarden import, v1.2.0 folders + multi-select).
- Not yet verified on real hardware: passkey/biometric unlock (needs HTTPS and a hostname) and `install.sh` on a Proxmox host where the first run failed to start the container with Debian 13.
- Open issue: on the user's container the `update` command was not found after install. Cause unknown (`setup.sh` creates it; they were given the one-liner `curl -fsSL .../scripts/setup.sh | bash` to recreate it). Check `ls -l /usr/local/bin` and `echo $PATH` in the container if it recurs.

## Commands

```bash
make dev     # run on http://localhost:8080, data in ./data
make test    # go vet + go test ./...
make build   # static binary in dist/
```

`node --check` only catches syntax errors. After any change to `web/static/app.js`, exercise the UI in a real browser (e.g. headless Edge/Chrome driven over the DevTools protocol: create the password, add an email and a service, search, click every tab at desktop and ~400px widths) and look for runtime exceptions. v1.0.0 shipped with a runtime bug (`append()` misuse) that blanked every page after login and that no Go test or syntax check could see.

CI (`.github/workflows/ci.yml`) checks gofmt, vet, tests, `node --check` on the JS and `bash -n` on the scripts. Run `gofmt -l .` before committing.

## Releasing

`gh` is not installed on the user's Windows machine. Releases are built by GitHub Actions when a `v*` tag is pushed:

```bash
git tag vX.Y.Z && git push origin vX.Y.Z
```

The workflow publishes `maildex-linux-amd64`, `maildex-linux-arm64` and `SHA256SUMS`. `scripts/setup.sh` (also run by the in-container `update` command) downloads and verifies exactly those names, so do not rename them. Bump the tag following semver; the in-app update notice compares against the latest release tag.

## Ideas not built yet

- IMAP scan to discover services from mailboxes automatically (needs mailbox credentials, so deliberately left out of v1).
- Logos for services (Netflix etc.): only email providers have logos today; a bundled catalogue would be large, fetching favicons would need internet.
- Nested folders (deliberately flat for now), attachments, per-service TOTP-free "recovery hints" templates.
