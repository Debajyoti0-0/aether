# Release Integrity Notice

## v4.0.0-rc2 Tag Integrity Issue

**The `v4.0.0-rc2` tag is BROKEN and NOT VALID for release consumption.**

### Details
- **Tag:** `v4.0.0-rc2`
- **Target Commit:** `72d17d2ad1f92eb9533e6e2071b0edaa010aaecb`
- **Target Commit VERSION:** `4.0.0-rc1`
- **Tag Claims:** `v4.0.0-rc2`
- **MISMATCH:** Tag claims rc2 but target commit has VERSION=4.0.0-rc1

### Root Cause
Tag was created at commit `72d17d2` (VERSION=4.0.0-rc1) before the version bump to 4.0.0-rc2 in commit `5cd008b`.

### DO NOT USE THIS TAG FOR RELEASE CONSUMPTION.

### Correct RC2 Candidate
Commit `5cd008b` (VERSION=4.0.0-rc2) — tag needs to be recreated there.

### SHA Collision Warning
Tag `v3.8.0-stage8-backfill` also points to `72d17d2`, creating a SHA collision with `v4.0.0-rc2`.