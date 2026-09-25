package ldap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidSecurityDescriptor = errors.New("invalid security descriptor")
	ErrInvalidACE                = errors.New("invalid ACE")
	ErrInvalidSID                = errors.New("invalid SID")
	ErrInvalidGUID               = errors.New("invalid GUID")
)

const (
	SE_DACL_PRESENT          = 0x00000004
	SE_DACL_DEFAULTED        = 0x00000008
	SE_SACL_PRESENT          = 0x00000010
	SE_SACL_DEFAULTED        = 0x00000020
	SE_DACL_AUTO_INHERIT_REQ = 0x00000100
	SE_SACL_AUTO_INHERIT_REQ = 0x00000200
	SE_DACL_AUTO_INHERITED   = 0x00000400
	SE_SACL_AUTO_INHERITED   = 0x00000800
	SE_DACL_PROTECTED        = 0x00001000
	SE_SACL_PROTECTED        = 0x00002000
	SE_RM_CONTROL_VALID      = 0x00004000
	SE_SELF_RELATIVE         = 0x00008000
)

const (
	ACE_TYPE_ACCESS_ALLOWED                 = 0x00
	ACE_TYPE_ACCESS_DENIED                  = 0x01
	ACE_TYPE_SYSTEM_AUDIT                   = 0x02
	ACE_TYPE_SYSTEM_ALARM                   = 0x03
	ACE_TYPE_ACCESS_ALLOWED_COMPOUND        = 0x04
	ACE_TYPE_ACCESS_ALLOWED_OBJECT          = 0x05
	ACE_TYPE_ACCESS_DENIED_OBJECT           = 0x06
	ACE_TYPE_SYSTEM_AUDIT_OBJECT            = 0x07
	ACE_TYPE_SYSTEM_ALARM_OBJECT            = 0x08
	ACE_TYPE_ACCESS_ALLOWED_CALLBACK        = 0x09
	ACE_TYPE_ACCESS_DENIED_CALLBACK         = 0x0A
	ACE_TYPE_SYSTEM_AUDIT_CALLBACK          = 0x0B
	ACE_TYPE_SYSTEM_ALARM_CALLBACK          = 0x0C
	ACE_TYPE_ACCESS_ALLOWED_CALLBACK_OBJECT = 0x0D
	ACE_TYPE_ACCESS_DENIED_CALLBACK_OBJECT  = 0x0E
	ACE_TYPE_SYSTEM_AUDIT_CALLBACK_OBJECT   = 0x0F
	ACE_TYPE_SYSTEM_ALARM_CALLBACK_OBJECT   = 0x10
	ACE_TYPE_SYSTEM_MANDATORY_LABEL         = 0x11
	ACE_TYPE_SYSTEM_RESOURCE_ATTRIBUTE      = 0x12
	ACE_TYPE_SYSTEM_SCOPED_POLICY_ID        = 0x13
	ACE_TYPE_SYSTEM_PROCESS_TRUST_LABEL     = 0x14
	ACE_TYPE_SYSTEM_ACCESS_FILTER           = 0x15
)

const (
	ACE_FLAG_INHERITED         = 0x10
	ACE_FLAG_CONTAINER_INHERIT = 0x02
	ACE_FLAG_INHERIT_ONLY      = 0x08
	ACE_FLAG_NO_PROPAGATE      = 0x04
	ACE_FLAG_OBJECT_INHERIT    = 0x01
	ACE_FLAG_FAILED_ACCESS     = 0x80
	ACE_FLAG_SUCCESSFUL_ACCESS = 0x40
)

const (
	RIGHT_DELETE                 = 0x00010000
	RIGHT_READ_CONTROL           = 0x00020000
	RIGHT_WRITE_DAC              = 0x00040000
	RIGHT_WRITE_OWNER            = 0x00080000
	RIGHT_SYNCHRONIZE            = 0x00100000
	RIGHT_ACCESS_SYSTEM_SECURITY = 0x01000000
	RIGHT_MAXIMUM_ALLOWED        = 0x02000000
	RIGHT_GENERIC_ALL            = 0x10000000
	RIGHT_GENERIC_EXECUTE        = 0x20000000
	RIGHT_GENERIC_WRITE          = 0x40000000
	RIGHT_GENERIC_READ           = 0x80000000

	RIGHT_CREATE_CHILD   = 0x00000001
	RIGHT_DELETE_CHILD   = 0x00000002
	RIGHT_LIST_CHILDREN  = 0x00000004
	RIGHT_SELF           = 0x00000008
	RIGHT_READ_PROPERTY  = 0x00000010
	RIGHT_WRITE_PROPERTY = 0x00000020
	RIGHT_DELETE_TREE    = 0x00000040
	RIGHT_LIST_OBJECT    = 0x00000080
	RIGHT_CONTROL_ACCESS = 0x00000100
)

