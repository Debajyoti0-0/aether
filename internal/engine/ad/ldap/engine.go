package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/ldap"
)

type Engine struct {
	host        string
	port        int
	useTLS      bool
	useStartTLS bool
	conn        net.Conn
	tlsConn     *tls.Conn
	msgID       int
	timeout     time.Duration
	baseDN      string
	BindDN      string
	BindPass    string
}

type SearchOptions struct {
	BaseDN string
	Scope  int
	// ScopeSet disambiguates "caller asked for baseObject" (RFC 4511 §4.5.1
	// baseObject == 0) from "caller left the scope at its zero value". Without
	// it an explicit base-scope search is indistinguishable from an unset one,
	// which is what made GetRootDSE silently widen to SUBTREE.
	ScopeSet     bool
	Filter       string
	Attributes   []string
	SizeLimit    int
	TimeLimit    int
	DerefAliases int
	TypesOnly    bool
	Controls     []ldap.Control
}

type SearchResult struct {
	Entries      []*ldap.SearchResultEntry
	References   []*ldap.SearchResultReference
	ResultCode   int
	MatchedDN    string
	ErrorMessage string
	Controls     []ldap.Control
}

func NewEngine(host string, port int, useTLS bool) *Engine {
	return &Engine{
		host:    host,
		port:    port,
		useTLS:  useTLS,
		timeout: 30 * time.Second,
	}
}

func NewEngineWithStartTLS(host string, port int) *Engine {
	return &Engine{
		host:        host,
		port:        port,
		useStartTLS: true,
		timeout:     30 * time.Second,
	}
}

func (e *Engine) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

func (e *Engine) SetBaseDN(baseDN string) {
	e.baseDN = baseDN
}

func (e *Engine) SetBindCredentials(bindDN, bindPass string) {
	e.BindDN = bindDN
	e.BindPass = bindPass
}

func (e *Engine) Connect(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	dialer := &net.Dialer{Timeout: e.timeout}

	var conn net.Conn
	var err error

	if e.useTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return fmt.Errorf("TLS dial: %w", err)
		}
		e.tlsConn = conn.(*tls.Conn)
		e.conn = conn
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("dial: %w", err)
		}
		e.conn = conn
	}

	return nil
}

func (e *Engine) StartTLS(ctx context.Context) error {
	if e.tlsConn != nil {
		return nil
	}

	config := &tls.Config{
		ServerName:         e.host,
		InsecureSkipVerify: true,
	}
	e.tlsConn = tls.Client(e.conn, config)
	e.conn = e.tlsConn
	return nil
}

func (e *Engine) Bind(ctx context.Context, dn, password string) error {
	if e.conn == nil {
		return fmt.Errorf("not connected")
	}

	e.BindDN = dn
	e.BindPass = password

	msgID := e.nextMsgID()
	reqBytes, err := ldap.EncodeBindRequest(msgID, dn, password, nil)
	if err != nil {
		return fmt.Errorf("encode bind request: %w", err)
	}

	if err := e.send(reqBytes); err != nil {
		return fmt.Errorf("send bind request: %w", err)
	}

	respBytes, err := e.receive(ctx)
	if err != nil {
		return fmt.Errorf("receive bind response: %w", err)
	}

	resp, err := ldap.DecodeBindResponse(respBytes)
	if err != nil {
		return fmt.Errorf("decode bind response: %w", err)
	}

	if resp.ResultCode != ldap.LDAP_SUCCESS {
		return fmt.Errorf("bind failed: %s", resp.ErrorMessage)
	}

	return nil
}

