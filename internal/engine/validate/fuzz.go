package validate

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// fuzzRNG is swappable for deterministic tests.
var fuzzRNG = rand.New(rand.NewSource(time.Now().UnixNano()))

// randNew builds a seeded RNG (tests).
func randNew(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

// NewFuzzForStream returns a fresh unseeded RNG (public stream entry).
func NewFuzzForStream() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }

// FuzzVariant is one mutated telemetry record for detection-rule
// robustness testing.
type FuzzVariant struct {
	Mutations []string          `json:"mutations"`
	Record    map[string]any    `json:"record"`
}

// FuzzTelemetry generates mutated variants of a base telemetry record
// (e.g., a Sentinel/Defender event) so purple teams can verify their
// detection rules fire across realistic noise: timestamp skew, field
// casing, injected fields, missing fields, and encoding tricks.
func FuzzTelemetry(base map[string]any, count int) []FuzzVariant {
	if count < 1 {
		count = 10
	}
	if count > 100 {
		count = 100
	}

	variants := make([]FuzzVariant, 0, count)
	for i := 0; i < count; i++ {
		mutated := copyMap(base)
		var mutations []string

		// Each variant applies 1-4 random mutations.
		nMutations := 1 + fuzzRNG.Intn(4)
		for m := 0; m < nMutations; m++ {
			switch fuzzRNG.Intn(7) {
			case 0:
				if ts, ok := mutated["time"].(string); ok {
					mutated["time"] = skewTimestamp(ts)
					mutations = append(mutations, "timestamp_skew")
				}
			case 1:
				mutated = mutateKeyCasing(mutated)
				mutations = append(mutations, "key_casing")
			case 2:
				mutated[fuzzFieldName()] = fuzzValue()
				mutations = append(mutations, "extra_field")
			case 3:
				if target := pickKey(mutated); target != "" {
					delete(mutated, target)
					mutations = append(mutations, "missing_field:"+target)
				}
			case 4:
				if v, ok := pickString(mutated); ok {
					mutated[pickKey(mutated)] = unicodeEscape(v)
					mutations = append(mutations, "unicode_escape")
				}
			case 5:
				mutated["AdditionalDetails"] = map[string]any{
					"injected": true,
					"seq":      i,
				}
				mutations = append(mutations, "nested_injection")
			case 6:
				if v, ok := pickString(mutated); ok {
					mutated[pickKey(mutated)] = strings.ReplaceAll(v, " ", "\u00a0")
					mutations = append(mutations, "nbsp_substitution")
				}
			}
		}

		variants = append(variants, FuzzVariant{Mutations: mutations, Record: mutated})
	}
	return variants
}

// skewTimestamp shifts a timestamp by minutes-hours in either direction.
func skewTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	offset := time.Duration(fuzzRNG.Intn(240)+1) * time.Minute * time.Duration(sign())
	return t.Add(offset).Format(time.RFC3339)
}

func sign() int {
	if fuzzRNG.Intn(2) == 0 {
		return -1
	}
	return 1
}

// mutateKeyCasing upper/lowercases one key.
func mutateKeyCasing(m map[string]any) map[string]any {
	key := pickKey(m)
	if key == "" {
		return m
	}
	out := copyMap(m)
	delete(out, key)
	if fuzzRNG.Intn(2) == 0 {
		out[strings.ToUpper(key)] = m[key]
	} else {
		out[strings.ToLower(key)] = m[key]
	}
	return out
}

func fuzzFieldName() string {
	fields := []string{"ClientIP", "correlationId", "UserAgent", "RiskLevel", "AppId", "RawData"}
	return fields[fuzzRNG.Intn(len(fields))]
}

func fuzzValue() any {
	vals := []any{nil, 0, "", "N/A", false, fuzzRNG.Intn(1000)}
	return vals[fuzzRNG.Intn(len(vals))]
}

func pickKey(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	// Sorted keys keep seeded fuzzing reproducible.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys[fuzzRNG.Intn(len(keys))]
}

func pickString(m map[string]any) (string, bool) {
	for tries := 0; tries < 5; tries++ {
		if v, ok := m[pickKey(m)].(string); ok && v != "" {
			return v, true
		}
	}
	return "", false
}

func unicodeEscape(s string) string {
	// Encode the first character as its \uXXXX escape.
	if s == "" {
		return s
	}
	return fmt.Sprintf("\\u%04x%s", s[0], s[1:])
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RenderFuzzSummary prints a compact summary of a fuzz batch.
func RenderFuzzSummary(variants []FuzzVariant) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Telemetry Fuzz Batch: %d variants ===\n", len(variants))

	// Mutation frequency histogram.
	freq := map[string]int{}
	for _, v := range variants {
		for _, m := range v.Mutations {
			freq[strings.SplitN(m, ":", 2)[0]]++
		}
	}
	for m, n := range freq {
		fmt.Fprintf(&b, "  %-22s %d\n", m+":", n)
	}
	return b.String()
}

// FuzzJSON renders the batch as NDJSON (one variant per line) suitable
// for SIEM replay pipelines.
func FuzzJSON(variants []FuzzVariant) (string, error) {
	var b strings.Builder
	for _, v := range variants {
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String(), nil
}
