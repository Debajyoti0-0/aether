package kerberos

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"time"
)

var (
	ErrInvalidPAC = errors.New("invalid PAC structure")
)

type PACLogonInfoV1 struct {
	LogonTime        int64
	LogoffTime       int64
	KickOffTime      int64
	PasswordLastSet  int64
	PasswordCanChange int64
	PasswordMustChange int64
	EffectiveName    string
	FullName         string
	LogonScript      string
	ProfilePath      string
	HomeDirectory    string
	HomeDirectoryDrive string
	LogonCount       int16
	BadPasswordCount int16
	UserID           int32
	PrimaryGroupID   int32
	GroupCount       int32
	GroupIDs         []uint32
	UserFlags        int32
	UserSessionKey   []byte
	LogonServer      string
	LogonDomainName  string
	LogonDomainID    []byte
}

type PACLogonInfoV2 struct {
	LogonTime        int64
	LogoffTime       int64
	KickOffTime      int64
	PasswordLastSet  int64
	PasswordCanChange int64
	PasswordMustChange int64
	EffectiveName    string
	FullName         string
	LogonScript      string
	ProfilePath      string
	HomeDirectory    string
	HomeDirectoryDrive string
	LogonCount       int16
	BadPasswordCount int16
	UserID           int32
	PrimaryGroupID   int32
	GroupCount       int32
	GroupIDs         []uint32
	UserFlags        int32
	UserSessionKey   []byte
	LogonServer      string
	LogonDomainName  string
	LogonDomainID    []byte
	SidCount         int32
	ExtraSids        []PACSidAndAttributes
	ResourceGroupDomainSid []byte
	ResourceGroupCount int32
	ResourceGroupIDs []uint32
}

type PACSidAndAttributes struct {
	Sid []byte
	Attributes int32
}

type PACClientInfo struct {
	ClientId time.Time
	Name     string
}

type PACUpnDnsInfo struct {
	Upn string
	DnsDomainName string
	Flags int32
}

type PACSignatureData struct {
	SignatureType uint16
	Signature     []byte
	RODCIdentifier uint16
	Reserved      uint32
}

type PACInfoBuffer struct {
	ULType int32
	CBBufferSize int32
	Offset int64
}

type PAC struct {
	CBuffers int32
	Buffers  []PACInfoBuffer
	Data     []byte
}

func ParsePAC(data []byte) (*PAC, error) {
	if len(data) < 8 {
		return nil, ErrInvalidPAC
	}

	cBuffers := binary.LittleEndian.Uint32(data[0:4])
	_ = binary.LittleEndian.Uint32(data[4:8]) // version

	buffers := make([]PACInfoBuffer, cBuffers)
	offset := 8

	for i := uint32(0); i < cBuffers; i++ {
		if offset+16 > len(data) {
			return nil, ErrInvalidPAC
		}

		ulType := binary.LittleEndian.Uint32(data[offset : offset+4])
		cbBufferSize := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		off := int64(binary.LittleEndian.Uint64(data[offset+8 : offset+16]))

		buffers[i] = PACInfoBuffer{
			ULType: int32(ulType),
			CBBufferSize: int32(cbBufferSize),
			Offset: off,
		}
		offset += 16
	}

	pac := &PAC{
		CBuffers: int32(cBuffers),
		Buffers:  buffers,
		Data:     data,
	}

	return pac, nil
}

func (pac *PAC) GetBuffer(bufferType int32) ([]byte, error) {
	for _, buf := range pac.Buffers {
		if buf.ULType == bufferType {
			start := buf.Offset
			end := start + int64(buf.CBBufferSize)
			if start < 0 || end > int64(len(pac.Data)) {
				return nil, ErrInvalidPAC
			}
			return pac.Data[start:end], nil
		}
	}
	return nil, ErrInvalidPAC
}

