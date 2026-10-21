#!/usr/bin/env bash
# Maildex - Proxmox VE installer.
# Creates an unprivileged Debian LXC container and installs Maildex as a service.
#
# Run from the Proxmox VE host shell:
#   bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Maildex/main/install.sh)"
set -Eeuo pipefail

REPO="${MAILDEX_REPO:-Bagareal/Maildex}"
BRANCH="${MAILDEX_BRANCH:-main}"
RAW="https://raw.githubusercontent.com/${REPO}/${BRANCH}"
WT_TITLE="Maildex - Proxmox VE LXC Installer"

C_OK=$'\e[32m'; C_ERR=$'\e[31m'; C_DIM=$'\e[2m'; C_B=$'\e[1m'; C_0=$'\e[0m'
info() { echo "${C_DIM}-${C_0} $*"; }
ok()   { echo "${C_OK}ok${C_0} $*"; }
die()  { echo "${C_ERR}error:${C_0} $*" >&2; exit 1; }
trap 'die "command failed at line $LINENO"' ERR

msgbox()  { whiptail --backtitle "$WT_TITLE" --title "$1" --msgbox "$2" "${3:-12}" "${4:-70}"; }
inputbox() { whiptail --backtitle "$WT_TITLE" --title "$1" --inputbox "$2" 10 70 "$3" 3>&1 1>&2 2>&3; }
passwordbox() { whiptail --backtitle "$WT_TITLE" --title "$1" --passwordbox "$2" 10 70 3>&1 1>&2 2>&3; }
menu() { # menu TITLE TEXT TAG1 ITEM1 [TAG2 ITEM2 ...]  -- Cancel means "go back"
  local title="$1" text="$2"; shift 2
  local n=$(( $# / 2 ))
  whiptail --backtitle "$WT_TITLE" --title "$title" --menu "$text" $(( n + 9 )) 70 "$n" "$@" 3>&1 1>&2 2>&3
}

[[ $EUID -eq 0 ]] || die "run this script as root."
command -v pct >/dev/null 2>&1 || die "pct not found: run this script on the Proxmox VE host."
if ! command -v whiptail >/dev/null 2>&1; then
  info "Installing whiptail"
  apt-get update -qq && apt-get install -y -qq whiptail >/dev/null
fi

msgbox "Welcome" "Maildex - know which email you used for which service\n\nThis will create a Debian LXC container on this Proxmox VE host and install Maildex as a systemd service inside it.\n\nPress Cancel on any screen to go back to the previous one."

ROOTFS_STORAGE="$(pvesm status -content rootdir | awk 'NR>1 {print $1; exit}')"
TPL_STORAGE="$(pvesm status -content vztmpl | awk 'NR>1 {print $1; exit}')"
[[ -n "$ROOTFS_STORAGE" ]] || die "no storage enabled for LXC disks (content rootdir)."
[[ -n "$TPL_STORAGE" ]] || die "no storage enabled for templates (content vztmpl)."

# Defaults
CTID="$(pvesh get /cluster/nextid)"
CT_HOSTNAME="maildex"
CORES=1
RAM=512
DISK=4
BRIDGE="vmbr0"
NETTYPE="dhcp"
NET="dhcp"
GW=""
DNS_NS=""
DNS_SEARCH=""
UNPRIVILEGED=1
SSH_ENABLE="no"
ROOT_PASSWORD=""
VERBOSE="no"

# Discover real storages/bridges on this host so the menus only ever show valid choices.
STORAGE_MENU_ITEMS=()
while read -r name type avail; do
  gb=$(( avail / 1024 / 1024 ))
  STORAGE_MENU_ITEMS+=("$name" "${type}, ${gb} GB free")
done < <(pvesm status -content rootdir | awk 'NR>1 {print $1, $2, $6}')

BRIDGE_MENU_ITEMS=()
while read -r br; do
  BRIDGE_MENU_ITEMS+=("$br" "")
done < <(ip -o link show type bridge 2>/dev/null | awk -F': ' '{print $2}')
[[ ${#BRIDGE_MENU_ITEMS[@]} -eq 0 ]] && BRIDGE_MENU_ITEMS=("vmbr0" "")
BRIDGE_MENU_ITEMS+=("custom" "Enter a bridge name manually")

# Step-based wizard: every dialog's Cancel button moves to the previous step
# instead of aborting, so the whole flow behaves like a normal back/forward wizard.
STEP="mode"
while [[ "$STEP" != "done" ]]; do
case "$STEP" in
  mode)
    CHOICE="$(menu "Settings" "ID ${CTID}, hostname ${CT_HOSTNAME}, ${CORES} core, ${RAM} MB RAM, ${DISK} GB disk on ${ROOTFS_STORAGE}, ${BRIDGE} in DHCP, unprivileged, SSH disabled.\n\nUse these defaults, or configure everything yourself?" \
      default "Use default settings" advanced "Configure advanced settings")" || { echo "Installation cancelled."; exit 0; }
    [[ "$CHOICE" == "default" ]] && STEP="done" || STEP="ctid"
    ;;
  ctid)
    VAL="$(inputbox "Container ID" "Container ID" "$CTID")" || { STEP="mode"; continue; }
    CTID="$VAL"; STEP="hostname"
    ;;
  hostname)
    VAL="$(inputbox "Hostname" "Hostname" "$CT_HOSTNAME")" || { STEP="ctid"; continue; }
    CT_HOSTNAME="$VAL"; STEP="cores"
    ;;
  cores)
    VAL="$(inputbox "CPU Cores" "Number of CPU cores" "$CORES")" || { STEP="hostname"; continue; }
    CORES="$VAL"; STEP="ram"
    ;;
  ram)
    VAL="$(inputbox "RAM" "RAM in MB" "$RAM")" || { STEP="cores"; continue; }
    RAM="$VAL"; STEP="disk"
    ;;
  disk)
    VAL="$(inputbox "Disk" "Disk size in GB" "$DISK")" || { STEP="ram"; continue; }
    DISK="$VAL"; STEP="storage"
    ;;
  storage)
    VAL="$(menu "Storage" "Storage for the container disk" "${STORAGE_MENU_ITEMS[@]}")" || { STEP="disk"; continue; }
    ROOTFS_STORAGE="$VAL"; STEP="bridge"
    ;;
  bridge)
    VAL="$(menu "Bridge" "Network bridge" "${BRIDGE_MENU_ITEMS[@]}")" || { STEP="storage"; continue; }
    if [[ "$VAL" == "custom" ]]; then
      VAL="$(inputbox "Bridge" "Network bridge name" "$BRIDGE")" || { STEP="bridge"; continue; }
    fi
    BRIDGE="$VAL"; STEP="nettype"
    ;;
  nettype)
    VAL="$(menu "IP Address" "How should this container get its IP address?" dhcp "Automatic (DHCP)" static "Manual (Static IP)")" || { STEP="bridge"; continue; }
    NETTYPE="$VAL"
    if [[ "$NETTYPE" == "dhcp" ]]; then NET="dhcp"; GW=""; STEP="dns_ns"; else STEP="ipaddr"; fi
    ;;
  ipaddr)
    VAL="$(inputbox "IP Address" "IP address with prefix (e.g. 192.168.1.10/24)" "$([[ "$NET" == "dhcp" ]] && echo "" || echo "$NET")")" || { STEP="nettype"; continue; }
    [[ -n "$VAL" ]] || { msgbox "IP Address" "The IP address cannot be empty." || true; continue; }
    NET="$VAL"; STEP="gateway"
    ;;
  gateway)
    VAL="$(inputbox "Gateway" "Gateway IP address" "$GW")" || { STEP="ipaddr"; continue; }
    [[ -n "$VAL" ]] || { msgbox "Gateway" "A static IP requires a gateway." || true; continue; }
    GW="$VAL"; STEP="dns_ns"
    ;;
  dns_ns)
    VAL="$(inputbox "DNS" "DNS nameserver IP (leave empty to inherit from the Proxmox host)" "$DNS_NS")" || { [[ "$NETTYPE" == "static" ]] && STEP="gateway" || STEP="nettype"; continue; }
    DNS_NS="$VAL"; STEP="dns_search"
    ;;
  dns_search)
    VAL="$(inputbox "DNS Search Domain" "DNS search domain (leave empty to inherit from the Proxmox host)" "$DNS_SEARCH")" || { STEP="dns_ns"; continue; }
    DNS_SEARCH="$VAL"; STEP="privileged"
    ;;
  privileged)
    VAL="$(menu "Container Type" "Container type" unpriv "Unprivileged (recommended)" priv "Privileged")" || { STEP="dns_search"; continue; }
    [[ "$VAL" == "unpriv" ]] && UNPRIVILEGED=1 || UNPRIVILEGED=0
    STEP="ssh"
    ;;
  ssh)
    VAL="$(menu "SSH" "Enable root SSH access (password login)?" no "Disabled (default)" yes "Enabled")" || { STEP="privileged"; continue; }
    SSH_ENABLE="$VAL"
    [[ "$SSH_ENABLE" == "yes" ]] && STEP="ssh_password" || STEP="verbose"
    ;;
  ssh_password)
    VAL="$(passwordbox "SSH" "Root password for the container")" || { STEP="ssh"; continue; }
    [[ -n "$VAL" ]] || { msgbox "SSH" "The password cannot be empty." || true; continue; }
    ROOT_PASSWORD="$VAL"; STEP="verbose"
    ;;
  verbose)
    VAL="$(menu "Verbose Mode" "Show full command output during installation?" no "Hide command output (default)" yes "Show full command output")" || { [[ "$SSH_ENABLE" == "yes" ]] && STEP="ssh_password" || STEP="ssh"; continue; }
    VERBOSE="$VAL"; STEP="confirm"
    ;;
  confirm)
    SUMMARY="ID: ${CTID}\nHostname: ${CT_HOSTNAME}\nCores: ${CORES}\nRAM: ${RAM} MB\nDisk: ${DISK} GB on ${ROOTFS_STORAGE}\nBridge: ${BRIDGE}\nNetwork: $([[ "$NETTYPE" == "dhcp" ]] && echo "DHCP" || echo "${NET}, gateway ${GW}")\nDNS: ${DNS_NS:-host default} / ${DNS_SEARCH:-host default}\nType: $([[ "$UNPRIVILEGED" == 1 ]] && echo "Unprivileged" || echo "Privileged")\nSSH root access: ${SSH_ENABLE}\nVerbose: ${VERBOSE}"
    VAL="$(menu "Confirm" "$SUMMARY" start "Create the container now" back "Go back and change something")" || { STEP="verbose"; continue; }
    [[ "$VAL" == "start" ]] && STEP="done" || STEP="verbose"
    ;;
