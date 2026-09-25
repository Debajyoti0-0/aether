package kerberos

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"errors"

	"golang.org/x/crypto/md4"
	"golang.org/x/crypto/pbkdf2"
)

const (
	KeyUsage_AS_REQ_PA_ENC_TIMESTAMP     = 1
	KeyUsage_TGS_REQ_PA_TGS_REQ          = 2 // TGT enc-part (server key)
	KeyUsage_AS_REP_ENCPART              = 3
	KeyUsage_TGS_REQ_AD_SESSKEY          = 4
	KeyUsage_TGS_REQ_AUTH_CKSUM          = 6 // TGS-REQ req-body checksum in authenticator
	KeyUsage_AP_REQ_AUTHENTICATOR        = 7 // authenticator encrypted with TGS session key
	KeyUsage_TGS_REP_ENCPART             = 8
	KeyUsage_AP_REQ_AUTHENTICATOR_SUBKEY = 9
	KeyUsage_AP_REP_ENCPART              = 12
	KeyUsage_KRB_PRIV_ENCPART            = 13
	KeyUsage_KRB_SAFE_CKSUM              = 14
	KeyUsage_KRB_CRED_ENCPART            = 14
)

var (
	ErrKeySize            = errors.New("invalid key size")
	ErrCiphertextTooShort = errors.New("ciphertext too short")
	ErrIntegrityCheck     = errors.New("integrity check failed")
)

// ---------------------------------------------------------------------------
// RFC 3961 §5.1 key derivation: DK(Key, Constant) = k-truncate(DR(Key, Constant))
// DR(Key, Constant) = k-truncate(K1 | K2 | K3 | ...) where
//   K1 = E(Key, n-fold(Constant), zero-IV)
//   Ki+1 = E(Key, Ki, zero-IV)
// E is AES with a zero IV, so each step is a single-block ECB encryption of
// the previous block. The previous implementation used HMAC feedback, which
// is NOT the RFC construction and produced keys no real KDC could derive.
// ---------------------------------------------------------------------------

// nFold implements RFC 3961 §A n-folding: replicate the input to the least
// common multiple of the input length (bits) and the output length (bits),
// rotating the input right by 13 bit positions before each repetition, then
// sum the successive n-bit chunks with 1's-complement (end-around carry)
// addition. Bit 0 is the most significant bit of the first octet.
// Verified against MIT krb5 t_nfold.c vectors (RFC 3961 §A).
func nFold(outBits int, in []byte) []byte {
	inBits := len(in) * 8
	bits := make([]byte, inBits)
	for i, b := range in {
		for j := 0; j < 8; j++ {
			bits[i*8+j] = (b >> uint(7-j)) & 1
		}
	}
	lcmBits := lcm(inBits, outBits)
	copies := lcmBits / inBits
	// Replicated stream: rotate the full input 13 bits right before each copy.
	stream := make([]byte, 0, lcmBits)
	rot := make([]byte, inBits)
	copy(rot, bits)
	for j := 0; j < copies; j++ {
		stream = append(stream, rot...)
		rot = rotateRight13(rot)
	}
	// Sum the lcmBits/outBits chunks of outBits each, end-around carry.
	chunks := lcmBits / outBits
	acc := make([]byte, outBits)
	for k := 0; k < chunks; k++ {
		carry := byte(0)
		for i := outBits - 1; i >= 0; i-- {
			s := acc[i] + stream[k*outBits+i] + carry
			acc[i] = s & 1
			carry = s >> 1
		}
		if carry != 0 {
			for i := outBits - 1; i >= 0 && carry != 0; i-- {
				s := acc[i] + carry
				acc[i] = s & 1
				carry = s >> 1
			}
		}
	}
	out := make([]byte, outBits/8)
	for i := 0; i < outBits; i++ {
		if acc[i] == 1 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}
	return out
}

func lcm(a, b int) int {
	x, y := a, b
	for y != 0 {
		x, y = y, x%y
	}
	return a / x * b
}

