package types

// RiskInput captures environmental factors that affect OPSEC risk.
type RiskInput struct {
	EDRDetected    bool   `json:"edr_detected"`
	SIEMLogging    bool   `json:"siem_logging"`
	BlueTeamActive bool   `json:"blue_team_active"`
	TimeOfDay      string `json:"time_of_day"` // business_hours, off_hours
}

// CommandResult is the output of a remote command execution.
type CommandResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Status   string `json:"status,omitempty"`
}
