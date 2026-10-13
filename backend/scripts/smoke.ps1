# End-to-end smoke test for the bs-blog backend.
# NOTE: this machine only has Windows PowerShell 5.1 (Desktop edition) -- no pwsh 7.
# So we avoid Invoke-WebRequest/-SkipHttpErrorCheck/-Form and use System.Net.WebRequest.
#
# -Port lets this run while a real dev server owns the default 8080.
param([int]$Port = 8080)

$ErrorActionPreference = 'Stop'
$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$be = Join-Path $root 'backend'
Set-Location $be
New-Item -ItemType Directory -Force -Path (Join-Path $be '.gotmp') | Out-Null

$env:GOCACHE = Join-Path $root '.gocache'
$env:GOTMPDIR = Join-Path $root '.gotmp'
$env:GOFLAGS = '-mod=mod'
$env:GOPROXY = 'off'
$env:GOSUMDB = 'off'
$env:GOTOOLCHAIN = 'local'

# Ephemeral data dir so the smoke test never touches real content.
$run = Join-Path $root '.smoke'
if (Test-Path $run) { Remove-Item -Recurse -Force $run }
New-Item -ItemType Directory -Force -Path $run | Out-Null
Set-Location $run

Write-Output '=== build ==='
Push-Location $be
& go build -o (Join-Path $run 'server.exe') ./cmd/server
$code = $LASTEXITCODE
Pop-Location
Write-Output "build exit=$code"
if ($code -ne 0) { exit 1 }

Copy-Item "$be\configs\config.yaml" (Join-Path $run 'config.yaml')

