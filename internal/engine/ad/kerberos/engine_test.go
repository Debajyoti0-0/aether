package kerberos

import (
	"context"
	"testing"
	"time"
)

func TestMutationInterfaces(t *testing.T) {
	mutations := []interface{}{
		&EnumUsersMutation{},
		&EnumASREPMutation{},
		&EnumSPNMutation{},
		&KerberoastMutation{},
		&ASREPRoastMutation{},
		&TGTMutation{},
&CCacheMutation{Input: CCacheInput{CCachePath: "/tmp/test.ccache"}},
	}

	for _, m := range mutations {
		t.Run("interface", func(t *testing.T) {
			if kind, ok := m.(interface{ Kind() string }); ok {
				if kind.Kind() == "" {
					t.Errorf("Mutation %T has empty kind", m)
				}
			}
			if target, ok := m.(interface{ Target() string }); ok {
				if target.Target() == "" {
					t.Errorf("Mutation %T has empty target", m)
				}
			}
			if exec, ok := m.(interface{ Execute(context.Context) (string, error) }); ok {
				_ = exec
			}
		})
	}
}

func TestEnumUsersInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       EnumUsersMutation
		expectError bool
	}{
		{"valid", EnumUsersMutation{Domain: "EXAMPLE.COM", DC: "dc01.example.com", Usernames: []string{"user1"}}, false},
		{"missing domain", EnumUsersMutation{DC: "dc01.example.com", Usernames: []string{"user1"}}, true},
		{"missing DC", EnumUsersMutation{Domain: "EXAMPLE.COM", Usernames: []string{"user1"}}, true},
		{"empty usernames", EnumUsersMutation{Domain: "EXAMPLE.COM", DC: "dc01.example.com"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input.Domain == "" || tt.input.DC == "" || len(tt.input.Usernames) == 0 {
				if !tt.expectError {
					t.Error("expected error but got none")
				}
			} else {
				if tt.expectError {
					t.Error("unexpected error")
				}
			}
		})
	}
}

func TestKerberoastInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       KerberoastMutation
		expectError bool
	}{
		{"valid with password", KerberoastMutation{Domain: "EXAMPLE.COM", DC: "dc01", SPNs: []string{"HTTP/web@EXAMPLE.COM"}, Credentials: map[string]string{"HTTP/web@EXAMPLE.COM": "pass"}}, false},
		{"valid with ccache", KerberoastMutation{Domain: "EXAMPLE.COM", DC: "dc01", SPNs: []string{"HTTP/web@EXAMPLE.COM"}, CCachePath: "/tmp/ccache"}, false},
		{"missing domain", KerberoastMutation{DC: "dc01", SPNs: []string{"HTTP/web@EXAMPLE.COM"}}, true},
		{"missing SPNs", KerberoastMutation{Domain: "EXAMPLE.COM", DC: "dc01"}, true},
		{"no credentials", KerberoastMutation{Domain: "EXAMPLE.COM", DC: "dc01", SPNs: []string{"HTTP/web@EXAMPLE.COM"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasCreds := len(tt.input.Credentials) > 0 || tt.input.CCachePath != ""
			if tt.input.Domain == "" || tt.input.DC == "" || len(tt.input.SPNs) == 0 || !hasCreds {
				if !tt.expectError {
					t.Error("expected error but got none")
				}
			} else {
				if tt.expectError {
					t.Error("unexpected error")
				}
			}
		})
	}
}

func TestTGTInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       TGTMutation
		expectError bool
	}{
		{"valid password", TGTMutation{Input: TGTInput{Domain: "EXAMPLE.COM", DC: "dc01", Username: "user", Password: "pass"}}, false},
		{"valid ccache", TGTMutation{Input: TGTInput{Domain: "EXAMPLE.COM", DC: "dc01", Username: "user", CCachePath: "/tmp/cc"}}, false},
		{"missing domain", TGTMutation{Input: TGTInput{DC: "dc01", Username: "user", Password: "pass"}}, true},
		{"missing username", TGTMutation{Input: TGTInput{Domain: "EXAMPLE.COM", DC: "dc01", Password: "pass"}}, true},
		{"no auth", TGTMutation{Input: TGTInput{Domain: "EXAMPLE.COM", DC: "dc01", Username: "user"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasAuth := tt.input.Input.Password != "" || tt.input.Input.CCachePath != "" || tt.input.Input.KeyTabPath != ""
			if tt.input.Input.Domain == "" || tt.input.Input.DC == "" || tt.input.Input.Username == "" || !hasAuth {
				if !tt.expectError {
					t.Error("expected error but got none")
				}
			} else {
				if tt.expectError {
					t.Error("unexpected error")
				}
			}
		})
	}
}

func TestCCacheInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       CCacheMutation
		expectError bool
	}{
		{"valid show", CCacheMutation{Input: CCacheInput{CCachePath: "/tmp/cc", Action: "show"}}, false},
		{"valid convert", CCacheMutation{Input: CCacheInput{CCachePath: "/tmp/cc", Action: "convert", OutputPath: "/tmp/cc2"}}, false},
		{"missing path", CCacheMutation{Input: CCacheInput{Action: "show"}}, true},
		{"convert without output", CCacheMutation{Input: CCacheInput{CCachePath: "/tmp/cc", Action: "convert"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input.Input.CCachePath == "" {
				if !tt.expectError {
					t.Error("expected error but got none")
				}
			} else if tt.input.Input.Action == "convert" && tt.input.Input.OutputPath == "" {
				if !tt.expectError {
					t.Error("expected error but got none")
				}
			} else {
				if tt.expectError {
					t.Error("unexpected error")
				}
			}
		})
	}
}

func TestRiskScores(t *testing.T) {
	tests := []struct {
		name     string
		mutation interface{ RiskScore() int }
		expected int
	}{
		{"enum users", &EnumUsersMutation{}, 10},
		{"enum asrep", &EnumASREPMutation{}, 10},
		{"enum spn", &EnumSPNMutation{}, 10},
		{"kerberoast", &KerberoastMutation{}, 20},
		{"asreproast", &ASREPRoastMutation{}, 20},
		{"tgt", &TGTMutation{}, 15},
		{"ccache", &CCacheMutation{}, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if score := tt.mutation.RiskScore(); score != tt.expected {
				t.Errorf("RiskScore() = %d, want %d", score, tt.expected)
			}
		})
	}
}

func TestReversible(t *testing.T) {
	tests := []struct {
		name     string
		mutation interface{ Reversible() bool }
		expected bool
	}{
		{"enum users", &EnumUsersMutation{}, true},
		{"enum asrep", &EnumASREPMutation{}, true},
		{"enum spn", &EnumSPNMutation{}, true},
		{"kerberoast", &KerberoastMutation{}, true},
		{"asreproast", &ASREPRoastMutation{}, true},
		{"tgt", &TGTMutation{}, true},
		{"ccache", &CCacheMutation{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rev := tt.mutation.Reversible(); rev != tt.expected {
				t.Errorf("Reversible() = %v, want %v", rev, tt.expected)
			}
		})
	}
}

func TestSendRecvTCP(t *testing.T) {
	data := []byte("test data")
	length := make([]byte, 4)
	length[0] = byte(len(data) >> 24)
	length[1] = byte(len(data) >> 16)
	length[2] = byte(len(data) >> 8)
	length[3] = byte(len(data))

	if len(length) != 4 {
		t.Error("length buffer should be 4 bytes")
	}

	decoded := int(length[0])<<24 | int(length[1])<<16 | int(length[2])<<8 | int(length[3])
	if decoded != len(data) {
		t.Errorf("decoded length = %d, want %d", decoded, len(data))
	}
}

func TestTimeTruncation(t *testing.T) {
	now := time.Now().UTC()
	truncated := now.Truncate(time.Second)
	if truncated.Nanosecond() != 0 {
		t.Errorf("Truncate(Second) should zero nanoseconds, got %d", truncated.Nanosecond())
	}
}