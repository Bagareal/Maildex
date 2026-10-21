# Maildex

**Know which email you used for which service.**

If you juggle many email addresses, you have probably lost track of which one holds your Netflix, your bank, or that forum from 2016. Maildex is a small self-hosted catalogue that answers one question quickly: *"Which email is this service under?"*

- Add your email addresses, then record the services that live under each one.
- Search any service, email or username and see the answer instantly.
- Works great on desktop and phone: it is an installable **PWA** with **fingerprint / Face ID / Windows Hello unlock**.
- Runs as a tiny single binary in a Proxmox LXC container. Installs and updates with one command.
- **No passwords are stored.** Maildex only records *where* your accounts are, not how to get in.

## Features

- **Search that answers the question**: type "netflix" and see the email, username and sign-in method.
- **Per-service details**: username, sign-in method (password, Google, Apple, passkey…), category, status (active / to close / closed), website, renewal date and notes.
- **Email overview**: every address with its recovery info and the list of services that depend on it.
- **At-risk emails**: flag a leaked or old address and see every service you need to secure.
- **Renewal reminders**: upcoming renewals are surfaced at the top of the list.
- **Import / export**: JSON backup and restore, plus CSV import (Bitwarden, Chrome or a spreadsheet). Passwords in CSV files are never read.
- **Biometric unlock**: WebAuthn passkeys on your device, verified on the server. The app locks itself after a period of inactivity.
- **Light and dark** themes that follow your system.

## Install on Proxmox

Run this on the **Proxmox host** shell:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Maildex/main/install.sh)"
```

A step-by-step wizard opens. Pick the defaults (1 core, 512 MB RAM, 4 GB disk, DHCP, unprivileged, start on boot) or choose advanced settings: container ID, hostname, resources, storage, bridge, DHCP or static IP, DNS server and search domain, privileged or unprivileged, root SSH and verbose output. Cancel goes back one screen. The installer then creates the Debian LXC container, downloads the latest release, verifies its checksum and starts the service.

If the container will not start with Debian 13, the installer saves LXC's debug log to `/tmp/maildex-lxc-<ID>.log` and retries automatically with Debian 12. You can also force it with `MAILDEX_DEBIAN=12 bash -c "$(curl -fsSL ...)"`.

Then open `http://<container-ip>:8080` and choose a password.

### Update

From the Proxmox host:

```bash
pct exec <CTID> -- update
```

or run `update` inside the container. The updater downloads the newest GitHub Release, verifies it and restarts the service. Your data is kept. Maildex also shows a notice in **Settings** when a new version is available.

### Forgot your password?

```bash
pct exec <CTID> -- maildex-reset-password
```

This removes the password, registered biometrics and sessions (your emails and services are kept). Open the web UI to choose a new password.

## Biometric unlock needs HTTPS

Browsers only allow fingerprint / Face ID (WebAuthn) on secure origins with a real hostname, so plain `http://192.168.x.x:8080` cannot use it. Put Maildex behind HTTPS using one of:

- **Tailscale Serve**: `tailscale serve --bg 8080` inside the container gives you `https://maildex.<tailnet>.ts.net`.
- **A reverse proxy** such as Caddy, Nginx Proxy Manager or Traefik with a certificate for a name like `maildex.home.example.com`.
- **Built-in TLS**: create `/etc/default/maildex` with `MAILDEX_TLS_CERT=/path/cert.pem` and `MAILDEX_TLS_KEY=/path/key.pem` (the `maildex` user must be able to read them), then `systemctl restart maildex`.

Open Maildex from that HTTPS address, then go to **Settings → Biometric unlock → Add this device**. On a phone, use your browser's *Add to Home Screen* / *Install app* to get the full-screen app.

When a reverse proxy terminates TLS, make sure it forwards the `Host`, `X-Forwarded-Proto` and `X-Forwarded-Host` headers (most do by default).

## Configuration

Environment variables (set them in `/etc/default/maildex`):

| Variable | Default | Purpose |
| --- | --- | --- |
| `MAILDEX_ADDR` | `:8080` | Listen address |
| `MAILDEX_DATA` | `/var/lib/maildex` | Data directory |
| `MAILDEX_TLS_CERT` / `MAILDEX_TLS_KEY` | unset | Serve HTTPS directly |

Data lives in two files in the data directory: `data.json` (emails, services, settings) and `auth.json` (password hash, passkeys, sessions). Back up the directory, or use **Settings → Export JSON**.

## Security notes

- The password is stored as a salted PBKDF2-SHA256 hash. Sessions use random tokens stored hashed on the server and sent in an `HttpOnly`, `SameSite=Lax` cookie.
- Wrong passwords are throttled per client address.
- Biometric unlock uses WebAuthn with user verification required. Private keys never leave your device.
- Maildex is designed for your home network or a private VPN. Do not expose it directly to the internet without HTTPS.

## Development

Requires Go 1.24+. There is no front-end build step: the UI in `web/static` is plain HTML, CSS and JavaScript embedded into the binary.

```bash
make dev      # run on http://localhost:8080 with data in ./data
make test     # go vet + go test
make build    # static binary in dist/
```

Releases are built by GitHub Actions when you push a tag:

```bash
git tag v1.0.0 && git push origin v1.0.0
```

This publishes `maildex-linux-amd64`, `maildex-linux-arm64` and `SHA256SUMS`, which is what `update` downloads.

## License

[MIT](LICENSE)
