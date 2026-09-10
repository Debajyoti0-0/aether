package types

// ConditionalAccessPolicy represents an Entra ID Conditional Access policy.
type ConditionalAccessPolicy struct {
	ID            string         `json:"id"`
	DisplayName   string         `json:"displayName"`
	State         string         `json:"state"` // enabled, disabled, enabledForReportingButNotEnforced
	Conditions    Conditions     `json:"conditions"`
	GrantControls GrantControls  `json:"grantControls"`
}

// Conditions bundles all condition blocks of a CAP.
type Conditions struct {
	Applications ApplicationCondition `json:"applications"`
	Users        UserCondition        `json:"users"`
	Locations    LocationCondition    `json:"locations"`
	Platforms    PlatformCondition    `json:"platforms"`
	ClientApps   ClientAppCondition   `json:"clientApps"`
	DeviceStates DeviceStateCondition `json:"deviceStates"`
	SignInRisk   []string             `json:"signInRisk,omitempty"`
	UserRisk     []string             `json:"userRisk,omitempty"`
}

// ApplicationCondition filters which apps a policy applies to.
type ApplicationCondition struct {
	IncludeApplications []string `json:"includeApplications"`
	ExcludeApplications []string `json:"excludeApplications"`
	IncludeUserActions  []string `json:"includeUserActions,omitempty"`
}

// UserCondition filters which users/groups a policy applies to.
type UserCondition struct {
	IncludeUsers []string `json:"includeUsers"`
	ExcludeUsers []string `json:"excludeUsers"`
	IncludeGroups []string `json:"includeGroups,omitempty"`
	ExcludeGroups []string `json:"excludeGroups,omitempty"`
	IncludeRoles  []string `json:"includeRoles,omitempty"`
	ExcludeRoles  []string `json:"excludeRoles,omitempty"`
}

// LocationCondition filters named/trusted locations.
type LocationCondition struct {
	IncludeLocations []string `json:"includeLocations"`
	ExcludeLocations []string `json:"excludeLocations"`
}

// PlatformCondition filters device platforms.
type PlatformCondition struct {
	IncludePlatforms []string `json:"includePlatforms"`
	ExcludePlatforms []string `json:"excludePlatforms"`
}

// ClientAppCondition filters client application types.
type ClientAppCondition struct {
	IncludeClientApps []string `json:"includeClientApps"`
	ExcludeClientApps []string `json:"excludeClientApps,omitempty"`
}

// DeviceStateCondition filters device compliance state.
type DeviceStateCondition struct {
	IncludeDeviceStates []string `json:"includeDeviceStates"`
	ExcludeDeviceStates []string `json:"excludeDeviceStates,omitempty"`
}

// GrantControls defines what must be satisfied to grant access.
type GrantControls struct {
	Operator        string   `json:"operator"` // AND, OR
	BuiltInControls []string `json:"builtInControls"`
	CustomControls  []string `json:"customControls,omitempty"`
}

// BypassStrategy tells the operator what to spoof to satisfy policy.
type BypassStrategy struct {
	RequiredOS               string   `json:"required_os"`
	RequiredLocation         string   `json:"required_location"`
	RequiredBrowser          string   `json:"required_browser"`
	RequiredDeviceCompliance bool     `json:"required_device_compliance"`
	RequiresMFA              bool     `json:"requires_mfa"`
	RiskLevel                int      `json:"risk_level"`
	SpoofingRecommendations  []string `json:"spoofing_recommendations"`
}
