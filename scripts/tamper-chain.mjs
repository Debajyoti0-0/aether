// Produces a tampered copy of an audit chain for the Stage 52 evidence.
//
// One entry's `result` is altered while its `hash` and `signature` are left
// exactly as they were, which is what an attacker who can write to the chain
// file would produce. The point of the demonstration is that both the server
// and the in-browser verifier reject the result, and that they agree.
//
//   node scripts/tamper-chain.mjs <in.jsonl> <out.jsonl> <entry-number>
import { readFileSync, writeFileSync } from "node:fs";

const [, , inPath, outPath, seqArg] = process.argv;
const seq = Number(seqArg || 2);
const lines = readFileSync(inPath, "utf8").split("\n").filter((l) => l.trim() !== "");

let tampered = 0;
const out = lines.map((line) => {
  const entry = JSON.parse(line);
  if (entry.seq !== seq) return line;
  const before = entry.result;
  entry.result = before.replace(/"label":"[^"]*"/, '"label":"escalated"');
  if (entry.result === before) {
    entry.result = JSON.stringify({ tampered: true, note: "result rewritten after signing" });
  }
  tampered += 1;
  return JSON.stringify(entry);
});

if (tampered === 0) {
  console.error(`no entry with seq ${seq} in ${inPath}`);
  process.exit(2);
}
writeFileSync(outPath, out.join("\n") + "\n");
console.log(
  JSON.stringify(
    { ok: true, entries: lines.length, tampered_seq: seq, out: outPath, note: "hash and signature left untouched" },
    null,
    2
  )
);
