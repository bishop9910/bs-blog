$ErrorActionPreference = 'Stop'

# Verifies the REAL production integration: the Go server hosting the actual
# dist/ produced by `pnpm build` (not a fixture). Catches the classic breakages
# a fixture cannot see -- an asset index.html points at but Vite never emitted,
# a wrong base path, or the SPA shell shadowing the API.
#
# Run `pnpm build` first. If dist/index.html is missing this script builds it.
#
# NOTE: keep this file 100% ASCII. Windows PowerShell 5.1 reads a BOM-less .ps1
# as ANSI/CP936, and an odd byte count in a CJK comment swallows the next line.

$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$be = Join-Path $root 'backend'
$dist = Join-Path $root 'dist'
$run = Join-Path $root '.smokeprod'
$port = 8092
$base = "http://127.0.0.1:$port"

if (-not (Test-Path (Join-Path $dist 'index.html'))) {
    Write-Output 'FAIL: dist/index.html is missing -- run `pnpm build` first'
    exit 1
}

$env:GOCACHE = Join-Path $root '.gocache'
$env:GOTMPDIR = Join-Path $root '.gotmp'
$env:GOFLAGS = '-mod=mod'
$env:GOPROXY = 'off'
$env:GOSUMDB = 'off'
$env:GOTOOLCHAIN = 'local'

$script:failures = 0

function Check($name, $condition, $detail) {
    if ($condition) {
        Write-Output "PASS: $name"
    } else {
        Write-Output "FAIL: $name -- $detail"
        $script:failures = $script:failures + 1
    }
}

function Fetch($url) {
    $req = [System.Net.WebRequest]::Create($url)
    $req.Proxy = $null
    $req.AllowAutoRedirect = $false
    $req.Timeout = 15000
    try {
        $res = $req.GetResponse()
        $code = [int]$res.StatusCode
        $sr = New-Object System.IO.StreamReader($res.GetResponseStream())
        $body = $sr.ReadToEnd()
        $type = $res.ContentType
        $sr.Close()
        $res.Close()
        return New-Object psobject -Property @{ Status = $code; Body = $body; Type = $type }
    } catch [System.Net.WebException] {
        $res = $_.Exception.Response
        if ($res -eq $null) {
            return New-Object psobject -Property @{ Status = 0; Body = $_.Exception.Message; Type = '' }
        }
        $code = [int]$res.StatusCode
        $sr = New-Object System.IO.StreamReader($res.GetResponseStream())
        $body = $sr.ReadToEnd()
        $type = $res.ContentType
        $sr.Close()
        return New-Object psobject -Property @{ Status = $code; Body = $body; Type = $type }
    }
}

Remove-Item -Recurse -Force $run -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path (Join-Path $run 'data\uploads') | Out-Null

$configPath = Join-Path $run 'config.yaml'
$config = @(
    'server:'
    "  addr: `"127.0.0.1:$port`""
    "  staticDir: `"$($dist -replace '\\', '/')`""
    'data:'
    '  database: "data/blog.db"'
    '  uploadDir: "data/uploads"'
    '  uploadUrl: "/uploads"'
    'auth:'
    '  secretFile: "data/jwt.secret"'
    'log:'
    '  level: "warn"'
)
Set-Content -Path $configPath -Value $config -Encoding ASCII

Write-Output '=== build ==='
Push-Location $be
& go build -o (Join-Path $run 'server.exe') ./cmd/server
$code = $LASTEXITCODE
Pop-Location
if ($code -ne 0) { Write-Output "FAIL: go build exit=$code"; exit 1 }
Write-Output 'PASS: go build'