type SecurityDescriptor struct {
	Revision    byte
	Sbz1        byte
	Control     uint32
	OwnerOffset uint32
	GroupOffset uint32
	DaclOffset  uint32
	SaclOffset  uint32
	Owner       *SID
	Group       *SID
	Dacl        *ACL
	Sacl        *ACL
}

type ACL struct {
	Revision byte
	Sbz1     byte
	Count    uint16
	ACEs     []ACE
}

type ACE struct {
	Type                byte
	Flags               byte
	Size                uint16
	Mask                uint32
	SID                 *SID
	ObjectType          *GUID
	InheritedObjectType *GUID
	ApplicationData     []byte
}

type SID struct {
	Revision            byte
	SubAuthorityCount   byte
	IdentifierAuthority [6]byte
	SubAuthority        []uint32
}

func (s *SID) String() string {
	if s == nil {
		return ""
	}
	var parts []string
	parts = append(parts, "S")
	parts = append(parts, fmt.Sprintf("%d", s.Revision))
	ia := uint64(s.IdentifierAuthority[0])<<40 | uint64(s.IdentifierAuthority[1])<<32 |
		uint64(s.IdentifierAuthority[2])<<24 | uint64(s.IdentifierAuthority[3])<<16 |
		uint64(s.IdentifierAuthority[4])<<8 | uint64(s.IdentifierAuthority[5])
	parts = append(parts, fmt.Sprintf("%d", ia))
	for _, sub := range s.SubAuthority {
		parts = append(parts, fmt.Sprintf("%d", sub))
	}
	return strings.Join(parts, "-")
}

func (s *SID) Equal(other *SID) bool {
	if s == nil || other == nil {
		return s == other
	}
	if s.Revision != other.Revision || s.SubAuthorityCount != other.SubAuthorityCount {
		return false
	}
	for i := 0; i < 6; i++ {
		if s.IdentifierAuthority[i] != other.IdentifierAuthority[i] {
			return false
		}
	}
	if len(s.SubAuthority) != len(other.SubAuthority) {
		return false
	}
	for i := range s.SubAuthority {
		if s.SubAuthority[i] != other.SubAuthority[i] {
			return false
		}
	}
	return true
}

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

func (g *GUID) String() string {
	if g == nil {
		return ""
	}
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1],
		g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

func (g *GUID) Equal(other *GUID) bool {
	if g == nil || other == nil {
		return g == other
	}
	return g.Data1 == other.Data1 && g.Data2 == other.Data2 &&
		g.Data3 == other.Data3 && g.Data4 == other.Data4
}

func (g *GUID) IsZero() bool {
	return g.Data1 == 0 && g.Data2 == 0 && g.Data3 == 0 &&
		g.Data4[0] == 0 && g.Data4[1] == 0 && g.Data4[2] == 0 &&
		g.Data4[3] == 0 && g.Data4[4] == 0 && g.Data4[5] == 0 &&
		g.Data4[6] == 0 && g.Data4[7] == 0
}

