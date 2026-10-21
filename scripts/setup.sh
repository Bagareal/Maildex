#!/usr/bin/env bash
# Installs or updates Maildex inside a Debian/Ubuntu container or VM.
# Run as root. Safe to re-run: data in /var/lib/maildex is never touched.
set -euo pipefail

REPO="Bagareal/Maildex"
BASE="${MAILDEX_RELEASE_BASE:-https://github.com/${REPO}/releases}"
RAW="https://raw.githubusercontent.com/${REPO}/main"
BIN=/usr/local/bin/maildex
DATA=/var/lib/maildex

[ "$(id -u)" -eq 0 ] || { echo "Run as root." >&2; exit 1; }

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

export DEBIAN_FRONTEND=noninteractive
if ! command -v curl >/dev/null || ! command -v sha256sum >/dev/null; then
  apt-get update -qq
  apt-get install -y -qq curl ca-certificates coreutils >/dev/null
fi

# Resolve the latest release tag from the redirect (no API rate limits).
if [ -n "${MAILDEX_VERSION:-}" ]; then
  TAG="$MAILDEX_VERSION"
else
  TAG="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "${BASE}/latest" | sed 's|.*/||')"
fi
[ -n "$TAG" ] && [ "$TAG" != "latest" ] || { echo "Could not determine the latest release. Has one been published?" >&2; exit 1; }

CURRENT=""
[ -x "$BIN" ] && CURRENT="$("$BIN" version 2>/dev/null || true)"

if [ "$CURRENT" = "$TAG" ]; then
  echo "Maildex $TAG is already installed."
else
  echo "Installing Maildex $TAG (${ARCH})..."
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  curl -fsSL -o "$TMP/maildex-linux-${ARCH}" "${BASE}/download/${TAG}/maildex-linux-${ARCH}"
  curl -fsSL -o "$TMP/SHA256SUMS" "${BASE}/download/${TAG}/SHA256SUMS"
  (cd "$TMP" && grep " maildex-linux-${ARCH}\$" SHA256SUMS | sha256sum -c --quiet -) \
    || { echo "Checksum verification failed, aborting." >&2; exit 1; }

  id maildex >/dev/null 2>&1 || useradd --system --home-dir "$DATA" --shell /usr/sbin/nologin maildex
  install -d -m 0700 -o maildex -g maildex "$DATA"
  install -m 0755 "$TMP/maildex-linux-${ARCH}" "$BIN"
fi

# Service definition. Optional settings (TLS, port) go in /etc/default/maildex.
cat >/etc/systemd/system/maildex.service <<'EOF'
[Unit]
Description=Maildex - which email is used for which service
After=network-online.target
Wants=network-online.target

[Service]
User=maildex
Group=maildex
EnvironmentFile=-/etc/default/maildex
Environment=MAILDEX_DATA=/var/lib/maildex
ExecStart=/usr/local/bin/maildex serve
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/maildex

[Install]
WantedBy=multi-user.target
EOF

# Helper commands.
cat >/usr/local/bin/maildex-update <<EOF
#!/usr/bin/env bash
curl -fsSL ${RAW}/scripts/setup.sh | bash
EOF
cat >/usr/local/bin/maildex-reset-password <<'EOF'
#!/usr/bin/env bash
# Removes the password, biometrics and sessions, then restarts the service.
# The next visit to the web UI asks for a new password.
set -e
systemctl stop maildex
runuser -u maildex -- env MAILDEX_DATA=/var/lib/maildex /usr/local/bin/maildex reset-password
systemctl start maildex
EOF
chmod 0755 /usr/local/bin/maildex-update /usr/local/bin/maildex-reset-password
ln -sf /usr/local/bin/maildex-update /usr/local/bin/update

systemctl daemon-reload
systemctl enable maildex >/dev/null 2>&1
systemctl restart maildex

# Keep the OS patched too: upgrades only, never removals or release upgrades.
if ! apt-get update -qq || ! apt-get -y -qq -o Dpkg::Options::=--force-confold upgrade >/dev/null; then
  echo "Warning: system package upgrade failed (Maildex itself is updated)." >&2
fi

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Maildex ${TAG} is running: http://${IP:-<container-ip>}:8080"
