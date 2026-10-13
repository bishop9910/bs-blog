$ErrorActionPreference = 'Stop'

# Verifies the production deployment shape: the Go server hosts the built
# frontend, deep links fall back to index.html, /api stays JSON, and the static
# handler cannot be walked out of its root.
#
# NOTE: keep this file 100% ASCII. Windows PowerShell 5.1 reads a BOM-less .ps1
# as ANSI/CP936, and an odd byte count in a CJK comment swallows the next line.

$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$be = Join-Path $root 'backend'
$run = Join-Path $root '.smokeweb'
$port = 8091
$base = "http://127.0.0.1:$port"

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
New-Item -ItemType Directory -Force -Path (Join-Path $run 'dist\assets') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $run 'data\uploads') | Out-Null

Set-Content -Path (Join-Path $run 'dist\index.html') -Value '<!doctype html><title>SPA-INDEX</title>' -Encoding ASCII
Set-Content -Path (Join-Path $run 'dist\assets\app.js') -Value 'console.log("asset")' -Encoding ASCII
Set-Content -Path (Join-Path $run 'data\uploads\pic.png') -Value 'PNGDATA' -Encoding ASCII
# A file OUTSIDE the static root that must never be reachable over HTTP.
Set-Content -Path (Join-Path $run 'dist\..\secret-outside-root.txt') -Value 'TOP-SECRET-OUTSIDE-ROOT' -Encoding ASCII

$configPath = Join-Path $run 'config.yaml'
$config = @(
    'server:'
    "  addr: `"127.0.0.1:$port`""
    '  staticDir: "dist"'
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
    Write-Output '=== static hosting ==='
    $r = Fetch "$base/"
    Check 'GET / serves index.html' ($r.Status -eq 200 -and $r.Body -match 'SPA-INDEX') "status=$($r.Status) body=$($r.Body)"

    $r = Fetch "$base/post/hello-world"
    Check 'deep link falls back to index.html' ($r.Status -eq 200 -and $r.Body -match 'SPA-INDEX') "status=$($r.Status)"

    $r = Fetch "$base/assets/app.js"
    Check 'real asset is served as-is' ($r.Status -eq 200 -and $r.Body -match 'console.log') "status=$($r.Status)"

    $r = Fetch "$base/manifest.webmanifest"
    Check 'missing non-asset path also returns the SPA shell' ($r.Status -eq 200 -and $r.Body -match 'SPA-INDEX') "status=$($r.Status)"

    Write-Output '=== api routing under static hosting ==='
    $r = Fetch "$base/api/v1/site"
    Check 'GET /api/v1/site is JSON' ($r.Status -eq 200 -and $r.Type -match 'json') "status=$($r.Status) type=$($r.Type)"

    $r = Fetch "$base/api/v1/nope"
    Check 'unknown API route is a JSON 404, not the SPA shell' `
        ($r.Status -eq 404 -and $r.Type -match 'json' -and $r.Body -notmatch 'SPA-INDEX') "status=$($r.Status) body=$($r.Body)"

    Write-Output '=== static root containment ==='
    $r = Fetch "$base/secret-outside-root.txt"
    Check 'file outside static root is not served' ($r.Body -notmatch 'TOP-SECRET-OUTSIDE-ROOT') "status=$($r.Status)"

    $r = Fetch "$base/uploads/%2e%2e/secret-outside-root.txt"
    Check 'encoded .. cannot escape the upload root' ($r.Body -notmatch 'TOP-SECRET-OUTSIDE-ROOT') "status=$($r.Status)"

    $r = Fetch "$base/uploads/..%2fsecret-outside-root.txt"
    Check 'encoded slash after .. cannot escape' ($r.Body -notmatch 'TOP-SECRET-OUTSIDE-ROOT') "status=$($r.Status)"

    $r = Fetch "$base/uploads/....//secret-outside-root.txt"
    Check 'doubled-dot trick cannot escape' ($r.Body -notmatch 'TOP-SECRET-OUTSIDE-ROOT') "status=$($r.Status)"

    Write-Output '=== uploads ==='
    $r = Fetch "$base/uploads/pic.png"
    Check 'uploaded file is served' ($r.Status -eq 200 -and $r.Body -match 'PNGDATA') "status=$($r.Status)"

    $r = Fetch "$base/uploads/"
    Check 'no directory listing at /uploads/' `
        (($r.Body -notmatch 'pic\.png') -and (($r.Status -eq 404) -or ($r.Body -match 'SPA-INDEX'))) `
        "status=$($r.Status) body=$($r.Body)"

    $r = Fetch "$base/uploads"
    Check 'no directory listing at /uploads' `
        (($r.Body -notmatch 'pic\.png') -and (($r.Status -eq 404) -or ($r.Body -match 'SPA-INDEX'))) `
        "status=$($r.Status) body=$($r.Body)"

    $r = Fetch "$base/uploads/missing.png"
    Check 'missing upload is a 404' ($r.Status -eq 404) "status=$($r.Status)"
} finally {
    if ($proc -and -not $proc.HasExited) { $proc.Kill() }
    Start-Sleep -Milliseconds 300
}

Write-Output '=== cleanup ==='
Remove-Item -Recurse -Force $run -ErrorAction SilentlyContinue

if ($script:failures -eq 0) {
    Write-Output 'ALL WEB HOSTING TESTS PASSED'
} else {
    Write-Output "$($script:failures) WEB HOSTING TEST(S) FAILED"
    exit 1
}
