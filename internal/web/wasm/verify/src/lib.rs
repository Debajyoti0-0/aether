//! Client-side audit-chain verifier for the Aether dashboard.
//!
//! # Why this exists
//!
//! The dashboard is served by the same process whose honesty is in question. A
//! green "chain verified" banner painted by that server is worth nothing: if the
//! server is compromised, it simply renders the green banner. So the
//! verification runs here, in the operator's browser, over bytes the server
//! cannot alter without invalidating an Ed25519 signature.
//!
//! # The wire contract, and why it is frozen
//!
//! This must agree with `internal/store/audit.go` **byte for byte**. A
//! verifier that is merely close is worse than no verifier, because it would
//! either reject an honest chain or accept a forged one.
//!
//! ```text
//! preimage = "<seq>|<timestamp>|<command>|<result>|<prev_hash>"
//! hash     = lowercase_hex(sha256(preimage))
//! signature= base64_std( ed25519_sign( ascii_bytes(hash) ) )
//! ```
//!
//! Two details are easy to get wrong and are therefore called out:
//!
//! 1. **The signature covers the ASCII of the hex hash, not the raw digest
//!    bytes.** Signing the digest would be the more natural design; this chain
//!    does not do that, so neither do we.
//! 2. **The timestamp enters the preimage as the exact string in the JSON.** We
//!    deliberately do not re-parse and re-format it. Reformatting would require
//!    reimplementing Go's `time.RFC3339Nano` (including its trailing-zero
//!    trimming) exactly, and any divergence would produce a false "tampered".
//!    Using the string verbatim is safe because the hash commits to those exact
//!    bytes: altering the timestamp changes the hash, which breaks the
//!    signature. Canonical form is checked separately, and separately
//!    reported, so a non-canonical timestamp is visible rather than fatal.

use base64::Engine;
use ed25519_dalek::{Signature, VerifyingKey, Verifier};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use wasm_bindgen::prelude::*;

/// Genesis hash: 64 zeros, matching `store.GenesisHash`.
const GENESIS_HASH: &str = "0000000000000000000000000000000000000000000000000000000000000000";

/// One wire-format audit entry. Field names and types mirror `web.AuditEntry`
/// exactly; a `deny_unknown_fields` is deliberately NOT used so that a future
/// additive field in the Go struct does not brick older dashboards.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ChainEntry {
    pub seq: i64,
    pub timestamp: String,
    #[serde(default)]
    pub command: String,
    #[serde(default)]
    pub result: String,
    #[serde(rename = "prev_hash")]
    pub prev_hash: String,
    pub hash: String,
    pub signature: String,
}

/// A per-entry verdict. Reporting every failure rather than stopping at the
/// first matters: "the chain broke at 41" and "41 through 900 are all
/// individually forged" call for completely different responses.
#[derive(Debug, Clone, Serialize)]
pub struct EntryVerdict {
    pub seq: i64,
    pub hash_ok: bool,
    pub signature_ok: bool,
    pub linkage_ok: bool,
    pub timestamp_canonical: bool,
}

/// The complete verification result, returned as a JSON string.
///
/// A string rather than a JS object: it avoids pulling in
/// `serde-wasm-bindgen` (one less dependency to audit) and lets the caller own
/// the parse, so a malformed result cannot produce a half-populated JS object.
#[derive(Debug, Clone, Serialize)]
pub struct VerifyResult {
    pub valid: bool,
    pub entry_count: usize,
    pub valid_count: usize,
    pub tampered_seq: Vec<i64>,
    pub broken_chain_at: Option<i64>,
    pub head_hash: String,
    pub genesis: String,
    pub pubkey_fingerprint: String,
    pub verified_by: String,
    pub error: Option<String>,
    pub notes: Vec<String>,
    /// Per-entry verdicts, omitted when the caller does not need them.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub entries: Vec<EntryVerdict>,
}

fn fail(msg: String) -> VerifyResult {
    VerifyResult {
        valid: false,
        entry_count: 0,
        valid_count: 0,
        tampered_seq: Vec::new(),
        broken_chain_at: None,
        head_hash: String::new(),
        genesis: GENESIS_HASH.to_string(),
        pubkey_fingerprint: String::new(),
        verified_by: "aether-verify.wasm (client-side)".to_string(),
        error: Some(msg),
        notes: Vec::new(),
        entries: Vec::new(),
    }
}

