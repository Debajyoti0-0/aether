//go:build integration

package ad_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	kerberosengine "github.com/Debajyoti0-0/aether/internal/engine/ad/kerberos"
)

const (
	testRealm      = "AETHER.TEST"
	testDomain     = "AETHER"
	testDC         = "127.0.0.1"
	testKDCPort    = 88
	testLDAPPort   = 389
	testAdminUser  = "Administrator"
	testAdminPass  = "Passw0rd123!"
	testUser1      = "user1"
	testUser1Pass  = "Passw0rd123!"
	testUser2      = "user2"
	testUser2Pass  = "Passw0rd123!"
	testUser3      = "user3"
	testUser3Pass  = "Passw0rd123!"
	testUser4      = "user4"
	testUser4Pass  = "Passw0rd123!"
	testSvcSQL     = "svc_sql"
	testSvcSQLPass = "SvcPass123!"
	testSvcWeb     = "svc_web"
	testSvcWebPass = "WebPass123!"
)

func TestMain(m *testing.M) {
	// Ensure artifacts directory exists
	artifactsDir := filepath.Join("artifacts", "stage45b")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create artifacts dir: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func writeArtifact(t *testing.T, name string, data interface{}) {
	t.Helper()
	artifactsDir := filepath.Join("artifacts", "stage45b")
	filename := filepath.Join(artifactsDir, name+".json")
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal artifact: %v", err)
	}
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write artifact: %v", err)
	}
	t.Logf("Artifact written: %s", filename)
}