func (pac *PAC) GetLogonInfo() (*PACLogonInfoV1, error) {
	data, err := pac.GetBuffer(PAC_LOGON_INFO)
	if err != nil {
		return nil, err
	}

	if len(data) < 72 {
		return nil, ErrInvalidPAC
	}

	logonTime := int64(binary.LittleEndian.Uint64(data[0:8]))
	logoffTime := int64(binary.LittleEndian.Uint64(data[8:16]))
	kickOffTime := int64(binary.LittleEndian.Uint64(data[16:24]))
	passwordLastSet := int64(binary.LittleEndian.Uint64(data[24:32]))
	passwordCanChange := int64(binary.LittleEndian.Uint64(data[32:40]))
	passwordMustChange := int64(binary.LittleEndian.Uint64(data[40:48]))

	offset := 48
	effectiveName, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}
	fullName, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}
	logonScript, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}
	profilePath, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}
	homeDirectory, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}
	homeDirectoryDrive, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}

	logonCount := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	badPasswordCount := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	userID := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	primaryGroupID := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	groupCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	groupIDs := make([]uint32, groupCount)
	for i := uint32(0); i < groupCount; i++ {
		groupIDs[i] = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
	}

	userFlags := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	userSessionKey := data[offset : offset+16]
	offset += 16

	logonServer, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}
	logonDomainName, offset, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}

	logonDomainID := data[offset : offset+28]

	return &PACLogonInfoV1{
		LogonTime:         logonTime,
		LogoffTime:        logoffTime,
		KickOffTime:       kickOffTime,
		PasswordLastSet:   passwordLastSet,
		PasswordCanChange: passwordCanChange,
		PasswordMustChange: passwordMustChange,
		EffectiveName:     effectiveName,
		FullName:          fullName,
		LogonScript:       logonScript,
		ProfilePath:       profilePath,
		HomeDirectory:     homeDirectory,
		HomeDirectoryDrive: homeDirectoryDrive,
		LogonCount:        int16(logonCount),
		BadPasswordCount:  int16(badPasswordCount),
		UserID:            int32(userID),
		PrimaryGroupID:    int32(primaryGroupID),
		GroupCount:        int32(groupCount),
		GroupIDs:          groupIDs,
		UserFlags:         int32(userFlags),
		UserSessionKey:    userSessionKey,
		LogonServer:       logonServer,
		LogonDomainName:   logonDomainName,
		LogonDomainID:     logonDomainID,
	}, nil
}

func (pac *PAC) GetClientInfo() (*PACClientInfo, error) {
	data, err := pac.GetBuffer(PAC_CLIENT_INFO)
	if err != nil {
		return nil, err
	}

	if len(data) < 8 {
		return nil, ErrInvalidPAC
	}

	clientId := int64(binary.LittleEndian.Uint64(data[0:8]))
	name, _, err := readUnicodeString(data, 8)
	if err != nil {
		return nil, err
	}

	return &PACClientInfo{
		ClientId: time.Unix(0, clientId*100),
		Name:     name,
	}, nil
}

func (pac *PAC) GetUpnDnsInfo() (*PACUpnDnsInfo, error) {
	data, err := pac.GetBuffer(PAC_UPN_DNS_INFO)
	if err != nil {
		return nil, err
	}

	if len(data) < 4 {
		return nil, ErrInvalidPAC
	}

	flags := binary.LittleEndian.Uint32(data[0:4])
	upn, offset, err := readUnicodeString(data, 4)
	if err != nil {
		return nil, err
	}
	dnsDomainName, _, err := readUnicodeString(data, offset)
	if err != nil {
		return nil, err
	}

	return &PACUpnDnsInfo{
		Upn: upn,
		DnsDomainName: dnsDomainName,
		Flags: int32(flags),
	}, nil
}

func (pac *PAC) GetSignature() (*PACSignatureData, error) {
	data, err := pac.GetBuffer(PAC_SERVER_CHECKSUM)
	if err != nil {
		return nil, err
	}

	if len(data) < 8 {
		return nil, ErrInvalidPAC
	}

	sigType := binary.LittleEndian.Uint16(data[0:2])
	rodcID := binary.LittleEndian.Uint16(data[2:4])
	reserved := binary.LittleEndian.Uint32(data[4:8])
	signature := data[8:]

	return &PACSignatureData{
		SignatureType: sigType,
		Signature:     signature,
		RODCIdentifier: rodcID,
		Reserved:      reserved,
	}, nil
}

func readUnicodeString(data []byte, offset int) (string, int, error) {
	if offset >= len(data) {
		return "", offset, nil
	}

	length := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	if offset+int(length) > len(data) {
		return "", offset, ErrInvalidPAC
	}

	s := string(data[offset : offset+int(length)])
	offset += int(length)

	return s, offset, nil
}

func (pac *PAC) VerifyServerSignature(key []byte) error {
	sig, err := pac.GetSignature()
	if err != nil {
		return err
	}

	logonInfoData, err := pac.GetBuffer(PAC_LOGON_INFO)
	if err != nil {
		return err
	}

	expectedHMAC := hmac.New(sha1.New, key)
	expectedHMAC.Write(logonInfoData)
	expectedSig := expectedHMAC.Sum(nil)

	if !hmac.Equal(sig.Signature, expectedSig) {
		return ErrIntegrityCheck
	}

	return nil
}