func (e *Engine) SaslBind(ctx context.Context, mechanism, initialCreds string) error {
	if e.conn == nil {
		return fmt.Errorf("not connected")
	}

	msgID := e.nextMsgID()
	reqBytes, err := ldap.EncodeSaslBindRequest(msgID, mechanism, initialCreds, nil)
	if err != nil {
		return fmt.Errorf("encode SASL bind request: %w", err)
	}

	if err := e.send(reqBytes); err != nil {
		return fmt.Errorf("send SASL bind request: %w", err)
	}

	respBytes, err := e.receive(ctx)
	if err != nil {
		return fmt.Errorf("receive SASL bind response: %w", err)
	}

	resp, err := ldap.DecodeBindResponse(respBytes)
	if err != nil {
		return fmt.Errorf("decode SASL bind response: %w", err)
	}

	if resp.ResultCode == ldap.LDAP_SASL_BIND_IN_PROGRESS {
		for resp.ResultCode == ldap.LDAP_SASL_BIND_IN_PROGRESS {
			if resp.ServerSaslCreds == nil {
				return fmt.Errorf("SASL bind in progress but no server credentials")
			}
			return fmt.Errorf("SASL continuation not fully implemented")
		}

		if resp.ResultCode != ldap.LDAP_SUCCESS {
			return fmt.Errorf("SASL bind failed: %s", resp.ErrorMessage)
		}
	}

	return nil
}

// ResolveScope returns the search scope to transmit.
//
// RFC 4511 §4.5.1 defines baseObject as 0, so a zero-valued Scope cannot be
// told apart from "the caller did not choose". SearchOptions.ScopeSet carries
// that distinction. An explicitly requested baseObject scope is preserved; only
// a genuinely unset scope is widened to subtree.
//
// The previous code keyed the decision off `Scope == 0 && BaseDN != ""`, which
// rewrote every base-scope request that had a base DN — including the RootDSE
// query — into a subtree search that returned an arbitrary child object with
// exit code 0 (Stage 46g defect D4).
func ResolveScope(scope int, scopeSet bool) int {
	if !scopeSet && scope == 0 {
		return ldap.LDAP_SCOPE_SUBTREE
	}
	return scope
}

func (e *Engine) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if e.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	if opts.BaseDN == "" {
		opts.BaseDN = e.baseDN
	}
	// Allow empty BaseDN for RootDSE queries
	if opts.BaseDN == "" && opts.Scope != ldap.LDAP_SCOPE_BASE {
		return nil, fmt.Errorf("base DN required")
	}
	if opts.Filter == "" {
		opts.Filter = "(objectClass=*)"
	}
	// RFC 4511 §4.5.1: baseObject is 0, so a zero Scope is ambiguous. Only
	// default an *unset* scope to subtree — never rewrite an explicit
	// base-scope request, which is what GetRootDSE depends on.
	opts.Scope = ResolveScope(opts.Scope, opts.ScopeSet)
	if opts.SizeLimit == 0 {
		opts.SizeLimit = 1000
	}
	if opts.TimeLimit == 0 {
		opts.TimeLimit = 30
	}
	if opts.Attributes == nil {
		opts.Attributes = []string{"*"}
	}

	filter := ldap.ParseFilter(opts.Filter)
	if filter == nil {
		return nil, fmt.Errorf("invalid filter: %s", opts.Filter)
	}

	req := ldap.SearchRequest{
		BaseObject:   opts.BaseDN,
		Scope:        opts.Scope,
		DerefAliases: opts.DerefAliases,
		SizeLimit:    opts.SizeLimit,
		TimeLimit:    opts.TimeLimit,
		TypesOnly:    opts.TypesOnly,
		Filter:       *filter,
		Attributes:   opts.Attributes,
		Controls:     opts.Controls,
	}

	msgID := e.nextMsgID()
	reqBytes, err := ldap.EncodeSearchRequest(msgID, req, opts.Controls)
	if err != nil {
		return nil, fmt.Errorf("encode search request: %w", err)
	}

	if err := e.send(reqBytes); err != nil {
		return nil, fmt.Errorf("send search request: %w", err)
	}

	var allEntries []*ldap.SearchResultEntry
	var allRefs []*ldap.SearchResultReference
	var finalDone *ldap.SearchResultDone

	for {
		respBytes, err := e.receive(ctx)
		if err != nil {
			return nil, fmt.Errorf("receive search response: %w", err)
		}

		entries, done, refs, err := ldap.DecodeSearchResponse(respBytes)
		if err != nil {
			return nil, fmt.Errorf("decode search response: %w", err)
		}

		allEntries = append(allEntries, entries...)
		allRefs = append(allRefs, refs...)

		if done != nil {
			finalDone = done
			break
		}
	}

	if finalDone == nil {
		return nil, fmt.Errorf("no search result done received")
	}

	return &SearchResult{
		Entries:      allEntries,
		References:   allRefs,
		ResultCode:   finalDone.ResultCode,
		MatchedDN:    finalDone.MatchedDN,
		ErrorMessage: finalDone.ErrorMessage,
		Controls:     finalDone.Controls,
	}, nil
}