func hashData(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// TestKerberosASREP tests AS-REQ -> AS-REP for user1
func TestKerberosASREP(t *testing.T) {
	ctx := context.Background()
	
	engine := kerberosengine.NewEngine(testDC, testKDCPort, testLDAPPort, testRealm, testDomain)
	defer engine.Close()
	
	tgt, err := engine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err != nil {
		writeArtifact(t, "TestKerberosASREP_error", map[string]string{
			"error": err.Error(),
			"user":  testUser1,
		})
		t.Fatalf("RequestTGT failed: %v", err)
	}
	
	result := map[string]interface{}{
		"user":         testUser1,
		"realm":        testRealm,
		"tgt_received": tgt != nil,
		"session_key":  tgt.SessionKey != nil,
		"ticket_flags": tgt.Ticket.Flags,
		"authtime":     tgt.Ticket.AuthTime.Format(time.RFC3339),
		"endtime":      tgt.Ticket.EndTime.Format(time.RFC3339),
	}
	writeArtifact(t, "TestKerberosASREP", result)
	
	if tgt == nil {
		t.Fatal("TGT is nil")
	}
	if tgt.SessionKey == nil {
		t.Fatal("Session key is nil")
	}
}

// TestKerberosTGSREP tests TGS-REQ -> TGS-REP for svc_sql
func TestKerberosTGSREP(t *testing.T) {
	ctx := context.Background()
	
	engine := kerberosengine.NewEngine(testDC, testKDCPort, testLDAPPort, testRealm, testDomain)
	defer engine.Close()
	
	// First get TGT for user1
	tgt, err := engine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err != nil {
		writeArtifact(t, "TestKerberosTGSREP_tgt_error", map[string]string{"error": err.Error()})
		t.Fatalf("RequestTGT failed: %v", err)
	}
	
	// Request TGS for svc_sql SPN
	spn := "MSSQLSvc/sql01.aether.test:1433"
	tgs, err := engine.RequestTGS(ctx, tgt, spn)
	if err != nil {
		writeArtifact(t, "TestKerberosTGSREP_error", map[string]string{
			"error": err.Error(),
			"spn":   spn,
		})
		t.Fatalf("RequestTGS failed: %v", err)
	}
	
	result := map[string]interface{}{
		"user":         testUser1,
		"spn":          spn,
		"tgs_received": tgs != nil,
		"service_name": tgs.Ticket.SName.String(),
		"flags":        tgs.Ticket.Flags,
	}
	writeArtifact(t, "TestKerberosTGSREP", result)
	
	if tgs == nil {
		t.Fatal("TGS is nil")
	}
	if tgs.Ticket.SName == nil {
		t.Fatal("Service name is nil")
	}
}

// TestKerberoast tests extracting crackable blob from svc_sql
func TestKerberoast(t *testing.T) {
	ctx := context.Background()
	
	engine := kerberosengine.NewEngine(testDC, testKDCPort, testLDAPPort, testRealm, testDomain)
	defer engine.Close()
	
	// Get TGT for user1
	tgt, err := engine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err != nil {
		writeArtifact(t, "TestKerberoast_tgt_error", map[string]string{"error": err.Error()})
		t.Fatalf("RequestTGT failed: %v", err)
	}
	
	// Request TGS for svc_sql SPN
	spn := "MSSQLSvc/sql01.aether.test:1433"
	tgs, err := engine.RequestTGS(ctx, tgt, spn)
	if err != nil {
		writeArtifact(t, "TestKerberoast_tgs_error", map[string]string{"error": err.Error(), "spn": spn})
		t.Fatalf("RequestTGS failed: %v", err)
	}
	
	// Extract kerberoast blob
	blob, err := engine.ExtractKerberoastBlob(tgs)
	if err != nil {
		writeArtifact(t, "TestKerberoast_error", map[string]string{"error": err.Error()})
		t.Fatalf("ExtractKerberoastBlob failed: %v", err)
	}
	
	result := map[string]interface{}{
		"spn":              spn,
		"blob_extracted":   blob != nil,
		"blob_format":      blob.Format,
		"hash_algorithm":   blob.HashAlgorithm,
		"hash":             blob.Hash[:32] + "...", // Truncate for readability
		"full_hash_sha256": hashData([]byte(blob.Hash)),
	}
	writeArtifact(t, "TestKerberoast", result)
	
	if blob == nil {
		t.Fatal("Kerberoast blob is nil")
	}
	if blob.Format != "krb5tgs" {
		t.Errorf("Unexpected format: %s", blob.Format)
	}
}

// TestASREPRoast tests extracting blob for user2 (DONT_REQ_PREAUTH)
func TestASREPRoast(t *testing.T) {
	ctx := context.Background()
	
	engine := kerberosengine.NewEngine(testDC, testKDCPort, testLDAPPort, testRealm, testDomain)
	defer engine.Close()
	
	// Request AS-REP for user2 (no pre-auth required)
	asrep, err := engine.RequestASREPNoPreauth(ctx, testUser2)
	if err != nil {
		writeArtifact(t, "TestASREPRoast_error", map[string]string{
			"error": err.Error(),
			"user":  testUser2,
		})
		t.Fatalf("RequestASREPNoPreauth failed: %v", err)
	}
	
	// Extract AS-REP roast blob
	blob, err := engine.ExtractASREPRoastBlob(asrep)
	if err != nil {
		writeArtifact(t, "TestASREPRoast_extract_error", map[string]string{"error": err.Error()})
		t.Fatalf("ExtractASREPRoastBlob failed: %v", err)
	}
	
	result := map[string]interface{}{
		"user":              testUser2,
		"blob_extracted":    blob != nil,
		"blob_format":       blob.Format,
		"hash_algorithm":    blob.HashAlgorithm,
		"hash":              blob.Hash[:32] + "...",
		"full_hash_sha256":  hashData([]byte(blob.Hash)),
	}
	writeArtifact(t, "TestASREPRoast", result)
	
	if blob == nil {
		t.Fatal("AS-REP roast blob is nil")
	}
	if blob.Format != "krb5asrep" {
		t.Errorf("Unexpected format: %s", blob.Format)
	}
}

// TestCcacheRoundTrip tests writing and reading MIT + Heimdal ccache
func TestCcacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	
	engine := kerberosengine.NewEngine(testDC, testKDCPort, testLDAPPort, testRealm, testDomain)
	defer engine.Close()
	
	// Get TGT for user1
	tgt, err := engine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err != nil {
		writeArtifact(t, "TestCcacheRoundTrip_tgt_error", map[string]string{"error": err.Error()})
		t.Fatalf("RequestTGT failed: %v", err)
	}
	
	// Export to MIT ccache format
	mitCcache, err := engine.ExportCcache(tgt, kerberosengine.CcacheFormatMIT)
	if err != nil {
		writeArtifact(t, "TestCcacheRoundTrip_mit_export_error", map[string]string{"error": err.Error()})
		t.Fatalf("ExportCcache (MIT) failed: %v", err)
	}
	
	// Export to Heimdal ccache format
	heimdalCcache, err := engine.ExportCcache(tgt, kerberosengine.CcacheFormatHeimdal)
	if err != nil {
		writeArtifact(t, "TestCcacheRoundTrip_heimdal_export_error", map[string]string{"error": err.Error()})
		t.Fatalf("ExportCcache (Heimdal) failed: %v", err)
	}
	
	// Parse MIT ccache back
	parsedMIT, err := engine.ParseCcache(mitCcache)
	if err != nil {
		writeArtifact(t, "TestCcacheRoundTrip_mit_parse_error", map[string]string{"error": err.Error()})
		t.Fatalf("ParseCcache (MIT) failed: %v", err)
	}
	
	// Parse Heimdal ccache back
	parsedHeimdal, err := engine.ParseCcache(heimdalCcache)
	if err != nil {
		writeArtifact(t, "TestCcacheRoundTrip_heimdal_parse_error", map[string]string{"error": err.Error()})
		t.Fatalf("ParseCcache (Heimdal) failed: %v", err)
	}
	
	// Verify semantic equivalence
	mitEqual := engine.CompareTickets(tgt.Ticket, parsedMIT.Ticket)
	heimdalEqual := engine.CompareTickets(tgt.Ticket, parsedHeimdal.Ticket)
	
	result := map[string]interface{}{
		"original_ticket_realm":  tgt.Ticket.Realm,
		"original_ticket_sname":  tgt.Ticket.SName.String(),
		"mit_ccache_bytes":       len(mitCcache),
		"heimdal_ccache_bytes":   len(heimdalCcache),
		"mit_parsed_equal":       mitEqual,
		"heimdal_parsed_equal":   heimdalEqual,
		"mit_hash":               hashData(mitCcache),
		"heimdal_hash":           hashData(heimdalCcache),
	}
	writeArtifact(t, "TestCcacheRoundTrip", result)
	
	if !mitEqual {
		t.Error("MIT ccache round-trip failed: tickets not semantically equivalent")
	}
	if !heimdalEqual {
		t.Error("Heimdal ccache round-trip failed: tickets not semantically equivalent")
	}
}

