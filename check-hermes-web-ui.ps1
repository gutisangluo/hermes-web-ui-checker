#Requires -Version 5.1

<#
.SYNOPSIS
    Hermes Web UI 检测修复工具 (PowerShell 版)
.DESCRIPTION
    自动检测 12 项 Hermes Web UI 前置条件并尝试修复
#>

$ErrorActionPreference = "Stop"
$autoFix = $true

# 颜色
$green = "Green"
$red = "Red"
$yellow = "Yellow"
$cyan = "Cyan"

$totalChecks = 0
$passedChecks = 0
$failedChecks = 0
$fixedChecks = 0

function Write-Step($n, $msg) {
    $script:totalChecks++
    Write-Host "`n[$n/12] $msg" -ForegroundColor White
}

function Write-Pass($msg) {
    $script:passedChecks++
    Write-Host "  [通过] $msg" -ForegroundColor $green
}

function Write-Fail($msg) {
    $script:failedChecks++
    Write-Host "  [失败] $msg" -ForegroundColor $red
}

function Write-Fix($msg) {
    $script:fixedChecks++
    Write-Host "  [已修复] $msg" -ForegroundColor $green
}

function Write-Info($msg) {
    Write-Host "  [*] $msg" -ForegroundColor $yellow
}

function Write-Cmd($msg) {
    Write-Host "  [命令] $msg" -ForegroundColor $cyan
}

function Invoke-Wsl($cmd) {
    $result = wsl.exe -d Ubuntu-24.04 -e bash -c $cmd 2>&1
    return $result
}

function Invoke-WslWithTimeout($timeoutSec, $cmd) {
    $fullCmd = "timeout $timeoutSec bash -c '$cmd'"
    $result = wsl.exe -d Ubuntu-24.04 -e bash -c $fullCmd 2>&1
    return $result
}

# ===== Banner =====
Write-Host "╔══════════════════════════════════════════════════╗" -ForegroundColor $cyan
Write-Host "║     Hermes Web UI 检测修复工具 v1.0            ║" -ForegroundColor $cyan
Write-Host "║     PowerShell 版                              ║" -ForegroundColor $cyan
Write-Host "╚══════════════════════════════════════════════════╝" -ForegroundColor $cyan
Write-Host ""

# ===== WSL 检查 =====
Write-Info "检查 WSL 运行状态..."
try {
    $wslTest = wsl.exe -d Ubuntu-24.04 echo "WSL_ALIVE" 2>&1
    if ($wslTest -match "WSL_ALIVE") {
        Write-Pass "WSL 运行正常"
    } else {
        throw "WSL 无法通信"
    }
} catch {
    Write-Info "WSL 未运行，正在启动..."
    wsl.exe -d Ubuntu-24.04 --cd "~" echo "WSL_STARTED" 2>&1 | Out-Null
    Start-Sleep -Seconds 3
}

if (-not $autoFix) {
    Write-Info "运行模式：仅检测（不修复）"
} else {
    Write-Info "运行模式：检测 + 自动修复"
}
Write-Host ("=" * 50)

# ===== 1. Node.js 版本 =====
Write-Step 1 "Node.js 版本检查 (>= 23.0.0)"
$ver = Invoke-Wsl "node --version 2>/dev/null || echo 'NOT_FOUND'"
if ($ver -match "NOT_FOUND" -or [string]::IsNullOrEmpty($ver)) {
    Write-Fail "Node.js 未安装，请先安装 Node.js >= 23.0.0"
} else {
    $ver = $ver -replace "^v", ""
    $major = $ver -split "\." | Select-Object -First 1
    if ($major -ge 23) {
        Write-Pass "Node.js v$ver (满足要求)"
    } else {
        Write-Fail "Node.js v$ver (需要 >= 23.0.0)"
    }
}

