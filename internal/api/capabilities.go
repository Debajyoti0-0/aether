package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Capability names (Stage 3, T1). These are architectural capability
// identifiers checked per command — they extend the spine's AuthZ
// stage to remote operators; they are never permission bypasses.
const (
	CapExecAzure     = "exec.azure"
	CapExecAWS       = "exec.aws"
	CapExecGitHub    = "exec.github"
	CapExecGCP       = "exec.gcp"
	CapExecParallel  = "exec.parallel"
	CapSimulateStream = "simulate.stream"
	CapPluginsInstall = "plugins.install"
	CapPrtImport     = "prt.import"
	CapPivotCloud    = "pivot.cloud-to-onprem"
	CapRelay         = "relay"
	CapReadAudit     = "read.audit"
	CapReadEvents    = "read.events"
	CapReadGraph     = "read.graph"
	CapReadWorkspace = "read.workspace"
)

// DefaultOperatorCaps are granted to a freshly issued operator unless
// the admin overrides them: read-only visibility. Execute capabilities
// are granted explicitly per operator by the teamserver admin.
var DefaultOperatorCaps = CapabilitySet{
	CapReadAudit:     true,
	CapReadEvents:    true,
	CapReadGraph:     true,
	CapReadWorkspace: true,
}

// CapabilitySet is a set of granted capability identifiers.
type CapabilitySet map[string]bool

// Has reports whether the capability is granted (missing ⇒ deny).
func (c CapabilitySet) Has(cap string) bool {
	return c != nil && c[cap]
}

// capabilitiesFile is the on-disk shape of operators/<name>/capabilities.json.
type capabilitiesFile struct {
	Capabilities []string `json:"capabilities"`
}

// LoadOperatorCaps reads an operator's granted capabilities from
// <operatorsDir>/<name>/capabilities.json. A missing file yields the
// default read-only set (fail closed for execute capabilities).
func LoadOperatorCaps(operatorsDir, name string) (CapabilitySet, error) {
	if err := validateOperatorName(name); err != nil {
		return nil, err
	}
	caps := CapabilitySet{}
	for k, v := range DefaultOperatorCaps {
		caps[k] = v
	}

	path := filepath.Join(operatorsDir, name, "capabilities.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return caps, nil
	}
	if err != nil {
		return nil, err
	}
	var cf capabilitiesFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// The file REPLACES defaults for execute caps but read caps stay
	// unless explicitly denied with "-cap" entries.
	for _, cap := range cf.Capabilities {
		if strings2HasPrefixDash(cap) {
			delete(caps, cap[1:])
			continue
		}
		caps[cap] = true
	}
	return caps, nil
}

func strings2HasPrefixDash(s string) bool { return len(s) > 0 && s[0] == '-' }

// WriteOperatorCaps persists an operator's capability file (used by
// `serve cert issue --caps ...`).
func WriteOperatorCaps(operatorsDir, name string, caps []string) error {
	if err := validateOperatorName(name); err != nil {
		return err
	}
	dir := filepath.Join(operatorsDir, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(capabilitiesFile{Capabilities: caps}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "capabilities.json"), append(data, '\n'), 0o600)
}
