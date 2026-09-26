# Regenerates the Stage 52 evidence from scratch on a clean tree.
#
# Every command it runs is also runnable by hand; this file exists so the
# evidence can be reproduced without reconstructing the session that produced it.
# Output lands in artifacts/stage52/.
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/stage52-evidence.ps1
#
# Nothing here fabricates a result. Commands that cannot run in this environment
# print NOT PERFORMED with the reason, and the gate matrix records them as such.

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $repo

$out = 'artifacts\stage52'
New-Item -ItemType Directory -Force -Path "$out\baseline", "$out\wasm", "$out\browser", "$out\race" | Out-Null

function Section($name) {
  Write-Output ''
  Write-Output "--- $name ---"
}

$stamp = (Get-Date).ToUniversalTime().ToString('yyyy-MM-dd HH:mm:ss') + ' UTC'
$head = (git rev-parse HEAD).Trim()
$dirty = (git status --porcelain | Measure-Object).Count

Section 'environment'
$envText = @()
$envText += "date: $stamp"
$envText += "HEAD: $head"
$envText += "worktree entries: $dirty"
$envText += "VERSION: $((Get-Content VERSION -Raw).Trim())"
$envText += "go: $((go version))"
$envText += "node: $((node --version))"
try { $envText += "rustc: $((rustc --version 2>$null))" } catch {}
$envText += "wasm-pack: $((wasm-pack --version 2>$null))"
$envText += "make: $((mingw32-make --version 2>$null | Select-Object -First 1))"
$envText += "chrome: $((Test-Path 'C:\Program Files\Google\Chrome\Application\chrome.exe'))"
$envText | Set-Content "$out\baseline\environment.txt" -Encoding UTF8
$envText | Write-Output

Section 'gates'
$script:gates = @()
function Gate($name, $cmd, $logfile) {
  $sw = [Diagnostics.Stopwatch]::StartNew()
  # A tool that writes progress to stderr (cargo does) is not a failure, and
  # with Stop in effect the first such line would abort the run. The exit code
  # is the verdict, not the presence of stderr output.
  $prev = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  $text = & powershell -NoProfile -Command $cmd 2>&1 | Out-String
  $code = $LASTEXITCODE
  $ErrorActionPreference = $prev
  $sw.Stop()
  $text | Set-Content $logfile -Encoding UTF8
  $script:gates += [pscustomobject]@{ gate = $name; exit = $code; seconds = [math]::Round($sw.Elapsed.TotalSeconds, 1) }
  Write-Output ("{0,-28} exit={1} {2}s" -f $name, $code, [math]::Round($sw.Elapsed.TotalSeconds, 1))
}

Gate 'go build ./...'            'go build ./...'                          "$out\baseline\go-build.txt"
Gate 'go vet ./...'              'go vet ./...'                            "$out\baseline\go-vet.txt"
Gate 'go test -count=1 ./...'    'go test -count=1 ./...'                  "$out\baseline\go-test.txt"
Gate 'go test -race ./internal/web' 'go test -race -count=1 ./internal/web' "$out\race\go-test-race-web.txt"
Gate 'cargo test (graph)'        'cargo test --manifest-path internal/web/wasm/graph/Cargo.toml'  "$out\wasm\cargo-test-graph.txt"
Gate 'cargo test (verify)'       'cargo test --manifest-path internal/web/wasm/verify/Cargo.toml' "$out\wasm\cargo-test-verify.txt"
Gate 'node wasm smoke test'      'node internal/web/wasm/smoke-test.mjs'   "$out\wasm\smoke-test.txt"

# Fuzzing is run for a fixed wall-clock budget, not until a crash. The point of
# the evidence is that the targets ran and found nothing, and a run that stops
# whenever it likes cannot show that.
foreach ($t in @('FuzzAuditChainJSON', 'FuzzGraphJSON', 'FuzzPartialQuery', 'FuzzTimeTravelSeq')) {
  Gate "fuzz $t" "go test -run '^$' -fuzz '^$t`' -fuzztime 20s ./internal/web" "$out\race\fuzz-$t.txt"
}

$script:gates | Format-Table -AutoSize | Out-String | Set-Content "$out\baseline\quality-gates.txt" -Encoding UTF8
$script:gates | Format-Table -AutoSize | Write-Output

Section 'assets'
$assets = @()
foreach ($p in @(
    'internal/web/static/app.js', 'internal/web/static/graph.js',
    'internal/web/static/verifier.js', 'internal/web/static/verify.js',
    'internal/web/static/style.css', 'internal/web/wasm/graph.wasm',
    'internal/web/wasm/verify.wasm')) {
  if (Test-Path $p) {
    $h = (Get-FileHash $p -Algorithm SHA256).Hash.ToLower()
    $assets += ("{0,-44} {1,8} bytes  sha256:{2}" -f $p, (Get-Item $p).Length, $h)
  } else {
    $assets += ("{0,-44} MISSING" -f $p)
  }
}
$assets | Set-Content "$out\baseline\asset-checksums.txt" -Encoding UTF8
$assets | Write-Output

Write-Output ''
Write-Output 'The live browser run and the tamper check are performed by hand against a'
Write-Output 'running dashboard; see artifacts/stage52/browser/README.md.'
