#!/bin/bash
# Provision Samba AD Domain

set -euo pipefail

REALM="${SAMBA_REALM:-AETHER.TEST}"
DOMAIN="${SAMBA_DOMAIN:-AETHER}"
ADMIN_PASS="${ADMIN_PASSWORD:-Passw0rd123!}"
DNS_FORWARDER="${DNS_FORWARDER:-1.1.1.1}"

echo "[+] Provisioning Samba AD Domain..."
echo "[+] Realm: ${REALM}"
echo "[+] Domain: ${DOMAIN}"

# Check if already provisioned
if [ -f /var/lib/samba/private/sam.ldb ]; then
    echo "[+] Domain already provisioned, skipping..."
    exit 0
fi

# Remove existing smb.conf if it exists (provision needs clean slate)
if [ -f /etc/samba/smb.conf ]; then
    echo "[+] Removing existing smb.conf..."
    rm -f /etc/samba/smb.conf
fi

# Provision the domain
samba-tool domain provision \
    --realm="${REALM}" \
    --domain="${DOMAIN}" \
    --adminpass="${ADMIN_PASS}" \
    --server-role=dc \
    --dns-backend=SAMBA_INTERNAL \
    --use-rfc2307 \
    --option="interfaces=lo eth0" \
    --option="bind interfaces only=yes"

# Configure Kerberos encryption types (command may not exist in all Samba versions)
if [ -n "${KERBEROS_ENCRYPTION_TYPES}" ]; then
    samba-tool domain setencryptiontypes "${KERBEROS_ENCRYPTION_TYPES}" 2>/dev/null || echo "[!] setencryptiontypes not available, skipping..."
fi

# Set DNS forwarder
if [ -n "${DNS_FORWARDER}" ]; then
    samba-tool dns zonecreate "${REALM}" "_msdcs.${REALM}" || true
    samba-tool dns add "${REALM}" @ NS dc01.${REALM} || true
    samba-tool dns add "${REALM}" @ A $(hostname -I | awk '{print $1}') || true
fi

echo "[+] Domain provisioning complete!"

# Create initial users and groups
create_test_users() {
    echo "[+] Creating test users..."
    
    # Normal user
    samba-tool user create user1 Passw0rd123! \
        --given-name=Test \
        --surname=User1 \
        --mail-address=user1@aether.test \
        --description="Normal test user" || true
    
    # AS-REP roastable user (no pre-auth)
    samba-tool user create user2 Passw0rd123! \
        --given-name=Test \
        --surname=User2 \
        --mail-address=user2@aether.test \
        --description="AS-REP roastable user" || true
    
    # Set DONT_REQ_PREAUTH on user2 (0x400000)
    samba-tool user setexpiry user2 --noexpiry || true
    ldbmodify -H /var/lib/samba/private/sam.ldb \
        -b "CN=user2,CN=Users,DC=aether,DC=test" \
        --controls=localoid:1.3.6.1.4.1.4203.1.11.2 \
        --option="controls=1.3.6.1.4.1.4203.1.11.2" \
        <<EOF
dn: CN=user2,CN=Users,DC=aether,DC=test
changetype: modify
replace: userAccountControl
userAccountControl: 4194304
EOF
    
    # Domain Admin user
    samba-tool user create user3 Passw0rd123! \
        --given-name=Admin \
        --surname=User3 \
        --mail-address=user3@aether.test \
        --description="Domain Admin test user" || true
    
    samba-tool group addmembers "Domain Admins" user3 || true
    
    # ACL target user
    samba-tool user create user4 Passw0rd123! \
        --given-name=Target \
        --surname=User4 \
        --mail-address=user4@aether.test \
        --description="ACL target user" || true
    
    # Service account for Kerberoasting
    samba-tool user create svc_sql SvcPass123! \
        --given-name=SQL \
        --surname=Service \
        --mail-address=svc_sql@aether.test \
        --description="Service account for MSSQL" || true
    
    samba-tool spn add MSSQLSvc/sql01.aether.test:1433 svc_sql || true
    
    samba-tool user create svc_web WebPass123! \
        --given-name=Web \
        --surname=Service \
        --mail-address=svc_web@aether.test \
        --description="Service account for HTTP" || true
    
    samba-tool spn add HTTP/web01.aether.test svc_web || true
    
    # Create groups for nested ACL testing
    samba-tool group add "Tier1-Admins" --description="Tier 1 Administrators" || true
    samba-tool group add "Tier2-Admins" --description="Tier 2 Administrators" || true
    
    # Nest Tier1-Admins into Domain Admins
    samba-tool group addmembers "Domain Admins" "Tier1-Admins" || true
    
    # Add user3 to Tier1-Admins
    samba-tool group addmembers "Tier1-Admins" user3 || true
    
    # Add user2 to Tier2-Admins
    samba-tool group addmembers "Tier2-Admins" user2 || true
}

# Run user creation
create_test_users || echo "[!] Some user operations failed, continuing..."

# Set ACLs
set_acls() {
    echo "[+] Setting test ACLs..."
    
    # Get SIDs
    USER1_SID=$(samba-tool user show user1 --attributes=objectSid | awk -F': ' '/objectSid/ {print $2}')
    USER2_SID=$(samba-tool user show user2 --attributes=objectSid | awk -F': ' '/objectSid/ {print $2}')
    
    # Get DNs
    USER1_DN=$(ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "${ADMIN_PASS}" -b "DC=aether,DC=test" "(sAMAccountName=user1)" dn | grep "^dn:" | head -1 | sed 's/^dn: //')
    USER4_DN=$(ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "${ADMIN_PASS}" -b "DC=aether,DC=test" "(sAMAccountName=user4)" dn | grep "^dn:" | head -1 | sed 's/^dn: //')
    
    # Set ACL: user1 -> GenericAll on user4
    if [ -n "${USER1_SID}" ] && [ -n "${USER4_DN}" ]; then
        ldbmodify -H /var/lib/samba/private/sam.ldb -b "${USER4_DN}" <<EOF
dn: ${USER4_DN}
changetype: modify
add: nTSecurityDescriptor
nTSecurityDescriptor: O:DAG:DAD:(A;;GA;;;${USER1_SID})
EOF
    fi
    
    # user2 -> WriteDACL on user1
    USER1_DN=$(ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "${ADMIN_PASS}" -b "DC=aether,DC=test" "(sAMAccountName=user1)" dn | grep "^dn:" | head -1 | sed 's/^dn: //')
    USER2_SID=$(samba-tool user show user2 --attributes=objectSid | awk -F': ' '/objectSid/ {print $2}')
    
    if [ -n "${USER2_SID}" ] && [ -n "${USER1_DN}" ]; then
        ldbmodify -H /var/lib/samba/private/sam.ldb -b "${USER1_DN}" <<EOF
dn: ${USER1_DN}
changetype: modify
add: nTSecurityDescriptor
nTSecurityDescriptor: O:DAG:DAD:(A;;WP;;;${USER2_SID})
EOF
    fi
}

set_acls || echo "[!] ACL setup had issues, continuing..."

echo "[+] Provisioning complete!"