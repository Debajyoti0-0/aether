#!/bin/bash
# Stage 45b — Samba4 AD Lab Seed Script
# Creates deterministic test identities for Stage 45 live qualification

set -euo pipefail

CONTAINER_NAME="aether-ad-lab"
REALM="AETHER.TEST"
DOMAIN="AETHER"
BASE_DN="DC=aether,DC=test"
ADMIN_USER="Administrator"
ADMIN_PASS="Passw0rd123!"

echo "[+] Seeding Samba4 AD with test identities..."
echo "[+] Realm: ${REALM}"
echo "[+] Domain: ${DOMAIN}"

# Helper function to run samba-tool in container
run_samba() {
    docker exec -i aether-ad-lab samba-tool "$@"
}

run_ldapmodify() {
    docker exec -i aether-ad-lab ldapmodify -H ldap://localhost:389 -D "CN=Administrator,CN=Users,${BASE_DN}" -w "${ADMIN_PASS}" "$@"
}

run_ldapsearch() {
    docker exec -i aether-ad-lab ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,${BASE_DN}" -w "${ADMIN_PASS}" -b "${BASE_DN}" "$@"
}

run_ldbmodify() {
    docker exec -i aether-ad-lab ldbmodify -H /var/lib/samba/private/sam.ldb "$@"
}

# Create normal user: user1
echo "[+] Creating normal user: user1"
run_samba user create user1 Passw0rd123! \
    --given-name=Test \
    --surname=User1 \
    --mail-address=user1@aether.test \
    --description="Normal test user for Kerberos authentication" 2>/dev/null || echo "[!] user1 may already exist"

# Create AS-REP roastable user: user2 (no pre-auth required)
echo "[+] Creating AS-REP roastable user: user2"
run_samba user create user2 Passw0rd123! \
    --given-name=Test \
    --surname=User2 \
    --mail-address=user2@aether.test \
    --description="AS-REP roastable user (no pre-auth)" 2>/dev/null || echo "[!] user2 may already exist"

# Set DONT_REQ_PREAUTH flag on user2 (UF_DONT_REQUIRE_PREAUTH = 0x400000).
# The value must also carry NORMAL_ACCOUNT (0x200): 0x400000 on its own is not
# a valid userAccountControl and is rejected/ignored, which silently left user2
# unroastable. 0x400200 = 4194816 is the correct combination.
echo "[+] Setting DONT_REQ_PREAUTH on user2"
docker exec -i aether-ad-lab ldbmodify -H /var/lib/samba/private/sam.ldb \
    -b "CN=user2,CN=Users,DC=aether,DC=test" \
    <<'EOF'
dn: CN=user2,CN=Users,DC=aether,DC=test
changetype: modify
replace: userAccountControl
userAccountControl: 4194816
EOF

# Create Domain Admin user: user3
echo "[+] Creating Domain Admin user: user3"
run_samba user create user3 Passw0rd123! \
    --given-name=Admin \
    --surname=User3 \
    --mail-address=user3@aether.test \
    --description="Domain Admin test user" 2>/dev/null || echo "[!] user3 may already exist"

# Add user3 to Domain Admins group
run_samba group addmembers "Domain Admins" user3 2>/dev/null || true

# Create target user for ACL testing: user4
echo "[+] Creating ACL target user: user4"
run_samba user create user4 Passw0rd123! \
    --given-name=Target \
    --surname=User4 \
    --mail-address=user4@aether.test \
    --description="ACL target user (user1 has GenericAll)" 2>/dev/null || echo "[!] user4 may already exist"

# Create service account for Kerberoasting: svc_sql
echo "[+] Creating service account: svc_sql"
run_samba user create svc_sql SvcPass123! \
    --given-name=SQL \
    --surname=Service \
    --mail-address=svc_sql@aether.test \
    --description="Service account for MSSQL (Kerberoast target)" 2>/dev/null || echo "[!] svc_sql may already exist"

# Add SPN for svc_sql
echo "[+] Adding SPN for svc_sql: MSSQLSvc/sql01.aether.test:1433"
run_samba spn add MSSQLSvc/sql01.aether.test:1433 svc_sql 2>/dev/null || true

# Create service account for Kerberoasting: svc_web
echo "[+] Creating service account: svc_web"
run_samba user create svc_web WebPass123! \
    --given-name=Web \
    --surname=Service \
    --mail-address=svc_web@aether.test \
    --description="Service account for HTTP (Kerberoast target)" 2>/dev/null || echo "[!] svc_web may already exist"

