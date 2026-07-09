#!/bin/bash
# kekkai egress firewall (§9). Runs as root via the single sudoers grant,
# before claude starts. Inputs come from env only — never bind-mounted files.
# New destinations are opened via user config, never by relaxing this script.
set -euo pipefail
IFS=$'\n\t'
cat >&2 <<'EOF'
[38;2;200;60;50m
  ██╗  ██╗███████╗██╗  ██╗██╗  ██╗ █████╗ ██╗
  ██║ ██╔╝██╔════╝██║ ██╔╝██║ ██╔╝██╔══██╗██║
  █████╔╝ █████╗  █████╔╝ █████╔╝ ███████║██║
  ██╔═██╗ ██╔══╝  ██╔═██╗ ██╔═██╗ ██╔══██║██║
  ██║  ██╗███████╗██║  ██╗██║  ██╗██║  ██║██║
  ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝
  https://github.com/filidorwiese/kekkai
[0m
EOF

echo "[kekkai] initializing egress firewall"

# --- 0. escape hatch (network.allow_all) -------------------------------------
if [ "${ALLOW_ALL:-}" = "1" ]; then
    cat >&2 <<'EOF'
[kekkai] ****************************************************************
[kekkai] *  WARNING: EGRESS FIREWALL DISABLED (network.allow_all)      *
[kekkai] *  The agent has unrestricted network access.                 *
[kekkai] ****************************************************************
EOF
    # Observe-only NFLOG for `kekkai watch` (§9): policies stay ACCEPT, no
    # group 2 exists — everything is allowed, so everything logs as ALLOW.
    # The NEW taps exclude udp/53: the DNS tap already logs those packets,
    # and nothing terminates rule traversal here (no ACCEPT rules), so they
    # would be logged twice.
    iptables -A OUTPUT -p udp --dport 53 -j NFLOG --nflog-group 1
    iptables -A INPUT -p udp --sport 53 -j NFLOG --nflog-group 1
    iptables -A OUTPUT -m state --state NEW -p udp ! --dport 53 -j NFLOG --nflog-group 1
    iptables -A OUTPUT -m state --state NEW ! -p udp -j NFLOG --nflog-group 1
    exit 0
fi

# --- 0b. GitHub meta CIDRs (network.allow_github) — fetched PRE-lockdown ----
# Fetch failure is fatal: silently starting without the requested access
# would strand the agent mid-session.
GITHUB_CIDRS=""
if [ "${ALLOW_GITHUB:-}" = "1" ]; then
    echo "[kekkai] fetching GitHub CIDRs from api.github.com/meta"
    meta=$(curl --silent --show-error --fail --max-time 15 https://api.github.com/meta) || {
        echo "[kekkai] ERROR: failed to fetch https://api.github.com/meta" >&2
        exit 1
    }
    GITHUB_CIDRS=$(echo "$meta" \
        | jq -r '(.git + .api + .web)[]' \
        | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/[0-9]+$' \
        | aggregate -q 2>/dev/null) || true
    if [ -z "$GITHUB_CIDRS" ]; then
        echo "[kekkai] ERROR: api.github.com/meta returned no usable IPv4 CIDRs" >&2
        exit 1
    fi
fi

# --- 1. flush, preserving Docker's embedded-DNS NAT rules (127.0.0.11) ------
DOCKER_DNS_RULES=$(iptables-save -t nat 2>/dev/null | grep -E '(^:|127\.0\.0\.11)' || true)

iptables -F
iptables -X
iptables -t nat -F
iptables -t nat -X
iptables -t mangle -F
iptables -t mangle -X
ipset destroy allowed-domains 2>/dev/null || true

if [ -n "$DOCKER_DNS_RULES" ]; then
    printf '*nat\n%s\nCOMMIT\n' "$DOCKER_DNS_RULES" | iptables-restore --noflush
fi

# --- 2. base allowances: loopback, DNS (udp 53), established ----------------
# No blanket port allowances — specifically no global tcp/22; the ipset match
# covers all ports to allowed IPs.
# NFLOG group-1 DNS taps for `kekkai watch` (§9): non-terminating, verdicts
# unchanged. They must precede the lo ACCEPTs — docker's embedded DNS rides
# lo post-NAT, so a later rule would miss it.
iptables -A OUTPUT -p udp --dport 53 -j NFLOG --nflog-group 1
iptables -A INPUT -p udp --sport 53 -j NFLOG --nflog-group 1
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A INPUT -p udp --sport 53 -j ACCEPT
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# --- 3. always allow the docker bridge subnet (from our own route) ----------
IFACE=$(ip route show default | awk '{print $5; exit}')
BRIDGE_SUBNET=$(ip route show dev "$IFACE" scope link | awk '{print $1; exit}')
if [ -z "$BRIDGE_SUBNET" ]; then
    echo "[kekkai] ERROR: could not determine docker bridge subnet" >&2
    exit 1
fi
echo "[kekkai] bridge subnet: $BRIDGE_SUBNET"
iptables -A INPUT -s "$BRIDGE_SUBNET" -j ACCEPT
# Observe-only NFLOG mirror of the bridge ACCEPT below (`kekkai watch`, §9).
iptables -A OUTPUT -d "$BRIDGE_SUBNET" -m state --state NEW -j NFLOG --nflog-group 1
iptables -A OUTPUT -d "$BRIDGE_SUBNET" -j ACCEPT

# --- 4. build the allowed-domains ipset --------------------------------------
ipset create allowed-domains hash:net

add_domain() { # $1=domain $2=fatal|warn
    local ips
    ips=$(dig +short A "$1" 2>/dev/null | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || true)
    if [ -z "$ips" ]; then
        if [ "$2" = "fatal" ]; then
            echo "[kekkai] ERROR: failed to resolve builtin host $1" >&2
            exit 1
        fi
        echo "[kekkai] WARNING: failed to resolve $1, skipping" >&2
        return 0
    fi
    local ip
    for ip in $ips; do
        ipset add allowed-domains "$ip" 2>/dev/null || true
    done
    echo "[kekkai] allowed: $1 ($(echo "$ips" | tr '\n' ' '))"
}

# Builtin hosts (§5.4) — not user-removable. api.anthropic.com must resolve
# (the verification probe needs it).
add_domain api.anthropic.com fatal
# host.docker.internal (§5.4): resolves on macOS runtimes → Mac-host parity
# with the Linux bridge-subnet allowance; on Linux it warn+skips.
add_domain host.docker.internal warn

# User domains (network.allowed_domains) — resolved once, warn+skip on failure.
# The env lists are space-separated; IFS is newline+tab, so split explicitly.
IFS=' ' read -ra user_domains <<< "${ALLOWED_DOMAINS:-}"
for domain in "${user_domains[@]}"; do
    add_domain "$domain" warn
done

# User CIDRs (network.allowed_cidrs) — validated literals from kekkai
IFS=' ' read -ra user_cidrs <<< "${ALLOWED_CIDRS:-}"
for cidr in "${user_cidrs[@]}"; do
    ipset add allowed-domains "$cidr" 2>/dev/null || true
    echo "[kekkai] allowed: $cidr"
done

# GitHub CIDRs fetched pre-lockdown above
if [ -n "$GITHUB_CIDRS" ]; then
    count=0
    for cidr in $GITHUB_CIDRS; do
        ipset add allowed-domains "$cidr" 2>/dev/null || true
        count=$((count + 1))
    done
    echo "[kekkai] allowed: github ($count CIDRs from api.github.com/meta)"
fi

# --- 5. lockdown: default DROP, ipset egress ACCEPT, reject rest ------------
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP
# NFLOG taps for `kekkai watch` (§9): each log rule sits immediately before
# the verdict rule it mirrors (group 1 = allowed, group 2 = blocked). NFLOG
# is non-terminating — packets continue to the unchanged ACCEPT/REJECT.
iptables -A OUTPUT -m set --match-set allowed-domains dst -m state --state NEW -j NFLOG --nflog-group 1
iptables -A OUTPUT -m set --match-set allowed-domains dst -j ACCEPT
iptables -A OUTPUT -m state --state NEW -j NFLOG --nflog-group 2
iptables -A OUTPUT -j REJECT --reject-with icmp-admin-prohibited

# --- 6. verification (never disabled) ----------------------------------------
echo "[kekkai] verifying firewall"
if curl --silent --output /dev/null --max-time 5 https://example.com; then
    echo "[kekkai] ERROR: verification failed — https://example.com is reachable" >&2
    exit 1
fi
echo "[kekkai] probe OK: https://example.com blocked"
if ! curl --silent --output /dev/null --max-time 10 https://api.anthropic.com; then
    echo "[kekkai] ERROR: verification failed — https://api.anthropic.com is NOT reachable" >&2
    exit 1
fi
echo "[kekkai] probe OK: https://api.anthropic.com reachable"
if [ "${ALLOW_GITHUB:-}" = "1" ]; then
    if ! curl --silent --output /dev/null --max-time 10 https://api.github.com/zen; then
        echo "[kekkai] ERROR: verification failed — https://api.github.com/zen is NOT reachable" >&2
        exit 1
    fi
    echo "[kekkai] probe OK: https://api.github.com/zen reachable"
fi
echo "[kekkai] egress firewall active"
clear
