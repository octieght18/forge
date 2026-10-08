param(
    [Parameter(Mandatory)][ValidateSet('apply','status','delete')][string]$Action,
    [Parameter(Mandatory)][string]$WorkloadId,
    [string]$Distro = 'Ubuntu',
    [string]$ServiceUser = 'owner',
    [string]$TokenFile
)
$ErrorActionPreference = 'Stop'
$repoPath = Split-Path -Parent $PSScriptRoot
$linuxRepo = (& wsl.exe -d $Distro -- wslpath -a -u $repoPath.Replace('\','/')).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot resolve repository in WSL' }
$arguments = @('-d',$Distro,'-u',$ServiceUser,'--','python3',"$linuxRepo/deploy/kubernetes/environment_cli.py",$Action,'--workload',$WorkloadId)
if ($TokenFile) { $arguments += @('--token-file',$TokenFile) }
& wsl.exe @arguments
if ($LASTEXITCODE -ne 0) { throw 'Environment intent operation failed' }
