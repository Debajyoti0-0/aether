package acl

import (
	"context"
	"fmt"

	ldapengine "github.com/Debajyoti0-0/aether/internal/engine/ad/ldap"
	ldapproto "github.com/Debajyoti0-0/aether/internal/protocol/ldap"
)

type Engine struct {
	ldapEngine *ldapengine.Engine
}

type Principal struct {
	DN          string
	SID         *ldapproto.SID
	SamAccountName string
	DisplayName string
	ObjectClass string
}

type ACEInfo struct {
	Type        string
	Flags       string
	Mask        string
	SID         string
	ObjectType  string
	InheritedObjectType string
	IsInherited bool
	IsContainerInherit bool
	IsObjectInherit bool
	IsInheritOnly bool
	IsNoPropagate bool
}

type ACEAnalysis struct {
	ACEInfo
	Principal *Principal
	EffectiveRights []string
}

type ObjectACL struct {
	ObjectDN       string
	ObjectSID      *ldapproto.SID
	ObjectClass    string
	SecurityDescriptor *ldapproto.SecurityDescriptor
	Owner          *Principal
	Group          *Principal
	DACL           []ACEAnalysis
	SACL           []ACEAnalysis
}

type ACLPath struct {
	StartPrincipal *Principal
	EndObject      *Principal
	Path           []PathStep
	RiskScore      int
}

type PathStep struct {
	FromPrincipal *Principal
	ToObject      *Principal
	Relation      string
	Rights        []string
	ACE           ACEInfo
}

type EnumerateObjectsOptions struct {
	ObjectClass string
	Filter      string
	Attributes  []string
	SizeLimit   int
	BaseDN      string
}

func NewEngine(ldapEngine *ldapengine.Engine) *Engine {
	return &Engine{
		ldapEngine: ldapEngine,
	}
}

func (e *Engine) EnumerateUsers(ctx context.Context, opts EnumerateObjectsOptions) ([]*Principal, error) {
	if opts.Filter == "" {
		opts.Filter = "(&(objectClass=user)(!(objectClass=computer)))"
	}
	if opts.Attributes == nil {
		opts.Attributes = []string{"distinguishedName", "objectSid", "sAMAccountName", "displayName", "objectClass", "userAccountControl", "servicePrincipalName"}
	}
	if opts.SizeLimit == 0 {
		opts.SizeLimit = 1000
	}
	if opts.BaseDN == "" {
		return nil, fmt.Errorf("base DN required for enumeration")
	}

	result, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
		BaseDN:       opts.BaseDN,
		Scope:        ldapproto.LDAP_SCOPE_SUBTREE,
		Filter:       opts.Filter,
		Attributes:   opts.Attributes,
		SizeLimit:    opts.SizeLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}

	var principals []*Principal
	for _, entry := range result.Entries {
		p := &Principal{
			DN:              entry.GetFirstAttributeValue("distinguishedName"),
			SamAccountName:  entry.GetFirstAttributeValue("sAMAccountName"),
			DisplayName:     entry.GetFirstAttributeValue("displayName"),
			ObjectClass:     "user",
		}

		sidBytes := entry.GetAttributeRawValues("objectSid")
		if len(sidBytes) > 0 {
			sid, err := ldapproto.ParseSID(sidBytes[0])
			if err == nil {
				p.SID = sid
			}
		}

		principals = append(principals, p)
	}

	return principals, nil
}

func (e *Engine) EnumerateGroups(ctx context.Context, opts EnumerateObjectsOptions) ([]*Principal, error) {
	if opts.Filter == "" {
		opts.Filter = "(objectClass=group)"
	}
	if opts.Attributes == nil {
		opts.Attributes = []string{"distinguishedName", "objectSid", "sAMAccountName", "displayName", "objectClass", "member", "groupType"}
	}
	if opts.SizeLimit == 0 {
		opts.SizeLimit = 1000
	}
	if opts.BaseDN == "" {
		return nil, fmt.Errorf("base DN required for enumeration")
	}

	result, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
		BaseDN:       opts.BaseDN,
		Scope:        ldapproto.LDAP_SCOPE_SUBTREE,
		Filter:       opts.Filter,
		Attributes:   opts.Attributes,
		SizeLimit:    opts.SizeLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("search groups: %w", err)
	}

	var principals []*Principal
	for _, entry := range result.Entries {
		p := &Principal{
			DN:              entry.GetFirstAttributeValue("distinguishedName"),
			SamAccountName:  entry.GetFirstAttributeValue("sAMAccountName"),
			DisplayName:     entry.GetFirstAttributeValue("displayName"),
			ObjectClass:     "group",
		}

		sidBytes := entry.GetAttributeRawValues("objectSid")
		if len(sidBytes) > 0 {
			sid, err := ldapproto.ParseSID(sidBytes[0])
			if err == nil {
				p.SID = sid
			}
		}

		principals = append(principals, p)
	}

	return principals, nil
}

