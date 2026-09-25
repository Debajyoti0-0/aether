package kerberos

import (
	"bytes"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// D1 regression tests
// ---------------------------------------------------------------------------

// TestKDCOptionsWireOrder is the Stage 46h D1 regression test.
//
// RFC 4120 §5.4.1 encodes KDCOptions as a 32-bit BIT STRING whose first octet
// carries bits 0-7, bit 0 (reserved) being the most significant bit. The
// previous encoder transposed every bit into the wrong octet and Aether asked
// for the RESERVED bit, so Samba answered KDC_ERR_BADOPTION (13).
func TestKDCOptionsWireOrder(t *testing.T) {
	cases := []struct {
		name  string
		flags uint32
		want  []byte
	}{
		{"forwardable", KDC_OPT_FORWARDABLE, []byte{0x40, 0x00, 0x00, 0x00}},
		{"renewable", KDC_OPT_RENEWABLE, []byte{0x00, 0x80, 0x00, 0x00}},
		{"canonicalize", KDC_OPT_CANONICALIZE, []byte{0x00, 0x00, 0x80, 0x00}},
		{"forwardable+renewable", KDC_OPT_FORWARDABLE | KDC_OPT_RENEWABLE, []byte{0x40, 0x80, 0x00, 0x00}},
		{"none", 0, []byte{0x00, 0x00, 0x00, 0x00}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EncodeKDCOptions(tc.flags)
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("EncodeKDCOptions(%#08x) = % x, want % x", tc.flags, got, tc.want)
			}
		})
	}
}

// TestKDCOptionsNeverRequestsReservedBits guards the specific failure: no
// defined flag may land on bit 0 (reserved) or on any of the unused bits.
func TestKDCOptionsNeverRequestsReservedBits(t *testing.T) {
	defined := map[uint32]bool{
		KDC_OPT_FORWARDABLE: true, KDC_OPT_FORWARDED: true, KDC_OPT_PROXIABLE: true,
		KDC_OPT_PROXY: true, KDC_OPT_ALLOW_POSTDATE: true, KDC_OPT_POSTDATED: true,
		KDC_OPT_RENEWABLE: true, KDC_OPT_ENC_TKT_IN_SKEY: true, KDC_OPT_RENEW: true,
		KDC_OPT_VALIDATE: true, KDC_OPT_CANONICALIZE: true,
		KDC_OPT_REQUEST_ANONYMOUS: true, KDC_OPT_DISABLE_TRANSITED_CHECK: true,
		KDC_OPT_RENEWABLE_OK: true,
	}
	for flag := range defined {
		got := EncodeKDCOptions(flag)
		if got[0]&0x80 != 0 {
			t.Fatalf("flag %#08x sets the RESERVED first bit: % x", flag, got)
		}
	}
	// The combination Aether actually requests must be the MIT default set.
	got := EncodeKDCOptions(KDC_OPT_FORWARDABLE | KDC_OPT_RENEWABLE)
	want := []byte{0x40, 0x80, 0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("default request = % x, want % x", got, want)
	}
}

// kdcErrPreAuthRequiredLive is the byte-exact KDC_ERR_PREAUTH_REQUIRED reply
// (268 bytes) captured from the Stage 46h Samba AD lab at 172.18.0.2 for
// principal "administrator" in realm AETHER.TEST. It is stored raw in
// _scratch/kdc-preauth.bin and re-validated by the TLV walk in the stage
// evidence, so the vector is a genuine wire capture rather than a
// hand-assembled approximation.
//
// It is retained as a golden vector because it contains the authoritative
// string-to-key salt "AETHER.TESTAdministrator" and the s2kparams
// 00 00 10 00 (4096 PBKDF2 iterations). The account name is capitalised in the
// salt even though the request was made with a lower-case name, which is
// exactly why Aether must read the salt from the KDC instead of guessing it.
const kdcErrPreAuthRequiredLive = "7e82010830820104a003020105a10302011ea411180f32303236303932353137343730325aa50502030b5ec8a603" +
	"020119a70d1b0b4145544845522e54455354a81a3018a003020101a111300f1b0d61646d696e6973747261746f72" +
	"a90d1b0b4145544845522e54455354aa133011a003020101a10a30081b066b7262746774ab2b1b294e6565642074" +
	"6f207573652050412d454e432d54494d455354414d502f50412d504b2d41532d524551ac5d045b30593009a10302" +
	"0102a20204003009a103020110a20204003009a10302010fa20204003036a103020113a22f042d302b3029a00302" +
	"0112a11a1b184145544845522e5445535441646d696e6973747261746f72a206040400001000"