func ParseSecurityDescriptor(data []byte) (*SecurityDescriptor, error) {
	if len(data) < 20 {
		return nil, ErrInvalidSecurityDescriptor
	}

	// MS-DTYP 2.4.6 SECURITY_DESCRIPTOR_RELATIVE layout:
	//   [0] Revision (1B), [1] Sbz1 (1B), [2:4] Control (UINT16!),
	//   [4:8] OwnerOffset, [8:12] GroupOffset, [12:16] SaclOffset,
	//   [16:20] DaclOffset. The previous parser read Control as UINT32 and
	//   shifted every offset by 2 bytes, producing garbage offsets
	//   (e.g. 0x300000) and a slice-bounds panic on real Samba SDs.
	sd := &SecurityDescriptor{
		Revision:    data[0],
		Sbz1:        data[1],
		Control:     uint32(binary.LittleEndian.Uint16(data[2:4])),
		OwnerOffset: binary.LittleEndian.Uint32(data[4:8]),
		GroupOffset: binary.LittleEndian.Uint32(data[8:12]),
		// MS-DTYP 2.4.6 wire order: OwnerOffset, GroupOffset, SaclOffset,
		// DaclOffset — the SACL offset precedes the DACL offset.
		SaclOffset: binary.LittleEndian.Uint32(data[12:16]),
		DaclOffset: binary.LittleEndian.Uint32(data[16:20]),
	}

	if sd.Control&SE_SELF_RELATIVE == 0 {
		// Absolute format - pointers (not typical in LDAP)
		return nil, fmt.Errorf("absolute format security descriptor not supported")
	}

	// Bounds-check every offset before slicing: malformed or fuzzed input
	// must yield a deterministic error, never a panic.
	checkOffset := func(off uint32, what string) error {
		if off != 0 && uint64(off) >= uint64(len(data)) {
			return fmt.Errorf("%s offset %d out of range (sd length %d)", what, off, len(data))
		}
		return nil
	}
	for _, c := range []struct {
		off uint32
		w   string
	}{{sd.OwnerOffset, "owner"}, {sd.GroupOffset, "group"}, {sd.DaclOffset, "DACL"}, {sd.SaclOffset, "SACL"}} {
		if err := checkOffset(c.off, c.w); err != nil {
			return nil, err
		}
	}

	// Self-relative format - offsets are from start of SD
	if sd.OwnerOffset > 0 {
		sid, err := ParseSID(data[sd.OwnerOffset:])
		if err != nil {
			return nil, fmt.Errorf("parse owner SID: %w", err)
		}
		sd.Owner = sid
	}
	if sd.GroupOffset > 0 {
		sid, err := ParseSID(data[sd.GroupOffset:])
		if err != nil {
			return nil, fmt.Errorf("parse group SID: %w", err)
		}
		sd.Group = sid
	}
	if sd.DaclOffset > 0 {
		acl, err := ParseACL(data[sd.DaclOffset:])
		if err != nil {
			return nil, fmt.Errorf("parse DACL: %w", err)
		}
		sd.Dacl = acl
	}
	if sd.SaclOffset > 0 {
		acl, err := ParseACL(data[sd.SaclOffset:])
		if err != nil {
			return nil, fmt.Errorf("parse SACL: %w", err)
		}
		sd.Sacl = acl
	}

	return sd, nil
}

func ParseSID(data []byte) (*SID, error) {
	if len(data) < 8 {
		return nil, ErrInvalidSID
	}

	sid := &SID{
		Revision:            data[0],
		SubAuthorityCount:   data[1],
		IdentifierAuthority: [6]byte{data[2], data[3], data[4], data[5], data[6], data[7]},
	}

	expectedLen := 8 + int(sid.SubAuthorityCount)*4
	if len(data) < expectedLen {
		return nil, ErrInvalidSID
	}

	sid.SubAuthority = make([]uint32, sid.SubAuthorityCount)
	for i := byte(0); i < sid.SubAuthorityCount; i++ {
		offset := 8 + int(i)*4
		sid.SubAuthority[i] = binary.LittleEndian.Uint32(data[offset : offset+4])
	}

	return sid, nil
}

// ParseStringSID parses an SDDL-form SID string (e.g. "S-1-5-21-...-1103")
// into a SID. The CLI accepts string SIDs from operators; the byte-form
// parser cannot consume them (a "S-1-..." string parses as garbage or fails
// the length check outright — Stage 46g live defect).
func ParseStringSID(s string) (*SID, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) < 3 || parts[0] != "S" {
		return nil, ErrInvalidSID
	}

	rev, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil {
		return nil, ErrInvalidSID
	}

	ia, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return nil, ErrInvalidSID
	}
	if ia > 0xFFFFFFFFFFFF {
		return nil, ErrInvalidSID
	}

	subs := make([]uint32, 0, len(parts)-3)
	for _, p := range parts[3:] {
		v, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, ErrInvalidSID
		}
		subs = append(subs, uint32(v))
	}
	if len(subs) > 255 {
		return nil, ErrInvalidSID
	}

	sid := &SID{
		Revision:            byte(rev),
		SubAuthorityCount:   byte(len(subs)),
		IdentifierAuthority: [6]byte{},
	}
	// IdentifierAuthority is big-endian across 6 bytes.
	for i := 0; i < 6; i++ {
		sid.IdentifierAuthority[5-i] = byte(ia >> (8 * i))
	}
	sid.SubAuthority = subs

	return sid, nil
}

