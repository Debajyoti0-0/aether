package ldap

import (
	"encoding/asn1"
	"fmt"
)

func EncodePagedResultsControl(size int, cookie []byte) (Control, error) {
	ctrl := PagedResultsControl{
		Size: size,
		Cookie: cookie,
	}

	ctrlBytes, err := asn1.Marshal(ctrl)
	if err != nil {
		return Control{}, fmt.Errorf("marshal paged results control: %w", err)
	}

	return Control{
		ControlType: LDAP_CONTROL_PAGED_RESULTS,
		Criticality: true,
		ControlValue: ctrlBytes,
	}, nil
}

func DecodePagedResultsControl(ctrl Control) (*PagedResultsControl, error) {
	if ctrl.ControlType != LDAP_CONTROL_PAGED_RESULTS {
		return nil, fmt.Errorf("not a paged results control")
	}

	var prc PagedResultsControl
	_, err := asn1.Unmarshal(ctrl.ControlValue, &prc)
	if err != nil {
		return nil, fmt.Errorf("unmarshal paged results control: %w", err)
	}

	return &prc, nil
}

func EncodeSortControl(attrs []string, reverse bool) (Control, error) {
	type sortKey struct {
		AttrType   string
		Reverse    bool `asn1:"optional"`
	}

	type sortControlValue struct {
		SortKeys []sortKey
	}

	keys := make([]sortKey, len(attrs))
	for i, attr := range attrs {
		keys[i] = sortKey{AttrType: attr, Reverse: reverse}
	}

	ctrlVal := sortControlValue{SortKeys: keys}
	ctrlBytes, err := asn1.Marshal(ctrlVal)
	if err != nil {
		return Control{}, fmt.Errorf("marshal sort control: %w", err)
	}

	return Control{
		ControlType: LDAP_CONTROL_SORT,
		Criticality: false,
		ControlValue: ctrlBytes,
	}, nil
}

func EncodeVLVControl(beforeCount, afterCount int, target, context []byte) (Control, error) {
	type vlvRequest struct {
		BeforeCount int
		AfterCount  int
		Target      []byte `asn1:"optional"`
		Context     []byte `asn1:"optional"`
	}

	vlv := vlvRequest{
		BeforeCount: beforeCount,
		AfterCount: afterCount,
		Target: target,
		Context: context,
	}

	ctrlBytes, err := asn1.Marshal(vlv)
	if err != nil {
		return Control{}, fmt.Errorf("marshal vlv control: %w", err)
	}

	return Control{
		ControlType: LDAP_CONTROL_VLV,
		Criticality: true,
		ControlValue: ctrlBytes,
	}, nil
}

func DecodeVLVControl(ctrl Control) (beforeCount, afterCount int, target, context, cookie []byte, err error) {
	if ctrl.ControlType != LDAP_CONTROL_VLV {
		return 0, 0, nil, nil, nil, fmt.Errorf("not a VLV control")
	}

	type vlvResponse struct {
		TargetPosition int `asn1:"optional"`
		ContentCount   int `asn1:"optional"`
		Cookie         []byte `asn1:"optional"`
	}

	var resp vlvResponse
	_, _ = asn1.Unmarshal(ctrl.ControlValue, &resp)

	return 0, 0, nil, nil, resp.Cookie, nil
}

func EncodeRangeControl(attr string, low, high int) (Control, error) {
	type rangeItem struct {
		AttrType string
		Low      int `asn1:"optional"`
		High     int `asn1:"optional"`
	}

	type rangeControl struct {
		Range []rangeItem
	}

	rc := rangeControl{
		Range: []rangeItem{{AttrType: attr, Low: low, High: high}},
	}

	ctrlBytes, err := asn1.Marshal(rc)
	if err != nil {
		return Control{}, fmt.Errorf("marshal range control: %w", err)
	}

	return Control{
		ControlType: LDAP_CONTROL_RANGE,
		Criticality: false,
		ControlValue: ctrlBytes,
	}, nil
}

func DecodeRangeControl(ctrl Control) (attr string, low, high int, err error) {
	if ctrl.ControlType != LDAP_CONTROL_RANGE {
		return "", 0, 0, fmt.Errorf("not a range control")
	}

	type rangeItem struct {
		AttrType string
		Low      int `asn1:"optional"`
		High     int `asn1:"optional"`
	}

	type rangeControl struct {
		Range []rangeItem
	}

	var rc rangeControl
	_, _ = asn1.Unmarshal(ctrl.ControlValue, &rc)

	if len(rc.Range) > 0 {
		return rc.Range[0].AttrType, rc.Range[0].Low, rc.Range[0].High, nil
	}
	return "", 0, 0, nil
}