# ===== 2. Hermes CLI =====
Write-Step 2 "Hermes CLI 检查"
$ver = Invoke-Wsl "hermes --version 2>/dev/null || echo 'NOT_FOUND'"
if ($ver -match "NOT_FOUND" -or [string]::IsNullOrEmpty($ver)) {
    Write-Fail "Hermes CLI 未安装，请先安装 Hermes Agent"
} else {
    Write-Pass "Hermes CLI: $($ver -split "`n" | Select-Object -First 1)"
}

# ===== 3. API Server 配置 =====
Write-Step 3 "API 服务器配置检查 (port: 8642, host: 127.0.0.1)"
$configPath = "/root/.hermes/profiles/deepseek/config.yaml"
$exists = Invoke-Wsl "test -f $configPath && echo 'EXISTS' || echo 'NOT_FOUND'"
if ($exists -match "NOT_FOUND") {
    Write-Fail "profile config.yaml 不存在"
} else {
    $cfg = Invoke-Wsl "grep -A5 'api_server:' $configPath 2>/dev/null || echo 'NOT_FOUND'"
    if ($cfg -match "NOT_FOUND") {
        Write-Fail "config.yaml 中缺少 api_server 配置段"
        if ($autoFix) {
            Write-Info "正在添加 api_server 配置..."
            Invoke-Wsl "sed -i '/^platforms:/a\  api_server:\n    extra:\n      port: 8642\n      host: 127.0.0.1\n    enabled: true\n    key: ''''\n    cors_origins: '*'" $configPath" 2>&1 | Out-Null
            $check = Invoke-Wsl "grep -c 'port: 8642' $configPath 2>/dev/null || echo '0'"
            if ($check -match "1") {
                Write-Fix "api_server 配置已添加"
            } else {
                Write-Fail "自动修复失败，请手动编辑 config.yaml"
            }
        }
    } else {
        if ($cfg -match "port: 8642" -and $cfg -match "host: 127.0.0.1") {
            if ($cfg -match "enabled: true") {
                Write-Pass "API 服务器配置正确 (8642, 127.0.0.1)"
            } else {
                Write-Fail "api_server 未启用 (缺少 enabled: true)"
                if ($autoFix) {
                    Invoke-Wsl "sed -i 's/enabled: false/enabled: true/g' $configPath"
                    Write-Fix "api_server 已启用"
                }
            }
        } else {
            Write-Fail "API 服务器端口/主机配置不正确"
            Write-Info "当前配置:"
            $cfg -split "`n" | ForEach-Object { Write-Info $_ }
        }
    }
}

# ===== 4. 环境变量 =====
Write-Step 4 "GATEWAY_ALLOW_ALL_USERS 环境变量检查"
$envPath = "/root/.hermes/.env"
$exists = Invoke-Wsl "test -f $envPath && echo 'EXISTS' || echo 'NOT_FOUND'"
if ($exists -match "NOT_FOUND") {
    Write-Fail ".env 文件不存在"
    if ($autoFix) {
        Invoke-Wsl "echo 'GATEWAY_ALLOW_ALL_USERS=true' > $envPath"
        $key = Invoke-Wsl "grep 'key:' /root/.hermes/profiles/deepseek/config.yaml 2>/dev/null | head -1 | awk '{print `$2}'"
        if ($key -ne "" -and $key -ne "''") {
            Invoke-Wsl "echo 'API_SERVER_KEY=$key' >> $envPath"
        }
        Write-Fix ".env 文件已创建"
    }
} else {
    $content = Invoke-Wsl "cat $envPath"
    if ($content -match "GATEWAY_ALLOW_ALL_USERS=true") {
        Write-Pass "GATEWAY_ALLOW_ALL_USERS=true 已配置"
    } else {
        Write-Fail "缺少 GATEWAY_ALLOW_ALL_USERS=true"
        if ($autoFix) {
            Invoke-Wsl "echo 'GATEWAY_ALLOW_ALL_USERS=true' >> $envPath"
            Write-Fix "已添加 GATEWAY_ALLOW_ALL_USERS=true"
        }
    }
}

# ===== 5. active_profile =====
Write-Step 5 "active_profile 文件检查"
$globalFile = "/root/.hermes/active_profile"
$profileFile = "/root/.hermes/profiles/deepseek/active_profile"

$globalOk = Invoke-Wsl "test -f $globalFile && echo 'EXISTS' || echo 'NOT_FOUND'"
$profileOk = Invoke-Wsl "test -f $profileFile && echo 'EXISTS' || echo 'NOT_FOUND'"

if ($globalOk -match "EXISTS") {
    $content = Invoke-Wsl "cat $globalFile"
    if ($content.Trim() -eq "deepseek") {
        Write-Pass "全局 active_profile: deepseek"
    } else {
        Write-Fail "全局 active_profile 内容错误: $($content.Trim())"
        if ($autoFix) { Invoke-Wsl "echo 'deepseek' > $globalFile"; Write-Fix "全局 active_profile 已修正" }
    }
} else {
    Write-Fail "全局 active_profile 文件不存在"
    if ($autoFix) { Invoke-Wsl "echo 'deepseek' > $globalFile"; Write-Fix "全局 active_profile 已创建" }
}

