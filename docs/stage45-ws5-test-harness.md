# Stage 45 WS5 — Samba4 Test Harness

## Harness Architecture

```
scripts/ad-lab/
├── docker-compose.yml      # Samba4 AD DC service
├── start.sh                # Start container
├── seed.sh                 # Provision test users/groups
├── stop.sh                 # Stop and cleanup
└── README.md               # Usage documentation
```

## Docker Compose

```yaml
version: '3.8'
services:
  aether-ad-lab:
    image: ghcr.io/servercontainers/samba:latest
    hostname: dc01
    domainname: example.com
    container_name: aether-ad-lab
    privileged: true
    environment:
      - SAMBA_DOMAIN=EXAMPLE
      - SAMBA_REALM=EXAMPLE.COM
      - ADMIN_PASSWORD=Passw0rd123!
      - KERBEROS_ENCRYPTION_TYPES=aes256-cts-hmac-sha1-96,aes128-cts-hmac-sha1-96,rc4-hmac
    ports:
      - "53:53/tcp"
      - "53:53/udp"
      - "88:88/tcp"
      - "88:88/udp"
      - "135:135/tcp"
      - "139:139/tcp"
      - "389:389/tcp"
      - "389:389/udp"
      - "445:445/tcp"
      - "464:464/tcp"
      - "464:464/udp"
      - "636:636/tcp"
      - "3268:3268/tcp"
      - "3269:3269/tcp"
    volumes:
      - samba_data:/var/lib/samba
      - samba_etc:/etc/samba
    networks:
      - aether-ad-net
    healthcheck:
      test: ["CMD", "samba-tool", "domain", "info", "EXAMPLE.COM"]
      interval: 30s
      timeout: 10s
      retries: 5

volumes:
  samba_data:
  samba_etc:

networks:
  aether-ad-net:
    driver: bridge
```

## Test Identities (Seeded by `seed.sh`)

| Username | Password | Attributes | Purpose |
|----------|----------|------------|---------|
| `user1` | `Passw0rd123!` | Normal user, pre-auth required | Normal auth, TGT test |
| `user2` | `Passw0rd123!` | **No pre-auth** (DONT_REQ_PREAUTH) | AS-REP roast target |
| `user3` | `Passw0rd123!` | Domain Admin | Privilege escalation test |
| `svc_sql` | `SvcPass123!` | SPN: `MSSQLSvc/sql01.example.com:1433` | Kerberoast target |
| `svc_web` | `WebPass123!` | SPN: `HTTP/web01.example.com` | Kerberoast target |
| `krbtgt` | (auto) | KDC service account | TGT issuing |

## Integration Tests

Located in `test/integration/ad/`:

| Test | Description |
|------|-------------|
| `TestKerberosASREP` | Validates AS-REQ/AS-REP exchange |
| `TestKerberosTGSREP` | Validates TGS-REQ/TGS-REP exchange |
| `TestKerberoast` | Extracts hash from `svc_sql` SPN |
| `TestASREPRoast` | Detects `user2` as AS-REP roastable |
| `TestCcacheRoundTrip` | Read/write MIT/Heimdal ccache |
| `TestEngagementBoundary` | Rejects outside-domain targets |
| `TestCapabilityAuthZ` | Verifies capability deny/grant |
| `TestRateLimiting` | Verifies rate limit enforcement |

## Test Execution

```bash
# Start lab
./scripts/ad-lab/start.sh
sleep 30  # wait for healthcheck

# Seed identities
./scripts/ad-lab/seed.sh

# Run integration tests
go test -tags=integration ./test/integration/ad/... -v

# Cleanup
./scripts/ad-lab/stop.sh
```

## Lab Verification Gates

| Gate | Command | Expected |
|------|---------|----------|
| G2041 | `docker ps \| grep aether-ad-lab` | Container running |
| G2042 | `aether ad enum users EXAMPLE.COM dc01.example.com` | Returns user1, user2, user3, svc_sql, svc_web |
| G2043 | `go test -tags=integration ./test/integration/ad/...` | All PASS |

## Negative Controls

| Test | Expected |
|------|----------|
| `aether ad enum users OTHER.COM dc01.example.com` | Exit 1 (unauthorized domain) |
| `aether ad kerberoast EXAMPLE.COM dc01.example.com --spn HTTP/fake@OTHER.COM` | Exit 1 (unauthorized SPN) |
| `aether ad tgt EXAMPLE.COM dc01.example.com --username fake --password wrong` | Exit 1 (auth failed) |
| `aether ad kerberoast EXAMPLE.COM dc01.example.com --spn MSSQLSvc/sql01` (no creds) | Exit 1 (no credentials) |

## Lab Lifecycle

| Script | Action |
|--------|--------|
| `start.sh` | `docker-compose up -d` + healthcheck wait |
| `seed.sh` | `samba-tool user create`, `samba-tool spn add`, set `DONT_REQ_PREAUTH` on user2 |
| `stop.sh` | `docker-compose down -v` (volumes removed) |
| `reset.sh` | `stop.sh` + `start.sh` + `seed.sh` |

## CI Integration

```yaml
# .github/workflows/ad-integration.yml
jobs:
  ad-integration:
    runs-on: ubuntu-latest
    services:
      aether-ad-lab:
        image: ghcr.io/servercontainers/samba:latest
        # ... ports, env, volumes
    steps:
      - uses: actions/checkout@v4
      - run: ./scripts/ad-lab/start.sh && sleep 30
      - run: ./scripts/ad-lab/seed.sh
      - run: go test -tags=integration ./test/integration/ad/... -v
      - run: ./scripts/ad-lab/stop.sh
```

## Known Limitations

1. **Single DC only** — No multi-DC replication testing
2. **No LDAPS** — Samba4 LDAPS requires additional cert setup
3. **Kerberos encryption** — Only RC4-HMAC, AES128-CTS, AES256-CTS tested
4. **No PKINIT** — Deferred to Batch 3
5. **No cross-realm** — Single realm only

## Troubleshooting

| Issue | Resolution |
|-------|------------|
| Container fails to start | Check Docker privileged mode, port conflicts |
| Healthcheck fails | Increase timeout, check Samba logs: `docker logs aether-ad-lab` |
| `seed.sh` fails | Verify container fully started, check `samba-tool` output |
| Integration tests timeout | Increase test timeout, check network connectivity |
| Port conflicts | Ensure ports 53, 88, 135, 139, 389, 445, 464, 636, 3268, 3269 free |