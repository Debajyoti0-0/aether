package validate

import (
	"strings"
	"testing"
)

func TestBuildNavigatorLayer(t *testing.T) {
	layer := BuildNavigatorLayer("ClientX", []string{
		"prt_exchange", "imds_hijack", "sp_hijack", "wstrust_relay", "unknown_action",
	})
	if layer.Domain != "enterprise-attack" {
		t.Errorf("domain = %q", layer.Domain)
	}
	if !strings.Contains(layer.Name, "ClientX") {
		t.Errorf("name = %q", layer.Name)
	}

	ids := map[string]bool{}
	for _, tech := range layer.Techniques {
		ids[tech.TechniqueID] = true
		if !tech.Enabled {
			t.Errorf("technique %s disabled", tech.TechniqueID)
		}
	}

	for _, want := range []string{T1550WebSessionCookie, T1528StealAppToken, T1098AccountManip, T1110BruteForce} {
		if !ids[want] {
			t.Errorf("technique %s missing (ids: %v)", want, ids)
		}
	}
	if ids[T1484DomainTrustMod] {
		t.Error("unrelated technique should not appear")
	}

	// Exercised techniques are red with score 3.
	for _, tech := range layer.Techniques {
		if tech.Color != "#e63946" || tech.Score != 3.0 {
			t.Errorf("technique %+v not marked exercised", tech)
		}
		break
	}
}

func TestNavigatorJSONValid(t *testing.T) {
	layer := BuildNavigatorLayer("W", []string{"prt_exchange"})
	data, err := layer.NavigatorJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, `"domain": "enterprise-attack"`) {
		t.Errorf("json = %s", data)
	}
	if !strings.Contains(data, `"techniqueID"`) {
		t.Errorf("json missing techniques: %s", data)
	}
}

func TestBuildNavigatorLayerEmpty(t *testing.T) {
	layer := BuildNavigatorLayer("Empty", nil)
	if len(layer.Techniques) != 0 {
		t.Errorf("techniques = %d, want 0", len(layer.Techniques))
	}
}
