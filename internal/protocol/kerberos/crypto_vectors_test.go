package kerberos

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// Vectors transcribed from RFC 3962 §B (string-to-key) and §B (CBC-CTS).
// These pin PBKDF2 → DK("kerberos") → n-fold and the CTS mode to the RFC
// values. The previous implementation used a non-RFC HMAC-based DK and could
// not interoperate with Samba/AD (Stage 46g live finding).

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func TestRFC3962_StringToKey_Iter1_AES128(t *testing.T) {
	key, err := StringToKey(ETYPE_AES128_CTS_HMAC_SHA1_96, "password", "ATHENA.MIT.EDUraeburn", "\x00\x00\x00\x01")
	if err != nil {
		t.Fatalf("StringToKey: %v", err)
	}
	want := mustHex(t, "42263c6e89f4fc28b8df68ee09799f15")
	if !bytes.Equal(key, want) {
		t.Fatalf("key = %x, want %x", key, want)
	}
}

func TestRFC3962_StringToKey_Iter1_AES256(t *testing.T) {
	key, err := StringToKey(ETYPE_AES256_CTS_HMAC_SHA1_96, "password", "ATHENA.MIT.EDUraeburn", "\x00\x00\x00\x01")
	if err != nil {
		t.Fatalf("StringToKey: %v", err)
	}
	want := mustHex(t, "fe697b52bc0d3ce14432ba036a92e65bbb52280990a2fa27883998d72af30161")
	if !bytes.Equal(key, want) {
		t.Fatalf("key = %x, want %x", key, want)
	}
}

func TestRFC3962_StringToKey_Iter2_AES256(t *testing.T) {
	key, err := StringToKey(ETYPE_AES256_CTS_HMAC_SHA1_96, "password", "ATHENA.MIT.EDUraeburn", "\x00\x00\x00\x02")
	if err != nil {
		t.Fatalf("StringToKey: %v", err)
	}
	want := mustHex(t, "a2e16d16b36069c135d5e9d2e25f896102685618b95914b467c67622225824ff")
	if !bytes.Equal(key, want) {
		t.Fatalf("key = %x, want %x", key, want)
	}
}

func TestRFC3962_StringToKey_BlockSizePassphrase(t *testing.T) {
	// iteration count 1200 (0x04b0), 64-octet passphrase (block-size boundary)
	key, err := StringToKey(ETYPE_AES256_CTS_HMAC_SHA1_96,
		"XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
		"pass phrase equals block size", "\x00\x00\x04\xb0")
	if err != nil {
		t.Fatalf("StringToKey: %v", err)
	}
	want := mustHex(t, "89adee3608db8bc71f1bfbfe459486b05618b70cbae22092534e56c553ba4b34")
	if !bytes.Equal(key, want) {
		t.Fatalf("key = %x, want %x", key, want)
	}
}

// MIT krb5 t_nfold.c vectors (RFC 3961 §A).
func TestRFC3961_NFold_MITVectors(t *testing.T) {
	cases := []struct {
		in   string
		bits int
		want string
	}{
		{"012345", 64, "be072631276b1955"},
		{"password", 56, "78a07b6caf85fa"},
		{"Rough Consensus, and Running Code", 64, "bb6ed30870b7f0e0"},
		{"password", 168, "59e4a8ca7c0385c3c37b3f6d2000247cb6e6bd5b3e"},
		{"MASSACHVSETTS INSTITVTE OF TECHNOLOGY", 192,
			"db3b0d8f0b061e603282b308a50841229ad798fab9540c1b"},
	}
	for _, c := range cases {
		got := nFold(c.bits, []byte(c.in))
		if !bytes.Equal(got, mustHex(t, c.want)) {
			t.Fatalf("nFold(%d, %q) = %x, want %s", c.bits, c.in, got, c.want)
		}
	}
	// Note: t_nfold.c only *prints* fold_kerberos(8/16/21/32) without an
	// expected value, so there is no assertable golden for it here. The
	// DK("kerberos") chain (n-fold 9 octets → 128 bits) is already proven
	// by the passing RFC 3962 string-to-key vectors above.
}

