package kerberos

import (
	"testing"
	"time"
)

func TestPrincipalParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user@EXAMPLE.COM", "user@EXAMPLE.COM"},
		{"host/server.example.com@EXAMPLE.COM", "host/server.example.com@EXAMPLE.COM"},
		{"HTTP/web.example.com@EXAMPLE.COM", "HTTP/web.example.com@EXAMPLE.COM"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pn, realm, err := ParsePrincipal(tt.input)
			if err != nil {
				t.Fatalf("ParsePrincipal(%q) failed: %v", tt.input, err)
			}
			if pn.FullName(realm) != tt.expected {
				t.Errorf("FullName() = %q, want %q", pn.FullName(realm), tt.expected)
			}
		})
	}
}

func TestMakePrincipalName(t *testing.T) {
	pn := MakeUserPrincipal("testuser", "EXAMPLE.COM")
	if pn.NameType != NAME_TYPE_PRINCIPAL {
		t.Errorf("NameType = %d, want %d", pn.NameType, NAME_TYPE_PRINCIPAL)
	}
	if len(pn.NameString) != 1 {
		t.Errorf("NameString len = %d, want 1", len(pn.NameString))
	}
	if pn.NameString[0] != "testuser" {
		t.Errorf("NameString = %v", pn.NameString)
	}
}

func TestMakeSPNPrincipal(t *testing.T) {
	pn := MakeSPNPrincipal("HTTP", "web.example.com", "EXAMPLE.COM")
	if pn.NameType != NAME_TYPE_SRV_INST {
		t.Errorf("NameType = %d, want %d", pn.NameType, NAME_TYPE_SRV_INST)
	}
	if len(pn.NameString) != 2 {
		t.Errorf("NameString len = %d, want 2", len(pn.NameString))
	}
	if pn.NameString[0] != "HTTP" || pn.NameString[1] != "web.example.com" {
		t.Errorf("NameString = %v", pn.NameString)
	}
}

func TestEtypeSupport(t *testing.T) {
	supported := SupportedEtypes()
	if len(supported) == 0 {
		t.Fatal("SupportedEtypes() returned empty list")
	}

	found := false
	for _, e := range supported {
		if e == ETYPE_AES256_CTS_HMAC_SHA1_96 {
			found = true
			break
		}
	}
	if !found {
		t.Error("AES256 not in supported etypes")
	}
}

func TestPreferredEtype(t *testing.T) {
	offered := []int32{ETYPE_RC4_HMAC, ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_AES256_CTS_HMAC_SHA1_96}
	preferred := PreferredEtype(offered)
	if preferred != ETYPE_AES256_CTS_HMAC_SHA1_96 {
		t.Errorf("PreferredEtype = %d, want %d", preferred, ETYPE_AES256_CTS_HMAC_SHA1_96)
	}
}

func TestKeySize(t *testing.T) {
	tests := []struct {
		etype int32
		size  int
	}{
		{ETYPE_AES128_CTS_HMAC_SHA1_96, 16},
		{ETYPE_AES256_CTS_HMAC_SHA1_96, 32},
		{ETYPE_RC4_HMAC, 16},
	}

	for _, tt := range tests {
		size, err := KeySize(tt.etype)
		if err != nil {
			t.Errorf("KeySize(%d) error: %v", tt.etype, err)
			continue
		}
		if size != tt.size {
			t.Errorf("KeySize(%d) = %d, want %d", tt.etype, size, tt.size)
		}
	}
}

func TestEtypeName(t *testing.T) {
	tests := []struct {
		etype int32
		name  string
	}{
		{ETYPE_AES128_CTS_HMAC_SHA1_96, "aes128-cts-hmac-sha1-96"},
		{ETYPE_AES256_CTS_HMAC_SHA1_96, "aes256-cts-hmac-sha1-96"},
		{ETYPE_RC4_HMAC, "rc4-hmac"},
	}

	for _, tt := range tests {
		name := EtypeName(tt.etype)
		if name != tt.name {
			t.Errorf("EtypeName(%d) = %q, want %q", tt.etype, name, tt.name)
		}
	}
}

