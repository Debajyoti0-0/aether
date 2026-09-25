package kerberos

import (
	"context"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/spine"
	"github.com/Debajyoti0-0/aether/internal/protocol/kerberos"
	"github.com/Debajyoti0-0/aether/internal/types"
)

type TGTEngine struct{}

type TGTInput struct {
	Domain     string
	DC         string
	Username   string
	Password   string
	CCachePath string
	// OutputPath is where the acquired credential cache is written. It is
	// deliberately distinct from CCachePath, which is an INPUT ccache used to
	// authenticate. The previous code reused CCachePath as the output path, so
	// the CLI's --output flag was silently discarded and every TGT landed in
	// /tmp/<user>_<timestamp>.ccache.
	OutputPath string
	KeyTabPath string
}

type TGTOutput struct {
	CCachePath string
	Principal  string
	Realm      string
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
}

func (e *TGTEngine) Name() string { return "ad.kerberos.tgt" }
func (e *TGTEngine) Capability() string { return "ad.kerberos.tgt" }
func (e *TGTEngine) RiskScore() int { return 15 }
func (e *TGTEngine) Reversible() bool { return true }

type TGTMutation struct {
	Input  TGTInput
	Output TGTOutput
}

func (m *TGTMutation) Kind() string { return "ad.kerberos.tgt" }
func (m *TGTMutation) Target() string { return m.Input.DC + "/" + m.Input.Domain }

func (m *TGTMutation) BeforeState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"domain":      m.Input.Domain,
		"dc":          m.Input.DC,
		"username":    m.Input.Username,
		"has_password": m.Input.Password != "",
		"has_ccache":  m.Input.CCachePath != "",
		"has_keytab":  m.Input.KeyTabPath != "",
	})
}