/// Recompute the entry hash from the frozen preimage format.
fn compute_hash(e: &ChainEntry) -> String {
    let preimage = format!(
        "{}|{}|{}|{}|{}",
        e.seq, e.timestamp, e.command, e.result, e.prev_hash
    );
    let digest = Sha256::digest(preimage.as_bytes());
    hex::encode(digest)
}

/// Check that a timestamp is in the canonical layout `store.computeHash` hashes:
/// UTC, RFC 3339, with Go's trailing-zero trimming applied to the fractional
/// seconds.
///
/// This is reported separately from the hash check on purpose. A non-canonical
/// timestamp whose hash still verifies means the signer used a different layout,
/// which is worth knowing about; it is not by itself evidence of forgery.
fn timestamp_is_canonical(ts: &str) -> bool {
    // Minimum shape: 2026-09-26T01:15:30Z with an optional ".fff..." part.
    let bytes = ts.as_bytes();
    if bytes.len() < 20 {
        return false;
    }
    if bytes[4] != b'-' || bytes[7] != b'-' || bytes[10] != b'T' {
        return false;
    }
    if bytes[13] != b':' || bytes[16] != b':' {
        return false;
    }
    // Must be UTC. A numeric offset is a different byte string and would change
    // the preimage, so the signer must have used Z for the chain to be
    // self-consistent.
    let tail = &ts[19..];
    if tail.starts_with('.') {
        let rest = match tail[1..].find('Z') {
            Some(i) => &tail[1..1 + i],
            None => return false,
        };
        if rest.is_empty() || !rest.bytes().all(|b| b.is_ascii_digit()) {
            return false;
        }
        // Go trims trailing zeros from the fractional part, so a canonical
        // timestamp never ends in '0' before the Z.
        !rest.ends_with('0')
    } else {
        tail == "Z"
    }
}

/// SHA-256 fingerprint of a public key, rendered like the Go side's
/// `FingerprintPublicKey` so the two can be compared by eye.
fn fingerprint(key_bytes: &[u8]) -> String {
    let digest = Sha256::digest(key_bytes);
    let hexed = hex::encode(digest);
    let groups: Vec<String> = (0..4)
        .map(|i| hexed[i * 8..(i + 1) * 8].to_string())
        .collect();
    groups.join("-")
}

/// Verify a chain against a base64 Ed25519 public key.
///
/// `include_entries` controls whether per-entry verdicts are returned; they are
/// large on a long chain, so the default is off.
#[wasm_bindgen]
pub fn verify_chain(
    chain_json: &str,
    pubkey_b64: &str,
    include_entries: bool,
) -> String {
    match verify_inner(chain_json, pubkey_b64, include_entries) {
        Ok(r) => serde_json::to_string(&r).unwrap_or_else(|e| {
            format!("{{\"valid\":false,\"error\":\"result serialisation failed: {}\"}}", e)
        }),
        Err(e) => serde_json::to_string(&fail(e)).unwrap_or_else(|_| {
            "{\"valid\":false,\"error\":\"unserialisable failure\"}".to_string()
        }),
    }
}

/// Verify a chain from NDJSON, one entry per line.
///
/// A long chain is large; NDJSON lets a streaming client verify the entries it
/// has already received instead of waiting for one multi-megabyte array.
#[wasm_bindgen]
pub fn verify_chain_ndjson(ndjson: &str, pubkey_b64: &str, include_entries: bool) -> String {
    let mut lines: Vec<&str> = ndjson.lines().map(|l| l.trim()).filter(|l| !l.is_empty()).collect();
    let json = format!("[{}]", lines.join(","));
    lines.clear();
    match verify_inner(&json, pubkey_b64, include_entries) {
        Ok(r) => serde_json::to_string(&r).unwrap_or_else(|e| {
            format!("{{\"valid\":false,\"error\":\"result serialisation failed: {}\"}}", e)
        }),
        Err(e) => serde_json::to_string(&fail(e)).unwrap_or_else(|_| {
            "{\"valid\":false,\"error\":\"unserialisable failure\"}".to_string()
        }),
    }
}