func TestKerberosTime(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	kt := NewKerberosTime(now)

	data, err := kt.MarshalASN1()
	if err != nil {
		t.Fatalf("MarshalASN1 failed: %v", err)
	}

	var kt2 KerberosTime
	err = kt2.UnmarshalASN1(data)
	if err != nil {
		t.Fatalf("UnmarshalASN1 failed: %v", err)
	}

	if !kt2.Time.Equal(kt.Time) {
		t.Errorf("Time mismatch: got %v, want %v", kt2.Time, kt.Time)
	}
}

func TestStringToKeyRC4(t *testing.T) {
	key, err := StringToKey(ETYPE_RC4_HMAC, "password123", "EXAMPLE.COMuser", "")
	if err != nil {
		t.Fatalf("StringToKey failed: %v", err)
	}
	// RFC 4757: RC4-HMAC keys are the 16-byte MD4 of UTF-16LE(password).
	if len(key) != 16 {
		t.Errorf("RC4 key length = %d, want 16", len(key))
	}
}

func TestStringToKeyAES128(t *testing.T) {
	key, err := StringToKey(ETYPE_AES128_CTS_HMAC_SHA1_96, "password123", "EXAMPLE.COMuser", "")
	if err != nil {
		t.Fatalf("StringToKey failed: %v", err)
	}
	if len(key) != 16 {
		t.Errorf("AES128 key length = %d, want 16", len(key))
	}
}