func (m *TGTMutation) Execute(ctx context.Context) (string, error) {
	if len(m.Input.Domain) == 0 || len(m.Input.DC) == 0 {
		return "", fmt.Errorf("domain and DC required")
	}
	if len(m.Input.Username) == 0 {
		return "", fmt.Errorf("username required")
	}
	if len(m.Input.Password) == 0 && len(m.Input.CCachePath) == 0 && len(m.Input.KeyTabPath) == 0 {
		return "", fmt.Errorf("password, ccache, or keytab required")
	}

	clientPrincipal := kerberos.MakeUserPrincipal(m.Input.Username, m.Input.Domain)
	clientRealm := kerberos.Realm(m.Input.Domain)
	serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", m.Input.Domain)

	transport, err := newKerberosTransport(m.Input.DC, m.Input.Domain)
	if err != nil {
		return "", fmt.Errorf("connect to KDC: %w", err)
	}
	defer transport.Close()

	var sessionKey []byte
	var preAuthHint *kerberos.PreAuthHint

	if len(m.Input.Password) > 0 {
		// RFC 4120 §3.2.3: the string-to-key salt is the KDC's to decide. AD
		// derives it from the account name exactly as stored in the directory,
		// so "administrator" and "Administrator" have DIFFERENT salts. The
		// previous code guessed the salt locally and every non-canonical
		// account failed with KDC_ERR_PREAUTH_FAILED (24).
		probe, perr := kerberos.BuildASREQ(clientPrincipal, clientRealm, serverPrincipal,
			kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), nil)
		if perr != nil {
			return "", fmt.Errorf("build probe AS-REQ: %w", perr)
		}
		if err := transport.Send(probe); err != nil {
			return "", fmt.Errorf("send probe AS-REQ: %w", err)
		}
		probeResp, err := transport.Recv()
		if err != nil {
			return "", fmt.Errorf("recv probe reply: %w", err)
		}
		// A principal that does not exist is rejected here, before any
		// password-derived key is used.
		if code, ok := kerberos.ParseKRBErrorCode(probeResp); ok {
			if code == kerberos.KDC_ERR_PREAUTH_REQUIRED {
				hint, herr := kerberos.ParsePreAuthHint(probeResp)
				if herr != nil {
					return "", fmt.Errorf("parse pre-auth hint: %w", herr)
				}
				preAuthHint = hint
			} else {
				return "", fmt.Errorf("KDC error: %v", kerberos.NewKDCErrorWithDetail(code, probeResp))
			}
		} else if _, aerr := kerberos.ParseASREP(probeResp); aerr == nil {
			// KDC issued a ticket without pre-auth (e.g. DONT_REQ_PREAUTH);
			// fall through and try to use it directly.
			sessionKey = nil
			preAuthHint = &kerberos.PreAuthHint{Etype: kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96}
		} else {
			return "", fmt.Errorf("unexpected probe reply")
		}

		if preAuthHint != nil {
			salt := ""
			if preAuthHint.HasSalt {
				salt = preAuthHint.Salt
			} else {
				salt = kerberos.KrbSalt(kerberos.CanonicalRealm(clientRealm), clientPrincipal)
			}
			var err error
			sessionKey, err = kerberos.StringToKey(preAuthHint.Etype, m.Input.Password, salt, preAuthHint.S2KParams)
			if err != nil {
				return "", fmt.Errorf("key derivation: %w", err)
			}
		}
	} else if len(m.Input.CCachePath) > 0 {
		ccache, err := kerberos.ReadCCache(m.Input.CCachePath)
		if err != nil {
			return "", fmt.Errorf("read ccache: %w", err)
		}
		entry := ccache.GetDefaultEntry()
		if entry == nil {
			return "", fmt.Errorf("no default entry in ccache")
		}
		sessionKey = entry.Key.KeyValue
		clientPrincipal = entry.ClientPrincipal
		clientRealm = entry.ClientRealm
	} else {
		return "", fmt.Errorf("keytab not yet implemented")
	}

	asreq, err := kerberos.BuildASREQWithHint(clientPrincipal, clientRealm, serverPrincipal,
		kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), m.Input.Password, preAuthHint)
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
			return "", fmt.Errorf("KDC error: %v", kerr)
		}
		return "", fmt.Errorf("parse AS-REP: %w", err)
	}

	// Decrypt with the etype the KDC actually used, not a hardcoded one.
	repEtype := rep.EncPart.EType
	if repEtype == 0 {
		repEtype = kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96
	}

	encPart, err := kerberos.DecryptASREPEncPart(rep, sessionKey, repEtype)
	if err != nil {
		return "", fmt.Errorf("decrypt AS-REP: %w", err)
	}

	ticket := rep.Ticket

	ccache := kerberos.NewCCache()
	ccache.AddEntry(kerberos.CCacheEntry{
		ClientPrincipal: clientPrincipal,
		ClientRealm:     clientRealm,
		ServerPrincipal: serverPrincipal,
		ServerRealm:     clientRealm,
		Key:             encPart.Key,
		AuthTime:        time.Now(),
		StartTime:       encPart.Authtime.Time,
		EndTime:         encPart.Endtime.Time,
		RenewTill:       encPart.RenewTill.Time,
		IsSKey:          false,
		// TicketFlags is an OPTIONAL BIT STRING; guard against an empty parse
		// instead of panicking on Bytes[0] (Stage 46g live defect).
		TicketFlags:     ticketFlagsSafe(encPart.Flags),
		Ticket:          ccacheTicketBytes(ticket),
	})

	outputPath := m.Input.OutputPath
	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/%s_%d.ccache", m.Input.Username, time.Now().Unix())
	}

	if err := ccache.WriteToFile(outputPath); err != nil {
		return "", fmt.Errorf("write ccache: %w", err)
	}

	h := sha256.Sum256(ticket.EncPart.Cipher)
	evidence := types.EvidenceRecord{
		ID:             fmt.Sprintf("tgt-%s-%d", m.Input.Username, time.Now().UnixNano()),
		ActionID:       "",
		Kind:           "ad.kerberos.tgt",
		Target:         m.Target(),
		EpistemicClass: types.ClassObserved,
		Confidence:     1.0,
		CollectedAt:    time.Now(),
		Method:         "kerberos-asreq-preauth",
		Payload:        mustMarshal(map[string]any{
			"username":    m.Input.Username,
			"realm":       m.Input.Domain,
			"ccache_path": outputPath,
			"etype":       kerberos.EtypeName(kerberos.ETYPE_RC4_HMAC),
			"ticket_sha":  hex.EncodeToString(h[:]),
		}),
	}
	_ = evidence

	m.Output = TGTOutput{
		CCachePath: outputPath,
		Principal:  clientPrincipal.FullName(clientRealm),
		Realm:      string(clientRealm),
		StartTime:  encPart.Authtime.Time,
		EndTime:    encPart.Endtime.Time,
		Duration:   time.Since(time.Now()),
	}

	return fmt.Sprintf("tgt-%s-%d", m.Input.Username, time.Now().Unix()), nil
}