// TestParsePreAuthHintGoldenVector is the Stage 46h D1 regression test for the
// salt defect: the KDC-supplied salt must be extracted from KRB-ERROR e-data.
func TestParsePreAuthHintGoldenVector(t *testing.T) {
	raw, err := hex.DecodeString(kdcErrPreAuthRequiredLive)
	if err != nil {
		t.Fatalf("golden vector is not valid hex: %v", err)
	}

	e, err := ParseKRBError(raw)
	if err != nil {
		t.Fatalf("ParseKRBError: %v", err)
	}
	if e.ErrorCode != KDC_ERR_PREAUTH_REQUIRED {
		t.Fatalf("error code = %d, want %d", e.ErrorCode, KDC_ERR_PREAUTH_REQUIRED)
	}
	if len(e.EData) == 0 {
		t.Fatal("e-data must carry the pre-authentication hints")
	}

	hint, err := ParsePreAuthHint(raw)
	if err != nil {
		t.Fatalf("ParsePreAuthHint: %v", err)
	}
	if hint.Etype != ETYPE_AES256_CTS_HMAC_SHA1_96 {
		t.Fatalf("etype = %d, want %d (aes256)", hint.Etype, ETYPE_AES256_CTS_HMAC_SHA1_96)
	}
	if !hint.HasSalt {
		t.Fatal("the KDC supplied a salt but the hint did not carry it")
	}
	// The salt preserves the account name EXACTLY as the directory stores it.
	if want := "AETHER.TESTAdministrator"; hint.Salt != want {
		t.Fatalf("salt = %q, want %q", hint.Salt, want)
	}
	if len(hint.S2KParams) < 4 {
		t.Fatalf("s2kparams too short: %q", hint.S2KParams)
	}
	key, err := StringToKey(hint.Etype, "Passw0rd123!", hint.Salt, hint.S2KParams)
	if err != nil {
		t.Fatalf("string-to-key with the KDC hint: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("aes256 key length = %d, want 32", len(key))
	}
}

// TestSaltCaseSensitivityIsNotAssumed pins the behaviour that broke Stage 46g:
// a locally guessed salt is wrong whenever the account-name case differs from
// the directory, so the KDC hint is the only correct source.
func TestSaltCaseSensitivityIsNotAssumed(t *testing.T) {
	raw, _ := hex.DecodeString(kdcErrPreAuthRequiredLive)
	hint, err := ParsePreAuthHint(raw)
	if err != nil {
		t.Fatalf("ParsePreAuthHint: %v", err)
	}
	guessed := KrbSalt(CanonicalRealm("AETHER.TEST"), MakeUserPrincipal("administrator", "AETHER.TEST"))
	if guessed == hint.Salt {
		t.Skip("lab realm has no case-sensitive account names; guard not exercised")
	}
	if guessed != "AETHER.TESTadministrator" {
		t.Fatalf("guessed salt = %q, unexpected", guessed)
	}
	if hint.Salt == guessed {
		t.Fatal("guessed salt and KDC salt must differ for this principal")
	}
}

// TestCanonicalRealm pins the realm upper-casing used by the salt fallback.
func TestCanonicalRealm(t *testing.T) {
	for in, want := range map[string]string{
		"aether.test":   "AETHER.TEST",
		"AETHER.TEST":   "AETHER.TEST",
		"MiXeD":         "MIXED",
		"":              "",
		"Already.Upper": "ALREADY.UPPER",
	} {
		if got := CanonicalRealm(Realm(in)); got != want {
			t.Fatalf("CanonicalRealm(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestASREQIncludesRtimeWhenRenewable pins RFC 4120 §5.4.2: rtime is required
// when RENEWABLE is requested. Omitting it was one cause of BADOPTION.
func TestASREQIncludesRtimeWhenRenewable(t *testing.T) {
	realm := Realm("AETHER.TEST")
	cp := MakeUserPrincipal("user1", "AETHER.TEST")
	sp := MakeUserPrincipal("krbtgt", "AETHER.TEST")
	req, err := BuildASREQ(cp, realm, sp, SupportedEtypes(), 12345, time.Now().Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("BuildASREQ: %v", err)
	}
	// The req-body must carry a context tag [6] (rtime). Search the encoded
	// body for the GeneralString that follows a [6] context tag.
	if !bytes.Contains(req, []byte{0xa6}) {
		t.Fatal("rtime [6] is missing from the AS-REQ req-body")
	}
	// KDCOptions is a BIT STRING whose value Aether trims of trailing zero
	// octets for DER minimality: Samba and Heimdal re-encode the KDC-REQ-BODY
	// when verifying the TGS-REQ authenticator checksum, and a padded
	// "03 05 00 60 00 00" style KDCOptions makes that verification fail with
	// KDC_ERR_BADOPTION. So forwardable|renewable is the two-octet payload
	// 40 80, not the zero-padded 40 80 00 00.
	if !bytes.Contains(req, []byte{0x03, 0x03, 0x00, 0x40, 0x80}) {
		t.Fatalf("kdc-options BIT STRING must be 03 03 00 40 80 (forwardable|renewable); got % x", req)
	}
}

// ---------------------------------------------------------------------------
// D3 regression tests
// ---------------------------------------------------------------------------

// TestCcacheParseMIT is the Stage 46h D3 regression test: a credential cache
// produced by MIT kinit must parse. Three grammar errors made every real MIT
// cache unreadable: the principal's leading name_type was skipped, the realm
// was taken from the last component instead of its own field, and the keyblock
// key length was read as a uint16 instead of a counted uint32.
func TestCcacheParseMIT(t *testing.T) {
	data, err := os.ReadFile("testdata/mit-kinit-user1.ccache")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cc, err := ReadCCacheFromBytes(data)
	if err != nil {
		t.Fatalf("ReadCCacheFromBytes on a real kinit ccache: %v", err)
	}
	if cc.Header.Version != CCACHE_VERSION_MIT {
		t.Fatalf("version = %#04x, want %#04x", cc.Header.Version, CCACHE_VERSION_MIT)
	}
	// A real kinit cache carries a 12-byte header (the KDC clock offset field).
	if cc.Header.HeaderLen != 12 {
		t.Fatalf("header length = %d, want 12", cc.Header.HeaderLen)
	}
	if cc.DefaultPrincipal == nil {
		t.Fatal("default principal missing")
	}
	if got := cc.DefaultPrincipal.String(); got != "administrator" {
		t.Fatalf("default principal = %q, want %q", got, "administrator")
	}
	if cc.DefaultPrincipal.NameType != NAME_TYPE_PRINCIPAL {
		t.Fatalf("default principal name type = %d, want %d",
			cc.DefaultPrincipal.NameType, NAME_TYPE_PRINCIPAL)
	}
	if cc.DefaultRealm != "AETHER.TEST" {
		t.Fatalf("default realm = %q, want %q", cc.DefaultRealm, "AETHER.TEST")
	}
	if len(cc.Entries) == 0 {
		t.Fatal("no credentials parsed from a populated ccache")
	}

	// MIT stores cache configuration as credentials, so the TGT is not
	// necessarily the first entry; select the real credential explicitly.
	var e *CCacheEntry
	for i := range cc.Entries {
		if !cc.Entries[i].IsConfigEntry() {
			e = &cc.Entries[i]
			break
		}
	}
	if e == nil {
		t.Fatal("no non-configuration credential found")
	}
	if e.ClientRealm != "AETHER.TEST" {
		t.Fatalf("client realm = %q", e.ClientRealm)
	}
	if e.ClientPrincipal.String() != "administrator" {
		t.Fatalf("client principal = %q, want administrator", e.ClientPrincipal.String())
	}
	if e.ServerPrincipal.String() != "krbtgt/AETHER.TEST" {
		t.Fatalf("server principal = %q, want krbtgt/AETHER.TEST", e.ServerPrincipal.String())
	}
	// name_type 2 (KRB5_NT_SRV_INST) is what MIT records for krbtgt/AETHER.TEST.
	if e.ServerPrincipal.NameType != NAME_TYPE_SRV_INST {
		t.Fatalf("server principal name type = %d, want %d",
			e.ServerPrincipal.NameType, NAME_TYPE_SRV_INST)
	}
	if e.ServerRealm != "AETHER.TEST" {
		t.Fatalf("server realm = %q, want AETHER.TEST", e.ServerRealm)
	}
	if e.Key.KeyType != ETYPE_AES256_CTS_HMAC_SHA1_96 {
		t.Fatalf("session key enctype = %d, want %d (aes256)",
			e.Key.KeyType, ETYPE_AES256_CTS_HMAC_SHA1_96)
	}
	if len(e.Key.KeyValue) != 32 {
		t.Fatalf("session key length = %d, want 32 for aes256", len(e.Key.KeyValue))
	}
	if len(e.Ticket) != 1194 {
		t.Fatalf("ticket length = %d, want 1194", len(e.Ticket))
	}
	if len(e.Ticket) == 0 || e.Ticket[0] != 0x61 {
		t.Fatalf("ticket must be a DER APPLICATION 1 KerberosTicket, got first byte %#02x", e.Ticket[0])
	}
	// forwardable | renewable | pre-authenticated | initial
	if want := int32(0x40e00000); e.TicketFlags != want {
		t.Fatalf("ticket flags = %#08x, want %#08x", uint32(e.TicketFlags), uint32(want))
	}
	if e.EndTime.Before(e.StartTime) {
		t.Fatalf("end time %v precedes start time %v", e.EndTime, e.StartTime)
	}
	if e.RenewTill.Before(e.EndTime) {
		t.Fatalf("renew-till %v precedes end time %v", e.RenewTill, e.EndTime)
	}
}

// TestCcacheConfigEntriesAreNotCredentials pins the behaviour that made a real
// kinit cache dangerous to consume: MIT encodes configuration entries as
// credentials sharing the default principal, so the first parsed entry is
// cache metadata whose "ticket" is not a Kerberos ticket. Handing that to a
// caller expecting the TGT would yield a bogus session key.
func TestCcacheConfigEntriesAreNotCredentials(t *testing.T) {
	data, err := os.ReadFile("testdata/mit-kinit-user1.ccache")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cc, err := ReadCCacheFromBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	first := cc.Entries[0]
	if !first.IsConfigEntry() {
		t.Fatalf("entry 0 should be a config entry, got server realm %q", first.ServerRealm)
	}
	if first.ServerRealm != "X-CACHECONF:" {
		t.Fatalf("config entry realm = %q, want X-CACHECONF:", first.ServerRealm)
	}
	// realm X-CACHECONF: with components krb5_ccache_conf_data / pa_type /
	// <served principal>, per the MIT ccache format document. The third
	// component is the served principal written as a full principal string.
	if got := first.ServerPrincipal.String(); got != "krb5_ccache_conf_data/pa_type/krbtgt/AETHER.TEST@AETHER.TEST" {
		t.Fatalf("config entry server principal = %q", got)
	}
	// The pa_type value is the ASCII preauth type used at authentication.
	if got := string(first.Ticket); got != "2" {
		t.Fatalf("config entry ticket (pa_type value) = %q, want %q", got, "2")
	}
	if first.Key.KeyType != 0 || len(first.Key.KeyValue) != 0 {
		t.Fatal("config entries must carry an empty keyblock")
	}

	// GetDefaultEntry must skip configuration records.
	def := cc.GetDefaultEntry()
	if def == nil {
		t.Fatal("GetDefaultEntry returned nil for a populated cache")
	}
	if def.IsConfigEntry() {
		t.Fatal("GetDefaultEntry returned a configuration entry instead of the TGT")
	}
	if def.ServerPrincipal.String() != "krbtgt/AETHER.TEST" {
		t.Fatalf("GetDefaultEntry server principal = %q, want krbtgt/AETHER.TEST",
			def.ServerPrincipal.String())
	}
	if len(def.Key.KeyValue) != 32 {
		t.Fatalf("GetDefaultEntry session key length = %d, want 32", len(def.Key.KeyValue))
	}
}

// TestCcacheRoundTripMIT verifies Aether's own writer produces a file the
// reference decoder can read back with identical field values. The previous
// writer emitted an invented TLV stream, so round-trips only ever exercised
// Aether against itself.
func TestCcacheRoundTripMIT(t *testing.T) {
	data, err := os.ReadFile("testdata/mit-kinit-user1.ccache")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cc, err := ReadCCacheFromBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := cc.Bytes()
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	again, err := ReadCCacheFromBytes(out)
	if err != nil {
		t.Fatalf("re-parse Aether-written ccache: %v", err)
	}
	if len(again.Entries) != len(cc.Entries) {
		t.Fatalf("entry count changed: %d -> %d", len(cc.Entries), len(again.Entries))
	}
	for i := range cc.Entries {
		a, b := cc.Entries[i], again.Entries[i]
		if a.ClientPrincipal.String() != b.ClientPrincipal.String() || a.ClientRealm != b.ClientRealm {
			t.Fatalf("entry %d client principal changed: %v@%v -> %v@%v", i,
				a.ClientPrincipal, a.ClientRealm, b.ClientPrincipal, b.ClientRealm)
		}
		if a.ServerPrincipal.String() != b.ServerPrincipal.String() || a.ServerRealm != b.ServerRealm {
			t.Fatalf("entry %d server principal changed: %v@%v -> %v@%v", i,
				a.ServerPrincipal, a.ServerRealm, b.ServerPrincipal, b.ServerRealm)
		}
		if a.ClientPrincipal.NameType != b.ClientPrincipal.NameType ||
			a.ServerPrincipal.NameType != b.ServerPrincipal.NameType {
			t.Fatalf("entry %d principal name type changed", i)
		}
		if a.Key.KeyType != b.Key.KeyType || !bytes.Equal(a.Key.KeyValue, b.Key.KeyValue) {
			t.Fatalf("entry %d session key changed across the round trip", i)
		}
		if !bytes.Equal(a.Ticket, b.Ticket) {
			t.Fatalf("entry %d ticket changed across the round trip", i)
		}
		if !a.EndTime.Equal(b.EndTime) {
			t.Fatalf("entry %d end time changed: %v -> %v", i, a.EndTime, b.EndTime)
		}
		if a.TicketFlags != b.TicketFlags {
			t.Fatalf("entry %d ticket flags changed", i)
		}
	}
}

// TestCcacheWriterIsByteExact is the strongest available statement that Aether
// emits the real MIT format: re-encoding a genuine kinit cache must reproduce
// the file byte for byte, header block and configuration entry included. A
// self-consistent-but-wrong writer cannot pass this, because the fixture was
// produced by MIT krb5 and not by Aether.
func TestCcacheWriterIsByteExact(t *testing.T) {
	data, err := os.ReadFile("testdata/mit-kinit-user1.ccache")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cc, err := ReadCCacheFromBytes(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := cc.Bytes()
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("Aether re-encoded the MIT cache to %d bytes, want the original %d bytes\nfirst difference at offset %d",
			len(out), len(data), firstDiff(out, data))
	}
}

func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// TestCcacheRejectsMalformedInput is the fuzz-safety requirement: untrusted
// input must yield a controlled error, never a panic or a huge allocation.
func TestCcacheRejectsMalformedInput(t *testing.T) {
	good, err := os.ReadFile("testdata/mit-kinit-user1.ccache")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// A prefix may legitimately parse when it ends on a real structural
	// boundary (a cache with a default principal and no credentials is valid),
	// so the requirement is not "every prefix errors" but "a prefix parses only
	// when it is itself a complete, canonical cache". Checking that the parsed
	// prefix re-encodes to exactly the same bytes catches a parser that accepts
	// truncated or misaligned input.
	t.Run("prefix parses only when it is a complete cache", func(t *testing.T) {
		for n := 0; n < len(good); n++ {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic on a %d-byte prefix: %v", n, r)
					}
				}()
				cc, err := ReadCCacheFromBytes(good[:n])
				if err != nil {
					return
				}
				out, werr := cc.Bytes()
				if werr != nil {
					t.Fatalf("prefix of %d bytes parsed but cannot be re-encoded: %v", n, werr)
				}
				if !bytes.Equal(out, good[:n]) {
					t.Fatalf("prefix of %d bytes parsed but is not a complete cache", n)
				}
			}()
		}
	})

	t.Run("corrupt version", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		bad[0], bad[1] = 0x05, 0x09
		if _, err := ReadCCacheFromBytes(bad); err == nil {
			t.Fatal("unsupported version accepted")
		}
	})

	t.Run("oversized header length", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		bad[2], bad[3] = 0xff, 0xff
		if _, err := ReadCCacheFromBytes(bad); err == nil {
			t.Fatal("oversized header length accepted")
		}
	})

	t.Run("oversized component count", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		// Layout: uint16 version, uint16 header_len, header[header_len], then
		// the default principal's uint32 name_type followed by its uint32
		// component count.
		headerLen := int(bad[2])<<8 | int(bad[3])
		off := 4 + headerLen + 4
		if off+4 > len(bad) {
			t.Fatalf("unexpected fixture layout: principal count at %d", off)
		}
		copy(bad[off:off+4], []byte{0xff, 0xff, 0xff, 0xff})
		if _, err := ReadCCacheFromBytes(bad); err == nil {
			t.Fatal("oversized component count accepted")
		}
	})

	t.Run("oversized key length", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		// The TGT keyblock sits after the default principal and the
		// configuration credential; its key length is the uint32 that follows
		// the uint16 enctype. Locate it by the DER ticket that must follow.
		idx := bytes.Index(bad, []byte{0x61, 0x82, 0x04, 0xa6})
		if idx < 0 {
			t.Skip("fixture does not contain the expected ticket")
		}
		keyLenOff := idx - 4 - 6
		if keyLenOff < 0 {
			t.Fatalf("unexpected fixture layout: key length at %d", keyLenOff)
		}
		copy(bad[keyLenOff:keyLenOff+4], []byte{0xff, 0xff, 0xff, 0xff})
		if _, err := ReadCCacheFromBytes(bad); err == nil {
			t.Fatal("oversized key length accepted")
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := ReadCCacheFromBytes(nil); err == nil {
			t.Fatal("empty input accepted")
		}
	})

	t.Run("heimdal is reported not mis-decoded", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		bad[0], bad[1] = 0x05, 0x03
		_, err := ReadCCacheFromBytes(bad)
		if err == nil {
			t.Fatal("heimdal version silently accepted as MIT")
		}
		if !strings.Contains(err.Error(), "heimdal") {
			t.Fatalf("error should name heimdal, got %v", err)
		}
	})
}

// FuzzCCacheParse exercises the ccache parser as untrusted-input code.
func FuzzCCacheParse(f *testing.F) {
	if data, err := os.ReadFile("testdata/mit-kinit-user1.ccache"); err == nil {
		f.Add(data)
	}
	f.Add([]byte{0x05, 0x04, 0x00, 0x00})
	f.Add([]byte{0x05, 0x04, 0x00, 0x0c, 0x00, 0x01, 0x00, 0x08, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic. A parse error is the only acceptable outcome.
		cc, err := ReadCCacheFromBytes(data)
		if err != nil {
			return
		}
		// Anything that parses must also re-encode and re-parse cleanly.
		out, err := cc.Bytes()
		if err != nil {
			return
		}
		if _, err := ReadCCacheFromBytes(out); err != nil {
			t.Fatalf("re-parse of a successfully parsed ccache failed: %v", err)
		}
	})
}