func TestStringToKeyAES256(t *testing.T) {
	key, err := StringToKey(ETYPE_AES256_CTS_HMAC_SHA1_96, "password123", "EXAMPLE.COMuser", "")
	if err != nil {
		t.Fatalf("StringToKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("AES256 key length = %d, want 32", len(key))
	}
}

func TestStringToKeyUnsupported(t *testing.T) {
	_, err := StringToKey(ETYPE_DES_CBC_CRC, "password", "salt", "")
	if err != ErrUnsupportedEncryption {
		t.Errorf("Expected ErrUnsupportedEncryption, got %v", err)
	}
}

func TestEncryptDecryptRC4(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := []byte("test plaintext for encryption")

	ciphertext, err := Encrypt(ETYPE_RC4_HMAC, key, KeyUsage_AS_REQ_PA_ENC_TIMESTAMP, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(ETYPE_RC4_HMAC, key, KeyUsage_AS_REQ_PA_ENC_TIMESTAMP, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted != plaintext: %q vs %q", string(decrypted), string(plaintext))
	}
}

func TestEncryptDecryptAES128(t *testing.T) {
	key, _ := StringToKey(ETYPE_AES128_CTS_HMAC_SHA1_96, "password123", "salt", "")
	plaintext := []byte("test plaintext for AES encryption")

	ciphertext, err := Encrypt(ETYPE_AES128_CTS_HMAC_SHA1_96, key, KeyUsage_AS_REP_ENCPART, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(ETYPE_AES128_CTS_HMAC_SHA1_96, key, KeyUsage_AS_REP_ENCPART, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted != plaintext: %q vs %q", string(decrypted), string(plaintext))
	}
}

func TestEncryptDecryptAES256(t *testing.T) {
	key, _ := StringToKey(ETYPE_AES256_CTS_HMAC_SHA1_96, "password123", "salt", "")
	plaintext := []byte("test plaintext for AES256 encryption")

	ciphertext, err := Encrypt(ETYPE_AES256_CTS_HMAC_SHA1_96, key, KeyUsage_AS_REP_ENCPART, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(ETYPE_AES256_CTS_HMAC_SHA1_96, key, KeyUsage_AS_REP_ENCPART, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted != plaintext: %q vs %q", string(decrypted), string(plaintext))
	}
}

func TestTamperDetection(t *testing.T) {
	key, _ := StringToKey(ETYPE_AES128_CTS_HMAC_SHA1_96, "password123", "salt", "")
	plaintext := []byte("test plaintext")

	ciphertext, err := Encrypt(ETYPE_AES128_CTS_HMAC_SHA1_96, key, KeyUsage_AS_REP_ENCPART, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ciphertext[0] ^= 0xFF

	_, err = Decrypt(ETYPE_AES128_CTS_HMAC_SHA1_96, key, KeyUsage_AS_REP_ENCPART, ciphertext)
	if err != ErrIntegrityCheck {
		t.Errorf("Expected ErrIntegrityCheck on tampered ciphertext, got %v", err)
	}
}

func TestWrongKeyDetection(t *testing.T) {
	key1, _ := StringToKey(ETYPE_AES128_CTS_HMAC_SHA1_96, "password123", "salt", "")
	key2, _ := StringToKey(ETYPE_AES128_CTS_HMAC_SHA1_96, "wrongpass", "salt", "")
	plaintext := []byte("test plaintext")

	ciphertext, err := Encrypt(ETYPE_AES128_CTS_HMAC_SHA1_96, key1, KeyUsage_AS_REP_ENCPART, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(ETYPE_AES128_CTS_HMAC_SHA1_96, key2, KeyUsage_AS_REP_ENCPART, ciphertext)
	if err != ErrIntegrityCheck {
		t.Errorf("Expected ErrIntegrityCheck with wrong key, got %v", err)
	}
}

func TestSPNParsing(t *testing.T) {
	tests := []struct {
		input    string
		service  string
		host     string
		realm    string
	}{
		{"HTTP/web.example.com@EXAMPLE.COM", "HTTP", "web.example.com", "EXAMPLE.COM"},
		{"host/server.example.com@EXAMPLE.COM", "host", "server.example.com", "EXAMPLE.COM"},
		{"MSSQLSvc/sql.example.com:1433@EXAMPLE.COM", "MSSQLSvc", "sql.example.com", "EXAMPLE.COM"},
	}

	for _, tt := range tests {
		spn, err := ParseSPN(tt.input)
		if err != nil {
			t.Errorf("ParseSPN(%q) failed: %v", tt.input, err)
			continue
		}
		if spn.ServiceClass != tt.service {
			t.Errorf("ServiceClass = %q, want %q", spn.ServiceClass, tt.service)
		}
		if spn.Host != tt.host {
			t.Errorf("Host = %q, want %q", spn.Host, tt.host)
		}
		if string(spn.Realm) != tt.realm {
			t.Errorf("Realm = %q, want %q", string(spn.Realm), tt.realm)
		}
	}
}

func TestSPNCanonicalize(t *testing.T) {
	canonical, err := CanonicalizeSPN("HTTP/Web.Example.Com@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("CanonicalizeSPN failed: %v", err)
	}
	if canonical != "HTTP/Web.Example.Com@EXAMPLE.COM" {
		t.Errorf("Canonical = %q", canonical)
	}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		code int32
		err  error
	}{
		{KDC_ERR_PREAUTH_REQUIRED, ErrPreauthRequired},
		{KDC_ERR_C_PRINCIPAL_UNKNOWN, ErrUnknownUser},
		{KDC_ERR_PREAUTH_FAILED, ErrWrongPassword},
		{KDC_ERR_ETYPE_NOSUPP, ErrUnsupportedEncryption},
		{KRB_AP_ERR_SKEW, ErrClockSkew},
		{KRB_AP_ERR_TKT_EXPIRED, ErrTicketExpired},
		{KRB_AP_ERR_TKT_NYV, ErrFutureTicket},
	}

	for _, tt := range tests {
		classified := ClassifyError(tt.code)
		if classified != tt.err {
			t.Errorf("ClassifyError(%d) = %v, want %v", tt.code, classified, tt.err)
		}
	}
}