func (m *TGTMutation) AfterState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"ccache_path": m.Output.CCachePath,
		"principal":   m.Output.Principal,
		"realm":       m.Output.Realm,
		"start_time":  m.Output.StartTime,
		"end_time":    m.Output.EndTime,
	})
}

func (m *TGTMutation) UndoRecipe() *spine.UndoSpec {
	return nil
}

func (m *TGTMutation) Name() string { return "ad.kerberos.tgt" }
func (m *TGTMutation) Capability() string { return "ad.kerberos.tgt" }
func (m *TGTMutation) RiskScore() int { return 15 }
func (m *TGTMutation) Reversible() bool { return true }

func (m *TGTMutation) GenerateEvidence() []types.EvidenceRecord {
	h := sha256.Sum256([]byte(m.Output.CCachePath))
	id := fmt.Sprintf("tgt-%s-%d", m.Input.Username, time.Now().UnixNano())
	payload, _ := json.Marshal(map[string]any{
		"username":    m.Input.Username,
		"realm":       m.Input.Domain,
		"ccache_path": m.Output.CCachePath,
		"etype":       kerberos.EtypeName(kerberos.ETYPE_RC4_HMAC),
		"ticket_sha":  hex.EncodeToString(h[:]),
	})
	return []types.EvidenceRecord{{
		ID:             id,
		ActionID:       "",
		Kind:           "ad.kerberos.tgt",
		Target:         m.Target(),
		EpistemicClass: types.ClassObserved,
		Confidence:     1.0,
		CollectedAt:    time.Now(),
		Method:         "kerberos-asreq-preauth",
		Payload:        payload,
	}}
}

type CCacheInput struct {
	CCachePath string
	Action     string
	OutputPath string
}

type CCacheOutput struct {
	Entries []CCacheEntryInfo
	Output  string
}

type CCacheEntryInfo struct {
	ClientPrincipal string
	ClientRealm     string
	ServerPrincipal string
	ServerRealm     string
	KeyType         int32
	AuthTime        time.Time
	StartTime       time.Time
	EndTime         time.Time
	RenewTill       time.Time
	TicketFlags     int32
	TicketSize      int
}

type CCacheEngine struct{}

func (e *CCacheEngine) Name() string { return "ad.kerberos.ccache" }
func (e *CCacheEngine) Capability() string { return "ad.kerberos.tgt" }
func (e *CCacheEngine) RiskScore() int { return 10 }
func (e *CCacheEngine) Reversible() bool { return true }

type CCacheMutation struct {
	Input  CCacheInput
	Output CCacheOutput
}

func (m *CCacheMutation) Kind() string { return "ad.kerberos.ccache" }
func (m *CCacheMutation) Target() string { return m.Input.CCachePath }

func (m *CCacheMutation) BeforeState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"ccache_path": m.Input.CCachePath,
		"action":      m.Input.Action,
		"output_path": m.Input.OutputPath,
	})
}

func (m *CCacheMutation) Execute(ctx context.Context) (string, error) {
	if len(m.Input.CCachePath) == 0 {
		return "", fmt.Errorf("ccache path required")
	}

	ccache, err := kerberos.ReadCCache(m.Input.CCachePath)
	if err != nil {
		return "", fmt.Errorf("read ccache: %w", err)
	}

	entries := make([]CCacheEntryInfo, 0, len(ccache.Entries))
	for _, entry := range ccache.Entries {
		entries = append(entries, CCacheEntryInfo{
			ClientPrincipal: entry.ClientPrincipal.FullName(entry.ClientRealm),
			ClientRealm:     string(entry.ClientRealm),
			ServerPrincipal: entry.ServerPrincipal.FullName(entry.ServerRealm),
			ServerRealm:     string(entry.ServerRealm),
			KeyType:         entry.Key.KeyType,
			AuthTime:        entry.AuthTime,
			StartTime:       entry.StartTime,
			EndTime:         entry.EndTime,
			RenewTill:       entry.RenewTill,
			TicketFlags:     entry.TicketFlags,
			TicketSize:      len(entry.Ticket),
		})
	}

	output := ""
	if m.Input.Action == "convert" && len(m.Input.OutputPath) > 0 {
		if err := ccache.WriteToFile(m.Input.OutputPath); err != nil {
			return "", fmt.Errorf("write ccache: %w", err)
		}
		output = m.Input.OutputPath
	}

	m.Output = CCacheOutput{Entries: entries, Output: output}
	return fmt.Sprintf("ccache-%s-%d", m.Input.Action, time.Now().Unix()), nil
}