# Pin the listen address to $Port. Written through .NET so the file keeps its
# UTF-8 comments without a BOM (PowerShell 5.1's -Encoding UTF8 adds one).
$cfgPath = Join-Path $run 'config.yaml'
$cfgText = [System.IO.File]::ReadAllText($cfgPath) -replace 'addr:\s*"[^"]*"', "addr: `"127.0.0.1:$Port`""
[System.IO.File]::WriteAllText($cfgPath, $cfgText, (New-Object System.Text.UTF8Encoding($false)))

$out = Join-Path $run 'out.log'
$err = Join-Path $run 'err.log'
$proc = Start-Process -FilePath (Join-Path $run 'server.exe') -ArgumentList '-conf', 'config.yaml' -WorkingDirectory $run -PassThru -RedirectStandardOutput $out -RedirectStandardError $err

function Fail($msg) {
    Write-Output "FAIL: $msg"
    Write-Output '--- stdout ---'; Get-Content $out -ErrorAction SilentlyContinue | Select-Object -Last 40
    Write-Output '--- stderr ---'; Get-Content $err -ErrorAction SilentlyContinue | Select-Object -Last 40
    if (-not $proc.HasExited) { $proc.Kill() }
    exit 1
}

$base = "http://127.0.0.1:$Port/api/v1"
$up = $false
$lastErr = '(none)'
for ($i = 0; $i -lt 80; $i++) {
    Start-Sleep -Milliseconds 250
    $tcp = New-Object System.Net.Sockets.TcpClient
    try {
        $tcp.Connect('127.0.0.1', $Port)
        $up = $true
        $tcp.Close()
        break
    } catch {
        $lastErr = $_.Exception.Message
        $tcp.Dispose()
    }
}
if (-not $up) { Write-Output "probe failed: up=$up lastErr=$lastErr"; Fail 'server did not come up' }
Write-Output '=== server up ==='

# --- HTTP primitive: works on PowerShell 5.1 and 7 alike -------------------
function RawHttp($method, $url, $jsonBody, $token, $extraHeaders, $rawBytes, $contentType) {
    $req = [System.Net.WebRequest]::Create($url)
    $req.Method = $method
    $req.Timeout = 30000
    $req.AllowAutoRedirect = $false
    $req.Proxy = $null
    if ($token) { $req.Headers.Add('Authorization', "Bearer $token") }
    if ($extraHeaders) { foreach ($k in $extraHeaders.Keys) { $req.Headers.Add($k, $extraHeaders[$k]) } }
    if ($rawBytes) {
        $req.ContentType = $contentType
        $req.ContentLength = $rawBytes.Length
        $s = $req.GetRequestStream(); $s.Write($rawBytes, 0, $rawBytes.Length); $s.Close()
    } elseif ($null -ne $jsonBody) {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($jsonBody)
        $req.ContentType = 'application/json; charset=utf-8'
        $req.ContentLength = $bytes.Length
        $s = $req.GetRequestStream(); $s.Write($bytes, 0, $bytes.Length); $s.Close()
    }
    try {
        $res = $req.GetResponse()
    } catch [System.Net.WebException] {
        $res = $_.Exception.Response
        if ($null -eq $res) { throw }
    }
    $code = [int]$res.StatusCode
    $text = ''
    try {
        $stream = $res.GetResponseStream()
        if ($null -ne $stream) {
            $reader = New-Object System.IO.StreamReader($stream)
            $text = $reader.ReadToEnd()
            $reader.Close()
        }
    } catch { }
    $hdrs = @{}
    foreach ($k in $res.Headers.AllKeys) { $hdrs[$k] = $res.Headers[$k] }
    $res.Close()
    return New-Object psobject -Property @{ Status = $code; Text = $text; Headers = $hdrs }
}

function Call($method, $path, $body, $token, $expect) {
    $json = $null
    if ($null -ne $body) { $json = $body | ConvertTo-Json -Depth 10 -Compress }
    $r = RawHttp $method "$base$path" $json $token $null $null $null
    $tag = "$method $path"
    if ($r.Status -ne $expect) { Fail "$tag expected $expect got $($r.Status): $($r.Text)" }
    Write-Output "ok  $tag -> $($r.Status)"
    if ($r.Text -and $r.Text.Length -gt 0) {
        try { return $r.Text | ConvertFrom-Json } catch { return $r.Text }
    }
    return $null
}

function UploadMedia($filePath, $token, $expect) {
    $boundary = '----bsblog' + [guid]::NewGuid().ToString('N')
    $fileBytes = [System.IO.File]::ReadAllBytes($filePath)
    $name = [System.IO.Path]::GetFileName($filePath)
    $head = "--$boundary`r`nContent-Disposition: form-data; name=`"file`"; filename=`"$name`"`r`nContent-Type: image/png`r`n`r`n"
    $tail = "`r`n--$boundary--`r`n"
    $hb = [System.Text.Encoding]::UTF8.GetBytes($head)
    $tb = [System.Text.Encoding]::UTF8.GetBytes($tail)
    $buf = New-Object byte[] ($hb.Length + $fileBytes.Length + $tb.Length)
    [Array]::Copy($hb, 0, $buf, 0, $hb.Length)
    [Array]::Copy($fileBytes, 0, $buf, $hb.Length, $fileBytes.Length)
    [Array]::Copy($tb, 0, $buf, $hb.Length + $fileBytes.Length, $tb.Length)
    $r = RawHttp 'POST' "$base/admin/media" $null $token $null $buf "multipart/form-data; boundary=$boundary"
    if ($r.Status -ne $expect) { Fail "POST /admin/media expected $expect got $($r.Status): $($r.Text)" }
    Write-Output "ok  POST /admin/media -> $($r.Status)"
    return $r.Text | ConvertFrom-Json
}

Write-Output '=== public ==='
$site = Call GET '/site' $null $null 200
Write-Output "    site.title=$($site.title)"

$setup = Call GET '/auth/setup' $null $null 200
if ($setup.initialised -ne $false) { Fail 'expected fresh database to be uninitialised' }

Write-Output '=== auth guard ==='
Call GET '/auth/me' $null $null 401 | Out-Null
Call GET '/admin/posts' $null $null 401 | Out-Null