func ParseACL(data []byte) (*ACL, error) {
	if len(data) < 8 {
		return nil, ErrInvalidSecurityDescriptor
	}

	acl := &ACL{
		Revision: data[0],
		Sbz1:     data[1],
		// MS-DTYP 2.4.5 ACL header: Revision(1) Sbz1(1) AclSize(u16)
		// AceCount(u16) Sbz2(u16). AceCount lives at [4:6]; [2:4] is AclSize.
		// Reading the count from [2:4] made every real ACL fail with
		// ErrInvalidACE (Stage 46g live finding).
		Count: binary.LittleEndian.Uint16(data[4:6]),
	}
	// Each ACE is at least 8 bytes; a count above that bound is malformed.
	if int(acl.Count) > (len(data)-8)/8 {
		return nil, ErrInvalidACE
	}

	offset := 8
	acl.ACEs = make([]ACE, 0, acl.Count)

	for i := uint16(0); i < acl.Count; i++ {
		if offset+4 > len(data) {
			return nil, ErrInvalidACE
		}

		ace := ACE{
			Type:  data[offset],
			Flags: data[offset+1],
			Size:  binary.LittleEndian.Uint16(data[offset+2 : offset+4]),
		}

		// Minimum ACE header: Type(1) Flags(1) Size(2) Mask(4) = 8 bytes.
		// Smaller sizes would panic on the mask read below.
		if int(ace.Size) < 8 {
			return nil, ErrInvalidACE
		}
		if int(ace.Size) > len(data)-offset {
			return nil, ErrInvalidACE
		}
		aceData := data[offset : offset+int(ace.Size)]
		ace.Mask = binary.LittleEndian.Uint32(aceData[4:8])

		isObjectACE := ace.Type == ACE_TYPE_ACCESS_ALLOWED_OBJECT || ace.Type == ACE_TYPE_ACCESS_DENIED_OBJECT ||
			ace.Type == ACE_TYPE_SYSTEM_AUDIT_OBJECT || ace.Type == ACE_TYPE_SYSTEM_ALARM_OBJECT ||
			ace.Type == ACE_TYPE_ACCESS_ALLOWED_CALLBACK_OBJECT || ace.Type == ACE_TYPE_ACCESS_DENIED_CALLBACK_OBJECT ||
			ace.Type == ACE_TYPE_SYSTEM_AUDIT_CALLBACK_OBJECT || ace.Type == ACE_TYPE_SYSTEM_ALARM_CALLBACK_OBJECT

		// MS-DTYP 2.4.4: after Mask, OBJECT ACEs carry Flags(u32) then the
		// optional ObjectType / InheritedObjectType GUIDs, then the SID.
		// Non-object ACEs carry the SID directly after the mask. The previous
		// parser read the SID before the Flags field, so every object ACE
		// produced a garbage SID (e.g. "S-1-1394242219").
		sidOffset := 8
		if isObjectACE {
			if len(aceData) < sidOffset+4 {
				return nil, ErrInvalidACE
			}
			objFlags := binary.LittleEndian.Uint32(aceData[sidOffset : sidOffset+4])
			sidOffset += 4
			if objFlags&1 != 0 {
				if len(aceData) < sidOffset+16 {
					return nil, ErrInvalidACE
				}
				guid := &GUID{}
				guid.Data1 = binary.LittleEndian.Uint32(aceData[sidOffset : sidOffset+4])
				guid.Data2 = binary.LittleEndian.Uint16(aceData[sidOffset+4 : sidOffset+6])
				guid.Data3 = binary.LittleEndian.Uint16(aceData[sidOffset+6 : sidOffset+8])
				copy(guid.Data4[:], aceData[sidOffset+8:sidOffset+16])
				ace.ObjectType = guid
				sidOffset += 16
			}
			if objFlags&2 != 0 {
				if len(aceData) < sidOffset+16 {
					return nil, ErrInvalidACE
				}
				guid := &GUID{}
				guid.Data1 = binary.LittleEndian.Uint32(aceData[sidOffset : sidOffset+4])
				guid.Data2 = binary.LittleEndian.Uint16(aceData[sidOffset+4 : sidOffset+6])
				guid.Data3 = binary.LittleEndian.Uint16(aceData[sidOffset+6 : sidOffset+8])
				copy(guid.Data4[:], aceData[sidOffset+8:sidOffset+16])
				ace.InheritedObjectType = guid
				sidOffset += 16
			}
		}

		sid, err := ParseSID(aceData[sidOffset:])
		if err != nil {
			return nil, fmt.Errorf("parse ACE SID: %w", err)
		}
		ace.SID = sid

		// Remaining is application data
		if sidOffset < len(aceData) {
			ace.ApplicationData = aceData[sidOffset:]
		}

		acl.ACEs = append(acl.ACEs, ace)
		offset += int(ace.Size)
	}

	return acl, nil
}

