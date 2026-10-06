param(
    [ValidateSet('up', 'stop', 'status', 'login')][string]$Action = 'up',
    [string]$Distro = 'Ubuntu',
    [string]$ServiceUser = 'owner',
    [string]$Go = 'go',
    [string]$KeycloakArchive = ''
)
$ErrorActionPreference = 'Stop'
$repoPath = Split-Path -Parent $PSScriptRoot
$linuxRepo = (& wsl.exe -d $Distro -- wslpath -a -u $repoPath.Replace('\', '/')).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot resolve repository in WSL' }
if ($Action -eq 'login') {
    & wsl.exe -d $Distro -u $ServiceUser -- bash -c 'exec "$HOME/.local/share/forge-native/bin/forge-login" --token-file "$HOME/.local/share/forge-native/tokens/access.json"'
} else {
    $stackArgs = @('-d', $Distro, '-u', 'root', '--', 'python3', "$linuxRepo/deploy/native/stack.py", $Action, '--user', $ServiceUser, '--go', $Go)
    if ($KeycloakArchive) {
        $linuxArchive = (& wsl.exe -d $Distro -- wslpath -a -u $KeycloakArchive.Replace('\', '/')).Trim()
        $stackArgs += @('--keycloak-archive', $linuxArchive)
    }
    & wsl.exe @stackArgs
}
if ($LASTEXITCODE -ne 0) { throw 'Native Forge operation failed' }
if ($Action -eq 'up') {
    # WSL systemd services do not keep the VM alive. This unprivileged helper
    # holds it while Forge units run and exits after normal stop.
    Start-Process -FilePath wsl.exe -ArgumentList @('-d', $Distro, '-u', $ServiceUser, '--', 'python3', "$linuxRepo/deploy/native/stack.py", 'hold', '--user', $ServiceUser) -WindowStyle Hidden | Out-Null
    $connected = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try {
            $ready = Invoke-RestMethod 'http://127.0.0.1:8081/readyz' -TimeoutSec 2
            $discovery = Invoke-RestMethod 'http://127.0.0.1:8082/realms/forge/.well-known/openid-configuration' -TimeoutSec 2
            $connected = $true
            break
        } catch { Start-Sleep -Seconds 1 }
    }
    if (!$connected) { throw 'Windows cannot reach the native WSL stack on loopback' }
    if ($ready.status -ne 'ok' -or $discovery.issuer -ne 'http://127.0.0.1:8082/realms/forge') { throw 'Windows loopback validation failed' }
    Write-Output 'Windows loopback connectivity verified.'
}