// rotateRight13 rotates a bit slice right by 13 bit positions (RFC 3961 §A).
func rotateRight13(bits []byte) []byte {
	l := len(bits)
	out := make([]byte, l)
	for i := 0; i < l; i++ {
		out[(i+13)%l] = bits[i]
	}
	return out
}

// deriveRandom implements RFC 3961 §5.1 DR(Key, Constant) using AES.
func deriveRandom(key []byte, constant []byte, outLen int) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	cbs := block.BlockSize()
	folded := constant
	if len(folded) != cbs {
		folded = nFold(cbs*8, constant)
	}
	b := make([]byte, cbs)
	block.Encrypt(b, folded)
	out := make([]byte, 0, outLen+cbs)
	out = append(out, b...)
	for len(out) < outLen {
		nb := make([]byte, cbs)
		block.Encrypt(nb, b)
		out = append(out, nb...)
		b = nb
	}
	return out[:outLen]
}

// dk implements DK(Key, Constant) = k-truncate(DR(Key, Constant)).
func dk(key []byte, constant []byte, outLen int) []byte {
	return deriveRandom(key, constant, outLen)
}

// usageConstant builds the RFC 3961 §5.3 well-known constant:
// key usage as 4 big-endian octets followed by one tag octet
// (0xAA Ke, 0x55 Ki, 0x99 Kc).
func usageConstant(usage int32, tag byte) []byte {
	c := make([]byte, 5)
	binary.BigEndian.PutUint32(c, uint32(usage))
	c[4] = tag
	return c
}

// deriveKeyKe derives the encryption key Ke = DK(base-key, usage | 0xAA).
func deriveKeyKe(key []byte, usage int32, etype int32) ([]byte, error) {
	return dk(key, usageConstant(usage, 0xAA), keyLen(etype)), nil
}

// deriveKeyKi derives the integrity key Ki = DK(base-key, usage | 0x55).
func deriveKeyKi(key []byte, usage int32, etype int32) ([]byte, error) {
	return dk(key, usageConstant(usage, 0x55), keyLen(etype)), nil
}

// deriveKeyKc derives the checksum key Kc = DK(base-key, usage | 0x99).
func deriveKeyKc(key []byte, usage int32, etype int32) ([]byte, error) {
	return dk(key, usageConstant(usage, 0x99), keyLen(etype)), nil
}

// DeriveKey returns the per-usage cipher key: Ke for AES etypes, K1 for RC4.
func DeriveKey(key []byte, usage int32, etype int32) ([]byte, error) {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_AES256_CTS_HMAC_SHA1_96:
		return deriveKeyKe(key, usage, etype)
	case ETYPE_RC4_HMAC:
		k1 := hmac.New(md5.New, key)
		binary.Write(k1, binary.LittleEndian, usage)
		return k1.Sum(nil), nil
	default:
		return nil, ErrUnsupportedEncryption
	}
}

func keyLen(etype int32) int {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96:
		return 16
	case ETYPE_AES256_CTS_HMAC_SHA1_96:
		return 32
	case ETYPE_RC4_HMAC:
		return 16
	default:
		return 16
	}
}

func blockSize(etype int32) int {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_AES256_CTS_HMAC_SHA1_96:
		return aes.BlockSize
	case ETYPE_RC4_HMAC:
		return 1
	default:
		return aes.BlockSize
	}
}

// ---------------------------------------------------------------------------
// String-to-key
// ---------------------------------------------------------------------------

// StringToKey derives the protocol key for an etype.
//
// AES (RFC 3962 §4): tkey = PBKDF2(passphrase, salt, iter, keylen);
// key = DK(tkey, "kerberos"). The default iteration count is 4096
// (params 00 00 10 00); a 4-octet big-endian params string overrides it.
//
// RC4-HMAC (RFC 4757 §3): K = MD4(UTF-16LE(password)).
func StringToKey(etype int32, password, salt string, params string) ([]byte, error) {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96:
		return aesStringToKey(password, salt, params, 16)
	case ETYPE_AES256_CTS_HMAC_SHA1_96:
		return aesStringToKey(password, salt, params, 32)
	case ETYPE_RC4_HMAC:
		return rc4Key(password), nil
	default:
		return nil, ErrUnsupportedEncryption
	}
}