func (e *Engine) EnumerateComputers(ctx context.Context, opts EnumerateObjectsOptions) ([]*Principal, error) {
	if opts.Filter == "" {
		opts.Filter = "(objectClass=computer)"
	}
	if opts.Attributes == nil {
		opts.Attributes = []string{"distinguishedName", "objectSid", "sAMAccountName", "displayName", "objectClass", "operatingSystem", "operatingSystemVersion"}
	}
	if opts.SizeLimit == 0 {
		opts.SizeLimit = 1000
	}
	if opts.BaseDN == "" {
		return nil, fmt.Errorf("base DN required for enumeration")
	}

	result, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
		BaseDN:       opts.BaseDN,
		Scope:        ldapproto.LDAP_SCOPE_SUBTREE,
		Filter:       opts.Filter,
		Attributes:   opts.Attributes,
		SizeLimit:    opts.SizeLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("search computers: %w", err)
	}

	var principals []*Principal
	for _, entry := range result.Entries {
		p := &Principal{
			DN:              entry.GetFirstAttributeValue("distinguishedName"),
			SamAccountName:  entry.GetFirstAttributeValue("sAMAccountName"),
			DisplayName:     entry.GetFirstAttributeValue("displayName"),
			ObjectClass:     "computer",
		}

		sidBytes := entry.GetAttributeRawValues("objectSid")
		if len(sidBytes) > 0 {
			sid, err := ldapproto.ParseSID(sidBytes[0])
			if err == nil {
				p.SID = sid
			}
		}

		principals = append(principals, p)
	}

	return principals, nil
}

func (e *Engine) EnumerateOUs(ctx context.Context, opts EnumerateObjectsOptions) ([]*Principal, error) {
	if opts.Filter == "" {
		opts.Filter = "(objectClass=organizationalUnit)"
	}
	if opts.Attributes == nil {
		opts.Attributes = []string{"distinguishedName", "objectSid", "name", "objectClass"}
	}
	if opts.SizeLimit == 0 {
		opts.SizeLimit = 1000
	}
	if opts.BaseDN == "" {
		return nil, fmt.Errorf("base DN required for enumeration")
	}

	result, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
		BaseDN:       opts.BaseDN,
		Scope:        ldapproto.LDAP_SCOPE_SUBTREE,
		Filter:       opts.Filter,
		Attributes:   opts.Attributes,
		SizeLimit:    opts.SizeLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("search OUs: %w", err)
	}

	var principals []*Principal
	for _, entry := range result.Entries {
		p := &Principal{
			DN:              entry.GetFirstAttributeValue("distinguishedName"),
			SamAccountName:  entry.GetFirstAttributeValue("name"),
			DisplayName:     entry.GetFirstAttributeValue("name"),
			ObjectClass:     "organizationalUnit",
		}

		sidBytes := entry.GetAttributeRawValues("objectSid")
		if len(sidBytes) > 0 {
			sid, err := ldapproto.ParseSID(sidBytes[0])
			if err == nil {
				p.SID = sid
			}
		}

		principals = append(principals, p)
	}

	return principals, nil
}

