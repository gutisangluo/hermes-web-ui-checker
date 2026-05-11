package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ============ 公共 ============

var (
	totalChecks  = 0
	passedChecks = 0
	failedChecks = 0
	fixedChecks  = 0
	autoFix      = true
)

const (
	green  = "\x1b[32m"
	red    = "\x1b[31m"
	yellow = "\x1b[33m"
	cyan   = "\x1b[36m"
	reset  = "\x1b[0m"
)

func printBanner() {
	fmt.Println(cyan + "╔══════════════════════════════════════════════════╗" + reset)
	fmt.Println(cyan + "║     Hermes Web UI 检测修复工具 v1.0            ║" + reset)
	fmt.Println(cyan + "║     自动检测 + 自动修复                       ║" + reset)
	fmt.Println(cyan + "╚══════════════════════════════════════════════════╝" + reset)
	fmt.Println()
}

func printStep(n int, msg string) {
	totalChecks++
	fmt.Printf("\n[%d/%d] %s\n", n, 12, msg)
}

func printPass(msg string) {
	passedChecks++
	fmt.Printf("  %s[通过]%s %s\n", green, reset, msg)
}

func printFail(msg string) {
	failedChecks++
	fmt.Printf("  %s[失败]%s %s\n", red, reset, msg)
}

func printFix(msg string) {
	fixedChecks++
	fmt.Printf("  %s[已修复]%s %s\n", green, reset, msg)
}

func printInfo(msg string) {
	fmt.Printf("  %s[*]%s %s\n", yellow, reset, msg)
}

func printCmd(msg string) {
	fmt.Printf("  %s[命令]%s %s\n", cyan, reset, msg)
}

// ============ WSL 执行 ============