# Add SPN for svc_web
echo "[+] Adding SPN for svc_web: HTTP/web01.aether.test"
run_samba spn add HTTP/web01.aether.test svc_web 2>/dev/null || true

# Create groups for nested ACL testing
echo "[+] Creating Tier1-Admins group"
run_samba group add "Tier1-Admins" --description="Tier 1 Administrators" 2>/dev/null || true

echo "[+] Creating Tier2-Admins group"
run_samba group add "Tier2-Admins" --description="Tier 2 Administrators" 2>/dev/null || true

# Nest Tier1-Admins into Domain Admins
run_samba group addmembers "Domain Admins" "Tier1-Admins" 2>/dev/null || true

# Add user3 to Tier1-Admins
run_samba group addmembers "Tier1-Admins" user3 2>/dev/null || true

# Add user2 to Tier2-Admins
run_samba group addmembers "Tier2-Admins" user2 2>/dev/null || true

# Set ACL: user1 has GenericAll on user4
echo "[+] Setting ACL: user1 -> GenericAll on user4"
USER1_SID=$(docker exec aether-ad-lab samba-tool user show user1 --attributes=objectSid 2>/dev/null | awk -F': ' '/objectSid/ {print $2}')
USER4_DN=$(docker exec aether-ad-lab ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "Passw0rd123!" -b "DC=aether,DC=test" "(sAMAccountName=user4)" dn | grep "^dn:" | head -1 | sed 's/^dn: //')

if [ -n "${USER1_SID}" ] && [ -n "${USER4_DN}" ]; then
    docker exec -i aether-ad-lab ldbmodify -H /var/lib/samba/private/sam.ldb -b "${USER4_DN}" <<EOF
dn: ${USER4_DN}
changetype: modify
add: nTSecurityDescriptor
nTSecurityDescriptor: O:DAG:DAD:(A;;GA;;;${USER1_SID})
EOF
fi

# Set ACL: user2 has WriteDACL on user1
echo "[+] Setting ACL: user2 -> WriteDACL on user1"
USER2_SID=$(docker exec aether-ad-lab samba-tool user show user2 --attributes=objectSid 2>/dev/null | awk -F': ' '/objectSid/ {print $2}')
USER1_DN=$(docker exec aether-ad-lab ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "Passw0rd123!" -b "DC=aether,DC=test" "(sAMAccountName=user1)" dn | grep "^dn:" | head -1 | sed 's/^dn: //')

if [ -n "${USER2_SID}" ] && [ -n "${USER1_DN}" ]; then
    docker exec -i aether-ad-lab ldbmodify -H /var/lib/samba/private/sam.ldb -b "${USER1_DN}" <<EOF
dn: ${USER1_DN}
changetype: modify
add: nTSecurityDescriptor
nTSecurityDescriptor: O:DAG:DAD:(A;;WP;;;${USER2_SID})
EOF
fi

# D6: normalise the AD zone so service discovery is unambiguous.
#
# Two distinct problems are fixed here, both caused by the samba_data volume
# outliving changes to the lab's docker networking.
#
# 1. Stale A records. Samba's DNS update task re-registers the container's
#    current addresses and never purges the previous ones, so every rebuild on
#    a different docker network left an extra A record behind. A client that
#    picked a stale address would fail to reach the KDC, and there was no way to
#    tell from the DNS answer which address was authoritative. Remove any A
#    record that is not the pinned DC address.
#
# 2. Missing per-DC SRV records. Provisioning creates only the zone-apex SRVs
#    (_kerberos._tcp.aether.test and friends). Windows-style clients query
#    _kerberos._tcp.dc01.aether.test directly, so they could not discover the
#    KDC. Add the per-DC records alongside the existing apex ones.
#
# Note on samba-tool's SRV rdata: this image expects four space-separated
# elements in the order <server> <port> <priority> <weight>, which is the
# reverse of the RFC 2782 wire order. Getting this wrong fails with
# "SRV port, priority, and weight must be integers".
DC_IP="172.18.0.2"
ZONE="aether.test"
ZONE_SERVER="127.0.0.1"
# This Samba rejects the realm-qualified and NT-style forms
# ("Administrator@AETHER.TEST%..." and "AETHER.TEST\Administrator%...") for the
# DNS RPC operations, failing with "Configuration information could not be read
# from the domain controller". Only the bare "Administrator%pass" form binds.
DNS_AUTH=(-U "Administrator%${ADMIN_PASS}" -P)