Write-Output '=== claim + login ==='
$claim = Call POST '/auth/setup' @{ username = 'bishop'; password = 'smoke-test-password' } $null 200
if (-not $claim.token) { Fail 'setup returned no token' }
$token = $claim.token
Call GET '/auth/me' $null $token 200 | Out-Null
$state = Call GET '/auth/setup' $null $null 200
if ($state.initialised -ne $true) { Fail 'setup state should be initialised after claim' }
$reclaim = RawHttp 'POST' "$base/auth/setup" '{"username":"x","password":"y"}' $null $null $null $null
if ($reclaim.Status -ne 403) { Fail "second claim should be forbidden, got $($reclaim.Status)" }
Write-Output "ok  POST /auth/setup (second time) -> $($reclaim.Status)"

Write-Output '=== posts ==='
# NOTE: PowerShell 5.1 reads BOM-less .ps1 files as ANSI (CP936 on this box), not as
# UTF-8. Keep THIS FILE 100% ASCII: non-ASCII literals get mangled, and an odd number
# of UTF-8 bytes in a trailing byte-pair can even swallow the following newline and
# silently comment out the next line. Build CJK strings from code points instead.
# U+7B2C U+4E00 U+7BC7 U+6587 U+7AE0 == the 5-char CJK word used as the test title.
$cjk = -join ([char]0x7B2C, [char]0x4E00, [char]0x7BC7, [char]0x6587, [char]0x7AE0)
$expectedSlug = "$cjk-hello-world"
$created = Call POST '/admin/posts' @{
    title       = "$cjk Hello World"
    contentHtml = '<h1>Hello</h1><p>This is the first post, with a link to <a href="https://example.com">example</a>.</p><script>alert(1)</script>'
    contentJson = '{"type":"doc"}'
    tagNames    = @('Go', 'Vue')
    visibility  = 'public'
} $token 201
$id = $created.id
$slug = $created.slug
Write-Output "    id=$id slug=$slug status=$($created.status)"
if ($created.status -ne 'draft') { Fail 'new post should start as a draft' }
if ($slug -ne $expectedSlug) { Fail "CJK slug was mangled: got '$slug', expected '$expectedSlug'" }
if ($created.contentHtml -match '<script') { Fail 'script tag survived sanitisation' }
if ($created.contentHtml -notmatch 'example.com') { Fail 'legitimate markup was stripped by the sanitiser' }

$list = Call GET '/posts' $null $null 200
if ($list.total -ne 0) { Fail "draft leaked into the public list (total=$($list.total))" }
Write-Output '    draft correctly hidden from public list'

$published = Call POST "/admin/posts/$id/publish" $null $token 200
if ($published.status -ne 'published') { Fail 'publish did not set status' }

$list = Call GET '/posts' $null $null 200
if ($list.total -ne 1) { Fail "expected 1 public post, got $($list.total)" }
Write-Output "    public list total=$($list.total) readingMinutes=$($list.items[0].readingMinutes) words=$($list.items[0].wordCount) tags=$($list.items[0].tags.Count)"

$detailRaw = RawHttp 'GET' "$base/posts/$slug" $null $null $null $null $null
if ($detailRaw.Status -ne 200) { Fail "GET /posts/{slug} expected 200 got $($detailRaw.Status): $($detailRaw.Text)" }
if ($detailRaw.Text -notmatch '"nav"') { Fail "detail payload has no nav key: $($detailRaw.Text)" }
if ($detailRaw.Text -notmatch '"post"') { Fail "detail payload has no post key: $($detailRaw.Text)" }
$detail = $detailRaw.Text | ConvertFrom-Json
if (-not $detail.post.contentHtml) { Fail 'detail missing content' }
if ($detail.post.nav.PSObject.Properties.Name -contains 'prev') { Write-Output "    nav.prev=$($detail.post.nav.prev.slug)" }
Write-Output '    detail carries post + nav (single post => empty nav)'

