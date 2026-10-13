$ErrorActionPreference = 'Stop'

# Verifies the documented development loop: a real `vite dev` server plus the Go
# API, with Vite proxying /api and /uploads to the backend.
#
# A passing `vite build` does NOT prove any of this -- the dev server, the
# proxy config and the on-demand SFC transforms are separate code paths.
#
# NOTE: keep this file 100% ASCII. Windows PowerShell 5.1 reads a BOM-less .ps1
# as ANSI/CP936, and an odd byte count in a CJK comment swallows the next line.

$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$be = Join-Path $root 'backend'
$run = Join-Path $root '.smokedev'
$goPort = 8080
$webPort = 5173
$web = "http://localhost:$webPort"

$env:GOCACHE = Join-Path $root '.gocache'
$env:GOTMPDIR = Join-Path $root '.gotmp'
$env:GOFLAGS = '-mod=mod'
$env:GOPROXY = 'off'
$env:GOSUMDB = 'off'
$env:GOTOOLCHAIN = 'local'

$script:failures = 0

function Check($name, $condition, $detail) {
    if ($condition) { Write-Output "PASS: $name" }
    else { Write-Output "FAIL: $name -- $detail"; $script:failures = $script:failures + 1 }
}

function Fetch($url) {
    $req = [System.Net.WebRequest]::Create($url)
    $req.Proxy = $null
    $req.Timeout = 20000
    try {
        $res = $req.GetResponse()
        $code = [int]$res.StatusCode
        $sr = New-Object System.IO.StreamReader($res.GetResponseStream())
        $body = $sr.ReadToEnd()
        $type = $res.ContentType
        $sr.Close(); $res.Close()
        return New-Object psobject -Property @{ Status = $code; Body = $body; Type = $type }
    } catch [System.Net.WebException] {
        $res = $_.Exception.Response
        if ($res -eq $null) { return New-Object psobject -Property @{ Status = 0; Body = $_.Exception.Message; Type = '' } }
        $sr = New-Object System.IO.StreamReader($res.GetResponseStream())
        $body = $sr.ReadToEnd(); $type = $res.ContentType
        $sr.Close()
        return New-Object psobject -Property @{ Status = [int]$res.StatusCode; Body = $body; Type = $type }
    }
}

function WaitPort($port) {
    for ($i = 0; $i -lt 120; $i++) {
        Start-Sleep -Milliseconds 250
        $tcp = New-Object System.Net.Sockets.TcpClient
        try { $tcp.Connect('127.0.0.1', $port); $tcp.Close(); return $true } catch { $tcp.Dispose() }
    }
    return $false
}

# A bare TcpClient is IPv4-only, so a TCP probe cannot see a dev server that
# bound ::1. Poll over HTTP instead -- WebRequest tries both families.
function WaitHttp($url) {
    for ($i = 0; $i -lt 120; $i++) {
        Start-Sleep -Milliseconds 250
        if ((Fetch $url).Status -gt 0) { return $true }
    }
    return $false
}

Remove-Item -Recurse -Force $run -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path (Join-Path $run 'data\uploads') | Out-Null

