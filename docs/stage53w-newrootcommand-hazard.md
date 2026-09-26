# Stage 53W — The Partial-Command-Tree Hazard

Captured (UTC): 2026-09-26. Fix commit: `c624f55` (Stage 53V).
Regression guard: `4364ab4` (Stage 53W, flag-matrix completeness).

## The defect class

A CLI-tree constructor whose **documentation promises a complete tree but whose
implementation returns a partial one**, where the missing part is attached
somewhere else in the call path.

Any consumer that builds the tree directly — a documentation generator, an
inventory tool, a matrix builder, a linter rule, a test — receives a tree that is
silently incomplete. Nothing errors. Nothing warns. The output looks entirely
plausible: fewer rows, no gaps, a clean JSON document.

That is what makes this class dangerous. A crash is a nuisance; a short inventory
that reads as complete is a corrupted evidence base.

## The specific instance

`internal/cli/root.go` had:

```go
// NewRootCommand builds and returns the full command tree. It is used
// both by Execute() and by the docs/man generators.
func NewRootCommand() *cobra.Command { ... }

func Execute() error {
    loadModules()                    // <-- module commands attached HERE
    return NewRootCommand().Execute()
}
```

`loadModules()` walks the `Module` registry and calls `rootCmd.AddCommand` for
every registered module. The registry is populated by `init()` in module packages,
which reach it through **blank imports** — `cmd/aether/main.go` has
`_ "github.com/Debajyoti0-0/aether/internal/cli/ad"`.

`NewRootCommand()` did not call it. So every generator that walked the tree saw a
root with none of the module commands attached.

**A second, independent mechanism** was also in play, and the two are worth
keeping distinct because only one of them is laziness:

- **Module commands were never attached at all** — not lazy, simply absent. The
  `ad` subtree (25 commands) and the `ldap` subtree (14) were missing.
- **cobra's `help` and `completion` commands** *are* genuinely lazy. cobra
  attaches them inside `ExecuteC` via `InitDefaultHelpCmd` /
  `InitDefaultCompletionCmd`, so a direct tree walk never saw them either (6
  commands).
- **cobra's `--help` and `--version` flags** are likewise attached during
  execution, per command, via `InitDefaultHelpFlag` / `InitDefaultVersionFlag`.
  This is what made the flag matrix 945 entries against 1101 real bindings.

## The consequence

The Stage 54 matrices contained **110 commands against the binary's 152**. Every
`ad` and `ldap` command was absent — that is the entire engagement-gated surface,
the fail-closed posture that Stages 53S and 53V spent their effort hardening.

Any gate that consumed those matrices was evaluating an incomplete universe, and
the most safety-critical commands in the tool were the ones missing. The failure
was invisible because the artifacts were well-formed, plausibly sized, and
committed.

Stage 53S's own inventory (`artifacts/stage53s/cli-inventory.mjs`) had found 152
by walking the **binary** rather than the API. Stage 54's Go generator walked the
API and got 110. Two methods, two answers, and nothing reconciled them — which is
the actual lesson.

## The fix (Fix 1, applied)

`NewRootCommand()` now loads the modules itself, so its documented contract is
true for every caller:

```go
func NewRootCommand() *cobra.Command {
    loadModules()
    ...
}
```

`loadModules()` is guarded by a `modulesLoaded` flag. This guard is not optional:
cobra's `AddCommand` appends unconditionally, so an unguarded second call — and
`Execute()` still calls it — would register the same `*cobra.Command` pointers
twice and duplicate every module subcommand in help output and in every generated
inventory. Verified after the fix: 155 nodes, no duplicates at any level.

The generators additionally call `InitDefaultHelpCmd`, `InitDefaultCompletionCmd`,
`InitDefaultHelpFlag` and `InitDefaultVersionFlag` so the in-process tree matches
what the binary exposes.

## The general rule

**Any tool that enumerates a CLI tree must derive it from the same entry point the
binary uses, and must reconcile its output against the binary before trusting it.**

Concretely, for this codebase:

1. Call `cli.NewRootCommand()`, never construct a root yourself.
2. Trigger cobra's lazy attachments explicitly (`InitDefaultHelpCmd`,
   `InitDefaultCompletionCmd`, and per-command `InitDefaultHelpFlag` /
   `InitDefaultVersionFlag`).
3. Import the module packages that register commands, or accept that the tree is
   partial.
4. **Diff the result against the binary's own `--help` output** and require
   0 missing / 0 extra before publishing any matrix.

Step 4 is the one that actually catches this class, and it is the reason
`artifacts/stage53w/matrix-verification.txt` exists. Both prior inventories were
self-consistent and mutually contradictory; only comparison against an
independent source of truth exposed it.

## Verification status

| Check | Result |
| --- | --- |
| Binary tree vs `command-details.json` | 155 vs 155, 0 missing, 0 extra |
| Binary tree vs `command-inventory.json` | 154 nested + root = 155, 0 missing, 0 extra |
| Binary flag bindings vs `flag-matrix.json` | 1101 vs 1101, 0 missing, 0 extra, 0 duplicates |
| Reconciliation with Stage 53S's 152 | exact: 155 − root − help − completion = 152 |
| Duplicate subcommands in help output | none, at every level |
| Six-matrix reproducibility | byte-identical, drift 0 of 6 |
| Regression guard | `go build`, `go vet`, `go test`, `go test -race`, `golangci-lint` all pass |

## Note on an earlier misdescription

The Stage 53W brief stated the generators "were fixed to call `Execute()`." They
were not. Calling `Execute()` from a generator is not a fix — it would *run* the
root command, and its own `RunE` prints help. The actual fix was to move
`loadModules()` into `NewRootCommand()`. The distinction matters: one is a
workaround that executes the program you are trying to inspect; the other makes
the API match its contract.