Call GET '/archive' $null $null 200 | Out-Null
$tags = Call GET '/tags' $null $null 200
Write-Output "    tags=$($tags.tags.Count)"
Call GET '/posts/does-not-exist' $null $null 404 | Out-Null
Call GET '/admin/posts' $null $token 200 | Out-Null
Call GET "/admin/posts/$id" $null $token 200 | Out-Null
$slugCheck = Call GET '/admin/slug?slug=brand-new' $null $token 200
Write-Output "    slug check: available=$($slugCheck.available)"

Write-Output '=== slug conflict ==='
$dup = RawHttp 'POST' "$base/admin/posts" (@{ title = 'dup'; slug = $slug; contentHtml = '<p>x</p>'; contentJson = '{}'; tagNames = @() } | ConvertTo-Json -Depth 10 -Compress) $token $null $null $null
if ($dup.Status -ne 409) { Fail "explicit slug conflict should be 409, got $($dup.Status): $($dup.Text)" }
Write-Output "ok  POST /admin/posts (duplicate explicit slug) -> $($dup.Status)"

Write-Output '=== validation ==='
$badTitle = RawHttp 'POST' "$base/admin/posts" (@{ title = '   '; contentHtml = '<p>x</p>'; contentJson = '{}'; tagNames = @() } | ConvertTo-Json -Depth 10 -Compress) $token $null $null $null
if ($badTitle.Status -ne 400) { Fail "blank title should be 400, got $($badTitle.Status)" }
$badJson = RawHttp 'POST' "$base/admin/posts" '{not json' $token $null $null $null
if ($badJson.Status -ne 400) { Fail "malformed json should be 400, got $($badJson.Status)" }
$unauth = RawHttp 'POST' "$base/admin/posts" '{}' 'not-a-real-token' $null $null $null
if ($unauth.Status -ne 401) { Fail "bogus token should be 401, got $($unauth.Status)" }
Write-Output "    blank-title=$($badTitle.Status) bad-json=$($badJson.Status) bogus-token=$($unauth.Status)"

Write-Output '=== site update ==='
$siteBody = @{
    title       = 'smoke site'
    tagline     = 'a tagline'
    description = 'desc'
    author      = 'bishop'
    avatarUrl   = ''
    socials     = @(
        @{ icon = 'github'; label = 'GitHub'; url = 'https://github.com/bishop9910' },
        @{ icon = 'evil'; label = 'bad'; url = 'javascript:alert(1)' }
    )
    footerNote  = 'footer'
} | ConvertTo-Json -Depth 10 -Compress
$siteRes = RawHttp 'PUT' "$base/admin/site" $siteBody $token $null $null $null
if ($siteRes.Status -ne 200) { Fail "PUT /admin/site expected 200 got $($siteRes.Status): $($siteRes.Text)" }
$updated = $siteRes.Text | ConvertFrom-Json
if ($updated.socials.Count -ne 1) { Fail "expected 1 valid social link, got $($updated.socials.Count) from $($siteRes.Text)" }
if ($updated.socials[0].icon -ne 'github') { Fail "unexpected surviving social icon $($updated.socials[0].icon)" }
Write-Output "    socials kept=$($updated.socials[0].icon) stats.postCount=$($updated.stats.postCount)"
$emptyTitle = Call PUT '/admin/site' @{ title = '' } $token 200
if ($emptyTitle.title -ne 'smoke site') { Fail "partial update should keep the previous title, got '$($emptyTitle.title)'" }
Write-Output "    partial update kept title='$($emptyTitle.title)'"

Write-Output '=== media ==='
$png = Join-Path $run 'pixel.png'
[IO.File]::WriteAllBytes($png, [Convert]::FromBase64String('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='))
$uploaded = UploadMedia $png $token 201
Write-Output "    uploaded url=$($uploaded.url) size=$($uploaded.size) dims=$($uploaded.width)x$($uploaded.height)"
if (-not $uploaded.url) { Fail 'upload returned no url' }

$media = Call GET '/admin/media' $null $token 200
if ($media.total -ne 1) { Fail "expected 1 asset, got $($media.total)" }