Write-Output '=== start ==='
$outLog = Join-Path $run 'out.log'
$errLog = Join-Path $run 'err.log'
$proc = Start-Process -FilePath (Join-Path $run 'server.exe') -ArgumentList '-conf', 'config.yaml' `
    -WorkingDirectory $run -RedirectStandardOutput $outLog -RedirectStandardError $errLog -PassThru

$up = $false
for ($i = 0; $i -lt 60; $i++) {
    Start-Sleep -Milliseconds 250
    $tcp = New-Object System.Net.Sockets.TcpClient
    try { $tcp.Connect('127.0.0.1', $port); $up = $true; $tcp.Close(); break }
    catch { $tcp.Dispose() }
}
if (-not $up) {
    Write-Output 'FAIL: server did not come up'
    Get-Content $outLog, $errLog -ErrorAction SilentlyContinue
    if ($proc -and -not $proc.HasExited) { $proc.Kill() }
    exit 1
}
Write-Output 'PASS: server up'

try {
    Write-Output '=== real shell ==='
    $index = Get-Content (Join-Path $dist 'index.html') -Raw
    $r = Fetch "$base/"
    Check 'GET / serves the built shell' `
        ($r.Status -eq 200 -and $r.Body -match 'id="app"' -and $r.Type -match 'text/html') `
        "status=$($r.Status) type=$($r.Type)"
    Check 'shell is not the dev index (no /src/main.ts)' ($r.Body -notmatch '/src/main\.ts') 'dev entry leaked'
    Check 'shell carries the pre-paint theme guard' ($r.Body -match 'bs-blog\.theme') 'theme bootstrap missing'
    Check 'shell sets lang=zh-CN' ($r.Body -match '<html[^>]*lang="zh-CN"') 'lang attribute missing'

    Write-Output '=== every asset the shell references resolves ==='
    # Only same-origin references matter; the Google Fonts links are external.
    $refs = [regex]::Matches($index, '(?:src|href)="(/[^"]+)"') | ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique
    Check 'shell references at least one built asset' ($refs.Count -gt 0) "refs=$($refs -join ', ')"
    foreach ($ref in $refs) {
        $a = Fetch "$base$ref"
        Check "asset $ref" ($a.Status -eq 200) "status=$($a.Status)"
    }

    Write-Output '=== deep links ==='
    foreach ($path in @('/posts', '/archive', '/tags', '/about', '/admin', '/post/does-not-exist-yet')) {
        $d = Fetch "$base$path"
        Check "deep link $path returns the shell" `
            ($d.Status -eq 200 -and $d.Body -match 'id="app"') "status=$($d.Status)"
    }

    Write-Output '=== api still wins over the shell ==='
    $r = Fetch "$base/api/v1/site"
    Check 'GET /api/v1/site is JSON' ($r.Status -eq 200 -and $r.Type -match 'json') "status=$($r.Status) type=$($r.Type)"
    Check 'site payload is the real profile' ($r.Body -match '"title"') $r.Body

    $r = Fetch "$base/api/v1/nope"
    Check 'unknown API route is a JSON 404, not the shell' `
        ($r.Status -eq 404 -and $r.Type -match 'json' -and $r.Body -notmatch 'id="app"') "status=$($r.Status) body=$($r.Body)"

    $r = Fetch "$base/api/v1/admin/posts"
    Check 'admin API without a token is 401' ($r.Status -eq 401) "status=$($r.Status)"

    Write-Output '=== uploads ==='
    $r = Fetch "$base/uploads/"
    Check 'no directory listing at /uploads/' `
        (($r.Body -notmatch '<a href') -or ($r.Body -match 'id="app"')) "status=$($r.Status) body=$($r.Body)"

    Write-Output '=== source maps are not exposed ==='
    $maps = Get-ChildItem (Join-Path $dist 'assets') -Filter '*.map' -ErrorAction SilentlyContinue
    Check 'no .map files in the build output' ($null -eq $maps -or $maps.Count -eq 0) "found=$($maps.Count)"
} finally {
    if ($proc -and -not $proc.HasExited) { $proc.Kill() }
    Start-Sleep -Milliseconds 300
}

Write-Output '=== cleanup ==='
Remove-Item -Recurse -Force $run -ErrorAction SilentlyContinue

if ($script:failures -eq 0) {
    Write-Output 'ALL PRODUCTION INTEGRATION TESTS PASSED'
} else {
    Write-Output "$($script:failures) PRODUCTION INTEGRATION TEST(S) FAILED"
    exit 1
}