func aesStringToKey(password, salt, params string, keyLen int) ([]byte, error) {
	iter := 4096
	if len(params) >= 4 {
		iter = int(binary.BigEndian.Uint32([]byte(params[:4])))
		if iter == 0 {
			// RFC 3962 §4: 00 00 00 00 means 2**32; clamp to keep this usable.
			iter = 4096
		}
	}
	tkey := pbkdf2.Key([]byte(password), []byte(salt), iter, keyLen, sha1.New)
	return dk(tkey, []byte("kerberos"), keyLen), nil
}

// rc4Key derives the RC4-HMAC (etype 23) key from a password per RFC 4757 §3:
// K = MD4(UTF-16LE(password)). Samba and AD both use this.
func rc4Key(password string) []byte {
	utf16le := make([]byte, 0, len(password)*2)
	for _, r := range password {
		if r > 0xFFFF {
			r = 0xFFFD
		}
		utf16le = append(utf16le, byte(r), byte(r>>8))
	}
	h := md4.New()
	h.Write(utf16le)
	return h.Sum(nil)
}

// ---------------------------------------------------------------------------
// AES encyption (RFC 3962 §5/§6 simplified profile)
// ---------------------------------------------------------------------------

const aesCksumSize = 12 // RFC 3962 §6: HMAC output size h = 12 octets

// encryptAESCTS implements the RFC 3961 §5.3 simplified profile with the
// RFC 3962 AES parameters: confounder(c=16 random) || plaintext, encrypted
// with Ke under CBC-CTS; HMAC-SHA1(Ki, conf||plaintext)[1..12] appended.
func encryptAESCTS(key []byte, usage int32, plaintext []byte) ([]byte, error) {
	ki, err := deriveKeyKi(key, usage, etypeFor(key))
	if err != nil {
		return nil, err
	}
	ke, err := deriveKeyKe(key, usage, etypeFor(key))
	if err != nil {
		return nil, err
	}

	conf := make([]byte, aes.BlockSize)
	if _, err := rand.Read(conf); err != nil {
		return nil, err
	}
	m := make([]byte, 0, aes.BlockSize+len(plaintext))
	m = append(m, conf...)
	m = append(m, plaintext...)

	ct := ctsEncrypt(ke, m)
	h := hmac.New(sha1.New, ki)
	h.Write(m)
	tag := h.Sum(nil)[:aesCksumSize]

	out := make([]byte, 0, len(ct)+len(tag))
	out = append(out, ct...)
	out = append(out, tag...)
	return out, nil
}

func decryptAESCTS(key []byte, usage int32, ciphertext []byte) ([]byte, error) {
	ki, err := deriveKeyKi(key, usage, etypeFor(key))
	if err != nil {
		return nil, err
	}
	ke, err := deriveKeyKe(key, usage, etypeFor(key))
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aesCksumSize+aes.BlockSize {
		return nil, ErrCiphertextTooShort
	}
	ct := ciphertext[:len(ciphertext)-aesCksumSize]
	tag := ciphertext[len(ciphertext)-aesCksumSize:]

	m, err := ctsDecrypt(ke, ct)
	if err != nil {
		return nil, err
	}
	h := hmac.New(sha1.New, ki)
	h.Write(m)
	if !hmac.Equal(tag, h.Sum(nil)[:aesCksumSize]) {
		return nil, ErrIntegrityCheck
	}
	return m[aes.BlockSize:], nil
}

func etypeFor(key []byte) int32 {
	if len(key) == 16 {
		return ETYPE_AES128_CTS_HMAC_SHA1_96
	}
	return ETYPE_AES256_CTS_HMAC_SHA1_96
}