$served = RawHttp 'GET' "http://127.0.0.1:$Port$($uploaded.url)" $null $null $null $null $null
if ($served.Status -ne 200) { Fail "uploaded file not served: $($served.Status)" }
Write-Output '    uploaded file is served'

$rejected = RawHttp 'POST' "$base/admin/media" $null $token $null ([System.Text.Encoding]::UTF8.GetBytes('plain text, not an image')) 'multipart/form-data; boundary=nope'
if ($rejected.Status -ne 400 -and $rejected.Status -ne 413) { Write-Output "    note: non-multipart upload returned $($rejected.Status)" }

Call DELETE "/admin/media/$($uploaded.id)" $null $token 204 | Out-Null
$gone = RawHttp 'GET' "http://127.0.0.1:$Port$($uploaded.url)" $null $null $null $null $null
if ($gone.Status -ne 404) { Fail "deleted asset still served: $($gone.Status)" }
Write-Output '    deleted asset is no longer served'

Write-Output '=== password change ==='
# A wrong *current* password is 400, deliberately not 401: the session is still
# valid, and the frontend clears its token on ANY 401, which would log the owner
# out in the middle of changing their password.
$wrongOld = Call POST '/auth/password' @{ oldPassword = 'wrong'; newPassword = 'another-password' } $token 400
if ($wrongOld.code -ne 400) { Fail "wrong old password should carry code 400, got $($wrongOld.code)" }
Call POST '/auth/password' @{ oldPassword = 'smoke-test-password'; newPassword = 'another-password' } $token 204 | Out-Null
$stale = RawHttp 'GET' "$base/auth/me" $null $token $null $null $null
if ($stale.Status -ne 401) { Fail "old token should be revoked, got $($stale.Status)" }
Write-Output '    old token revoked after password change'
$relogin = Call POST '/auth/login' @{ username = 'bishop'; password = 'another-password' } $null 200
$token = $relogin.token
Call POST '/auth/login' @{ username = 'bishop'; password = 'nope' } $null 401 | Out-Null

Write-Output '=== cors preflight ==='
$pre = RawHttp 'OPTIONS' "$base/posts" $null $null @{ Origin = 'http://localhost:5173'; 'Access-Control-Request-Method' = 'GET' } $null $null
Write-Output "    status=$($pre.Status) allow-origin=$($pre.Headers['Access-Control-Allow-Origin'])"
if ($pre.Status -ne 204) { Fail 'preflight did not return 204' }
if ($pre.Headers['Access-Control-Allow-Origin'] -ne 'http://localhost:5173') { Fail 'preflight missing allow-origin' }
$evil = RawHttp 'OPTIONS' "$base/posts" $null $null @{ Origin = 'http://evil.example'; 'Access-Control-Request-Method' = 'GET' } $null $null
if ($evil.Headers['Access-Control-Allow-Origin']) { Fail 'disallowed origin got an allow-origin header' }
Write-Output "    disallowed origin -> $($evil.Status) (no allow-origin)"

Write-Output '=== unknown route ==='
$nf = RawHttp 'GET' "$base/nope" $null $null $null $null $null
if ($nf.Status -ne 404) { Fail "unknown API route should be 404, got $($nf.Status)" }
$badMethod = RawHttp 'DELETE' "$base/posts" $null $null $null $null $null
Write-Output "    unknown route=$($nf.Status) bad method=$($badMethod.Status)"

Write-Output '=== cleanup ==='
Call DELETE "/admin/posts/$id" $null $token 204 | Out-Null
$after = Call GET '/posts' $null $null 200
if ($after.total -ne 0) { Fail 'delete did not remove the post from the public list' }
Write-Output '    post deleted'

if (-not $proc.HasExited) { $proc.Kill(); $proc.WaitForExit(5000) }
Write-Output '--- server log (tail) ---'
Get-Content $out -ErrorAction SilentlyContinue | Select-Object -Last 20
Write-Output 'ALL SMOKE TESTS PASSED'
