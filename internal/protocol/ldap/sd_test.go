package ldap

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildSelfRelativeSD constructs a spec-correct self-relative security
// descriptor (MS-DTYP 2.4.6): Revision(1) Sbz1(1) Control(u16) Owner(4)
// Group(4) Dacl(4) Sacl(4) + bodies. Offsets are absolute from SD start.
func buildSelfRelativeSD(t *testing.T, control uint16, ownerSID, groupSID []byte, dacl []byte) []byte {
	t.Helper()
	sid := func(subs ...uint32) []byte {
		b := []byte{1, byte(len(subs)), 0, 0, 0, 0, 0, 5} // revision, count, auth=5
		for _, s := range subs {
			var sub [4]byte
			binary.LittleEndian.PutUint32(sub[:], s)
			b = append(b, sub[:]...)
		}
		return b
	}
	ownerSID = sid(32, 544) // S-1-5-32-544
	groupSID = sid(32, 544) // S-1-5-32-544
	everyone := sid(0)      // S-1-5-0
	if dacl == nil {
		// ACL header (8B) + one access-allowed ACE for Everyone.
		ace := make([]byte, 8+len(everyone)) // Type,Flags,Size(2),Mask(4) + SID
		ace[0] = ACE_TYPE_ACCESS_ALLOWED
		ace[1] = 0
		binary.LittleEndian.PutUint16(ace[2:4], uint16(len(ace)))
		binary.LittleEndian.PutUint32(ace[4:8], RIGHT_READ_CONTROL|RIGHT_READ_PROPERTY)
		copy(ace[8:], everyone)
		acl := make([]byte, 8)
		acl[0] = 1 // revision
		binary.LittleEndian.PutUint16(acl[2:4], uint16(8+len(ace))) // AclSize
		binary.LittleEndian.PutUint16(acl[4:6], 1)                  // AceCount
		dacl = append(acl, ace...)
	}

	ownerOff := uint32(20)
	groupOff := ownerOff + uint32(len(ownerSID))
	daclOff := groupOff + uint32(len(groupSID))
	total := daclOff + uint32(len(dacl))

	bin := binary.LittleEndian
	sd := make([]byte, total)
	sd[0] = 1 // revision
	bin.PutUint16(sd[2:4], control|SE_SELF_RELATIVE)
	bin.PutUint32(sd[4:8], ownerOff)  // OwnerOffset
	bin.PutUint32(sd[8:12], groupOff) // GroupOffset
	bin.PutUint32(sd[12:16], 0)       // SaclOffset (wire order: SACL first)
	bin.PutUint32(sd[16:20], daclOff) // DaclOffset
	copy(sd[ownerOff:], ownerSID)
	copy(sd[groupOff:], groupSID)
	copy(sd[daclOff:], dacl)
	return sd
}

func TestParseSecurityDescriptor_RealSambaLayout(t *testing.T) {
	sdBytes := buildSelfRelativeSD(t, SE_DACL_PRESENT, nil, nil, nil)
	sd, err := ParseSecurityDescriptor(sdBytes)
	if err != nil {
		t.Fatalf("ParseSecurityDescriptor: %v", err)
	}
	if sd.Control&SE_SELF_RELATIVE == 0 {
		t.Fatal("control should retain self-relative bit")
	}
	if sd.Owner == nil || sd.Owner.String() != "S-1-5-32-544" {
		t.Fatalf("owner SID = %v", sd.Owner)
	}
	if sd.Dacl == nil || len(sd.Dacl.ACEs) != 1 {
		t.Fatalf("DACL should have 1 ACE, got %+v", sd.Dacl)
	}
	if sd.Dacl.ACEs[0].SID == nil || sd.Dacl.ACEs[0].SID.String() != "S-1-5-0" {
		t.Fatalf("ACE SID = %v", sd.Dacl.ACEs[0].SID)
	}
}

// Regression for the Stage 46g live panic: the old parser read Control as
// UINT32 at [2:6] and shifted Owner/Group/DACL/SACL offsets by 2 bytes,
// producing garbage offsets (data[3145728:]) and a slice-bounds panic on a
// real 2200-byte Samba security descriptor.
func TestParseSecurityDescriptor_HeaderMisalignmentRegression(t *testing.T) {
	sdBytes := buildSelfRelativeSD(t, SE_DACL_PRESENT, nil, nil, nil)
	// The old parser would read OwnerOffset from sdBytes[6:10] — inside the
	// owner SID body — and eventually slice data[0x300000:] → panic.
	if _, err := ParseSecurityDescriptor(sdBytes); err != nil {
		t.Fatalf("spec-correct SD must parse: %v", err)
	}
	// Garbage offsets must now produce errors, not panics.
	bad := buildSelfRelativeSD(t, SE_DACL_PRESENT, nil, nil, nil)
	binary.LittleEndian.PutUint32(bad[4:8], 0x00300000) // 3145728
	if _, err := ParseSecurityDescriptor(bad); err == nil {
		t.Fatal("out-of-range owner offset must error, not panic")
	}
}

func TestParseSecurityDescriptor_NilACLsAndShortInput(t *testing.T) {
	sdBytes := buildSelfRelativeSD(t, 0, nil, nil, nil)
	binary.LittleEndian.PutUint32(sdBytes[16:20], 0) // no DACL ([16:20] = DaclOffset)
	sd, err := ParseSecurityDescriptor(sdBytes)
	if err != nil {
		t.Fatalf("SD without DACL should parse: %v", err)
	}
	if sd.Dacl != nil {
		t.Fatal("DACL should be nil")
	}
	if _, err := ParseSecurityDescriptor([]byte{1, 2, 3}); err == nil {
		t.Fatal("short input must error")
	}
}// Fuzz-guard: malformed DACL sizes/offsets must return errors, never panic.
func TestParseACL_MalformedDoesNotPanic(t *testing.T) {
	cases := [][]byte{
		{1, 0, 8, 0, 0, 0, 0, 0},                               // count=0, no ACEs
		{1, 0, 8, 0, 1, 0, 0, 0},                               // count=1, no ACE bytes
		{1, 0, 8, 0, 1, 0, 0, 0, 0x00, 0x00, 0x04, 0x00},       // ACE size 4 (< 8)
		{1, 0, 16, 0, 1, 0, 0, 0, 0x00, 0x00, 0xFF, 0xFF},      // ACE size 65535
		{1, 0, 8, 0, 0xFF, 0xFF, 0, 0},                         // count 65535 > bound
		{1, 0, 0xFF, 0xFF, 0, 0, 0, 0},                         // bogus AclSize
	}
	for i, c := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("case %d panicked: %v", i, r)
				}
			}()
			_, _ = ParseACL(c)
		}()
	}
}

func TestParseSecurityDescriptor_FuzzGuard(t *testing.T) {
	rnd := binary.LittleEndian
	base := buildSelfRelativeSD(t, SE_DACL_PRESENT, nil, nil, nil)
	// Fuzz each header field with hostile values; parser must never panic.
	for _, off := range []int{4, 8, 12, 16} {
		for _, v := range []uint32{0, 1, 19, 0xFFFFFFFF, 0x80000000} {
			b := bytes.Clone(base)
			rnd.PutUint32(b[off:off+4], v)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("offset field %d = %#x panicked: %v", off, v, r)
					}
				}()
				_, _ = ParseSecurityDescriptor(b)
			}()
		}
	}
}