if ($profileOk -match "EXISTS") {
    $content = Invoke-Wsl "cat $profileFile"
    if ($content.Trim() -eq "deepseek") {
        Write-Pass "profile 目录 active_profile: deepseek"
    } else {
        Write-Fail "profile 目录 active_profile 内容错误: $($content.Trim())"
        if ($autoFix) { Invoke-Wsl "echo 'deepseek' > $profileFile"; Write-Fix "profile 目录 active_profile 已修正" }
    }
} else {
    Write-Fail "profile 目录 active_profile 文件不存在"
    if ($autoFix) { Invoke-Wsl "echo 'deepseek' > $profileFile"; Write-Fix "profile 目录 active_profile 已创建" }
}

# ===== 6. SQLite 残留 =====
Write-Step 6 "SQLite 残留文件检查"
$files = @(
    "/root/.hermes-web-ui/hermes-web-ui.db-shm",
    "/root/.hermes-web-ui/hermes-web-ui.db-wal"
)
$found = $false
foreach ($f in $files) {
    $exists = Invoke-Wsl "test -f $f && echo 'EXISTS' || echo 'NOT_FOUND'"
    if ($exists -match "EXISTS") {
        $found = $true
        Write-Info "发现残留: $(Split-Path $f -Leaf)"
        if ($autoFix) { Invoke-Wsl "rm -f $f"; Write-Fix "已删除: $(Split-Path $f -Leaf)" }
    }
}
if (-not $found) { Write-Pass "无 SQLite 残留文件" }

# ===== 7. Login Lock =====
Write-Step 7 "Login Lock 残留检查"
$lockFile = "/root/.hermes-web-ui/.login-lock.json"
$exists = Invoke-Wsl "test -f $lockFile && echo 'EXISTS' || echo 'NOT_FOUND'"
if ($exists -match "EXISTS") {
    Write-Info "发现 login lock 残留文件"
    if ($autoFix) { Invoke-Wsl "rm -f $lockFile"; Write-Fix "已删除 login lock 文件" }
} else {
    Write-Pass "无 login lock 残留"
}

# ===== 8. YAML 格式 =====
Write-Step 8 "全局 config.yaml 格式检查"
$cfgPath = "/root/.hermes/config.yaml"
$exists = Invoke-Wsl "test -f $cfgPath && echo 'EXISTS' || echo 'NOT_FOUND'"
if ($exists -match "EXISTS") {
    $yamlCheck = Invoke-Wsl "python3 -c 'import yaml; yaml.safe_load(open(`"/root/.hermes/config.yaml`")); print(`"OK`")' 2>&1 || echo 'FAIL'"
    if ($yamlCheck -match "OK") {
        Write-Pass "YAML 格式正确"
    } else {
        Write-Fail "YAML 格式错误，请手动修复 /root/.hermes/config.yaml"
        Write-Info $yamlCheck
    }
} else {
    Write-Info "全局 config.yaml 不存在，跳过"
}

# ===== 9. 多余 Profile =====
Write-Step 9 "多余 Profile 检查"
$count = Invoke-Wsl "hermes profile list 2>/dev/null | grep -c 'default' || echo '0'"
$count = $count.Trim()
if ($count -ne "0") {
    Write-Info "发现 $count 个 default profile"
    if ($autoFix) {
        Invoke-Wsl "hermes profile delete default 2>/dev/null || rm -rf /root/.hermes/profiles/default 2>/dev/null"
        Write-Fix "已删除 default profile"
    }
} else {
    Write-Pass "无多余 default profile"
}

# ===== 10. 网关 =====
Write-Step 10 "Hermes 网关运行状态检查"
$health = Invoke-WslWithTimeout 5 "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'"
if ($health.Trim() -eq "1") {
    Write-Pass "Hermes 网关正在运行 (127.0.0.1:8642)"
} else {
    Write-Fail "Hermes 网关未运行"
    if ($autoFix) {
        Write-Info "正在启动 Hermes 网关（后台守护模式）..."
        Invoke-Wsl "rm -f /root/.hermes/profiles/deepseek/gateway.pid /root/.hermes/gateway.lock /root/.hermes/profiles/deepseek/gateway.lock"
        Invoke-Wsl "nohup hermes gateway run --replace </dev/null >/dev/null 2>&1 &"
        Start-Sleep -Seconds 4
        $check = Invoke-WslWithTimeout 5 "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'"
        if ($check.Trim() -eq "1") {
            Write-Fix "Hermes 网关已启动"
        } else {
            Write-Fail "网关启动失败，请手动在 WSL 中运行: hermes gateway run --replace"
        }
    }
}

