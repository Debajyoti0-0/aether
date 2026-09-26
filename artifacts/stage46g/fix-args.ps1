$p = 'internal\cli\ad\ldap.go'
$c = Get-Content $p -Raw
$old = 'e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])'
$new = "e.SetBindCredentials(fmt.Sprintf(`"%s@%s`", args[0], domain), args[1])`r`n`t`t`te.SetBaseDN(domainToBaseDN(domain))"
$count = ([regex]::Matches($c, [regex]::Escape($old))).Count
Write-Host "OCCURRENCES: $count"
$c = $c.Replace($old, $new)
Set-Content $p $c -NoNewline