fn verify_inner(
    chain_json: &str,
    pubkey_b64: &str,
    include_entries: bool,
) -> Result<VerifyResult, String> {
    let key_bytes = base64::engine::general_purpose::STANDARD
        .decode(pubkey_b64.trim())
        .map_err(|e| format!("public key is not valid base64: {e}"))?;
    if key_bytes.len() != 32 {
        return Err(format!(
            "public key is {} bytes; Ed25519 requires exactly 32",
            key_bytes.len()
        ));
    }
    let key_array: [u8; 32] = key_bytes[..].try_into().expect("length checked above");
    let verifying = VerifyingKey::from_bytes(&key_array)
        .map_err(|e| format!("public key is not a valid Ed25519 point: {e}"))?;

    // The payload is either a bare array or an object with an "entries" field,
    // so the same function serves /api/audit/chain and a raw export.
    let entries: Vec<ChainEntry> = match serde_json::from_str::<serde_json::Value>(chain_json) {
        Ok(serde_json::Value::Array(_)) => serde_json::from_str(chain_json)
            .map_err(|e| format!("chain is not a valid entry array: {e}"))?,
        Ok(serde_json::Value::Object(_)) => {
            let v: serde_json::Value = serde_json::from_str(chain_json)
                .map_err(|e| format!("chain is not valid JSON: {e}"))?;
            let arr = v
                .get("entries")
                .ok_or_else(|| "chain object has no \"entries\" field".to_string())?;
            serde_json::from_value(arr.clone())
                .map_err(|e| format!("\"entries\" is not a valid entry array: {e}"))?
        }
        _ => return Err("chain must be a JSON array or an object with entries".to_string()),
    };

    let mut res = VerifyResult {
        valid: true,
        entry_count: entries.len(),
        valid_count: 0,
        tampered_seq: Vec::new(),
        broken_chain_at: None,
        head_hash: GENESIS_HASH.to_string(),
        genesis: GENESIS_HASH.to_string(),
        pubkey_fingerprint: fingerprint(&key_bytes),
        verified_by: "aether-verify.wasm (client-side)".to_string(),
        error: None,
        notes: Vec::new(),
        entries: Vec::new(),
    };
    if entries.is_empty() {
        res.notes.push(
            "chain is empty: an empty chain is internally consistent but attests to nothing"
                .to_string(),
        );
        return Ok(res);
    }

    let mut prev = GENESIS_HASH.to_string();
    for e in &entries {
        let hash_ok = compute_hash(e) == e.hash;

        let linkage_ok = e.prev_hash == prev;

        let signature_ok = match base64::engine::general_purpose::STANDARD.decode(e.signature.trim()) {
            Ok(sig) => match <[u8; 64]>::try_from(sig.as_slice()) {
                Ok(arr) => verifying
                    .verify(e.hash.as_bytes(), &Signature::from_bytes(&arr))
                    .is_ok(),
                Err(_) => false,
            },
            Err(_) => false,
        };

        let ts_ok = timestamp_is_canonical(&e.timestamp);

        if !hash_ok {
            res.tampered_seq.push(e.seq);
            res.valid = false;
            res.notes.push(format!(
                "seq {}: recomputed hash does not match the recorded hash, so this entry was altered after signing",
                e.seq
            ));
        } else if !signature_ok {
            res.tampered_seq.push(e.seq);
            res.valid = false;
            res.notes.push(format!(
                "seq {}: hash is intact but the Ed25519 signature does not verify under the supplied key",
                e.seq
            ));
        } else {
            res.valid_count += 1;
        }

        if !linkage_ok {
            if res.broken_chain_at.is_none() {
                res.broken_chain_at = Some(e.seq);
            }
            res.valid = false;
            res.notes.push(format!(
                "seq {}: prev_hash does not match the previous entry's hash, so the chain is broken or reordered here",
                e.seq
            ));
        }

        if !ts_ok {
            res.notes.push(format!(
                "seq {}: timestamp \"{}\" is not in the canonical UTC RFC3339Nano form the signer hashes",
                e.seq, e.timestamp
            ));
        }

        if include_entries {
            res.entries.push(EntryVerdict {
                seq: e.seq,
                hash_ok,
                signature_ok,
                linkage_ok,
                timestamp_canonical: ts_ok,
            });
        }

        prev = e.hash.clone();
    }

    res.head_hash = entries.last().map(|e| e.hash.clone()).unwrap_or_default();
    Ok(res)
}