func (sd *SecurityDescriptor) GetDACL() *ACL {
	return sd.Dacl
}

func (sd *SecurityDescriptor) GetSACL() *ACL {
	return sd.Sacl
}

func (sd *SecurityDescriptor) GetOwner() *SID {
	return sd.Owner
}

func (sd *SecurityDescriptor) GetGroup() *SID {
	return sd.Group
}

func (acl *ACL) GetACEs() []ACE {
	return acl.ACEs
}

func (ace *ACE) GetMask() uint32 {
	return ace.Mask
}

func (ace *ACE) GetSID() *SID {
	return ace.SID
}

func (ace *ACE) GetObjectType() *GUID {
	return ace.ObjectType
}

func (ace *ACE) GetInheritedObjectType() *GUID {
	return ace.InheritedObjectType
}

func (ace *ACE) IsInherited() bool {
	return ace.Flags&ACE_FLAG_INHERITED != 0
}

func (ace *ACE) IsContainerInherit() bool {
	return ace.Flags&ACE_FLAG_CONTAINER_INHERIT != 0
}

func (ace *ACE) IsObjectInherit() bool {
	return ace.Flags&ACE_FLAG_OBJECT_INHERIT != 0
}

func (ace *ACE) IsInheritOnly() bool {
	return ace.Flags&ACE_FLAG_INHERIT_ONLY != 0
}

func (ace *ACE) IsNoPropagate() bool {
	return ace.Flags&ACE_FLAG_NO_PROPAGATE != 0
}

func (ace *ACE) AccessMaskString() string {
	mask := ace.Mask
	var parts []string

	if mask&RIGHT_GENERIC_ALL != 0 {
		parts = append(parts, "GENERIC_ALL")
	}
	if mask&RIGHT_GENERIC_READ != 0 {
		parts = append(parts, "GENERIC_READ")
	}
	if mask&RIGHT_GENERIC_WRITE != 0 {
		parts = append(parts, "GENERIC_WRITE")
	}
	if mask&RIGHT_GENERIC_EXECUTE != 0 {
		parts = append(parts, "GENERIC_EXECUTE")
	}
	if mask&RIGHT_MAXIMUM_ALLOWED != 0 {
		parts = append(parts, "MAXIMUM_ALLOWED")
	}
	if mask&RIGHT_ACCESS_SYSTEM_SECURITY != 0 {
		parts = append(parts, "ACCESS_SYSTEM_SECURITY")
	}
	if mask&RIGHT_SYNCHRONIZE != 0 {
		parts = append(parts, "SYNCHRONIZE")
	}
	if mask&RIGHT_WRITE_OWNER != 0 {
		parts = append(parts, "WRITE_OWNER")
	}
	if mask&RIGHT_WRITE_DAC != 0 {
		parts = append(parts, "WRITE_DAC")
	}
	if mask&RIGHT_READ_CONTROL != 0 {
		parts = append(parts, "READ_CONTROL")
	}
	if mask&RIGHT_DELETE != 0 {
		parts = append(parts, "DELETE")
	}

	if mask&RIGHT_CREATE_CHILD != 0 {
		parts = append(parts, "CREATE_CHILD")
	}
	if mask&RIGHT_DELETE_CHILD != 0 {
		parts = append(parts, "DELETE_CHILD")
	}
	if mask&RIGHT_LIST_CHILDREN != 0 {
		parts = append(parts, "LIST_CHILDREN")
	}
	if mask&RIGHT_SELF != 0 {
		parts = append(parts, "SELF")
	}
	if mask&RIGHT_READ_PROPERTY != 0 {
		parts = append(parts, "READ_PROPERTY")
	}
	if mask&RIGHT_WRITE_PROPERTY != 0 {
		parts = append(parts, "WRITE_PROPERTY")
	}
	if mask&RIGHT_DELETE_TREE != 0 {
		parts = append(parts, "DELETE_TREE")
	}
	if mask&RIGHT_LIST_OBJECT != 0 {
		parts = append(parts, "LIST_OBJECT")
	}
	if mask&RIGHT_CONTROL_ACCESS != 0 {
		parts = append(parts, "CONTROL_ACCESS")
	}

	if len(parts) == 0 {
		return fmt.Sprintf("0x%08x", mask)
	}
	return strings.Join(parts, " | ")
}