func (e *Engine) SearchPaged(ctx context.Context, opts SearchOptions, pageSize int) (*SearchResult, error) {
	if pageSize <= 0 {
		pageSize = 1000
	}

	pagedCtrl, err := ldap.EncodePagedResultsControl(pageSize, nil)
	if err != nil {
		return nil, fmt.Errorf("encode paged results control: %w", err)
	}

	if opts.Controls == nil {
		opts.Controls = []ldap.Control{}
	}
	opts.Controls = append(opts.Controls, pagedCtrl)

	var allEntries []*ldap.SearchResultEntry
	var allRefs []*ldap.SearchResultReference
	var cookie []byte

	for {
		if len(opts.Controls) > 1 {
			opts.Controls = opts.Controls[:1]
		}
		if cookie != nil {
			pagedCtrl, err = ldap.EncodePagedResultsControl(pageSize, cookie)
			if err != nil {
				return nil, fmt.Errorf("encode paged results control: %w", err)
			}
			opts.Controls[0] = pagedCtrl
		}

		result, err := e.Search(ctx, opts)
		if err != nil {
			return nil, err
		}

		allEntries = append(allEntries, result.Entries...)
		allRefs = append(allRefs, result.References...)

		for _, ctrl := range result.Controls {
			if ctrl.ControlType == ldap.LDAP_CONTROL_PAGED_RESULTS {
				prc, err := ldap.DecodePagedResultsControl(ctrl)
				if err == nil && prc != nil {
					cookie = prc.Cookie
				}
			}
		}

		if cookie == nil || len(cookie) == 0 {
			break
		}
	}

	return &SearchResult{
		Entries:      allEntries,
		References:   allRefs,
		ResultCode:   ldap.LDAP_SUCCESS,
		MatchedDN:    "",
		ErrorMessage: "",
		Controls:     nil,
	}, nil
}

// rootDSEAttributes are the RFC 4511 §5.1 RootDSE operational attributes.
// Aether previously requested "*", which a server MAY answer with just the
// userApplications attribute set, so the domain/forest attributes operators
// actually need were absent.
var rootDSEAttributes = []string{
	"objectClass",
	"namingContexts",
	"supportedLDAPVersion",
	"supportedControl",
	"supportedExtension",
	"supportedFeatures",
	"subschemaSubentry",
	"vendorName",
	"vendorVersion",
	"defaultNamingContext",
	"dnsHostName",
	"forestFunctionalLevel",
	"domainFunctionalLevel",
	"rootDomainNamingContext",
	"configurationNamingContext",
	"schemaNamingContext",
	"currentTime",
	"maxIdleTime",
	"maxSize",
	"allowedAttributesMask",
}

// GetRootDSE queries the RootDSE. RFC 4511 §5.1: the RootDSE has the EMPTY
// distinguished name and is read with a baseObject search.
//
// Stage 46g observed this command returning
// CN=bydefaults,CN=ypservers,CN=ypServ30,CN=RpcServices,CN=System,DC=aether,DC=test
// with exit code 0, because the CLI derived a "DC=aether,DC=test" base from
// --domain and the engine rewrote the base scope to SUBTREE. The base DN is
// now the empty string unconditionally and the result is validated.
func (e *Engine) GetRootDSE(ctx context.Context) (*SearchResult, error) {
	result, err := e.Search(ctx, SearchOptions{
		BaseDN:     "", // RootDSE is at the empty DN; never the domain NC
		Scope:      ldap.LDAP_SCOPE_BASE,
		ScopeSet:   true,
		Filter:     "(objectClass=*)",
		Attributes: rootDSEAttributes,
		SizeLimit:  1,
	})
	if err != nil {
		return nil, err
	}
	if result.ResultCode != 0 {
		return nil, fmt.Errorf("rootdse search: %s", result.ErrorMessage)
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("rootdse: server returned no entry")
	}
	// Fail loudly rather than report a wrong object as the RootDSE.
	if dn := string(result.Entries[0].ObjectName); dn != "" {
		return nil, fmt.Errorf("rootdse: expected the empty distinguished name, got %q", dn)
	}
	return result, nil
}

