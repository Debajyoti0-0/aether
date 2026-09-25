package kerberos

import (
	"encoding/binary"
	"errors"
	"os"
)

var (
	ErrInvalidKeytab = errors.New("invalid keytab format")
	ErrKeytabEntryNotFound = errors.New("keytab entry not found")
)

type KeytabEntry struct {
	Principal  PrincipalName
	Realm      Realm
	Timestamp  uint32
	Vno        uint32
	KeyType    int32
	Key        []byte
}

type Keytab struct {
	Version uint16
	Entries []KeytabEntry
}

func ReadKeytab(filename string) (*Keytab, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseKeytab(data)
}

func ParseKeytab(data []byte) (*Keytab, error) {
	if len(data) < 4 {
		return nil, ErrInvalidKeytab
	}

	version := binary.BigEndian.Uint16(data[0:2])
	if version != 0x0502 && version != 0x0501 {
		return nil, ErrInvalidKeytab
	}

	// MIT file keytab format (kt_file.c): after the 2-byte version header
	// there is NO record count — the file is a sequence of records, each
	// prefixed with a 4-byte big-endian total length, terminated by a
	// zero-length record. The previous parser read bytes [2:4] (part of the
	// first record length) as a count and silently produced zero entries.
	offset := 2

	kt := &Keytab{Version: version}

	for offset+4 <= len(data) {
		entrySize := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		if entrySize == 0 {
			break // terminator
		}
		if offset+int(entrySize) > len(data) {
			return nil, ErrInvalidKeytab
		}

		entryData := data[offset : offset+int(entrySize)]
		offset += int(entrySize)

		entry, err := parseKeytabEntry(entryData)
		if err != nil {
			return nil, err
		}
		kt.Entries = append(kt.Entries, entry)
	}

	return kt, nil
}

func parseKeytabEntry(data []byte) (KeytabEntry, error) {
	if len(data) < 2 {
		return KeytabEntry{}, ErrInvalidKeytab
	}

	numComponents := int16(binary.BigEndian.Uint16(data[0:2]))
	offset := 2

	// Realm: a single length-prefixed string (count is NOT included in
	// numComponents — it counts only the principal components).
	if offset+2 > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	realmLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	if offset+int(realmLen) > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	realm := Realm(data[offset : offset+int(realmLen)])
	offset += int(realmLen)

	// Components: each is a 2-byte big-endian length followed by the bytes.
	// There is NO per-component name type (the previous parser skipped two
	// phantom bytes per component, misaligning every later field).
	var components []string
	for i := int16(0); i < numComponents; i++ {
		if offset+2 > len(data) {
			return KeytabEntry{}, ErrInvalidKeytab
		}
		nameLen := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2
		if offset+int(nameLen) > len(data) {
			return KeytabEntry{}, ErrInvalidKeytab
		}
		components = append(components, string(data[offset:offset+int(nameLen)]))
		offset += int(nameLen)
	}

	// Principal name: the components only — the realm travels in its own
	// field (RFC 4120 §5.2.2; FullName() appends it). The previous parser
	// prepended the realm into NameString, producing
	// "AETHER.TEST/Administrator@AETHER.TEST" which never matched lookups.
	principal := PrincipalName{
		NameType:   NAME_TYPE_PRINCIPAL,
		NameString: components,
	}

	// Name type: 4 bytes, follows the strings (verified against a real
	// samba-tool export: 00 00 00 01 = NT_PRINCIPAL). The previous parser
	// skipped this field, shifting every subsequent field read.
	if offset+4 > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	nameType := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	principal.NameType = int32(nameType)

	// Timestamp: 4 bytes.
	if offset+4 > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	timestamp := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// VNO: 1 byte in the classic layout (records are length-prefixed, so
	// any trailing 4-byte kvno extension is skipped with the record body).
	if offset+1 > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	vno := uint32(data[offset])
	offset += 1

	// Key type: 2 bytes.
	if offset+2 > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	keyType := int32(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2

	// Key length: 2 bytes.
	if offset+2 > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	keyLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	if offset+int(keyLen) > len(data) {
		return KeytabEntry{}, ErrInvalidKeytab
	}
	key := data[offset : offset+int(keyLen)]

	return KeytabEntry{
		Principal: principal,
		Realm:     realm,
		Timestamp: timestamp,
		Vno:       vno,
		KeyType:   keyType,
		Key:       key,
	}, nil
}

func (k *Keytab) FindEntry(principalName string, realm Realm, keyType int32) (*KeytabEntry, error) {
	for i := range k.Entries {
		entry := &k.Entries[i]
		if entry.Realm == realm && entry.KeyType == keyType {
			if entry.Principal.FullName(entry.Realm) == principalName {
				return entry, nil
			}
		}
	}
	return nil, ErrKeytabEntryNotFound
}