echo "[+] D6: removing stale A records (keeping ${DC_IP})"
for name in "dc01.${ZONE}" "${ZONE}" "dc01"; do
    # Collect the A addresses currently registered for this owner, then delete
    # every one of them that is not the pinned DC address.
    for addr in $(run_samba dns query "${DNS_AUTH[@]}" "${ZONE_SERVER}" "${ZONE}" "${name}" A 2>/dev/null |
                  awk '/^ *A: /{print $2}'); do
        if [ "${addr}" != "${DC_IP}" ]; then
            echo "    removing ${name} A ${addr}"
            run_samba dns delete "${DNS_AUTH[@]}" "${ZONE_SERVER}" "${ZONE}" "${name}" A "${addr}" >/dev/null 2>&1 || true
        fi
    done
done

echo "[+] D6: ensuring per-DC SRV records"
for spec in "_kerberos._tcp.dc01 88" "_kerberos._udp.dc01 88" "_ldap._tcp.dc01 389"; do
    set -- ${spec}
    owner="$1"; port="$2"
    if ! run_samba dns query "${DNS_AUTH[@]}" "${ZONE_SERVER}" "${ZONE}" "${owner}.${ZONE}" SRV 2>/dev/null | grep -q "${port}"; then
        echo "    adding ${owner}.${ZONE} -> ${port}"
        run_samba dns add "${DNS_AUTH[@]}" "${ZONE_SERVER}" "${ZONE}" "${owner}" SRV \
            "dc01.${ZONE}. ${port} 0 100" >/dev/null 2>&1 || true
    fi
done

echo "[+] D6: verifying zone answers"
docker exec aether-ad-lab sh -c "
    for n in _kerberos._tcp.${ZONE} _kerberos._tcp.dc01.${ZONE} _ldap._tcp.dc01.${ZONE}; do
        printf '  %-40s ' \"\$n\"
        dig +short @${DC_IP} -t SRV \"\$n\" | tr '\n' ' '
        echo
    done
    printf '  %-40s ' dc01.${ZONE}
    dig +short @${DC_IP} -t A dc01.${ZONE} | tr '\n' ' '
    echo
" 2>/dev/null || true

# Verify seeded users
echo "[+] Verifying seeded users..."
docker exec aether-ad-lab ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "Passw0rd123!" -b "DC=aether,DC=test" "(|(sAMAccountName=user1)(sAMAccountName=user2)(sAMAccountName=user3)(sAMAccountName=user4)(sAMAccountName=svc_sql)(sAMAccountName=svc_web))" sAMAccountName userAccountControl servicePrincipalName

# List SPNs
echo "[+] Verifying SPNs..."
docker exec aether-ad-lab samba-tool spn list svc_sql 2>/dev/null || true
docker exec aether-ad-lab samba-tool spn list svc_web 2>/dev/null || true

# Verify group memberships
echo "[+] Verifying group memberships..."
docker exec aether-ad-lab samba-tool group listmembers "Domain Admins" 2>/dev/null || true
docker exec aether-ad-lab samba-tool group listmembers "Tier1-Admins" 2>/dev/null || true
docker exec aether-ad-lab samba-tool group listmembers "Tier2-Admins" 2>/dev/null || true

echo "[+] Seeding complete!"
echo ""
echo "Test identities created:"
echo "  user1         - Normal user, Passw0rd123!"
echo "  user2         - AS-REP roastable (DONT_REQ_PREAUTH), Passw0rd123!"
echo "  user3         - Domain Admin, Passw0rd123!"
echo "  user4         - ACL target (user1 has GenericAll), Passw0rd123!"
echo "  svc_sql       - SPN: MSSQLSvc/sql01.aether.test:1433, SvcPass123!"
echo "  svc_web       - SPN: HTTP/web01.aether.test, WebPass123!"
echo ""
echo "Groups:"
echo "  Tier1-Admins  -> Domain Admins (contains user3)"
echo "  Tier2-Admins  (contains user2)"
echo ""
echo "ACLs:"
echo "  user1 -> GenericAll on user4"
echo "  user2 -> WriteDACL on user1"