/// A known-answer vector, produced by a third independent implementation (Node's
/// `crypto`, signing over the ASCII hex hash) rather than by this crate.
///
/// Embedding a foreign vector is the point: a self-test built from this crate's
/// own `compute_hash` would pass even if the preimage format were wrong in the
/// same way everywhere, which is precisely the failure this module must not have.
const SELFTEST_PUBKEY: &str = "qJj5ywymSLgWfibbM9rqDirIVyeW18HYZbqA4TRk+NQ=";

/// A second, unrelated key. The self-test requires the vector to be *rejected*
/// under it, so a module that returned "valid" unconditionally would fail here
/// rather than pass.
const SELFTEST_OTHER_PUBKEY: &str = "x1KB+TxYkZ+oZ/YcJA30FD+I0zBU96VMP51oev/CcvQ=";

const SELFTEST_CHAIN: &str = r#"{"entries":[{"seq":1,"timestamp":"2026-01-03T13:21:41.789Z","command":"workspace.create","result":"{\"workspace\":\"selftest\"}","prev_hash":"0000000000000000000000000000000000000000000000000000000000000000","hash":"992207bf570fb66b5d00340a2be682bd2246c9be55a63ffe64ea3f6d4dd82609","signature":"1qeaRORU7VYTY6sKl/YEJbI1uVdhGfJScDebAQzlHG1BZn9/fXHG7ceOgFJ+fNPHwc5zNtidwRoghvzP4YtdCw=="},{"seq":2,"timestamp":"2026-01-03T13:21:41.789Z","command":"graph.addNode","result":"{\"id\":\"U-1\",\"label\":\"selftest\",\"type\":\"user\",\"provider\":\"selftest\"}","prev_hash":"992207bf570fb66b5d00340a2be682bd2246c9be55a63ffe64ea3f6d4dd82609","hash":"5327bee13602ddf94f7b4e4d86406aceb401f5fc4f850c5e84d5ae06d531eae7","signature":"YAzBMa22+46/2ON7MDsXfWJ6kuaYszRYzEkDhpuT0Y0aA9E/jwzswSBpFTfP/NFF45KdPXEeKA8fxuM0HPVRCw=="}]}"#;