func (ace *ACE) TypeString() string {
	switch ace.Type {
	case ACE_TYPE_ACCESS_ALLOWED:
		return "ACCESS_ALLOWED"
	case ACE_TYPE_ACCESS_DENIED:
		return "ACCESS_DENIED"
	case ACE_TYPE_SYSTEM_AUDIT:
		return "SYSTEM_AUDIT"
	case ACE_TYPE_SYSTEM_ALARM:
		return "SYSTEM_ALARM"
	case ACE_TYPE_ACCESS_ALLOWED_OBJECT:
		return "ACCESS_ALLOWED_OBJECT"
	case ACE_TYPE_ACCESS_DENIED_OBJECT:
		return "ACCESS_DENIED_OBJECT"
	case ACE_TYPE_SYSTEM_AUDIT_OBJECT:
		return "SYSTEM_AUDIT_OBJECT"
	case ACE_TYPE_SYSTEM_ALARM_OBJECT:
		return "SYSTEM_ALARM_OBJECT"
	case ACE_TYPE_SYSTEM_MANDATORY_LABEL:
		return "MANDATORY_LABEL"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", ace.Type)
	}
}

func ParseSDDL(sddl string) (*SecurityDescriptor, error) {
	// Simplified SDDL parser - in production use windows SDDL parser
	// This is a placeholder for the structure
	return nil, fmt.Errorf("SDDL parsing not implemented - use binary security descriptor")
}

func KnownSIDs() map[string]string {
	return map[string]string{
		"S-1-0":        "Null Authority",
		"S-1-0-0":      "Nobody",
		"S-1-1":        "World Authority",
		"S-1-1-0":      "Everyone",
		"S-1-2":        "Local Authority",
		"S-1-2-0":      "Local",
		"S-1-2-1":      "Console Logon",
		"S-1-3":        "Creator Authority",
		"S-1-3-0":      "Creator Owner",
		"S-1-3-1":      "Creator Group",
		"S-1-3-2":      "Creator Owner Server",
		"S-1-3-3":      "Creator Group Server",
		"S-1-3-4":      "Owner Rights",
		"S-1-4":        "Non-unique Authority",
		"S-1-5":        "NT Authority",
		"S-1-5-1":      "Dialup",
		"S-1-5-2":      "Network",
		"S-1-5-3":      "Batch",
		"S-1-5-4":      "Interactive",
		"S-1-5-5-x-y":  "Logon Session",
		"S-1-5-6":      "Service",
		"S-1-5-7":      "Anonymous",
		"S-1-5-8":      "Proxy",
		"S-1-5-9":      "Enterprise Domain Controllers",
		"S-1-5-10":     "Principal Self",
		"S-1-5-11":     "Authenticated Users",
		"S-1-5-12":     "Restricted Code",
		"S-1-5-13":     "Terminal Server Users",
		"S-1-5-14":     "Remote Interactive Logon",
		"S-1-5-15":     "This Organization",
		"S-1-5-17":     "IUSR",
		"S-1-5-18":     "Local System",
		"S-1-5-19":     "Local Service",
		"S-1-5-20":     "Network Service",
		"S-1-5-32-544": "Administrators",
		"S-1-5-32-545": "Users",
		"S-1-5-32-546": "Guests",
		"S-1-5-32-547": "Power Users",
		"S-1-5-32-548": "Account Operators",
		"S-1-5-32-549": "Server Operators",
		"S-1-5-32-550": "Print Operators",
		"S-1-5-32-551": "Backup Operators",
		"S-1-5-32-552": "Replicators",
		"S-1-5-32-553": "Pre-Windows 2000 Compatible Access",
		"S-1-5-32-554": "Remote Desktop Users",
		"S-1-5-32-555": "Network Configuration Operators",
		"S-1-5-32-556": "Incoming Forest Trust Builders",
		"S-1-5-32-557": "Performance Monitor Users",
		"S-1-5-32-558": "Performance Log Users",
		"S-1-5-32-559": "Windows Authorization Access Group",
		"S-1-5-32-560": "Terminal Server License Servers",
		"S-1-5-32-561": "Distributed COM Users",
		"S-1-5-32-562": "IIS_IUSRS",
		"S-1-5-32-563": "Cryptographic Operators",
		"S-1-5-32-564": "Event Log Readers",
		"S-1-5-32-565": "Certificate Service DCOM Access",
		"S-1-5-32-566": "RDS Remote Access Servers",
		"S-1-5-32-567": "RDS Management Servers",
		"S-1-5-32-568": "RDS Endpoint Servers",
		"S-1-5-32-569": "Hyper-V Administrators",
		"S-1-5-32-570": "Access Control Assistance Operators",
		"S-1-5-32-571": "Remote Management Users",
		"S-1-5-32-572": "Default Account",
		"S-1-5-32-573": "Storage Replica Administrators",
		"S-1-5-32-574": "Device Owners",
		"S-1-5-64-10":  "NTLM Authentication",
		"S-1-5-64-14":  "SChannel Authentication",
		"S-1-5-64-21":  "Digest Authentication",
		"S-1-5-80":     "NT Service",
		"S-1-5-80-0":   "All Services",
	}
}