# ===== 11. Web UI =====
Write-Step 11 "Web UI 运行状态检查"
$listening = Invoke-Wsl "ss -tlnp 2>/dev/null | grep 8648 | head -1 || echo 'NOT_LISTENING'"
if ($listening -notmatch "NOT_LISTENING") {
    Write-Pass "hermes-web-ui 正在运行 (0.0.0.0:8648)"
    # 检查网关是否被误杀
    $gwOk = Invoke-WslWithTimeout 5 "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'"
    if ($gwOk.Trim() -ne "1") {
        Write-Info "检测到网关可能被 Web UI 误杀，正在恢复..."
        Invoke-Wsl "nohup hermes gateway run --replace </dev/null >/dev/null 2>&1 &"
        Start-Sleep -Seconds 4
        $gwOk2 = Invoke-WslWithTimeout 5 "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'"
        if ($gwOk2.Trim() -eq "1") { Write-Fix "网关已恢复" } else { Write-Fail "网关恢复失败，请手动运行: hermes gateway run --replace" }
    }
} else {
    Write-Fail "hermes-web-ui 未运行"
    if ($autoFix) {
        Write-Info "正在启动 hermes-web-ui（后台守护模式，等待 15 秒）..."
        Invoke-Wsl "nohup /bin/sh -c 'HOME=/root hermes-web-ui start 8648' </dev/null >/dev/null 2>&1 &"
        Start-Sleep -Seconds 15
        $check = Invoke-Wsl "ss -tlnp 2>/dev/null | grep 8648 | head -1 || echo 'NOT_LISTENING'"
        if ($check -notmatch "NOT_LISTENING") {
            Write-Fix "hermes-web-ui 已启动"
            $gwOk = Invoke-WslWithTimeout 5 "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'"
            if ($gwOk.Trim() -ne "1") {
                Write-Info "网关被 GatewayManager 误杀，重新启动..."
                Invoke-Wsl "nohup hermes gateway run --replace </dev/null >/dev/null 2>&1 &"
                Start-Sleep -Seconds 4
                $gwOk2 = Invoke-WslWithTimeout 5 "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'"
                if ($gwOk2.Trim() -eq "1") { Write-Fix "网关已恢复" } else { Write-Fail "网关恢复失败" }
            }
        } else {
            Write-Fail "Web UI 启动失败，请手动运行: HOME=/root hermes-web-ui start 8648"
        }
    }
}

# ===== 12. 访问信息 =====
Write-Step 12 "访问信息"
$token = Invoke-Wsl "cat /root/.hermes-web-ui/.token 2>/dev/null || echo ''"
Write-Host ""
Write-Host "  浏览器访问地址:" -ForegroundColor White
Write-Host "  http://localhost:8648" -ForegroundColor $cyan
if ($token -ne "") {
    Write-Host "  http://localhost:8648/#/?token=$($token.Trim())" -ForegroundColor $cyan
}
Write-Host ""
Write-Host "  网关地址: http://127.0.0.1:8642" -ForegroundColor $cyan
Write-Host ""
Write-Host "  注意事项:" -ForegroundColor White
Write-Host "  - 确保 Docker Desktop 或 WSL 网络转发正常" -ForegroundColor White
Write-Host "  - 首次访问请使用带 token 的 URL" -ForegroundColor White
Write-Host "  - 如无法访问，请检查 Windows 防火墙" -ForegroundColor White

# ===== 汇总 =====
Write-Host ""
Write-Host ("=" * 50)
Write-Host "检测结果汇总:" -ForegroundColor White
Write-Host "  总检查项: $totalChecks" -ForegroundColor White
Write-Host "  [通过] $passedChecks" -ForegroundColor $green
Write-Host "  [失败] $failedChecks" -ForegroundColor $red
if ($autoFix) {
    Write-Host "  [已修复] $fixedChecks" -ForegroundColor $green
}

if ($failedChecks -gt 0 -and -not $autoFix) {
    Write-Info "存在未修复的问题，请去掉 -nofix 参数再次运行"
}
if ($failedChecks -gt 0 -and $autoFix) {
    Write-Info "部分项目修复失败，请手动处理"
}

Write-Host "`n按 Enter 键退出..." -ForegroundColor White
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
