package planner

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

func timeNowUnixNano() int64 { return time.Now().UnixNano() }

// Episode is one recorded engagement rollout: a sequence of
// (state, action, reward) transitions, exportable as JSONL for
// offline training.
type Episode struct {
	ID        string       `json:"id"`
	Workspace string       `json:"workspace"`
	StartedAt time.Time    `json:"started_at"`
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
	path string
}

// NewEpisodeStore opens (or creates) an episode store.
func NewEpisodeStore(path string) *EpisodeStore {
	return &EpisodeStore{path: path}
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
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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
