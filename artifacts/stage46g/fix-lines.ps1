$ErrorActionPreference = 'Stop'
foreach ($p in @('internal\cli\ad\enum.go', 'internal\cli\ad\roast.go')) {
    $c = Get-Content $p -Raw
    # Regex over the rune-iteration bug (tolerates CRLF/LF and exact indent).
    $pattern = 'for _, line := range string\(data\) \{\s*\r?\n\s*if line := string\(line\); line != "" \{\s*\r?\n\s*(usernames|spns) = append\(\1, line\)\s*\r?\n\s*\}\s*\r?\n\s*\}'
    $replacement = "for _, line := range strings.Split(string(data), `"\\n`") {`r`n`t`t`t`t`tline = strings.TrimSpace(line)`r`n`t`t`t`t`tif line != ````"" {`r`n`t`t`t`t`t`t`$1 = append(`$1, line)`r`n`t`t`t`t`t}`r`n`t`t`t`t}"
    $count = ([regex]::Matches($c, $pattern)).Count
    Write-Host "$p rune-iteration OCCURRENCES: $count"
    $c = [regex]::Replace($c, $pattern, $replacement)
    # Ensure strings import
    if ($c -notmatch '"strings"') {
        $c = $c -replace '(import \(\s*\r?\n)', "`$1`t`"strings`"`r`n`"
        Write-Host "$p : added strings import"
    }
    Set-Content $p $c -NoNewline
}
