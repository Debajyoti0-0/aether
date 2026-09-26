package kerberos

import (
	"strings"
)

type principalNameASN1 struct {
	NameType   int32   `asn1:"explicit,tag:0"`
	NameString []string `asn1:"explicit,tag:1"`
}

func ParsePrincipal(s string) (PrincipalName, Realm, error) {
	parts := strings.Split(s, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return PrincipalName{}, Realm(""), ErrInvalidPrincipal
	}

	realm := Realm(parts[1])

	nameType := NAME_TYPE_PRINCIPAL
	if strings.Contains(parts[0], "/") {
		nameType = NAME_TYPE_SRV_INST
	}

	var comps []string
	for _, c := range strings.Split(parts[0], "/") {
		if c == "" {
			return PrincipalName{}, Realm(""), ErrInvalidPrincipal
		}
		comps = append(comps, c)
	}

	// RFC 4120 §5.2.2: name-string holds the principal components only;
	// the realm travels in the message's realm field, never as a component.
	return PrincipalName{
		NameType:   int32(nameType),
		NameString: comps,
	}, realm, nil
}

func (p PrincipalName) FullName(realm Realm) string {
	if len(p.NameString) == 0 {
		return ""
	}
	return strings.Join(p.NameString, "/") + "@" + string(realm)
}

// Components returns the principal components (all of NameString).
func (p PrincipalName) Components() []string {
	if len(p.NameString) == 0 {
		return nil
	}
	return p.NameString
}

func (p PrincipalName) IsSPN() bool {
	if len(p.NameString) < 3 {
		return false
	}
	return p.NameType == NAME_TYPE_SRV_INST
}

func (p PrincipalName) SPNClass() string {
	comps := p.Components()
	if len(comps) < 1 {
		return ""
	}
	return comps[0]
}

func (p PrincipalName) SPNHost() string {
	comps := p.Components()
	if len(comps) < 2 {
		return ""
	}
	return comps[1]
}

func (p PrincipalName) Equal(other PrincipalName) bool {
	if p.NameType != other.NameType {
		return false
	}
	if len(p.NameString) != len(other.NameString) {
		return false
	}
	for i := range p.NameString {
		if p.NameString[i] != other.NameString[i] {
			return false
		}
	}
	return true
}

func MakePrincipalName(nameType int32, components ...string) PrincipalName {
	return PrincipalName{
		NameType:   nameType,
		NameString: components,
	}
}

// MakeUserPrincipal builds a user principal whose components are the name
// parts only. The realm is carried separately (Realm field) — it is NOT a
// PrincipalName component (RFC 4120 §5.2.2).
func MakeUserPrincipal(user, realm string) PrincipalName {
	return PrincipalName{
		NameType:   NAME_TYPE_PRINCIPAL,
		NameString: []string{user},
	}
}

func MakeSPNPrincipal(service, host, realm string) PrincipalName {
	return PrincipalName{
		NameType:   NAME_TYPE_SRV_INST,
		NameString: []string{service, host},
	}
}

