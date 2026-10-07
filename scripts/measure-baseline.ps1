#requires -Version 7.0
param(
    [string]$OutputDirectory = '',
    [ValidateRange(1,20)][int]$StartupTrials = 5,
    [ValidateRange(1,100)][int]$RegistrationTrials = 10,
    [string]$Distro = 'Ubuntu',
    [string]$ServiceUser = 'owner'
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$native = Join-Path $PSScriptRoot 'local-stack.ps1'
if (!$OutputDirectory) { $OutputDirectory = Join-Path $repo ('tmp/f08/' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')) }
$outputPath = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $outputPath) { throw 'Choose a new results directory; previous raw runs are never overwritten' }
New-Item -ItemType Directory -Path $outputPath | Out-Null
$rawFile = Join-Path $outputPath 'raw.jsonl'
$linuxRepo = (& wsl.exe -d $Distro --exec wslpath -u $repo.Replace('\','/')).Trim()
$homePath = (& wsl.exe -d $Distro -u $ServiceUser --exec printenv HOME).Trim()
$statePath = "$homePath/.local/share/forge-native"
if ($LASTEXITCODE -ne 0 -or !$linuxRepo -or !$homePath) { throw 'Cannot resolve native paths' }
$sequence = 0

function Write-Record($phase, $trial, $start, $watch, $outcome, $command, $details = @{}) {
    $script:sequence++
    $record = [ordered]@{ schema_version=1; record_id=('r{0:D4}' -f $script:sequence); phase=$phase; trial=$trial;
        start_utc=$start.ToString('o'); end_utc=[DateTime]::UtcNow.ToString('o'); elapsed_ms=[math]::Round($watch.Elapsed.TotalMilliseconds,3);
        outcome=$outcome; command_id=$command; human_effort_seconds=$null; details=$details }
    [IO.File]::AppendAllText($rawFile, (($record | ConvertTo-Json -Depth 12 -Compress) + "`n"), [Text.UTF8Encoding]::new($false))
}

function Measure-Native($phase, $trial, $action, $expectedFailure = $false) {
    $start = [DateTime]::UtcNow; $watch = [Diagnostics.Stopwatch]::StartNew()
    $failed = $false; $diagnostics = [Collections.Generic.List[string]]::new()
    try { & $native -Action $action -Distro $Distro -ServiceUser $ServiceUser *>&1 | ForEach-Object { $diagnostics.Add([string]$_) } }
    catch { $failed = $true }
    finally { $watch.Stop() }
    # Publish only a known safe diagnostic, never arbitrary process output.
    $busyPort = [bool]($diagnostics -match 'Native stack operation failed: OS error 98')
    $outcome = if ($expectedFailure -and $failed -and $busyPort) { 'expected_failure' } elseif (!$expectedFailure -and !$failed) { 'success' } else { 'failure' }
    Write-Record $phase $trial $start $watch $outcome ('native_' + $action) @{ expected_failure=$expectedFailure; command_failed=$failed; diagnostic=$(if($busyPort){'address_in_use'}else{$null}) }
    if ($outcome -eq 'failure') { throw "Unexpected native outcome in $phase trial $trial; raw result retained" }
}

$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
function Measure-Request($trial, $step, $method, $path, $body, $expectedStatus, $bearer) {
    $parameters = @{ Uri=('http://127.0.0.1:8081' + $path); Method=$method; Headers=@{ Authorization=('Bearer ' + $bearer) };
        WebSession=$session; TimeoutSec=10; SkipHttpErrorCheck=$true }
    if ($null -ne $body) { $parameters.Body = [Text.Encoding]::UTF8.GetBytes(($body | ConvertTo-Json -Depth 20 -Compress)); $parameters.ContentType='application/json' }
    $start=[DateTime]::UtcNow; $watch=[Diagnostics.Stopwatch]::StartNew(); $status=$null; $outcome='failure'; $payload=$null
    try {
        $response = Invoke-WebRequest @parameters
        $watch.Stop(); $status=[int]$response.StatusCode
        if ($status -eq $expectedStatus -and $response.Headers['X-Request-ID']) {
            $payload=$response.Content | ConvertFrom-Json
            $outcome='success'
        }
    } finally {
        $watch.Stop()
        Write-Record 'http_request' $trial $start $watch $outcome $step @{ method=$method; expected_status=$expectedStatus; actual_status=$status }
    }
    if ($outcome -ne 'success') { throw "Request failed in trial $trial step $step; no token or response body logged" }
    return $payload
}

$cpu=Get-CimInstance Win32_Processor | Select-Object @{Name='name';Expression={$_.Name.Trim()}},NumberOfCores,NumberOfLogicalProcessors
$os=Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,BuildNumber,@{Name='physical_memory_gib';Expression={[math]::Round($_.TotalVisibleMemorySize/1MB,3)}}
$nativeConditions = (& wsl.exe -d $Distro --exec python3 -c 'import json,os,platform,subprocess; print(json.dumps(dict(kernel=platform.release(),python=platform.python_version(),memory_kib=int(next(x for x in open("/proc/meminfo") if x.startswith("MemTotal:")).split()[1]),swap_kib=int(next(x for x in open("/proc/meminfo") if x.startswith("SwapTotal:")).split()[1]),postgres=subprocess.check_output(["/usr/lib/postgresql/18/bin/postgres","--version"],text=True).strip(),java=subprocess.run(["java","-version"],capture_output=True,text=True).stderr.splitlines()[0],cpu_visible=os.cpu_count())))') | ConvertFrom-Json
$beforeStatus = (& $native -Action status -Distro $Distro -ServiceUser $ServiceUser) | ConvertFrom-Json
$metadata = [ordered]@{
    schema_version=1; protocol='f08.native.v1'; recorded_utc=[DateTime]::UtcNow.ToString('o'); local_timezone=[TimeZoneInfo]::Local.Id;
    platform='Windows client to native Ubuntu WSL services'; cpu=$cpu; windows=$os; native=$nativeConditions;
    powershell_version=$PSVersionTable.PSVersion.ToString(); base_commit=(& git -C $repo rev-parse HEAD).Trim();
    source_tree_dirty=[bool]((& git -C $repo status --porcelain) | Select-Object -First 1);
    harness_sha256=(Get-FileHash -LiteralPath $PSCommandPath -Algorithm SHA256).Hash.ToLowerInvariant();
    initial_units=$beforeStatus.units; background_conditions='Codex and local Wekan remain active; other host activity is uncontrolled';
    startup_class='retained-data restart; prerequisites/archive and build caches already present';
    startup_includes=@('WSL entry','Go build','explicit migration and grants','database and Keycloak readiness','API readiness','Windows loopback verification');
    startup_excludes=@('package installation','initial archive download/extraction','fresh cluster/realm initialization','human effort');
    registration_includes=@('four sequential Windows HTTP requests','request construction','response parsing and checks');
    registration_excludes=@('one-time PKCE token setup','human login/typing time','workflow/model execution');
    expected_trials=@{retained_startup=$StartupTrials; registration_journey=$RegistrationTrials; http_request=($RegistrationTrials*4); automated_pkce_setup=1; occupied_port_detection=1; port_release_recovery=1};
    human_effort_seconds=$null; performance_targets='awaiting owner review; none applied';
    fixture_policy='synthetic workload/version per journey retained; existing native data never deleted';
    commands=@{ native_up='.\scripts\local-stack.ps1 up'; native_stop='.\scripts\local-stack.ps1 stop'; create_workload='POST /api/v1/workloads'; create_version='POST /api/v1/workloads/{workload_id}/versions'; read_workload='GET /api/v1/workloads/{workload_id}'; read_version='GET /api/v1/workloads/{workload_id}/versions/{version_id}' }
}
$metadata | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath (Join-Path $outputPath 'metadata.json') -Encoding utf8

try {
    for ($trial=1; $trial -le $StartupTrials; $trial++) {
        Measure-Native 'orderly_stop' $trial 'stop'
        Measure-Native 'retained_startup' $trial 'up'
        Write-Output "Retained startup trial $trial recorded."
    }
    # Real code/PKCE setup is recorded separately; passwords/tokens stay in memory.
    $setupStart=[DateTime]::UtcNow; $setupWatch=[Diagnostics.Stopwatch]::StartNew()
    $setupCode='import sys,json; from pathlib import Path; sys.path.insert(0,sys.argv[1]+"/deploy/native"); import smoke; r=Path(sys.argv[2]); v=json.loads((r/"secrets.json").read_text()); smoke.login(r,"ahmad",v["ahmad"])'
    & wsl.exe -d $Distro -u $ServiceUser --exec python3 -c $setupCode $linuxRepo $statePath | Out-Null
    $setupFailed=$LASTEXITCODE -ne 0; $setupWatch.Stop()
    Write-Record 'automated_pkce_setup' 1 $setupStart $setupWatch $(if($setupFailed){'failure'}else{'success'}) 'actual_keycloak_pkce_helper'
    if ($setupFailed) { throw 'Private PKCE setup failed' }
    $token = ((& wsl.exe -d $Distro -u $ServiceUser --exec cat "$statePath/tokens/ahmad.json") | ConvertFrom-Json).access_token
    $versionBody = Get-Content -LiteralPath (Join-Path $repo 'internal/contract/examples/create-version.json') -Raw | ConvertFrom-Json
    $prefix='f08-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,6)
    for ($trial=1; $trial -le $RegistrationTrials; $trial++) {
        $workload=$null; $version=$null
        $start=[DateTime]::UtcNow; $watch=[Diagnostics.Stopwatch]::StartNew(); $outcome='failure'
        try {
            $workload=Measure-Request $trial 'create_workload' 'POST' '/api/v1/workloads' @{name="$prefix-$trial";description='Synthetic F08 baseline fixture'} 201 $token
            $path='/api/v1/workloads/' + $workload.workload_id
            $version=Measure-Request $trial 'create_version' 'POST' ($path+'/versions') $versionBody 201 $token
            $readWorkload=Measure-Request $trial 'read_workload' 'GET' $path $null 200 $token
            $readVersion=Measure-Request $trial 'read_version' 'GET' ($path+'/versions/'+$version.version_id) $null 200 $token
            if ($readWorkload.workload_id -ne $workload.workload_id -or $readVersion.version_id -ne $version.version_id) { throw 'Readback identity mismatch' }
            $outcome='success'
        } finally {
            $watch.Stop()
            Write-Record 'registration_journey' $trial $start $watch $outcome 'register_version_readback' @{workload_id=$workload.workload_id; version_id=$version.version_id}
        }
    }
    Remove-Variable token
    Measure-Native 'pre_failure_stop' 1 'stop'
    $info=[Diagnostics.ProcessStartInfo]::new('wsl.exe'); $info.UseShellExecute=$false; $info.CreateNoWindow=$true
    $info.RedirectStandardOutput=$true; $info.RedirectStandardInput=$true; $info.RedirectStandardError=$true
    foreach ($argument in @('-d',$Distro,'-u',$ServiceUser,'--exec','python3',"$linuxRepo/scripts/baseline/busy_port.py")) { $info.ArgumentList.Add($argument) }
    $holder=[Diagnostics.Process]::Start($info)
    try {
        $line=$holder.StandardOutput.ReadLineAsync()
        if (!$line.Wait(30000) -or $line.Result -ne 'READY') { throw 'Isolated occupied-port fixture failed' }
        Measure-Native 'occupied_port_detection' 1 'up' $true
    } finally {
        $holder.StandardInput.Close()
        if (!$holder.WaitForExit(10000)) { $holder.Kill($true) }
        $holder.Dispose()
    }
    # Starts immediately after the fixture process has released the port. Human
    # diagnosis/think time is absent; this is programmatic recovery only.
    Measure-Native 'port_release_recovery' 1 'up'
    Write-Output "Baseline recorded in $outputPath."
} finally {
    if (Get-Variable token -ErrorAction SilentlyContinue) { Remove-Variable token }
}