esac
done

if [[ "$VERBOSE" == "yes" ]]; then exec 3>&1; else exec 3>/dev/null; fi

pct status "$CTID" >/dev/null 2>&1 && die "ID ${CTID} is already used by another guest."

# Debian templates, newest first: 13, then 12. They must match the host CPU
# architecture (the mirror lists amd64 and arm64 builds side by side).
# Set MAILDEX_DEBIAN=12 to skip Debian 13.
HOST_ARCH="$(dpkg --print-architecture)"
info "Looking for Debian templates (${HOST_ARCH})"
pveam update >&3
TEMPLATES=()
for v in ${MAILDEX_DEBIAN:-13 12}; do
  t="$(pveam available --section system | awk '{print $2}' | grep -E "^debian-${v}-standard.*_${HOST_ARCH}\.tar\.(zst|gz|xz)\$" | sort -V | tail -1 || true)"
  [[ -n "$t" ]] && TEMPLATES+=("$t")
done
[[ ${#TEMPLATES[@]} -gt 0 ]] || die "no Debian template available."

NET0="name=eth0,bridge=${BRIDGE},ip=${NET}"
[[ -n "$GW" ]] && NET0+=",gw=${GW}"

create_container() { # create_container TEMPLATE
  local args=(
    "$CTID" "${TPL_STORAGE}:vztmpl/$1"
    --hostname "$CT_HOSTNAME"
    --cores "$CORES"
    --memory "$RAM"
    --swap 256
    --rootfs "${ROOTFS_STORAGE}:${DISK}"
    --net0 "$NET0"
    --unprivileged "$UNPRIVILEGED"
    --features nesting=1
    --onboot 1
    --ostype debian
    --tags maildex
    --description "Maildex - which email is used for which service. https://github.com/${REPO}"
  )
  [[ -n "$DNS_NS" ]] && args+=(--nameserver "$DNS_NS")
  [[ -n "$DNS_SEARCH" ]] && args+=(--searchdomain "$DNS_SEARCH")
  [[ -n "$ROOT_PASSWORD" ]] && args+=(--password "$ROOT_PASSWORD")
  pct create "${args[@]}" >&3
}

start_container() {
  local attempt
  for attempt in 1 2 3; do
    if pct start "$CTID" >&3 2>&3; then
      return 0
    fi
    info "Start attempt ${attempt} failed, restarting lxcfs and retrying"
    systemctl restart lxcfs || true
    sleep 3
  done
  return 1
}

# Saves LXC's own debug output for a container that refuses to start.
collect_start_log() {
  local log="/tmp/maildex-lxc-${CTID}.log"
  timeout 30 lxc-start -n "$CTID" -F -l DEBUG -o "$log" >/dev/null 2>&1 || true
  pct stop "$CTID" >/dev/null 2>&1 || true
  if [[ -s "$log" ]]; then
    echo "${C_DIM}  Last LXC errors (full log: ${log}):${C_0}" >&2
    grep -E " ERROR | WARN " "$log" | tail -6 | sed 's/^/    /' >&2 || true
  fi
}

systemctl is-active --quiet lxcfs || systemctl restart lxcfs

STARTED=0
for TEMPLATE in "${TEMPLATES[@]}"; do
  if ! pveam list "$TPL_STORAGE" | grep -q "$TEMPLATE"; then
    info "Downloading ${TEMPLATE}"
    pveam download "$TPL_STORAGE" "$TEMPLATE" >&3
  fi
  ok "template ${TEMPLATE}"

  info "Creating LXC ${CTID}"
  create_container "$TEMPLATE"
  ok "LXC ${CTID} created"

  info "Starting the container"
  if start_container; then
    STARTED=1
    break
  fi
  echo "${C_ERR}error:${C_0} the container did not start with ${TEMPLATE}" >&2
  collect_start_log
  info "Removing the failed container"
  pct destroy "$CTID" --purge >&3 2>&3 || true
done
[[ "$STARTED" -eq 1 ]] || die "the container would not start with any template. Check 'systemctl status lxcfs' and the log above (also try: pct start ${CTID} --debug after recreating it)."
ok "container started"

info "Waiting for container network"
for _ in $(seq 1 30); do
  pct exec "$CTID" -- getent hosts github.com >/dev/null 2>&1 && break
  sleep 2
done
pct exec "$CTID" -- getent hosts github.com >/dev/null 2>&1 || die "the container cannot reach the internet: check network and DNS."
ok "network ready"

if [[ "$SSH_ENABLE" == "yes" ]]; then
  info "Enabling SSH root access"
  pct exec "$CTID" -- bash -c '
    set -e
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq openssh-server
    sed -i "s/^#\?PermitRootLogin.*/PermitRootLogin yes/" /etc/ssh/sshd_config
    systemctl enable --now ssh
  ' >&3 2>&3
  ok "SSH root access enabled"
fi

info "Installing Maildex in the container"
curl -fsSL "${RAW}/scripts/setup.sh" | pct exec "$CTID" -- env MAILDEX_REPO="$REPO" MAILDEX_BRANCH="$BRANCH" bash -s

IP="$(pct exec "$CTID" -- hostname -I | awk '{print $1}')"
echo
ok "Maildex is ready: ${C_B}http://${IP}:8080${C_0}"
echo "  To update it: pct exec ${CTID} -- update, or run 'update' from inside the container"
echo "  Biometric unlock needs HTTPS: see the README (reverse proxy / Tailscale Serve)."
echo
