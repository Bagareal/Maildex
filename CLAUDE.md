# Maildex

Self-hosted catalogue of "which email is used for which service". Go backend, plain-JS PWA front end, installed in a Proxmox LXC. Repo: https://github.com/Bagareal/Maildex (owner Bagareal). It deliberately mirrors the install/update pattern of the sibling project https://github.com/Bagareal/homie.

## Layout

- `cmd/maildex/main.go`: entry point. Subcommands: `serve` (default), `version`, `reset-password`. Env: `MAILDEX_ADDR`, `MAILDEX_DATA`, `MAILDEX_TLS_CERT/KEY`. `version` is injected with `-ldflags "-X main.version=..."`.
- `internal/store`: in-memory state persisted atomically to `data.json` (emails, services, settings) and `auth.json` (password hash, passkeys, sessions). Access only via `View` / `Update`.
- `internal/server`: HTTP API + static hosting. `auth.go` (sessions, password, idle lock), `passkeys.go` (WebAuthn endpoints), `data.go` (CRUD + validation), `transfer.go` (JSON import/export), `update.go` (GitHub latest-release check).
- `internal/webauthn`: minimal WebAuthn relying party (ES256 + RS256, "none" attestation, CBOR decoder). Has a software-authenticator test.
- `web/static`: the whole UI (`index.html`, `app.js`, `providers.js`, `styles.css`, `sw.js`, `manifest.webmanifest`, `icons/`), embedded via `web/embed.go`. **No build step, no npm.**
- `install.sh`: run on the Proxmox host, creates the LXC. `scripts/setup.sh`: runs inside the container, installs/updates the binary and systemd unit.
- `tools/genicons`: regenerates the PNG icons (`make icons`).

## Conventions and decisions

- Go stdlib only (no third-party modules). Keep it that way unless there is a strong reason.
- Front end uses a tiny `h()` DOM helper. Never build HTML from user data with `innerHTML` (only static icon strings use it).
- No passwords for third-party services are ever stored or imported. CSV import ignores password columns on purpose.
- Every state-changing API request must carry the `X-Maildex` header (CSRF guard, see `Server.api`). The front end's `api()` adds it.
- Sessions are 90-day cookies. The idle lock (`Settings.LockMinutes`, default 5) only applies once a passkey is enrolled; locked sessions get HTTP 423 on data routes. The client calls `/api/ping` on activity so typing in search counts as use.
- Passkeys require a secure origin and a hostname (not an IP). `expectFor` derives RP ID/origin from `Host` / `X-Forwarded-*`, so reverse proxies must forward those.
- Export/import files reference emails by address, not ID, so they can be edited by hand. Import is idempotent.
- README is in English (user requirement). Chat with the user is in Italian.

- `web/static/providers.js` holds the email provider catalogue (id, name, brand colour, Simple Icons SVG path, domains for auto-detection). Logos are bundled on purpose: no external requests. `Email.Provider` stores a provider id (e.g. `gmail`), `Own domain`, or free text; `providerFor()` also resolves legacy names. Providers without a logo render as a coloured letter tile.
- `Service.TwoFactor` is a list of ids (`totp`, `passkey`, `security-key`, `sms`, `email`, `push`, `backup`); the server whitelists and orders them (`cleanTwoFactor`). Empty means none recorded.

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
- Tags (a service currently has a single category), attachments, per-service TOTP-free "recovery hints" templates.