func EncodePermissiveModifyControl() (Control, error) {
	return Control{
		ControlType: LDAP_CONTROL_PERMISSIVE_MODIFY,
		Criticality: false,
		ControlValue: nil,
	}, nil
}

func EncodeShowDeletedControl() (Control, error) {
	return Control{
		ControlType: LDAP_CONTROL_SHOW_DELETED,
		Criticality: false,
		ControlValue: nil,
	}, nil
}

func EncodeTreeDeleteControl() (Control, error) {
	return Control{
		ControlType: LDAP_CONTROL_TREE_DELETE,
		Criticality: true,
		ControlValue: nil,
	}, nil
}

func EncodeDomainScopeControl() (Control, error) {
	return Control{
		ControlType: LDAP_CONTROL_DOMAIN_SCOPE,
		Criticality: false,
		ControlValue: nil,
	}, nil
}

func EncodeDirSyncControl(flags int, cookie []byte, maxAttrCount int) (Control, error) {
	type dirSyncControlValue struct {
		Flags         int
		Cookie        []byte `asn1:"optional"`
		MaxAttrCount  int `asn1:"optional"`
	}

	dsc := dirSyncControlValue{
		Flags: flags,
		Cookie: cookie,
		MaxAttrCount: maxAttrCount,
	}

	ctrlBytes, err := asn1.Marshal(dsc)
	if err != nil {
		return Control{}, fmt.Errorf("marshal dirsync control: %w", err)
	}

	return Control{
		ControlType: LDAP_CONTROL_DIRSYNC,
		Criticality: true,
		ControlValue: ctrlBytes,
	}, nil
}

func DecodeDirSyncControl(ctrl Control) (flags int, cookie []byte, err error) {
	if ctrl.ControlType != LDAP_CONTROL_DIRSYNC {
		return 0, nil, fmt.Errorf("not a dirsync control")
	}

	type dirSyncControlValue struct {
		Flags         int
		Cookie        []byte `asn1:"optional"`
		MaxAttrCount  int `asn1:"optional"`
	}

	var dsc dirSyncControlValue
	_, _ = asn1.Unmarshal(ctrl.ControlValue, &dsc)

	return dsc.Flags, dsc.Cookie, nil
}

func EncodePasswordPolicyControl() (Control, error) {
	return Control{
		ControlType: LDAP_CONTROL_PASSWORD_POLICY,
		Criticality: false,
		ControlValue: nil,
	}, nil
}

func DecodePasswordPolicyControl(ctrl Control) (expireWarning, graceLogin int, err error) {
	if ctrl.ControlType != LDAP_CONTROL_PASSWORD_POLICY {
		return 0, 0, fmt.Errorf("not a password policy control")
	}

	type ppResponse struct {
		Warning   int `asn1:"optional"`
		Grace     int `asn1:"optional"`
		Error     int `asn1:"optional"`
	}

	var resp ppResponse
	_, _ = asn1.Unmarshal(ctrl.ControlValue, &resp)

	return resp.Warning, resp.Grace, nil
}

func BuildControlSet(controls []Control) []Control {
	result := make([]Control, 0, len(controls))
	for _, c := range controls {
		if c.ControlValue == nil && c.ControlType != LDAP_CONTROL_PAGED_RESULTS &&
			c.ControlType != LDAP_CONTROL_SORT &&
			c.ControlType != LDAP_CONTROL_VLV &&
			c.ControlType != LDAP_CONTROL_RANGE &&
			c.ControlType != LDAP_CONTROL_DIRSYNC &&
			c.ControlType != LDAP_CONTROL_PASSWORD_POLICY {
			// Controls with no value and not needing encoding
			result = append(result, c)
		} else {
			result = append(result, c)
		}
	}
	return result
}

func ExtractControlValue(controls []Control, controlType string) ([]byte, bool) {
	for _, c := range controls {
		if c.ControlType == controlType {
			return c.ControlValue, true
		}
	}
	return nil, false
}

func HasControl(controls []Control, controlType string) bool {
	for _, c := range controls {
		if c.ControlType == controlType {
			return true
		}
	}
	return false
}