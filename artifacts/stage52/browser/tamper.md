# Tamper demonstration — Stage 52

What was done, and what came back. The raw harness output is `tamper-output.txt`
(it is text with a JSON body, not valid JSON on its own); this file is the
reasoning around it.

## The point

A dashboard that renders an audit chain is only useful if it is *able to say the
chain is broken*. Claiming tamper-evidence from a unit test proves the hashing is
correct; it does not prove that the thing an operator actually looks at notices a
forged entry. So the chain was forged and the real dashboard was pointed at it.

## The forgery

`scripts/tamper-chain.mjs` copies a verified chain and rewrites the `result` of
one entry (seq 2) while leaving the recorded `hash` and `signature` untouched.
This is the realistic case: an attacker with write access to the chain file
cannot re-sign, because the key is not theirs.

## What the server said, before serving anything

```text
WARNING: the audit chain at ...\aether-audit.jsonl does not verify
(1 of 23 entries failed hash or signature checks; linkage is intact).
The dashboard will still serve it, and the browser-side verifier will reach
the same verdict.
```

`linkage is intact` is the accurate statement here: the rewritten payload left
the `prev_hash` links alone, so the chain is structurally whole and one entry is
simply not what it claims to be. An earlier version of this message printed
"linkage first breaks at seq 0", which is a false claim about where the fault is
— it conflated a bad entry with a broken chain. Fixed; see the certification's
bug table.

## What the browser said

From `verify.wasm`, in the tab, with no input from the server's verdict:

```json
{
  "valid": false,
  "entryCount": 24,
  "validCount": 23,
  "tamperedSeq": [2],
  "brokenChainAt": null,
  "notes": ["seq 2: recomputed hash does not match the recorded hash, so this
             entry was altered after signing"]
}
```

The harness exits non-zero, which is the correct outcome: it is a check for a
valid chain, and this chain is not one.

## Two things this does not prove

- It does not prove the browser would catch a *re-signed* chain. It cannot: a
  re-signed chain with a different key is detected by the fingerprint mismatch
  between the key the operator was given and the one in the chain, which is a
  different check with a different failure mode.
- It does not prove anything about what an operator does with a red banner. The
  dashboard makes the disagreement loud; whether that is read is a human
  question, and this evidence says nothing about it.