func (e *Engine) GetObjectACL(ctx context.Context, objectDN string) (*ObjectACL, error) {
	result, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
		BaseDN:     objectDN,
		Scope:      ldapproto.LDAP_SCOPE_BASE,
		Filter:     "(objectClass=*)",
		Attributes: []string{"nTSecurityDescriptor", "distinguishedName", "objectSid", "objectClass", "owner", "group"},
		SizeLimit:  1,
	})
	if err != nil {
		return nil, fmt.Errorf("search object ACL: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("object not found: %s", objectDN)
	}

	entry := result.Entries[0]
	sdBytes := entry.GetAttributeRawValues("nTSecurityDescriptor")
	if len(sdBytes) == 0 {
		return nil, fmt.Errorf("no security descriptor on object: %s", objectDN)
	}

	sd, err := ldapproto.ParseSecurityDescriptor(sdBytes[0])
	if err != nil {
		return nil, fmt.Errorf("parse security descriptor: %w", err)
	}

	objectDN = entry.GetFirstAttributeValue("distinguishedName")
	var objSID *ldapproto.SID
	sidBytes := entry.GetAttributeRawValues("objectSid")
	if len(sidBytes) > 0 {
		sid, _ := ldapproto.ParseSID(sidBytes[0])
		objSID = sid
	}

	objClass := entry.GetFirstAttributeValue("objectClass")

	owner := &Principal{}
	if sd.Owner != nil {
		owner.SID = sd.Owner
	}
	group := &Principal{}
	if sd.Group != nil {
		group.SID = sd.Group
	}

	daclAnalysis := make([]ACEAnalysis, 0)
	if sd.Dacl != nil {
		for _, ace := range sd.Dacl.ACEs {
			analysis := e.analyzeACE(ace)
			daclAnalysis = append(daclAnalysis, analysis)
		}
	}

	saclAnalysis := make([]ACEAnalysis, 0)
	if sd.Sacl != nil {
		for _, ace := range sd.Sacl.ACEs {
			analysis := e.analyzeACE(ace)
			saclAnalysis = append(saclAnalysis, analysis)
		}
	}

	return &ObjectACL{
		ObjectDN:          objectDN,
		ObjectSID:         objSID,
		ObjectClass:       objClass,
		SecurityDescriptor: sd,
		Owner:             owner,
		Group:             group,
		DACL:              daclAnalysis,
		SACL:              saclAnalysis,
	}, nil
}

func (e *Engine) analyzeACE(ace ldapproto.ACE) ACEAnalysis {
	aceInfo := ACEInfo{
		Type:        ace.TypeString(),
		Flags:       ace.AccessMaskString(),
		Mask:        fmt.Sprintf("0x%08x", ace.Mask),
		SID:         ace.SID.String(),
		ObjectType:  "",
		InheritedObjectType: "",
		IsInherited: ace.IsInherited(),
		IsContainerInherit: ace.IsContainerInherit(),
		IsObjectInherit: ace.IsObjectInherit(),
		IsInheritOnly: ace.IsInheritOnly(),
		IsNoPropagate: ace.IsNoPropagate(),
	}

	if ace.ObjectType != nil && !ace.ObjectType.IsZero() {
		aceInfo.ObjectType = ace.ObjectType.String()
	}
	if ace.InheritedObjectType != nil && !ace.InheritedObjectType.IsZero() {
		aceInfo.InheritedObjectType = ace.InheritedObjectType.String()
	}

	var rights []string
	mask := ace.Mask
	if mask&ldapproto.RIGHT_GENERIC_ALL != 0 { rights = append(rights, "GENERIC_ALL") }
	if mask&ldapproto.RIGHT_GENERIC_READ != 0 { rights = append(rights, "GENERIC_READ") }
	if mask&ldapproto.RIGHT_GENERIC_WRITE != 0 { rights = append(rights, "GENERIC_WRITE") }
	if mask&ldapproto.RIGHT_GENERIC_EXECUTE != 0 { rights = append(rights, "GENERIC_EXECUTE") }
	if mask&ldapproto.RIGHT_GENERIC_ALL != 0 { rights = append(rights, "GENERIC_ALL") }
	if mask&ldapproto.RIGHT_MAXIMUM_ALLOWED != 0 { rights = append(rights, "MAXIMUM_ALLOWED") }
	if mask&ldapproto.RIGHT_ACCESS_SYSTEM_SECURITY != 0 { rights = append(rights, "ACCESS_SYSTEM_SECURITY") }
	if mask&ldapproto.RIGHT_SYNCHRONIZE != 0 { rights = append(rights, "SYNCHRONIZE") }
	if mask&ldapproto.RIGHT_WRITE_OWNER != 0 { rights = append(rights, "WRITE_OWNER") }
	if mask&ldapproto.RIGHT_WRITE_DAC != 0 { rights = append(rights, "WRITE_DAC") }
	if mask&ldapproto.RIGHT_READ_CONTROL != 0 { rights = append(rights, "READ_CONTROL") }
	if mask&ldapproto.RIGHT_DELETE != 0 { rights = append(rights, "DELETE") }
	if mask&ldapproto.RIGHT_CREATE_CHILD != 0 { rights = append(rights, "CREATE_CHILD") }
	if mask&ldapproto.RIGHT_DELETE_CHILD != 0 { rights = append(rights, "DELETE_CHILD") }
	if mask&ldapproto.RIGHT_LIST_CHILDREN != 0 { rights = append(rights, "LIST_CHILDREN") }
	if mask&ldapproto.RIGHT_SELF != 0 { rights = append(rights, "SELF") }
	if mask&ldapproto.RIGHT_READ_PROPERTY != 0 { rights = append(rights, "READ_PROPERTY") }
	if mask&ldapproto.RIGHT_WRITE_PROPERTY != 0 { rights = append(rights, "WRITE_PROPERTY") }
	if mask&ldapproto.RIGHT_DELETE_TREE != 0 { rights = append(rights, "DELETE_TREE") }
	if mask&ldapproto.RIGHT_LIST_OBJECT != 0 { rights = append(rights, "LIST_OBJECT") }
	if mask&ldapproto.RIGHT_CONTROL_ACCESS != 0 { rights = append(rights, "CONTROL_ACCESS") }

	return ACEAnalysis{
		ACEInfo: aceInfo,
		Principal: &Principal{
			SID: &ldapproto.SID{}, // Will be resolved later
		},
		EffectiveRights: rights,
	}
}