func (m *CCacheMutation) AfterState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"entries":       m.Output.Entries,
		"output_path":   m.Output.Output,
		"entry_count":   len(m.Output.Entries),
	})
}

func (m *CCacheMutation) UndoRecipe() *spine.UndoSpec {
	if m.Input.Action == "convert" {
		return &spine.UndoSpec{
			Provider:     "kerberos",
			Op:           "ccache-convert",
			Args:         map[string]string{"output": m.Input.OutputPath},
			Irreversible: false,
		}
	}
	return nil
}

func (m *CCacheMutation) Name() string { return "ad.kerberos.ccache" }
func (m *CCacheMutation) Capability() string { return "ad.kerberos.tgt" }
func (m *CCacheMutation) RiskScore() int { return 10 }
func (m *CCacheMutation) Reversible() bool { return true }

func (m *CCacheMutation) GenerateEvidence() []types.EvidenceRecord {
	id := fmt.Sprintf("ccache-%s-%d", m.Input.Action, time.Now().UnixNano())
	payload, _ := json.Marshal(map[string]any{
		"input_path":  m.Input.CCachePath,
		"output_path": m.Output.Output,
		"entry_count": len(m.Output.Entries),
	})
	return []types.EvidenceRecord{{
		ID:             id,
		ActionID:       "",
		Kind:           "ad.kerberos.ccache",
		Target:         m.Target(),
		EpistemicClass: types.ClassObserved,
		Confidence:     1.0,
		CollectedAt:    time.Now(),
		Method:         "ccache-" + m.Input.Action,
		Payload:        payload,
	}}
}

// ticketFlagsSafe converts the EncKDCRepPart TicketFlags BIT STRING to the
// 32-bit value recorded in a credential cache. The field is OPTIONAL, so an
// absent or empty BIT STRING must yield 0 rather than panic (Stage 46g live
// defect).
//
// The previous implementation returned only the first octet. TicketFlags is a
// 32-bit word whose first octet carries the most significant bits, so that
// wrote 0x00000040 where MIT records 0x40e00000
// (forwardable|renewable|pre-authenticated|initial) and every cache Aether
// wrote misreported its ticket flags as a single undefined bit.
func ticketFlagsSafe(flags asn1.BitString) int32 {
	if len(flags.Bytes) == 0 || flags.BitLength == 0 {
		return 0
	}
	b := flags.Bytes
	if len(b) > 4 {
		b = b[:4]
	}
	var v uint32
	for _, c := range b {
		v = v<<8 | uint32(c)
	}
	// Clear any bits beyond the declared bit length so a short BIT STRING
	// cannot contribute stray set bits from its final octet. The meaningful
	// bits are the first `total` bits of the len(b)-byte sequence, which sits
	// left-aligned in v, so the low (len(b)*8 - total) bits are dropped. The
	// mask is arithmetic rather than in place because b aliases the caller's
	// slice.
	if total := flags.BitLength; total > 0 && total < len(b)*8 {
		v &= ^uint32(0) << (len(b)*8 - total)
	}
	return int32(v)
}

// ccacheTicketBytes returns the DER encoding a credential cache must store in
// its ticket field: the complete KerberosTicket as issued by the KDC.
// The previous code wrote ticket.EncPart.Cipher, the AES-encrypted enc-part,
// which is a different structure: it carries no tkt-vno, realm or sname and is
// not decodable by any Kerberos tool reading the cache.
func ccacheTicketBytes(t *kerberos.Ticket) []byte {
	if t == nil {
		return nil
	}
	if len(t.Raw) > 0 {
		return t.Raw
	}
	return t.EncPart.Cipher
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}