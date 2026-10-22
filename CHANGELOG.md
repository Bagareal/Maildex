# Changelog

All notable changes to Maildex. Versions follow [semantic versioning](https://semver.org). To upgrade, run `update` in the container (see the README).

## v1.2.1 - 2026-10-22

**Fix: `update` not found**

- `maildex`, `update`, `maildex-update` and `maildex-reset-password` are now also linked into `/usr/bin`. Some consoles (`pct enter`, `pct exec`, the Proxmox shell) start with a `PATH` that lacks `/usr/local/bin`, so `update` reported "command not found" even though it was installed.
- Existing containers get the links the next time the installer runs: `/usr/local/bin/update` (or the one-liner in the README).

## v1.2.0 - 2026-10-22

**Folders and multi-select**

- New **Folders** tab: a grid of folders with a colour and a service count, plus a "No folder" bucket. Open a folder to see only its services, with the usual search and filters.
- Folders are flat and a service can be in several of them. Create them from the Folders tab, or by typing a name in the service form or in the bulk Folder dialog. Deleting a folder never deletes its services.
- Search also matches folder names.
- **Multi-select**: *Select* (or press and hold a service on a touch screen) shows checkboxes and an action bar: add to / remove from folders, add / remove 2FA methods, change status, delete. Adding 2FA keeps the methods a service already has. *Select all* covers the services currently listed.
- Tapping the email chip on a card while selecting now selects the service instead of copying the address.
- Import and export refer to folders by name and carry folder colours. Bitwarden folders become folders. Files from before folders (with `category`) still import.
- New endpoints: `POST /api/folders`, `PUT` / `DELETE /api/folders/{id}`, `POST /api/services/bulk`.

**Upgrading from v1.1.x**

- The old single *category* of each service becomes a folder automatically on first start (names that differ only by case are merged).
- Your original data is kept once, untouched, as `data.pre-folders.json` in the data directory. A rolling `data.json.bak` is also kept.

## v1.1.1 - 2026-10-22

**Safer import**

- The JSON import used to send the file as-is to the server, so a Bitwarden JSON export, passwords included, would have crossed the network in clear text (and imported nothing). Import files are now parsed only in the browser and the upload is built from a whitelist of harmless fields. Passwords, TOTP secrets, cards, identities and (for password-bearing files) notes never leave the page.
- Bitwarden and Vaultwarden **JSON** exports are supported (logins only). A TOTP or passkey on a login becomes a 2FA flag; the secret is dropped.
- Encrypted exports and unrecognised files are rejected before anything is sent.
- Rows without an email address are counted and reported instead of silently dropped.

## v1.1.0 - 2026-10-21

- **Email provider logos**: choose the provider from a grid (Gmail, Outlook, iCloud, Proton Mail, Yahoo, AOL, GMX, Zoho, Tuta; letter tiles for Fastmail, Libero, Virgilio, TIM; "Own domain" and "Other"). The provider is detected from the address. Logos are bundled (Simple Icons, CC0), so nothing is fetched from the internet.
- **Two-step verification per service**: authenticator app (OTP), passkey, security key, SMS, email code, app prompt, backup codes. Shown as badges, filterable, with an alert for active services without 2FA. Included in JSON export/import and the CSV importer.
- The action bar of dialogs stays visible while the form scrolls.

## v1.0.1 - 2026-10-21

- Fixed a bug that blanked every page right after signing in, leaving only the side menu.

## v1.0.0 - 2026-10-21

First release.

- Catalogue of emails and the services registered under each, with search across services, emails and usernames.
- Per-service username, sign-in method, status, website, renewal date and notes; at-risk emails; renewal reminders.
- Installable PWA, light and dark themes, desktop and phone layouts.
- Password login plus biometric unlock (WebAuthn passkeys verified on the server) with an idle lock.
- JSON export / import and CSV import. No third-party passwords are ever stored.
- Installs in a Proxmox LXC with one command and updates from GitHub Releases.

## Installer and scripts (no release needed)

`install.sh` and `scripts/setup.sh` are fetched from the `main` branch, so changes to them apply immediately without a new release.

- The installer is a step-by-step wizard (like homie's): defaults or advanced settings including DNS server and search domain, container type, root SSH and verbose output; Cancel goes back one screen.
- It checks `lxcfs`, picks Debian 13 then 12 templates for the host CPU, and if a container will not start it saves LXC's debug log to `/tmp/maildex-lxc-<ID>.log` and retries with the next template.
- `setup.sh` can be piped into `pct exec` and recreates the `update`, `maildex-update` and `maildex-reset-password` helpers whenever it runs.