// ctsEncrypt performs RFC 2040 §8 ciphertext stealing as specified for
// Kerberos by RFC 3962 §5 (with the §A errata). Verified against the RFC
// 3962 §B vectors: for r>0 trailing bytes the output is
// C1..Cq-1 || Cp || Cq[0:r] where Cq is the CBC encryption of the last full
// block (zero IV) and Cp = E((Pp||zeros) XOR Cq); for exact multiples the
// output is plain CBC with the last two ciphertext blocks swapped
// (pinned by the 32-byte vector: output C2 C1).
func ctsEncrypt(key, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	bs := block.BlockSize()
	n := len(plaintext)
	if n < bs {
		return nil
	}
	q := n / bs
	r := n % bs

	// CBC-encrypt all q full blocks with a zero IV.
	out := make([]byte, n)
	iv := make([]byte, bs)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out[:q*bs], plaintext[:q*bs])

	if r == 0 {
		if q >= 2 {
			a := out[(q-2)*bs : (q-1)*bs]
			b := out[(q-1)*bs : q*bs]
			for i := 0; i < bs; i++ {
				a[i], b[i] = b[i], a[i]
			}
		}
		return out
	}

	// Cp = E((Pp || zeros) XOR Cq)
	cq := out[(q-1)*bs : q*bs]
	x := make([]byte, bs)
	copy(x[:r], plaintext[q*bs:])
	xorInto(x, cq)
	cp := make([]byte, bs)
	block.Encrypt(cp, x)

	final := make([]byte, n)
	copy(final, out[:(q-1)*bs]) // C1..Cq-1
	copy(final[(q-1)*bs:], cp)  // Cp
	copy(final[q*bs:], cq[:r])  // Cq[0:r] (stolen)
	return final
}

// ctsDecrypt inverts ctsEncrypt.
func ctsDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	n := len(ciphertext)
	if n < bs {
		return nil, ErrCiphertextTooShort
	}
	q := n / bs
	r := n % bs

	if r == 0 {
		// Undo the encrypt-side swap BEFORE CBC-decrypting (P2 = D(C2) XOR C1
		// requires the canonical block order).
		buf := make([]byte, n)
		copy(buf, ciphertext)
		if q >= 2 {
			a := buf[(q-2)*bs : (q-1)*bs]
			b := buf[(q-1)*bs : q*bs]
			for i := 0; i < bs; i++ {
				a[i], b[i] = b[i], a[i]
			}
		}
		out := make([]byte, n)
		cipher.NewCBCDecrypter(block, make([]byte, bs)).CryptBlocks(out, buf)
		return out, nil
	}

	// Wire layout: C1..Cq-1 || Cp || Cq[0:r]. Reconstruct Cq:
	//   Xp = D(Cp); Cq[r:] = Xp[r:] (Xp = (Pp||0) XOR Cq)
	//   Cq[:r] = the r trailing ciphertext bytes.
	cp := ciphertext[(q-1)*bs : q*bs]
	xp := make([]byte, bs)
	block.Decrypt(xp, cp)
	cq := make([]byte, bs)
	copy(cq[:r], ciphertext[q*bs:])
	copy(cq[r:], xp[r:])

	// Pp = Xp[:r] XOR Cq[:r]; Pq = D(Cq) XOR Cq-1.
	pq := make([]byte, bs)
	block.Decrypt(pq, cq)
	var cprev []byte
	if q >= 2 {
		cprev = ciphertext[(q-2)*bs : (q-1)*bs]
	} else {
		cprev = make([]byte, bs) // IV (zeros) — RFC 3962 §A errata
	}
	xorInto(pq, cprev)
	pp := make([]byte, r)
	copy(pp, xp[:r])
	xorInto(pp, cq[:r])

	out := make([]byte, n)
	cipher.NewCBCDecrypter(block, make([]byte, bs)).CryptBlocks(out[:(q-1)*bs], ciphertext[:(q-1)*bs])
	copy(out[(q-1)*bs:], pq)
	copy(out[q*bs:], pp)
	return out, nil
}

func xorInto(dst, src []byte) {
	for i := range src {
		dst[i] ^= src[i]
	}
}

// ---------------------------------------------------------------------------
// RC4-HMAC (RFC 4757)
// ---------------------------------------------------------------------------