func (e *Engine) FindACLPaths(ctx context.Context, startPrincipal *Principal, targetObjectDN string, maxDepth int) ([]ACLPath, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if startPrincipal == nil || startPrincipal.SID == nil {
		return nil, fmt.Errorf("start principal with SID required")
	}

	targetACL, err := e.GetObjectACL(ctx, targetObjectDN)
	if err != nil {
		return nil, fmt.Errorf("get target ACL: %w", err)
	}

	// Resolve the start principal's SID set: its own SID plus every group
	// it is a member of (memberOf + primaryGroupID). ACLs grant access to
	// group SIDs, so a path exists only through principals the start SID
	// actually holds. The previous implementation ignored startPrincipal
	// entirely and reported every allow-ACE on the target as a path —
	// a false path for any principal lacking those grants.
	startSIDs, err := e.ResolvePrincipalSIDSet(ctx, startPrincipal)
	if err != nil {
		return nil, fmt.Errorf("resolve start principal: %w", err)
	}
	startSIDStrs := make(map[string]bool, len(startSIDs))
	for _, sid := range startSIDs {
		startSIDStrs[sid.String()] = true
	}

	var paths []ACLPath

	for _, aceAnalysis := range targetACL.DACL {
		if aceAnalysis.Type != "ACCESS_ALLOWED" && aceAnalysis.Type != "ACCESS_ALLOWED_OBJECT" {
			continue
		}
		// Edge exists only if the ACE grants to the start principal or one of
		// its groups. No matching grant → no path (never fabricate one).
		if !startSIDStrs[aceAnalysis.SID] {
			continue
		}
		risk := e.calculateRiskScore(aceAnalysis.EffectiveRights)
		if risk > 0 {
			path := ACLPath{
				StartPrincipal: startPrincipal,
				EndObject: &Principal{
					DN: targetACL.ObjectDN,
					SID: targetACL.ObjectSID,
				},
				Path: []PathStep{
					{
						FromPrincipal: startPrincipal,
						ToObject: &Principal{
							DN: targetACL.ObjectDN,
							SID: targetACL.ObjectSID,
						},
						Relation: "direct_ace",
						Rights:   aceAnalysis.EffectiveRights,
						ACE:      aceAnalysis.ACEInfo,
					},
				},
				RiskScore: risk,
			}
			paths = append(paths, path)
		}
	}

	return paths, nil
}