func (e *Engine) Close() error {
	if e.tlsConn != nil {
		e.tlsConn.Close()
	}
	if e.conn != nil {
		e.conn.Close()
	}
	return nil
}

func (e *Engine) nextMsgID() int {
	e.msgID++
	return e.msgID
}

func (e *Engine) send(data []byte) error {
	if e.conn == nil {
		return fmt.Errorf("not connected")
	}
	_, err := e.conn.Write(data)
	return err
}

func (e *Engine) receive(ctx context.Context) ([]byte, error) {
	if e.conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	result, err := e.readBERMessage(ctx)
	if err != nil {
		return nil, err
	}
	if len(result) > 10*1024*1024 {
		return nil, fmt.Errorf("response too large: %d bytes", len(result))
	}
	return result, nil
}

func (e *Engine) readBERMessage(ctx context.Context) ([]byte, error) {
	tagBuf := make([]byte, 1)
	if err := e.readFull(ctx, tagBuf); err != nil {
		return nil, err
	}
	tag := tagBuf[0]

	lenBuf := make([]byte, 1)
	if err := e.readFull(ctx, lenBuf); err != nil {
		return nil, err
	}
	firstLen := int(lenBuf[0])

	var totalLength int
	var lengthBytes []byte
	if firstLen&0x80 == 0 {
		totalLength = firstLen
		lengthBytes = []byte{byte(firstLen)}
	} else {
		numBytes := firstLen & 0x7f
		if numBytes > 4 {
			return nil, fmt.Errorf("invalid BER length")
		}
		longBytes := make([]byte, numBytes)
		if err := e.readFull(ctx, longBytes); err != nil {
			return nil, err
		}
		totalLength = 0
		for _, b := range longBytes {
			totalLength = (totalLength << 8) | int(b)
		}
		// Reconstruct the FULL original TLV header: the 0x8x marker byte
		// must be preserved, otherwise every response longer than 127
		// bytes is corrupted before decoding (Stage 46g wire defect:
		// "30 82 07 9b ..." was being re-emitted as "30 07 9b ...").
		lengthBytes = append([]byte{byte(firstLen)}, longBytes...)
	}

	value := make([]byte, totalLength)
	if err := e.readFull(ctx, value); err != nil {
		return nil, err
	}

	result := make([]byte, 0, 1+len(lengthBytes)+totalLength)
	result = append(result, tag)
	result = append(result, lengthBytes...)
	result = append(result, value...)
	return result, nil
}

func (e *Engine) readTLVHeader(ctx context.Context) (tag int, length int, err error) {
	tagBuf := make([]byte, 1)
	if err := e.readFull(ctx, tagBuf); err != nil {
		return 0, 0, err
	}
	tag = int(tagBuf[0])
	lenBuf := make([]byte, 1)
	if err := e.readFull(ctx, lenBuf); err != nil {
		return 0, 0, err
	}
	l := int(lenBuf[0])
	if l&0x80 != 0 {
		numBytes := l & 0x7f
		if numBytes > 4 {
			return 0, 0, fmt.Errorf("invalid BER length")
		}
		lenBytes := make([]byte, numBytes)
		if err := e.readFull(ctx, lenBytes); err != nil {
			return 0, 0, err
		}
		length = 0
		for _, b := range lenBytes {
			length = (length << 8) | int(b)
		}
		return tag, length, nil
	}
	return tag, l, nil
}

func (e *Engine) readFull(ctx context.Context, buf []byte) error {
	deadline := time.Now().Add(e.timeout)
	e.conn.SetReadDeadline(deadline)

	total := 0
	for total < len(buf) {
		n, err := e.conn.Read(buf[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}
