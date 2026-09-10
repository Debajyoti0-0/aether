package rl

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/store"
)

func timeNowUnixNano() int64 { return time.Now().UnixNano() }

// Episode is one recorded engagement rollout: a sequence of
// (state, action, reward) transitions, exportable as JSONL for
// offline training.
type Episode struct {
	ID        string        `json:"id"`
	Workspace string        `json:"workspace"`
	StartedAt time.Time     `json:"started_at"`
	Steps     []EpisodeStep `json:"steps"`
}

// EpisodeStep is one transition within an episode.
type EpisodeStep struct {
	StateKey  string  `json:"state"`
	ActionKey string  `json:"action"`
	Reward    float64 `json:"reward"`
	NextState string  `json:"next_state"`
}

// EpisodeStore persists episodes as JSONL (one episode per line).
type EpisodeStore struct {
	path  string
	bytes []byte
}

// NewEpisodeStore opens (or creates) an episode store.
func NewEpisodeStore(path string) *EpisodeStore {
	return &EpisodeStore{path: path}
}

// NewEpisodeStoreBytes wraps in-memory JSONL (legacy migration source).
func NewEpisodeStoreBytes(data []byte) *EpisodeStore {
	return &EpisodeStore{bytes: data}
}

// Append writes an episode.
func (s *EpisodeStore) Append(e Episode) error {
	if e.ID == "" {
		e.ID = fmt.Sprintf("ep-%d", time.Now().UnixNano())
	}
	if e.StartedAt.IsZero() {
		e.StartedAt = time.Now().UTC()
	}

	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// Load reads all episodes.
func (s *EpisodeStore) Load() ([]Episode, error) {
	var data []byte
	if s.bytes != nil {
		data = s.bytes
	} else {
		raw, err := os.ReadFile(s.path)
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		data = raw
	}

	var out []Episode
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		var e Episode
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("corrupt episode: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// BuildEpisode converts a workspace journal into a training episode by
// mapping recorded events to (state, action, reward) transitions.
// Token acquisition and command executions earn positive rewards;
// failures and SOC hits earn negative ones.
func BuildEpisode(workspaceID string, events []JournalEvent) Episode {
	e := Episode{Workspace: workspaceID, StartedAt: time.Now().UTC()}

	state := State{
		TokenBucket:   0,
		GraphDensity:  "low",
		CAPStrictness: "medium",
		Phase:         "recon",
		WorkspaceID:   workspaceID,
	}

	for _, ev := range events {
		actionKey := mapEventToAction(ev.Kind)
		action, ok := ActionByKey(actionKey)
		if !ok {
			continue
		}

		success := !strings.Contains(strings.ToLower(ev.Detail), "fail")
		detected := strings.Contains(strings.ToLower(ev.Kind), "detect")

		next, reward, done := Step(state, action, success, detected)

		e.Steps = append(e.Steps, EpisodeStep{
			StateKey:  state.Key(),
			ActionKey: action.Command,
			Reward:    reward,
			NextState: next.Key(),
		})

		state = next
		if done {
			break
		}
	}
	return e
}

// JournalEvent is the subset of workspace events used for episodes.
type JournalEvent struct {
	Kind   string
	Detail string
}

// mapEventToAction maps journal event kinds onto catalog actions.
func mapEventToAction(kind string) string {
	switch strings.ToLower(kind) {
	case "graph_built", "path_validated":
		return "graph build"
	case "cap_evaluated", "policy_export":
		return "cap evaluate"
	case "prt_converted", "prt_imported":
		return "prt convert"
	case "token_protected", "tls_binding_imported":
		return "token protect"
	case "cae_handled":
		return "relay cae-handler"
	case "fido2_downgraded":
		return "relay fido2-downgrade"
	case "tgt_extracted":
		return "pivot cloud-to-onprem"
	case "command_executed", "remote_command":
		return "exec azure"
	default:
		return "cap evaluate"
	}
}

// VaultEpisodeStore persists episodes inside the workspace vault
// (Stage 3, T5) under the planner_episodes record bucket, keyed by
// episode ID — no plaintext planner JSONL on disk. The JSONL
// EpisodeStore remains available as an explicit interchange format
// (plan export --output), but the vault is the default destination.
type VaultEpisodeStore struct {
	v *store.Vault
}

// NewVaultEpisodeStore opens the workspace-vault episode store.
func NewVaultEpisodeStore(v *store.Vault) *VaultEpisodeStore {
	return &VaultEpisodeStore{v: v}
}

const bucketPlannerEpisodes = "planner_episodes"

// Append writes an episode into the vault.
func (s *VaultEpisodeStore) Append(e Episode) error {
	if e.ID == "" {
		e.ID = fmt.Sprintf("ep-%d", time.Now().UnixNano())
	}
	if e.StartedAt.IsZero() {
		e.StartedAt = time.Now().UTC()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.v.PutRecord(bucketPlannerEpisodes, e.ID, data)
}

// Load reads all episodes from the vault (sorted by ID for
// deterministic training input ordering).
func (s *VaultEpisodeStore) Load() ([]Episode, error) {
	keys, err := s.v.ListRecords(bucketPlannerEpisodes)
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	var out []Episode
	for _, k := range keys {
		data, err := s.v.GetRecord(bucketPlannerEpisodes, k)
		if err != nil {
			return nil, err
		}
		var e Episode
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("corrupt episode %s: %w", k, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// Count returns the number of stored episodes.
func (s *VaultEpisodeStore) Count() (int, error) {
	keys, err := s.v.ListRecords(bucketPlannerEpisodes)
	if err != nil {
		return 0, err
	}
	return len(keys), nil
}