func wslExec(args ...string) (string, error) {
	cmdArgs := []string{"-d", "Ubuntu-24.04", "-e", "bash", "-c", strings.Join(args, " ")}
	cmd := exec.Command("wsl.exe", cmdArgs...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func wslExecWithTimeout(timeoutSec int, args ...string) (string, error) {
	fullCmd := fmt.Sprintf("timeout %d bash -c '%s'", timeoutSec, strings.Join(args, " "))
	cmd := exec.Command("wsl.exe", "-d", "Ubuntu-24.04", "-e", "bash", "-c", fullCmd)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func wslWriteFile(path, content string) error {
	escaped := strings.ReplaceAll(content, "'", "'\\''")
	_, err := wslExec(fmt.Sprintf("echo '%s' > %s", escaped, path))
	return err
}

func wslDeleteFile(path string) error {
	_, err := wslExec(fmt.Sprintf("rm -f %s", path))
	return err
}

func wslFileExists(path string) bool {
	out, err := wslExec(fmt.Sprintf("test -f %s && echo 'EXISTS' || echo 'NOT_FOUND'", path))
	return err == nil && strings.Contains(out, "EXISTS")
}

func wslReadFile(path string) (string, error) {
	return wslExec(fmt.Sprintf("cat %s 2>/dev/null || echo 'NOT_FOUND'", path))
}

// ============ 检查项 ============

func checkNodeVersion() {
	printStep(1, "Node.js 版本检查 (>= 23.0.0)")
	ver, err := wslExec("node --version 2>/dev/null || echo 'NOT_FOUND'")
	if err != nil || strings.Contains(ver, "NOT_FOUND") {
		printFail("Node.js 未安装，请先安装 Node.js >= 23.0.0")
		return
	}
	ver = strings.TrimPrefix(ver, "v")
	parts := strings.Split(ver, ".")
	if len(parts) < 1 {
		printFail("无法解析 Node.js 版本: " + ver)
		return
	}
	major, _ := strconv.Atoi(parts[0])
	if major >= 23 {
		printPass(fmt.Sprintf("Node.js v%s (满足要求)", ver))
	} else {
		printFail(fmt.Sprintf("Node.js v%s (需要 >= 23.0.0)", ver))
	}
}

func checkHermesCli() {
	printStep(2, "Hermes CLI 检查")
	ver, err := wslExec("hermes --version 2>/dev/null || echo 'NOT_FOUND'")
	if err != nil || strings.Contains(ver, "NOT_FOUND") {
		printFail("Hermes CLI 未安装，请先安装 Hermes Agent")
		return
	}
	printPass("Hermes CLI: " + strings.Split(ver, "\n")[0])
}

func checkApiServerConfig() {
	printStep(3, "API 服务器配置检查 (port: 8642, host: 127.0.0.1)")
	configPath := "/root/.hermes/profiles/deepseek/config.yaml"

	// 检查文件是否存在
	if !wslFileExists(configPath) {
		printFail("profile config.yaml 不存在")
		return
	}

	// 读取 api_server 段
	cfg, err := wslExec(fmt.Sprintf("grep -A5 'api_server:' %s 2>/dev/null || echo 'NOT_FOUND'", configPath))
	if err != nil || strings.Contains(cfg, "NOT_FOUND") {
		printFail("config.yaml 中缺少 api_server 配置段")
		if autoFix {
			printInfo("正在添加 api_server 配置...")
			// 读取完整配置文件，追加
			fullCfg, _ := wslReadFile(configPath)
			apiBlock := "\nplatforms:\n  api_server:\n    extra:\n      port: 8642\n      host: 127.0.0.1\n    enabled: true\n    key: ''\n    cors_origins: '*'\n"
			// 检查是否已有 platforms 段
			if strings.Contains(fullCfg, "platforms:") {
				// 已有 platforms，只追加 api_server
				_, err := wslExec(fmt.Sprintf("grep -q 'api_server:' %s || sed -i '/^platforms:/a\\  api_server:\\n    extra:\\n      port: 8642\\n      host: 127.0.0.1\\n    enabled: true\\n    key: ''''\\n    cors_origins: '\\''*'\\''" + configPath))
				if err != nil {
					printFail("自动修复失败，请手动编辑 config.yaml")
					return
				}
			} else {
				// 无 platforms，追加整个段
				escaped := strings.ReplaceAll(apiBlock, "'", "'\\''")
				wslExec(fmt.Sprintf("echo '%s' >> %s", escaped, configPath))
			}
			// 验证
			check := wslFileExists(configPath)
			cfg2, _ := wslExec(fmt.Sprintf("grep -c 'port: 8642' %s 2>/dev/null || echo '0'", configPath))
			if check && strings.Contains(cfg2, "1") {
				printFix("api_server 配置已添加")
				return
			}
			printFail("自动修复失败")
		}
		return
	}

	hasPort := strings.Contains(cfg, "port: 8642")
	hasHost := strings.Contains(cfg, "host: 127.0.0.1")
	hasEnabled := strings.Contains(cfg, "enabled: true")

	if hasPort && hasHost {
		if hasEnabled {
			printPass("API 服务器配置正确 (8642, 127.0.0.1)")
		} else {
			printFail("api_server 未启用 (缺少 enabled: true)")
			if autoFix {
				wslExec(fmt.Sprintf("sed -i 's/enabled: false/enabled: true/g' %s", configPath))
				printFix("api_server 已启用")
			}
		}
	} else {
		printFail("API 服务器端口/主机配置不正确")
		printInfo("当前配置:")
		for _, line := range strings.Split(cfg, "\n") {
			printInfo(line)
		}
	}
}

func checkEnvFile() {
	printStep(4, "GATEWAY_ALLOW_ALL_USERS 环境变量检查")
	envPath := "/root/.hermes/.env"

	if !wslFileExists(envPath) {
		printFail(".env 文件不存在")
		if autoFix {
			wslWriteFile(envPath, "GATEWAY_ALLOW_ALL_USERS=true\n")
			// 也检查 API_SERVER_KEY
			key, _ := wslExec("grep API_SERVER_KEY /root/.hermes/.env 2>/dev/null || echo ''")
			if key == "" {
				// 从 config 里读 key
				cfgKey, _ := wslExec("grep 'key:' /root/.hermes/profiles/deepseek/config.yaml 2>/dev/null | head -1 | awk '{print $2}'")
				if cfgKey != "" && cfgKey != "''" {
					wslExec(fmt.Sprintf("echo 'API_SERVER_KEY=%s' >> %s", cfgKey, envPath))
				}
			}
			printFix(".env 文件已创建")
		}
		return
	}

	content, _ := wslReadFile(envPath)
	if strings.Contains(content, "GATEWAY_ALLOW_ALL_USERS=true") {
		printPass("GATEWAY_ALLOW_ALL_USERS=true 已配置")
	} else {
		printFail("缺少 GATEWAY_ALLOW_ALL_USERS=true")
		if autoFix {
			wslExec(fmt.Sprintf("echo 'GATEWAY_ALLOW_ALL_USERS=true' >> %s", envPath))
			printFix("已添加 GATEWAY_ALLOW_ALL_USERS=true")
		}
	}
}

func checkActiveProfile() {
	printStep(5, "active_profile 文件检查")
	globalFile := "/root/.hermes/active_profile"
	profileFile := "/root/.hermes/profiles/deepseek/active_profile"

	globalOk := wslFileExists(globalFile)
	profileOk := wslFileExists(profileFile)

	if globalOk {
		content, _ := wslReadFile(globalFile)
		if strings.TrimSpace(content) == "deepseek" {
			printPass("全局 active_profile: deepseek")
		} else {
			printFail(fmt.Sprintf("全局 active_profile 内容错误: %s", strings.TrimSpace(content)))
			if autoFix {
				wslWriteFile(globalFile, "deepseek\n")
				printFix("全局 active_profile 已修正")
			}
		}
	} else {
		printFail("全局 active_profile 文件不存在")
		if autoFix {
			wslWriteFile(globalFile, "deepseek\n")
			printFix("全局 active_profile 已创建")
		}
	}

	if profileOk {
		content, _ := wslReadFile(profileFile)
		if strings.TrimSpace(content) == "deepseek" {
			printPass("profile 目录 active_profile: deepseek")
		} else {
			printFail(fmt.Sprintf("profile 目录 active_profile 内容错误: %s", strings.TrimSpace(content)))
			if autoFix {
				wslWriteFile(profileFile, "deepseek\n")
				printFix("profile 目录 active_profile 已修正")
			}
		}
	} else {
		printFail("profile 目录 active_profile 文件不存在")
		if autoFix {
			wslWriteFile(profileFile, "deepseek\n")
			printFix("profile 目录 active_profile 已创建")
		}
	}
}

func checkSqliteResidue() {
	printStep(6, "SQLite 残留文件检查")
	files := []string{
		"/root/.hermes-web-ui/hermes-web-ui.db-shm",
		"/root/.hermes-web-ui/hermes-web-ui.db-wal",
	}
	found := false
	for _, f := range files {
		if wslFileExists(f) {
			found = true
			printInfo("发现残留: " + filepath.Base(f))
			if autoFix {
				wslDeleteFile(f)
				printFix("已删除: " + filepath.Base(f))
			}
		}
	}
	if !found {
		printPass("无 SQLite 残留文件")
	}
}

func checkLoginLock() {
	printStep(7, "Login Lock 残留检查")
	lockFile := "/root/.hermes-web-ui/.login-lock.json"
	if wslFileExists(lockFile) {
		printInfo("发现 login lock 残留文件")
		if autoFix {
			wslDeleteFile(lockFile)
			printFix("已删除 login lock 文件")
		}
	} else {
		printPass("无 login lock 残留")
	}
}

func checkYamlFormat() {
	printStep(8, "全局 config.yaml 格式检查")
	configPath := "/root/.hermes/config.yaml"
	if !wslFileExists(configPath) {
		printInfo("全局 config.yaml 不存在，跳过")
		return
	}
	// 用 python 检查 YAML 格式
	out, err := wslExec("python3 -c \"import yaml; yaml.safe_load(open('/root/.hermes/config.yaml')); print('OK')\" 2>&1 || echo 'FAIL'")
	if err != nil || strings.Contains(out, "FAIL") {
		printFail("YAML 格式错误，请手动修复 /root/.hermes/config.yaml")
		printInfo(out)
	} else {
		printPass("YAML 格式正确")
	}
}

func checkDefaultProfile() {
	printStep(9, "多余 Profile 检查")
	out, err := wslExec("hermes profile list 2>/dev/null | grep -c 'default' || echo '0'")
	if err == nil && strings.TrimSpace(out) != "0" {
		count, _ := strconv.Atoi(strings.TrimSpace(out))
		if count > 0 {
			printInfo(fmt.Sprintf("发现 %d 个 default profile", count))
			if autoFix {
				wslExec("hermes profile delete default 2>/dev/null || rm -rf /root/.hermes/profiles/default 2>/dev/null")
				printFix("已删除 default profile")
			}
		}
	} else {
		printPass("无多余 default profile")
	}
}

func checkAndStartGateway() {
	printStep(10, "Hermes 网关运行状态检查")
	health, err := wslExecWithTimeout(5, "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'")
	if err == nil && strings.TrimSpace(health) == "1" {
		printPass("Hermes 网关正在运行 (127.0.0.1:8642)")
	} else {
		printFail("Hermes 网关未运行")
		if autoFix {
			printInfo("正在启动 Hermes 网关（后台守护模式）...")
			wslExec(fmt.Sprintf("rm -f /root/.hermes/profiles/deepseek/gateway.pid /root/.hermes/gateway.lock /root/.hermes/profiles/deepseek/gateway.lock"))
			wslExec("nohup hermes gateway run --replace </dev/null >/dev/null 2>&1 &")
			time.Sleep(4 * time.Second)

			check, _ := wslExecWithTimeout(5, "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'")
			if strings.TrimSpace(check) == "1" {
				printFix("Hermes 网关已启动")
			} else {
				printFail("网关启动失败，请手动在 WSL 中运行: hermes gateway run --replace")
			}
		}
	}
}

func checkAndStartWebUI() {
	printStep(11, "Web UI 运行状态检查")
	listening, err := wslExec("ss -tlnp 2>/dev/null | grep 8648 | head -1 || echo 'NOT_LISTENING'")
	if err == nil && !strings.Contains(listening, "NOT_LISTENING") {
		printPass("hermes-web-ui 正在运行 (0.0.0.0:8648)")
		// 网关可能被 GatewayManager 误杀，检查一下
		gwOk, _ := wslExecWithTimeout(5, "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'")
		if strings.TrimSpace(gwOk) != "1" {
			printInfo("检测到网关可能被 Web UI 误杀，正在恢复...")
			wslExec("nohup hermes gateway run --replace </dev/null >/dev/null 2>&1 &")
			time.Sleep(4 * time.Second)
			gwOk2, _ := wslExecWithTimeout(5, "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'")
			if strings.TrimSpace(gwOk2) == "1" {
				printFix("网关已恢复")
			} else {
				printFail("网关恢复失败，请手动运行: hermes gateway run --replace")
			}
		}
	} else {
		printFail("hermes-web-ui 未运行")
		if autoFix {
			printInfo("正在启动 hermes-web-ui（后台守护模式，等待 15 秒）...")
			wslExec("nohup /bin/sh -c 'HOME=/root hermes-web-ui start 8648' </dev/null >/dev/null 2>&1 &")
			time.Sleep(15 * time.Second)

			check, _ := wslExec("ss -tlnp 2>/dev/null | grep 8648 | head -1 || echo 'NOT_LISTENING'")
			if !strings.Contains(check, "NOT_LISTENING") {
				printFix("hermes-web-ui 已启动")
				// 检查网关是否还在
				gwOk, _ := wslExecWithTimeout(5, "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'")
				if strings.TrimSpace(gwOk) != "1" {
					printInfo("网关被 GatewayManager 误杀，重新启动...")
					wslExec("nohup hermes gateway run --replace </dev/null >/dev/null 2>&1 &")
					time.Sleep(4 * time.Second)
					gwOk2, _ := wslExecWithTimeout(5, "curl -s --max-time 3 http://127.0.0.1:8642/health 2>/dev/null | grep -c ok || echo '0'")
					if strings.TrimSpace(gwOk2) == "1" {
						printFix("网关已恢复")
					} else {
						printFail("网关恢复失败，请手动运行: hermes gateway run --replace")
					}
				}
			} else {
				printFail("Web UI 启动失败，请手动运行: HOME=/root hermes-web-ui start 8648")
			}
		}
	}
}

func showAccessInfo() {
	printStep(12, "访问信息")

	// 获取 token
	token, _ := wslExec("cat /root/.hermes-web-ui/.token 2>/dev/null || echo ''")

	// 获取 WSL 是否为默认分发
	_, _ = exec.Command("wsl.exe", "-d", "Ubuntu-24.04", "echo", "READY").Output()

	fmt.Println()
	fmt.Printf("  浏览器访问地址:\n")
	fmt.Printf("  %shttp://localhost:8648%s\n", cyan, reset)
	if token != "" {
		fmt.Printf("  %shttp://localhost:8648/#/?token=%s%s\n", cyan, strings.TrimSpace(token), reset)
	}
	fmt.Println()
	fmt.Printf("  网关地址: %shttp://127.0.0.1:8642%s\n", cyan, reset)
	fmt.Println()
	fmt.Printf("  注意事项:\n")
	fmt.Printf("  - 确保 Docker Desktop 或 WSL 网络转发正常\n")
	fmt.Printf("  - 首次访问请使用带 token 的 URL\n")
	fmt.Printf("  - 如无法访问，请检查 Windows 防火墙\n")
}

// ============ 启动 WSL ============

func ensureWslRunning() bool {
	printInfo("检查 WSL 运行状态...")
	out, err := exec.Command("wsl.exe", "-d", "Ubuntu-24.04", "echo", "WSL_ALIVE").Output()
	if err != nil || !strings.Contains(string(out), "WSL_ALIVE") {
		printInfo("WSL 未运行，正在启动...")
		cmd := exec.Command("wsl.exe", "-d", "Ubuntu-24.04", "echo", "WSL_STARTED")
		out2, err2 := cmd.Output()
		if err2 != nil || !strings.Contains(string(out2), "WSL_STARTED") {
			printFail("WSL 启动失败，请手动打开 WSL")
			return false
		}
	}
	printPass("WSL 运行正常")
	return true
}

// ============ 主流程 ============

func main() {
	// 解析参数
	for _, arg := range os.Args[1:] {
		if arg == "-nofix" || arg == "--nofix" || arg == "/nofix" || arg == "-checkonly" {
			autoFix = false
		}
	}

	printBanner()

	if !ensureWslRunning() {
		fmt.Println("\n按 Enter 键退出...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}

	if !autoFix {
		printInfo("运行模式：仅检测（不修复）")
	} else {
		printInfo("运行模式：检测 + 自动修复")
	}
	fmt.Println(strings.Repeat("=", 50))

	checkNodeVersion()
	checkHermesCli()
	checkApiServerConfig()
	checkEnvFile()
	checkActiveProfile()
	checkSqliteResidue()
	checkLoginLock()
	checkYamlFormat()
	checkDefaultProfile()
	checkAndStartGateway()
	checkAndStartWebUI()
	showAccessInfo()

	// 汇总
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("检测结果汇总:\n")
	fmt.Printf("  总检查项: %d\n", totalChecks)
	fmt.Printf("  %s通过: %d%s\n", green, passedChecks, reset)
	fmt.Printf("  %s失败: %d%s\n", red, failedChecks, reset)
	if autoFix {
		fmt.Printf("  %s已修复: %d%s\n", green, fixedChecks, reset)
	}
	fmt.Println()

	if failedChecks > 0 && !autoFix {
		printInfo("存在未修复的问题，请加 -nofix 参数再次运行查看详情")
	}
	if failedChecks > 0 && autoFix {
		printInfo("部分项目修复失败，请手动处理")
	}

	// 保存 JSON 结果用于界面显示
	result := map[string]interface{}{
		"total":  totalChecks,
		"passed": passedChecks,
		"failed": failedChecks,
		"fixed":  fixedChecks,
		"url":    "http://localhost:8648",
	}

	// 读取 token
	token, _ := wslExec("cat /root/.hermes-web-ui/.token 2>/dev/null || echo ''")
	if token != "" {
		result["tokenUrl"] = "http://localhost:8648/#/?token=" + strings.TrimSpace(token)
	}

	resultJSON, _ := json.Marshal(result)
	os.WriteFile("hermes-check-result.json", resultJSON, 0644)

	fmt.Println("\n按 Enter 键退出...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}