func rc4KeySet(key []byte, usage int32) (k2, k3 []byte) {
	// K1 = HMAC-MD5(key, LE32(usage))
	k1 := hmac.New(md5.New, key)
	binary.Write(k1, binary.LittleEndian, usage)
	k1b := k1.Sum(nil)
	// K2 = HMAC-MD5(K1, "signaturekey\x00") — checksum key
	h2 := hmac.New(md5.New, k1b)
	h2.Write([]byte("signaturekey\x00"))
	k2 = h2.Sum(nil)
	// K3 = HMAC-MD5(K1, 8 zero octets) — cipher key
	h3 := hmac.New(md5.New, k1b)
	h3.Write(make([]byte, 8))
	k3 = h3.Sum(nil)
	return k2, k3
}

func encryptRC4HMAC(key []byte, usage int32, plaintext []byte) ([]byte, error) {
	k2, k3 := rc4KeySet(key, usage)

	cksum := hmac.New(md5.New, k2)
	cksum.Write(plaintext)
	tag := cksum.Sum(nil)

	c, err := rc4.NewCipher(k3)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, len(plaintext))
	c.XORKeyStream(ciphertext, plaintext)

	out := make([]byte, 0, len(plaintext)+16)
	out = append(out, tag...)        // 16-byte HMAC-MD5 first
	out = append(out, ciphertext...) // then RC4(K3) data
	return out, nil
}

func decryptRC4HMAC(key []byte, usage int32, ciphertext []byte) ([]byte, error) {
	// RFC 4757 wire layout: [16-byte HMAC-MD5 checksum][RC4(K3) data]
	if len(ciphertext) < 16 {
		return nil, ErrCiphertextTooShort
	}
	k2, k3 := rc4KeySet(key, usage)

	tag := ciphertext[:16]
	ct := ciphertext[16:]

	c, err := rc4.NewCipher(k3)
	if err != nil {
		return nil, err
	}
	plaintext := make([]byte, len(ct))
	c.XORKeyStream(plaintext, ct)

	expected := hmac.New(md5.New, k2)
	expected.Write(plaintext)
	if !hmac.Equal(tag, expected.Sum(nil)) {
		return nil, ErrIntegrityCheck
	}
	return plaintext, nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

func Encrypt(etype int32, key []byte, usage int32, plaintext []byte) ([]byte, error) {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_AES256_CTS_HMAC_SHA1_96:
		return encryptAESCTS(key, usage, plaintext)
	case ETYPE_RC4_HMAC:
		return encryptRC4HMAC(key, usage, plaintext)
	default:
		return nil, ErrUnsupportedEncryption
	}
}

func Decrypt(etype int32, key []byte, usage int32, ciphertext []byte) ([]byte, error) {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_AES256_CTS_HMAC_SHA1_96:
		return decryptAESCTS(key, usage, ciphertext)
	case ETYPE_RC4_HMAC:
		return decryptRC4HMAC(key, usage, ciphertext)
	default:
		return nil, ErrUnsupportedEncryption
	}
}

func GenerateSubkey(etype int32, key []byte) ([]byte, error) {
	return DeriveKey(key, KeyUsage_AS_REP_ENCPART, etype)
}

func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

// ComputeChecksum implements the RFC 3961 §5.4 profile: AES uses
// HMAC(Kc, message)[1..12] with Kc = DK(key, usage | 0x99); RFC 4757 RC4
// uses S = HMAC-MD5(K2, data).
func ComputeChecksum(etype int32, key []byte, usage int32, data []byte) ([]byte, error) {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_AES256_CTS_HMAC_SHA1_96:
		kc, err := deriveKeyKc(key, usage, etype)
		if err != nil {
			return nil, err
		}
		h := hmac.New(sha1.New, kc)
		h.Write(data)
		return h.Sum(nil)[:12], nil
	case ETYPE_RC4_HMAC:
		k2, _ := rc4KeySet(key, usage)
		h := hmac.New(md5.New, k2)
		h.Write(data)
		return h.Sum(nil), nil
	default:
		return nil, ErrUnsupportedEncryption
	}
}

func VerifyChecksum(etype int32, key []byte, usage int32, data, checksum []byte) error {
	expected, err := ComputeChecksum(etype, key, usage, data)
	if err != nil {
		return err
	}
	if !hmac.Equal(checksum, expected) {
		return ErrIntegrityCheck
	}
	return nil
}