// ResolvePrincipalSIDSet returns the principal's own SID plus the SIDs of
// every group it belongs to (memberOf and primaryGroupID).
func (e *Engine) ResolvePrincipalSIDSet(ctx context.Context, p *Principal) ([]*ldapproto.SID, error) {
	if p == nil || p.SID == nil {
		return nil, fmt.Errorf("principal SID required")
	}
	start := p.DN
	if start == "" {
		// Resolve the DN for the given SID so memberOf can be queried.
		result, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
			Scope:      ldapproto.LDAP_SCOPE_SUBTREE,
			Filter:     fmt.Sprintf("(objectSid=%s)", p.SID.String()),
			Attributes: []string{"distinguishedName", "memberOf", "primaryGroupID", "objectSid"},
			SizeLimit:  1,
		})
		if err != nil {
			return nil, err
		}
		if len(result.Entries) == 0 {
			return nil, fmt.Errorf("start principal not found in directory: %s", p.SID.String())
		}
		start = result.Entries[0].GetFirstAttributeValue("distinguishedName")
		p.DN = start
		if p.SamAccountName == "" {
			p.SamAccountName = result.Entries[0].GetFirstAttributeValue("name")
		}
	}

	sids := []*ldapproto.SID{p.SID}
	seen := map[string]bool{p.SID.String(): true}

	result, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
		BaseDN:     start,
		Scope:      ldapproto.LDAP_SCOPE_BASE,
		Filter:     "(objectClass=*)",
		Attributes: []string{"memberOf", "primaryGroupID", "objectSid"},
		SizeLimit:  1,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Entries) == 0 {
		return sids, nil
	}
	entry := result.Entries[0]
	for _, gdn := range entry.GetAttributeValues("memberOf") {
		grp, err := e.ldapEngine.Search(ctx, ldapengine.SearchOptions{
			BaseDN:     gdn,
			Scope:      ldapproto.LDAP_SCOPE_BASE,
			Filter:     "(objectClass=*)",
			Attributes: []string{"objectSid"},
			SizeLimit:  1,
		})
		if err != nil || len(grp.Entries) == 0 {
			continue
		}
		gRaw := grp.Entries[0].GetAttributeRawValues("objectSid")
		if len(gRaw) == 0 {
			continue
		}
		gsid, err := ldapproto.ParseSID(gRaw[0])
		if err != nil {
			continue
		}
		if !seen[gsid.String()] {
			seen[gsid.String()] = true
			sids = append(sids, gsid)
		}
	}
	// Primary group (e.g. Domain Users 513) — count it as held.
	if pgRaw := entry.GetFirstAttributeValue("primaryGroupID"); pgRaw != "" {
		var rid uint32
		if _, err := fmt.Sscanf(pgRaw, "%d", &rid); err == nil && len(p.SID.SubAuthority) > 0 {
			pg := &ldapproto.SID{
				Revision:          p.SID.Revision,
				IdentifierAuthority: p.SID.IdentifierAuthority,
				SubAuthorityCount: p.SID.SubAuthorityCount,
				SubAuthority:      append(append([]uint32{}, p.SID.SubAuthority[:len(p.SID.SubAuthority)-1]...), rid),
			}
			if !seen[pg.String()] {
				seen[pg.String()] = true
				sids = append(sids, pg)
			}
		}
	}
	return sids, nil
}

func (e *Engine) calculateRiskScore(rights []string) int {
	score := 0
	for _, r := range rights {
		switch r {
		case "GENERIC_ALL":
			score += 10
		case "WRITE_DAC", "WRITE_OWNER":
			score += 8
		case "GENERIC_WRITE":
			score += 6
		case "GENERIC_READ", "READ_CONTROL":
			score += 2
		case "CREATE_CHILD", "DELETE_CHILD", "DELETE_TREE":
			score += 6
		case "WRITE_PROPERTY":
			score += 5
		case "CONTROL_ACCESS":
			score += 5
		case "SELF", "READ_PROPERTY", "LIST_CHILDREN", "LIST_OBJECT":
			score += 1
		}
	}
	return score
}

func (e *Engine) GetEffectiveRights(ctx context.Context, principalSID *ldapproto.SID, targetDN string) ([]string, error) {
	targetACL, err := e.GetObjectACL(ctx, targetDN)
	if err != nil {
		return nil, err
	}

	var effectiveRights []string
	principalSIDStr := principalSID.String()

	for _, aceAnalysis := range targetACL.DACL {
		if aceAnalysis.SID == principalSIDStr {
			effectiveRights = append(effectiveRights, aceAnalysis.EffectiveRights...)
		}
		// TODO: Check group membership recursively
	}

	return effectiveRights, nil
}
