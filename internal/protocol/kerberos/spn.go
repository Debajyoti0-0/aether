package kerberos

import (
	"errors"
	"strings"
)

var (
	ErrInvalidSPN = errors.New("invalid SPN format")
)

type SPN struct {
	ServiceClass string
	Host         string
	Port         int
	Realm        Realm
}

func ParseSPN(s string) (SPN, error) {
	parts := strings.Split(s, "@")
	// The @REALM suffix is OPTIONAL: directory-stored SPNs are
	// "service/host" (e.g. "HOST/dc01.aether.test") and the realm is implied
	// by the account's domain. Requiring the suffix rejected every real SPN.
	var spnPart string
	var realm Realm
	switch len(parts) {
	case 1:
		spnPart = parts[0]
	case 2:
		spnPart = parts[0]
		realm = Realm(parts[1])
	default:
		return SPN{}, ErrInvalidSPN
	}

	spnParts := strings.Split(spnPart, "/")
	if len(spnParts) < 2 {
		return SPN{}, ErrInvalidSPN
	}

	serviceClass := spnParts[0]
	hostPort := spnParts[1]

	host := hostPort
	port := 0

	if strings.Contains(hostPort, ":") {
		hp := strings.Split(hostPort, ":")
		host = hp[0]
		if len(hp) > 1 {
			// parse port if needed
		}
	}

	return SPN{
		ServiceClass: serviceClass,
		Host:         host,
		Port:         port,
		Realm:        realm,
	}, nil
}

func (spn SPN) String() string {
	return spn.ServiceClass + "/" + spn.Host + "@" + string(spn.Realm)
}

func (spn SPN) PrincipalName() PrincipalName {
	return MakeSPNPrincipal(spn.ServiceClass, spn.Host, string(spn.Realm))
}

func CanonicalizeSPN(spn string) (string, error) {
	parsed, err := ParseSPN(spn)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func ExtractSPNsFromTicket(ticket *Ticket) []SPN {
	var spns []SPN

	if ticket.SName.IsSPN() {
		spnStr := ticket.SName.FullName(ticket.Realm)
		if parsed, err := ParseSPN(spnStr); err == nil {
			spns = append(spns, parsed)
		}
	}

	return spns
}

func SPNMatches(spn1, spn2 SPN) bool {
	return spn1.ServiceClass == spn2.ServiceClass &&
		strings.EqualFold(spn1.Host, spn2.Host) &&
		strings.EqualFold(string(spn1.Realm), string(spn2.Realm))
}

func BuildSPN(serviceClass, host string, realm Realm) string {
	return serviceClass + "/" + host + "@" + string(realm)
}

func DecomposeSPN(principal PrincipalName, realm Realm) (serviceClass, host string, ok bool) {
	if !principal.IsSPN() {
		return "", "", false
	}
	comps := principal.Components()
	if len(comps) < 2 {
		return "", "", false
	}
	return comps[0], comps[1], true
}