// TestEngagementBoundary tests that operations outside scope are denied
func TestEngagementBoundary(t *testing.T) {
	ctx := context.Background()
	
	// Create engine with restricted engagement (only allow user1)
	engagement := &kerberosengine.EngagementConfig{
		AllowedRealms: []string{testRealm},
		AllowedUsers:  []string{testUser1},
		AllowedDCs:    []string{testDC},
		ExpiresAt:     time.Now().Add(1 * time.Hour),
	}
	
	engine := kerberosengine.NewEngineWithEngagement(testDC, testKDCPort, testLDAPPort, testRealm, testDomain, engagement)
	defer engine.Close()
	
	// Test 1: Valid engagement - user1 should succeed
	tgt, err := engine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err != nil {
		writeArtifact(t, "TestEngagementBoundary_valid_error", map[string]string{"error": err.Error()})
		t.Fatalf("Valid engagement should succeed: %v", err)
	}
	
	// Test 2: Wrong user (user2) - should be denied
	_, err = engine.RequestTGT(ctx, testUser2, testUser2Pass)
	if err == nil {
		writeArtifact(t, "TestEngagementBoundary_wrong_user", map[string]string{
			"error": "expected denial but got success",
			"user":  testUser2,
		})
		t.Fatal("Wrong user should be denied by engagement boundary")
	}
	
	// Test 3: Expired engagement - should be denied
	expiredEngagement := &kerberosengine.EngagementConfig{
		AllowedRealms: []string{testRealm},
		AllowedUsers:  []string{testUser1},
		AllowedDCs:    []string{testDC},
		ExpiresAt:     time.Now().Add(-1 * time.Hour), // Expired
	}
	expiredEngine := kerberosengine.NewEngineWithEngagement(testDC, testKDCPort, testLDAPPort, testRealm, testDomain, expiredEngagement)
	defer expiredEngine.Close()
	
	_, err = expiredEngine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err == nil {
		writeArtifact(t, "TestEngagementBoundary_expired", map[string]string{
			"error": "expected denial but got success",
		})
		t.Fatal("Expired engagement should be denied")
	}
	
	// Test 4: Out-of-scope realm - should be denied
	wrongRealmEngagement := &kerberosengine.EngagementConfig{
		AllowedRealms: []string{"WRONG.REALM"},
		AllowedUsers:  []string{testUser1},
		AllowedDCs:    []string{testDC},
		ExpiresAt:     time.Now().Add(1 * time.Hour),
	}
	wrongRealmEngine := kerberosengine.NewEngineWithEngagement(testDC, testKDCPort, testLDAPPort, testRealm, testDomain, wrongRealmEngagement)
	defer wrongRealmEngine.Close()
	
	_, err = wrongRealmEngine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err == nil {
		writeArtifact(t, "TestEngagementBoundary_wrong_realm", map[string]string{
			"error": "expected denial but got success",
		})
		t.Fatal("Wrong realm should be denied")
	}
	
	// Test 5: Capability not granted - should be denied
	limitedEngagement := &kerberosengine.EngagementConfig{
		AllowedRealms:     []string{testRealm},
		AllowedUsers:      []string{testUser1},
		AllowedDCs:        []string{testDC},
		AllowedCapabilities: []string{"kerberos:tgt"}, // Only TGT, not kerberoast
		ExpiresAt:         time.Now().Add(1 * time.Hour),
	}
	limitedEngine := kerberosengine.NewEngineWithEngagement(testDC, testKDCPort, testLDAPPort, testRealm, testDomain, limitedEngagement)
	defer limitedEngine.Close()
	
	// Get TGT first (should work)
	tgt, err = limitedEngine.RequestTGT(ctx, testUser1, testUser1Pass)
	if err != nil {
		t.Fatalf("TGT should work: %v", err)
	}
	
	// Try kerberoast (should be denied)
	_, err = limitedEngine.RequestTGS(ctx, tgt, "MSSQLSvc/sql01.aether.test:1433")
	if err == nil {
		writeArtifact(t, "TestEngagementBoundary_wrong_cap", map[string]string{
			"error": "expected denial but got success",
		})
		t.Fatal("Capability not granted should be denied")
	}
	
	result := map[string]interface{}{
		"valid_user1":       "ALLOWED",
		"wrong_user_user2":  "DENIED",
		"expired_engagement": "DENIED",
		"wrong_realm":       "DENIED",
		"wrong_capability":  "DENIED",
	}
	writeArtifact(t, "TestEngagementBoundary", result)
}