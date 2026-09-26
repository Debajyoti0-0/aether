# Stage 45 WS1 — Kerberos Protocol Package

## Package: `internal/protocol/kerberos/`

### Files Created

| File | Description | Lines |
|------|-------------|-------|
| `types.go` | Core ASN.1 types per RFC 4120 §5 | ~260 |
| `errors.go` | KDC error codes and classification | ~220 |
| `principal.go` | Principal name and realm parsing | ~110 |
| `etypes.go` | Encryption type negotiation and metadata | ~110 |
| `crypto.go` | RC4-HMAC, AES128-CTS, AES256-CTS crypto | ~270 |
| `asreq.go` | AS-REQ construction and AS-REP parsing | ~160 |
| `tgsreq.go` | TGS-REQ/TGS-REP, AP-REQ/AP-REP, authenticator | ~250 |
| `pac.go` | PAC parsing and verification (MS-PAC) | ~370 |
| `ccache.go` | MIT/Heimdal ccache read/write/convert | ~520 |
| `spn.go` | SPN parsing and canonicalization | ~90 |
| `kerberos_test.go` | Unit tests for all protocol functions | ~330 |

**Total**: ~2,670 lines of protocol implementation + tests

### Key Function Signatures

```go
// Principal handling
func ParsePrincipal(s string) (PrincipalName, Realm, error)
func MakeUserPrincipal(user, realm string) PrincipalName
func MakeSPNPrincipal(service, host, realm string) PrincipalName
func RealmFromString(s string) (Realm, error)

// Encryption types
func SupportedEtypes() []int32
func PreferredEtype(offered []int32) int32
func IsSupported(etype int32) bool
func KeySize(etype int32) (int, error)
func EtypeName(etype int32) string

// Key derivation
func StringToKey(etype int32, password, salt, params string) ([]byte, error)
func DeriveKey(key []byte, usage int32, etype int32) ([]byte, error)

// Encryption/Decryption
func Encrypt(etype int32, key []byte, usage int32, plaintext []byte) ([]byte, error)
func Decrypt(etype int32, key []byte, usage int32, ciphertext []byte) ([]byte, error)
func ComputeChecksum(etype int32, key []byte, usage int32, data []byte) ([]byte, error)
func VerifyChecksum(etype int32, key []byte, usage int32, data, checksum []byte) error

// AS-REQ/AS-REP
func BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, etypes, nonce, till, paData) ([]byte, error)
func BuildASREQWithPreauth(clientPrincipal, clientRealm, serverPrincipal, etypes, nonce, till, password) ([]byte, error)
func ParseASREP(data []byte) (*KDCREP, error)
func DecryptASREPEncPart(rep *KDCREP, key []byte, etype int32) (*EncTicketPart, error)
func BuildPAEncTimestamp(password, clientPrincipal, clientRealm, etype) (PAData, error)

// TGS-REQ/TGS-REP
func BuildTGSREQ(clientPrincipal, clientRealm, serverPrincipal, ticket, etypes, nonce, till, sessionKey, authenticator) ([]byte, error)
func BuildTGSREQWithAuthenticator(clientPrincipal, clientRealm, serverPrincipal, ticket, etypes, nonce, till, sessionKey, subkey, seqNumber) ([]byte, error)
func ParseTGSREP(data []byte) (*KDCREP, error)
func DecryptTGSREPEncPart(rep *KDCREP, key []byte, etype int32) (*EncTicketPart, error)
func BuildAPREQ(ticket, sessionKey, clientPrincipal, clientRealm, subkey, seqNumber, mutualRequired) ([]byte, error)
func ParseAPREQ(data []byte) (*APREQ, error)
func ParseAPREP(data []byte) (*APREP, error)
func BuildAuthenticator(clientPrincipal, clientRealm, sessionKey, subkey, seqNumber) ([]byte, error)

// PAC
func ParsePAC(data []byte) (*PAC, error)
func (pac *PAC) GetLogonInfo() (*PACLogonInfoV1, error)
func (pac *PAC) GetClientInfo() (*PACClientInfo, error)
func (pac *PAC) GetUpnDnsInfo() (*PACUpnDnsInfo, error)
func (pac *PAC) GetSignature() (*PACSignatureData, error)
func (pac *PAC) VerifyServerSignature(key []byte) error

// CCache
func ReadCCache(filename string) (*CCache, error)
func (c *CCache) WriteToFile(filename string) error
func (c *CCache) AddEntry(entry CCacheEntry)
func (c *CCache) GetEntry(clientPrincipal, clientRealm, serverPrincipal, serverRealm) *CCacheEntry
func ConvertMITToHeimdal(mitPath, heimdalPath string) error
func ConvertHeimdalToMIT(heimdalPath, mitPath string) error

// SPN
func ParseSPN(s string) (SPN, error)
func (spn SPN) PrincipalName() PrincipalName
func CanonicalizeSPN(spn string) (string, error)
func ExtractSPNsFromTicket(ticket *Ticket) []SPN
```

### Encryption Types Supported

| EType | Name | Key Size | Checksum Size | Status |
|-------|------|----------|---------------|--------|
| 17 | aes128-cts-hmac-sha1-96 | 16 bytes | 12 bytes | ✅ Implemented |
| 18 | aes256-cts-hmac-sha1-96 | 32 bytes | 12 bytes | ✅ Implemented |
| 23 | rc4-hmac | 16 bytes | 12 bytes | ✅ Implemented |

### Known Limitations

1. **PKINIT** (RFC 4556) — Deferred to Batch 3 (Stage 47)
2. **DES/3DES** — Not implemented (deprecated)
3. **AES-GCM** (RFC 8009) — Not implemented
4. **FAST/Pre-auth Framework** (RFC 6113) — Not implemented
5. **Cross-realm TGT** — Not implemented
5. **Kerberos Constrained Delegation** — Deferred to later stage

### Test Coverage

All 18 unit tests pass:
- Principal parsing (3 cases)
- Principal/SPN construction
- Encryption type support/preference/key size
- KerberosTime marshal/unmarshal
- StringToKey for RC4, AES128, AES256
- Encrypt/Decrypt for all 3 etypes
- Tamper detection (HMAC verification)
- Wrong key detection
- SPN parsing (3 cases)
- SPN canonicalization
- Error classification (8 error codes)

### Dependencies

- Go stdlib only: `crypto/*`, `encoding/asn1`, `encoding/binary`, `time`
- `golang.org/x/crypto/pbkdf2` (already in go.mod)
- **No external Kerberos libraries** (impacket, gokrb5, etc.)

### Next Steps (WS2)

Implement Batch 1 engines in `internal/engine/ad/kerberos/`:
- `EnumUsers` — Username validation via AS-REQ
- `EnumASREP` — Detect accounts without pre-auth
- `EnumSPN` — Enumerate SPN-registered accounts
- `Kerberoast` — Request TGS per SPN → extract crackable blob
- `ASREPRoast` — Request AS-REP for no-preauth accounts
- `AcquireTGT` — Get TGT for valid credential → ccache
- `CCache` — Read/write/inspect ccache files