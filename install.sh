#!/usr/bin/env bash
# Maildex installer for Proxmox VE. Run on the Proxmox host as root:
#   bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Maildex/main/install.sh)"
# Creates an unprivileged Debian LXC container and installs Maildex inside it.
set -euo pipefail

REPO="Bagareal/Maildex"
RAW="https://raw.githubusercontent.com/${REPO}/main"

die() { echo "ERROR: $*" >&2; exit 1; }
ask() { # ask "Prompt" default -> echoes answer
  local reply
  read -r -p "$1 [$2]: " reply </dev/tty || reply=""
  echo "${reply:-$2}"
}

[ "$(id -u)" -eq 0 ] || die "Run this as root on the Proxmox host."
command -v pct >/dev/null || die "pct not found. This script must run on a Proxmox VE host."

echo "== Maildex installer =="

CTID="$(pvesh get /cluster/nextid)"
HOSTNAME="maildex"
CORES=1
RAM=512
DISK=4
BRIDGE="vmbr0"
NET="ip=dhcp"
STORAGE="$(pvesm status -content rootdir | awk 'NR>1 {print $1; exit}')"
[ -n "$STORAGE" ] || die "No storage with container support found."

MODE="$(ask 'Use default settings? (y = defaults, n = customise)' y)"
if [[ "$MODE" =~ ^[Nn] ]]; then
  CTID="$(ask 'Container ID' "$CTID")"
  HOSTNAME="$(ask 'Hostname' "$HOSTNAME")"
  CORES="$(ask 'CPU cores' "$CORES")"
  RAM="$(ask 'RAM (MB)' "$RAM")"
  DISK="$(ask 'Disk (GB)' "$DISK")"
  echo "Available storages: $(pvesm status -content rootdir | awk 'NR>1 {printf "%s ", $1}')"
  STORAGE="$(ask 'Storage' "$STORAGE")"
  BRIDGE="$(ask 'Network bridge' "$BRIDGE")"
  IPCFG="$(ask 'IP address (dhcp or CIDR like 192.168.1.50/24)' dhcp)"
  if [ "$IPCFG" = "dhcp" ]; then
    NET="ip=dhcp"
  else
    GW="$(ask 'Gateway' "")"
    [ -n "$GW" ] || die "A gateway is required for a static IP."
    NET="ip=${IPCFG},gw=${GW}"
  fi
fi

pct status "$CTID" >/dev/null 2>&1 && die "Container ID $CTID is already in use."

# Newest Debian standard template (13, falling back to 12).
echo "Looking for a Debian template..."
pveam update >/dev/null
TEMPLATE=""
for v in 13 12; do
  TEMPLATE="$(pveam available --section system | awk -v v="debian-$v-standard" '$2 ~ v {print $2}' | sort -V | tail -1)"
  [ -n "$TEMPLATE" ] && break
done
[ -n "$TEMPLATE" ] || die "No Debian template found."

TSTORAGE="$(pvesm status -content vztmpl | awk 'NR>1 {print $1; exit}')"
[ -n "$TSTORAGE" ] || die "No storage for container templates found."
if ! pveam list "$TSTORAGE" | grep -q "$TEMPLATE"; then
  echo "Downloading $TEMPLATE..."
  pveam download "$TSTORAGE" "$TEMPLATE" >/dev/null
fi

echo "Creating container $CTID ($HOSTNAME)..."
pct create "$CTID" "${TSTORAGE}:vztmpl/${TEMPLATE}" \
  --hostname "$HOSTNAME" --cores "$CORES" --memory "$RAM" --swap 256 \
  --rootfs "${STORAGE}:${DISK}" --net0 "name=eth0,bridge=${BRIDGE},${NET}" \
  --unprivileged 1 --features nesting=1 --onboot 1 --ostype debian >/dev/null

started=0
for attempt in 1 2 3; do
  if pct start "$CTID" 2>/dev/null; then started=1; break; fi
  echo "Start failed (attempt $attempt), retrying..."
  systemctl restart lxcfs 2>/dev/null || true
  sleep 3
done
[ "$started" -eq 1 ] || die "Container did not start. Check: pct start $CTID"

echo "Waiting for network..."
online=0
for _ in $(seq 1 30); do
  if pct exec "$CTID" -- getent hosts github.com >/dev/null 2>&1; then online=1; break; fi
  sleep 2
done
[ "$online" -eq 1 ] || die "The container has no internet access. Check the bridge, IP and gateway."

echo "Installing Maildex..."
pct exec "$CTID" -- bash -c "apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null && curl -fsSL ${RAW}/scripts/setup.sh | bash"

IP="$(pct exec "$CTID" -- hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Done! Open http://${IP:-<container-ip>}:8080"
echo "Update later with:  pct exec $CTID -- update"
echo "Biometric unlock needs HTTPS: see the README for a reverse proxy setup."
