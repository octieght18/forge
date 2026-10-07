#requires -Version 7.0
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$count = 0
foreach ($file in Get-ChildItem -LiteralPath $PSScriptRoot -Filter '*.ps1') {
    $parseTokens=$null; $parseErrors=$null
    [System.Management.Automation.Language.Parser]::ParseFile($file.FullName,[ref]$parseTokens,[ref]$parseErrors) | Out-Null
    if ($parseErrors) { throw "PowerShell syntax errors in $($file.Name): $parseErrors" }
    $count++
}
$blocks = 0
$documents = @(Get-Item -LiteralPath (Join-Path $repo 'README.md')) + @(Get-ChildItem -LiteralPath (Join-Path $repo 'docs') -Filter '*.md' -Recurse)
foreach ($file in $documents) {
    $source = Get-Content -LiteralPath $file.FullName -Raw
    foreach ($match in [regex]::Matches($source, '(?ms)^```powershell\r?\n(.*?)^```[^\r\n]*')) {
        $parseTokens=$null; $parseErrors=$null
        [System.Management.Automation.Language.Parser]::ParseInput($match.Groups[1].Value,[ref]$parseTokens,[ref]$parseErrors) | Out-Null
        if ($parseErrors) { throw "PowerShell example syntax errors in $($file.Name): $parseErrors" }
        $blocks++
    }
}
Write-Output "Parsed $count PowerShell scripts and $blocks documented PowerShell blocks"