func WellKnownGUIDs() map[string]string {
	return map[string]string{
		"1131f6aa-9c07-11d1-f79f-00c04fc2dcd2": "Domain Controllers Container",
		"1131f6ab-9c07-11d1-f79f-00c04fc2dcd2": "Computers Container",
		"1131f6ac-9c07-11d1-f79f-00c04fc2dcd2": "Domain",
		"1131f6ad-9c07-11d1-f79f-00c04fc2dcd2": "System Container",
		"1131f6ae-9c07-11d1-f79f-00c04fc2dcd2": "Users Container",
		"1131f6af-9c07-11d1-f79f-00c04fc2dcd2": "Builtin Container",
		"1131f6b0-9c07-11d1-f79f-00c04fc2dcd2": "Lost and Found Container",
		"1131f6b1-9c07-11d1-f79f-00c04fc2dcd2": "NTDS Quotas Container",
		"1131f6b2-9c07-11d1-f79f-00c04fc2dcd2": "ForeignSecurityPrincipals Container",
		"1131f6b3-9c07-11d1-f79f-00c04fc2dcd2": "Program Data Container",
		"1131f6b4-9c07-11d1-f79f-00c04fc2dcd2": "Microsoft Exchange System Objects",
		"1131f6b5-9c07-11d1-f79f-00c04fc2dcd2": "Managed Service Accounts",
		"1131f6b6-9c07-11d1-f79f-00c04fc2dcd2": "Keys Container",
		"1131f6b7-9c07-11d1-f79f-00c04fc2dcd2": "TCPIP Container",
		"1131f6b8-9c07-11d1-f79f-00c04fc2dcd2": "Remote Storage Container",
		"1131f6b9-9c07-11d1-f79f-00c04fc2dcd2": "Dfs Configuration Container",
		"1131f6ba-9c07-11d1-f79f-00c04fc2dcd2": "File Replication Service Container",
		"1131f6bb-9c07-11d1-f79f-00c04fc2dcd2": "Partitions Container",
		"1131f6bc-9c07-11d1-f79f-00c04fc2dcd2": "Enterprise Configuration Container",
		"1131f6bd-9c07-11d1-f79f-00c04fc2dcd2": "Sites Container",
		"1131f6be-9c07-11d1-f79f-00c04fc2dcd2": "Lost and Found Config Container",
		"1131f6bf-9c07-11d1-f79f-00c04fc2dcd2": "Extended Rights Container",
		"1131f6c0-9c07-11d1-f79f-00c04fc2dcd2": "Domain DNS Zone",
		"1131f6c1-9c07-11d1-f79f-00c04fc2dcd2": "Forest DNS Zone",
		"ab721a54-1e2f-11d0-9819-00aa0040529b": "Domain DNS Zone (legacy)",
		"ab721a53-1e2f-11d0-9819-00aa0040529b": "Forest DNS Zone (legacy)",
	}
}
