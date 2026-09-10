package types

// AttackPath represents a path from a BloodHound JSON export.
type AttackPath struct {
	Nodes []PathNode `json:"nodes"`
	Edges []PathEdge `json:"edges"`
}

// PathNode is a single object (user, group, computer, OU) in a path.
type PathNode struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Properties map[string]string `json:"properties,omitempty"`
}

// PathEdge is a directed relationship between two nodes.
type PathEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// StepResult is the outcome of validating one edge in a path.
type StepResult struct {
	StepIndex int    `json:"step_index"`
	Edge      string `json:"edge"` // "source -> TYPE -> target"
	Valid     bool   `json:"valid"`
	RiskScore int    `json:"risk_score"`
	Reason    string `json:"reason,omitempty"`
}

// ValidationResult aggregates step results for a full path.
type ValidationResult struct {
	Steps       []StepResult `json:"steps"`
	OverallRisk int          `json:"overall_risk"`
	IsValid     bool         `json:"is_valid"`
}