# staticDir is empty on purpose: this check is about the dev server, and the
# backend must stay API-only so the proxy has something unambiguous to reach.
$config = @(
    'server:'
    "  addr: `"127.0.0.1:$goPort`""
    '  staticDir: ""'
    'data:'
    '  database: "data/blog.db"'
    '  uploadDir: "data/uploads"'
    '  uploadUrl: "/uploads"'
    'auth:'
    '  secretFile: "data/jwt.secret"'
    'log:'
    '  level: "warn"'
)
Set-Content -Path (Join-Path $run 'config.yaml') -Value $config -Encoding ASCII

Write-Output '=== build ==='
Push-Location $be
& go build -o (Join-Path $run 'server.exe') ./cmd/server
$code = $LASTEXITCODE
Pop-Location
if ($code -ne 0) { Write-Output "FAIL: go build exit=$code"; exit 1 }
Write-Output 'PASS: go build'

# Fail fast if something else already owns the port. Without this the probe
# below connects happily to a foreign server and reports a bogus PASS while our
# own backend silently died on `bind:`.
$busy = $false
$probe = New-Object System.Net.Sockets.TcpClient
try { $probe.Connect('127.0.0.1', $goPort); $busy = $true; $probe.Close() } catch { $probe.Dispose() }
if ($busy) {
    Write-Output "FAIL: port $goPort is already in use -- stop the running dev server first"
    exit 1
}

$goProc = Start-Process -FilePath (Join-Path $run 'server.exe') -ArgumentList '-conf', 'config.yaml' `
    -WorkingDirectory $run -RedirectStandardOutput (Join-Path $run 'go.out') `
    -RedirectStandardError (Join-Path $run 'go.err') -PassThru

$viteProc = $null
try {
    if (-not (WaitPort $goPort)) {
        Write-Output 'FAIL: backend did not come up'
        Get-Content (Join-Path $run 'go.out'), (Join-Path $run 'go.err') -ErrorAction SilentlyContinue
        exit 1
    }
    # And confirm the listener is really ours, not a leftover that won the race.
    Start-Sleep -Milliseconds 500
    $goErr = Get-Content (Join-Path $run 'go.err') -Raw -ErrorAction SilentlyContinue
    if ($goProc.HasExited -or ($goErr -and $goErr -match 'bind:')) {
        Write-Output 'FAIL: backend exited instead of serving -- port conflict?'
        Get-Content (Join-Path $run 'go.out'), (Join-Path $run 'go.err') -ErrorAction SilentlyContinue
        exit 1
    }
    Write-Output "PASS: backend up on $goPort"

    $viteProc = Start-Process -FilePath 'node' -ArgumentList (Join-Path $root 'node_modules\vite\bin\vite.js') `
        -WorkingDirectory $root -RedirectStandardOutput (Join-Path $run 'vite.out') `
        -RedirectStandardError (Join-Path $run 'vite.err') -PassThru

    if (-not (WaitHttp "$web/")) {
        Write-Output 'FAIL: vite dev did not come up'
        Get-Content (Join-Path $run 'vite.out'), (Join-Path $run 'vite.err') -ErrorAction SilentlyContinue
        exit 1
    }
    Write-Output 'PASS: vite dev up on 5173'

    $r = Fetch "$web/"
    Check 'dev server serves the shell' ($r.Status -eq 200 -and $r.Body -match 'id="app"') "status=$($r.Status)"
    Check 'dev shell uses the /src/main.ts entry' ($r.Body -match '/src/main\.ts') 'entry module missing'

    Write-Output '=== vite transforms the entry and its graph ==='
    $r = Fetch "$web/src/main.ts"
    Check 'GET /src/main.ts is transformed JS' `
        ($r.Status -eq 200 -and $r.Type -match 'javascript' -and $r.Body -match 'createApp') `
        "status=$($r.Status) type=$($r.Type)"

    # One file per risky area: the shell, the public layout, a page, the editor
    # (heaviest dependency graph) and the admin editor (largest SFC).
    foreach ($mod in @(
        '/src/App.vue',
        '/src/layouts/PublicLayout.vue',
        '/src/views/HomeView.vue',
        '/src/components/editor/RichTextEditor.vue',
        '/src/views/admin/PostEditorView.vue',
        '/src/styles/tokens.css'
    )) {
        $r = Fetch "$web$mod"
        Check "transform $mod" ($r.Status -eq 200) "status=$($r.Status)"
    }

    Write-Output '=== the api proxy is wired ==='
    $r = Fetch "$web/api/v1/site"
    Check 'GET /api/v1/site through the proxy is JSON' `
        ($r.Status -eq 200 -and $r.Type -match 'json' -and $r.Body -match '"title"') `
        "status=$($r.Status) type=$($r.Type) body=$($r.Body)"

    $r = Fetch "$web/api/v1/auth/setup"
    Check 'setup probe answers through the proxy' `
        ($r.Status -eq 200 -and $r.Body -match 'initialised') "status=$($r.Status) body=$($r.Body)"

    $r = Fetch "$web/api/v1/admin/posts"
    Check 'proxied admin route is still guarded (401)' ($r.Status -eq 401) "status=$($r.Status)"

    Write-Output '=== no transform errors were logged ==='
    $viteLog = Get-Content (Join-Path $run 'vite.out'), (Join-Path $run 'vite.err') -ErrorAction SilentlyContinue | Out-String
    Check 'vite logged no errors' ($viteLog -notmatch '(?i)\berror\b|failed to') $viteLog
} finally {
    if ($viteProc -and -not $viteProc.HasExited) { $viteProc.Kill() }
    if ($goProc -and -not $goProc.HasExited) { $goProc.Kill() }
    Start-Sleep -Milliseconds 500
}

Write-Output '=== cleanup ==='
Remove-Item -Recurse -Force $run -ErrorAction SilentlyContinue

if ($script:failures -eq 0) {
    Write-Output 'ALL DEV LOOP TESTS PASSED'
} else {
    Write-Output "$($script:failures) DEV LOOP TEST(S) FAILED"
    exit 1
}