func TestCTS_RoundTrip_AllLengths(t *testing.T) {
	key, err := StringToKey(ETYPE_AES256_CTS_HMAC_SHA1_96, "password", "ATHENA.MIT.EDUraeburn", "\x00\x00\x00\x01")
	if err != nil {
		t.Fatalf("StringToKey: %v", err)
	}
	for n := 1; n <= 80; n++ {
		pt := bytes.Repeat([]byte{byte('A' + n%26)}, n)
		ct, err := Encrypt(ETYPE_AES256_CTS_HMAC_SHA1_96, key, 3, pt)
		if err != nil {
			t.Fatalf("n=%d encrypt: %v", n, err)
		}
		if len(ct) != n+aesCksumSize+16 {
			t.Fatalf("n=%d ciphertext length = %d, want %d", n, len(ct), n+aesCksumSize+16)
		}
		got, err := Decrypt(ETYPE_AES256_CTS_HMAC_SHA1_96, key, 3, ct)
		if err != nil {
			t.Fatalf("n=%d decrypt: %v", n, err)
		}
		if !bytes.Equal(pt, got) {
			t.Fatalf("n=%d round-trip mismatch", n)
		}
	}
}

// RFC 3962 §B CBC-CTS vectors (key = "chicken teriyaki", IV = 0).
func TestRFC3962_CTS_17ByteVector(t *testing.T) {
	key := mustHex(t, "636869636b656e207465726979616b69")
	input := mustHex(t, "4920776f756c64206c696b652074686520")
	out := ctsEncrypt(key, input)
	want := mustHex(t, "c6353568f2bf8cb4d8a580362da7ff7f97")
	if !bytes.Equal(out, want) {
		t.Fatalf("cts = %x, want %x", out, want)
	}
}

func TestRFC3962_CTS_31ByteVector(t *testing.T) {
	key := mustHex(t, "636869636b656e207465726979616b69")
	input := mustHex(t, "4920776f756c64206c696b65207468652047656e6572616c20476175277320")
	out := ctsEncrypt(key, input)
	want := mustHex(t, "fc00783e0efdb2c1d445d4c8eff7ed2297687268d6ecccc0c07b25e25ecfe5")
	if !bytes.Equal(out, want) {
		t.Fatalf("cts = %x, want %x", out, want)
	}
}

func TestRFC3962_CTS_32ByteVector(t *testing.T) {
	key := mustHex(t, "636869636b656e207465726979616b69")
	input := mustHex(t, "4920776f756c64206c696b65207468652047656e6572616c2047617527732043")
	out := ctsEncrypt(key, input)
	want := mustHex(t, "39312523a78662d5be7fcbcc98ebf5a897687268d6ecccc0c07b25e25ecfe584")
	if !bytes.Equal(out, want) {
		t.Fatalf("cts = %x, want %x", out, want)
	}
}

func TestRFC3962_CTS_47ByteVector(t *testing.T) {
	key := mustHex(t, "636869636b656e207465726979616b69")
	input := mustHex(t, "4920776f756c64206c696b65207468652047656e6572616c20476175277320436869636b656e2c20706c656173652c")
	out := ctsEncrypt(key, input)
	want := mustHex(t, "97687268d6ecccc0c07b25e25ecfe584b3fffd940c16a18c1b5549d2f838029e39312523a78662d5be7fcbcc98ebf5")
	if !bytes.Equal(out, want) {
		t.Fatalf("cts = %x, want %x", out, want)
	}
}

func TestRFC3962_CTS_64ByteVector(t *testing.T) {
	key := mustHex(t, "636869636b656e207465726979616b69")
	input := mustHex(t, "4920776f756c64206c696b65207468652047656e6572616c20476175277320436869636b656e2c20706c656173652c20616e6420776f6e746f6e20736f75702e")
	out := ctsEncrypt(key, input)
	want := mustHex(t, "97687268d6ecccc0c07b25e25ecfe58439312523a78662d5be7fcbcc98ebf5a84807efe836ee89a526730dbc2f7bc8409dad8bbb96c4cdc03bc103e1a194bbd8")
	if !bytes.Equal(out, want) {
		t.Fatalf("cts = %x, want %x", out, want)
	}
}

func TestRFC3962_CTS_DecryptVectors(t *testing.T) {
	key := mustHex(t, "636869636b656e207465726979616b69")
	vectors := []struct{ in, want string }{
		{"c6353568f2bf8cb4d8a580362da7ff7f97", "4920776f756c64206c696b652074686520"},
		{"fc00783e0efdb2c1d445d4c8eff7ed2297687268d6ecccc0c07b25e25ecfe5",
			"4920776f756c64206c696b65207468652047656e6572616c20476175277320"},
		{"39312523a78662d5be7fcbcc98ebf5a897687268d6ecccc0c07b25e25ecfe584",
			"4920776f756c64206c696b65207468652047656e6572616c2047617527732043"},
	}
	for i, v := range vectors {
		got, err := ctsDecrypt(key, mustHex(t, v.in))
		if err != nil {
			t.Fatalf("vector %d: %v", i, err)
		}
		if !bytes.Equal(got, mustHex(t, v.want)) {
			t.Fatalf("vector %d plaintext = %x, want %s", i, got, v.want)
		}
	}
}
