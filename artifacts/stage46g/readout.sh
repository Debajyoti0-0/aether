#!/bin/sh
import re
segs = []
cur = None
for line in open("/tmp/capture-3890.log"):
    m = re.match(r"\[[\d.]+\] (S->C) (\d+) bytes", line)
    if m:
        cur = (m.group(1), m.group(2), [])
        continue
    if cur is not None:
        m2 = re.match(r"  [0-9a-f]{4}  ((?:[0-9a-f]{2} )+[0-9a-f]{2}?)", line)
        if m2:
            cur[2].extend(int(b, 16) for b in m2.group(1).split())
            continue
        if re.match(r"\[[\d.]+\] (C->S|S->C)", line):
            if cur[1] != "0":
                segs.append(cur)
            cur = None
if cur and cur[1] != "0":
    segs.append(cur)
print(f"total S->C segments: {len(segs)}")
for i, (tag, n, data) in enumerate(segs[-4:]):
    head = " ".join(f"{b:02x}" for b in data[:40])
    print(f"seg{i}: {tag} declared={n} parsed={len(data)} head={head}")