/// Self-test exported for the browser-side integration check.
///
/// It signs nothing and needs no private key: it verifies an embedded
/// known-answer vector produced by an independent signer, then requires that the
/// same vector be *rejected* under an unrelated key.
///
/// The two halves matter equally. Accepting a valid chain proves the hashes and
/// signatures agree with a foreign implementation; rejecting a chain signed by
/// the wrong key proves the module discriminates at all, so a build that had
/// lost its verification step could not report "ok" here.
///
/// A failure is reported as a string beginning with "FAILED", never as a bare
/// "ok", so a caller cannot mistake a broken module for a healthy one.
#[wasm_bindgen]
pub fn selftest() -> String {
    let good = match verify_inner(SELFTEST_CHAIN, SELFTEST_PUBKEY, true) {
        Ok(r) => r,
        Err(e) => return format!("FAILED: the known-answer vector did not verify: {e}"),
    };
    if !good.valid || good.valid_count != good.entry_count || !good.tampered_seq.is_empty() {
        return format!(
            "FAILED: a valid chain was rejected (valid_count {}/{}, tampered {:?})",
            good.valid_count, good.entry_count, good.tampered_seq
        );
    }
    if good.entry_count != 2 {
        return format!(
            "FAILED: the known-answer vector has {} entries; the fixture is wrong",
            good.entry_count
        );
    }
    match verify_inner(SELFTEST_CHAIN, SELFTEST_OTHER_PUBKEY, false) {
        Ok(r) if r.valid => {
            return "FAILED: a chain signed by a different key was accepted".to_string();
        }
        Ok(_) => {}
        Err(e) => return format!("FAILED: the wrong-key control errored instead of failing: {e}"),
    }
    format!(
        "ok: {}/{} entries verified, and the same chain was rejected under an unrelated key",
        good.valid_count, good.entry_count
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer, SigningKey};

    /// Build and sign a chain the way `internal/store` does, so the tests
    /// exercise the same preimage and signature convention the Go side uses.
    fn build(signing: &SigningKey, steps: &[(&str, &str)], ts: &str) -> Vec<ChainEntry> {
        let mut out = Vec::new();
        let mut prev = GENESIS_HASH.to_string();
        for (i, (command, result)) in steps.iter().enumerate() {
            let seq = (i + 1) as i64;
            let hash = {
                let e = ChainEntry {
                    seq,
                    timestamp: ts.to_string(),
                    command: (*command).to_string(),
                    result: (*result).to_string(),
                    prev_hash: prev.clone(),
                    hash: String::new(),
                    signature: String::new(),
                };
                compute_hash(&e)
            };
            let sig = signing.sign(hash.as_bytes());
            out.push(ChainEntry {
                seq,
                timestamp: ts.to_string(),
                command: (*command).to_string(),
                result: (*result).to_string(),
                prev_hash: prev.clone(),
                hash: hash.clone(),
                signature: base64::engine::general_purpose::STANDARD.encode(sig.to_bytes()),
            });
            prev = hash;
        }
        out
    }

    fn pub_b64(signing: &SigningKey) -> String {
        base64::engine::general_purpose::STANDARD.encode(signing.verifying_key().to_bytes())
    }

    fn to_json(entries: &[ChainEntry]) -> String {
        serde_json::to_string(&serde_json::json!({ "entries": entries })).expect("serialise")
    }

    fn key_from_seed(b: u8) -> SigningKey {
        SigningKey::from_bytes(&[b; 32])
    }

    const TS: &str = "2026-01-02T03:04:05.123456789Z";
    const STEPS: [(&str, &str); 2] = [
        ("workspace.create", "{\"workspace\":\"t\"}"),
        ("graph.addNode", "{\"id\":\"U-1\",\"label\":\"a\"}"),
    ];

    #[test]
    fn a_correctly_signed_chain_is_valid() {
        let k = key_from_seed(7);
        let r = verify_inner(&to_json(&build(&k, &STEPS, TS)), &pub_b64(&k), true).expect("verify");
        assert!(r.valid, "notes: {:?}", r.notes);
        assert_eq!(r.valid_count, 2);
        assert!(r.tampered_seq.is_empty());
        assert_eq!(r.broken_chain_at, None);
        assert_eq!(r.entries.len(), 2);
        assert!(r.entries.iter().all(|e| e.hash_ok && e.signature_ok && e.linkage_ok));
    }

    #[test]
    fn a_tampered_payload_is_caught_by_the_hash() {
        let k = key_from_seed(7);
        let mut entries = build(&k, &STEPS, TS);
        entries[1].result = "{\"id\":\"U-1\",\"label\":\"admin\"}".to_string();
        let r = verify_inner(&to_json(&entries), &pub_b64(&k), true).expect("verify");
        assert!(!r.valid);
        assert_eq!(r.tampered_seq, vec![2]);
    }

    #[test]
    fn a_chain_signed_by_another_key_is_rejected() {
        let mine = key_from_seed(7);
        let theirs = key_from_seed(9);
        let r = verify_inner(&to_json(&build(&theirs, &STEPS, TS)), &pub_b64(&mine), false)
            .expect("verify");
        assert!(!r.valid, "a foreign signature must not verify");
        assert_eq!(r.tampered_seq, vec![1, 2]);
    }

    #[test]
    fn a_signature_over_the_raw_digest_is_rejected() {
        // The chain signs the ASCII of the hex hash. Signing the 32 raw digest
        // bytes is the natural mistake, and it must not verify.
        let k = key_from_seed(7);
        let mut entries = build(&k, &STEPS, TS);
        for e in entries.iter_mut() {
            let sig = k.sign(&hex::decode(&e.hash).expect("hex"));
            e.signature = base64::engine::general_purpose::STANDARD.encode(sig.to_bytes());
        }
        let r = verify_inner(&to_json(&entries), &pub_b64(&k), false).expect("verify");
        assert!(!r.valid);
    }

    #[test]
    fn a_reordered_chain_breaks_linkage() {
        let k = key_from_seed(7);
        let mut entries = build(&k, &STEPS, TS);
        entries.swap(0, 1);
        // The head entry now sits first, so its prev_hash is the second entry's
        // hash rather than the genesis hash: the break is reported at the
        // sequence number of the entry that is out of place.
        let r = verify_inner(&to_json(&entries), &pub_b64(&k), false).expect("verify");
        assert!(!r.valid);
        assert_eq!(r.broken_chain_at, Some(2));
    }

    #[test]
    fn a_non_canonical_timestamp_is_reported_but_not_fatal() {
        let k = key_from_seed(7);
        // Signed exactly as sent, so the hash and signature both verify; only the
        // layout differs from what Go's RFC3339Nano produces.
        let r = verify_inner(&to_json(&build(&k, &STEPS, "2026-01-02 03:04:05Z")), &pub_b64(&k), true)
            .expect("verify");
        assert!(r.valid, "a layout difference is not evidence of forgery");
        assert!(r
            .entries
            .iter()
            .all(|e| !e.timestamp_canonical), "but it must be visible");
    }

    #[test]
    fn a_malformed_key_is_an_error_not_a_panic() {
        for bad in ["", "not base64!!", "c2hvcnQ="] {
            let r = verify_inner("{\"entries\":[]}", bad, false);
            assert!(r.is_err(), "key {bad:?} should be rejected");
        }
    }

    #[test]
    fn a_well_formed_but_wrong_key_verifies_nothing() {
        // 32 zero bytes is not a usable key, but dalek accepts it as a point
        // rather than erroring, so the guarantee it has to provide is that no
        // signature verifies under it. A key that merely failed to parse would
        // be a weaker outcome than one that parses and rejects every entry.
        let zero = base64::engine::general_purpose::STANDARD.encode([0u8; 32]);
        let r = verify_inner(SELFTEST_CHAIN, &zero, false).expect("verify");
        assert!(!r.valid);
        assert_eq!(r.tampered_seq, vec![1, 2]);
    }

    #[test]
    fn an_empty_chain_is_consistent_but_attests_to_nothing() {
        let k = key_from_seed(3);
        let r = verify_inner("{\"entries\":[]}", &pub_b64(&k), false).expect("verify");
        assert!(r.valid);
        assert_eq!(r.entry_count, 0);
        assert!(!r.notes.is_empty(), "an empty chain must carry a caveat");
    }

    #[test]
    fn the_embedded_vector_agrees_with_an_independent_signer() {
        // The vector was produced by Node's crypto, not by this crate.
        let r = verify_inner(SELFTEST_CHAIN, SELFTEST_PUBKEY, true).expect("verify");
        assert!(r.valid, "notes: {:?}", r.notes);
        assert_eq!(r.valid_count, 2);
        assert!(r.entries.iter().all(|e| e.timestamp_canonical));
    }

    #[test]
    fn the_embedded_vector_is_rejected_under_another_key() {
        let r = verify_inner(SELFTEST_CHAIN, SELFTEST_OTHER_PUBKEY, false).expect("verify");
        assert!(!r.valid);
    }

    #[test]
    fn selftest_reports_ok_only_when_it_actually_verifies() {
        let s = selftest();
        assert!(s.starts_with("ok:"), "selftest said: {s}");
        assert!(s.contains("2/2"), "selftest said: {s}");
    }

    #[test]
    fn a_bare_array_is_accepted_as_well_as_an_object() {
        let k = key_from_seed(7);
        let entries = build(&k, &STEPS, TS);
        let arr = serde_json::to_string(&entries).expect("serialise");
        let r = verify_inner(&arr, &pub_b64(&k), false).expect("verify");
        assert!(r.valid);
    }

    #[test]
    fn a_payload_that_is_not_a_chain_is_rejected() {
        let k = key_from_seed(7);
        assert!(verify_inner("not json", &pub_b64(&k), false).is_err());
        assert!(verify_inner("{}", &pub_b64(&k), false).is_err());
        assert!(verify_inner("42", &pub_b64(&k), false).is_err());
        assert!(verify_inner(r#"{"entries":{}}"#, &pub_b64(&k), false).is_err());
    }
}
