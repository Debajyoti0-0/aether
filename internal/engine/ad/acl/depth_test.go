package acl

import (
	"context"
	"strings"
	"testing"

	ldapproto "github.com/Debajyoti0-0/aether/internal/protocol/ldap"
)

// The depth contract is a regression test for a real false-success.
//
// FindACLPaths normalises a non-positive maxDepth to 5 and then never reads the
// value again: the traversal only reads the target's DACL, so it is single-hop
// by construction. `aether ad ldap path --max-depth N` therefore exited 0 having
// bounded nothing, and an operator who scoped an ACL path search was told it
// had been scoped.
//
// The check runs before any directory access, so these cases need no LDAP
// server -- which is the point: a depth the engine cannot honour must be
// refused before it can do anything at all.
func TestFindACLPathsRefusesUnsupportedDepth(t *testing.T) {
	sid := &ldapproto.SID{}
	p := &Principal{SID: sid}

	for _, depth := range []int{0, -1, 2, 5, 100} {
		_, err := (&Engine{}).FindACLPaths(context.Background(), p, "CN=target,DC=example,DC=com", depth)
		if err == nil {
			t.Errorf("--max-depth %d was accepted, but the search is single-hop", depth)
			continue
		}
		if !strings.Contains(err.Error(), "single-hop") {
			t.Errorf("--max-depth %d: error does not explain the limit: %v", depth, err)
		}
	}
}

func TestFindACLPathsAcceptsTheSupportedDepth(t *testing.T) {
	// Depth 1 gets past the contract check. With no directory reachable it then
	// fails on the lookup, which is the expected next step and proves the depth
	// itself was not what stopped it.
	p := &Principal{SID: &ldapproto.SID{}}
	_, err := (&Engine{}).FindACLPaths(context.Background(), p, "CN=target,DC=example,DC=com", PathSearchDepth)
	if err != nil {
		if strings.Contains(err.Error(), "single-hop") {
			t.Fatalf("the supported depth %d was rejected as unsupported", PathSearchDepth)
		}
		// Anything else is the directory lookup failing, which is fine here.
		return
	}
	t.Log("the supported depth passed the contract check and the lookup succeeded")
}

func TestPathSearchDepthIsSingle(t *testing.T) {
	if PathSearchDepth != 1 {
		t.Errorf("PathSearchDepth = %d; the traversal reads one object's DACL, so this must be 1", PathSearchDepth)
	}
}
