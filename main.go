// check-hermes-web-ui v2 — Go cross-compiled Windows exe
// 检测 14 项 + 自动修复 Hermes Web UI + 网关问题
//
// 编译：GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o check-hermes-web-ui.exe .
// GitHub: https://github.com/gutisangluo/hermes-web-ui-checker
//
// v2 新增（2026-05-13）：
// - 端口一致性检查：检测 GatewayManager 是否篡改 ports.api_server.extra.port
// - SQLite 增强：检测 Web UI 进程挂起，自动删主 .db 文件重建
// - 从 config.yaml 读取实际端口，不硬编码 8642

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	wslDistro  = "Ubuntu-24.04"
	hermesHome = "/root/.hermes"
	webUIDir   = "/root/.hermes-web-ui"
	profile    = "deepseek"
)

type CheckResult struct {
	Total  int    `json:"total"`
	Passed int    `json:"passed"`
	Failed int    `json:"failed"`
	Fixed  int    `json:"fixed"`
	URL    string `json:"url"`
	Token  string `json:"tokenUrl"`
}

var result = CheckResult{Total: 14}
var fixes int

func main() {
	fmt.Println("=== Hermes Web UI 一键检测工具 ===")
	fmt.Println()

	check("1. Node.js 版本 >= 23", checkNodeVersion)
	check("2. Hermes CLI 已安装", checkHermesCLI)
	check("3. api_server 配置正确", checkAPIServerConfig, fixAPIServerConfig)
	check("4. GATEWAY_ALLOW_ALL_USERS", checkGateWayEnv, fixGateWayEnv)
	check("5. active_profile 文件", checkActiveProfile, fixActiveProfile)
	check("6. SQLite 数据库清理", checkSQLite, fixSQLite)
	check("7. Login lock 清理", checkLoginLock, fixLoginLock)
	check("8. 全局 config.yaml 格式", checkGlobalYAML, fixGlobalYAML)
	check("9. 多余 default profile", checkDefaultProfile, fixDefaultProfile)
	check("10. Hermes 网关运行", checkGateway, fixGateway)
	check("11. Web UI 运行", checkWebUI, fixWebUI)
	check("12. 端口一致性检查", checkPortConsistency, fixPortConsistency)
	check("13. 网关健康检查", checkGatewayHealth)
	check("14. 访问信息", checkAccessInfo)

	fmt.Println()
	fmt.Printf("结果: %d 项通过, %d 项失败, %d 项已修复 (共 %d 项)\n",
		result.Passed, result.Failed, fixes, result.Total)

	result.URL = "http://localhost:" + getActualPort()
	token := getToken()
	if token != "" {
		result.Token = fmt.Sprintf("http://localhost:%s/#/?token=%s", getActualPort(), token)
		fmt.Printf("访问地址: %s\n", result.Token)
	} else {
		fmt.Println("注意: 无法读取 token")
	}

	if _, err := os.Stat("/mnt/e/桌面"); err == nil {
		data, _ := json.Marshal(result)
		os.WriteFile("/mnt/e/桌面/hermes-check-result.json", data, 0644)
	}
}

func wsl(cmd string) (string, error) {
	out, err := exec.Command("wsl.exe", "-d", wslDistro, "-e", "bash", "-c", cmd).CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func wslExit0(cmd string) bool {
	err := exec.Command("wsl.exe", "-d", wslDistro, "-e", "bash", "-c", cmd).Run()
	return err == nil
}

func check(name string, fn func() bool, fixFn ...func() bool) {
	fmt.Printf("  [检查] %s... ", name)
	if fn() {
		result.Passed++
		fmt.Println("✓ 通过")
	} else {
		result.Failed++
		fmt.Println("✗ 失败")
		if len(fixFn) > 0 && !hasFlag("-nofix") {
			fmt.Printf("    → 尝试修复... ")
			if fixFn[0]() {
				fixes++
				fmt.Println("✓ 已修复")
				result.Fixed++
			} else {
				fmt.Println("✗ 修复失败")
			}
		}
	}
}

func hasFlag(name string) bool {
	for _, arg := range os.Args {
		if arg == name {
			return true
		}
	}
	return false
}

// — Checks —

func checkNodeVersion() bool {
	out, err := wsl("node --version 2>/dev/null")
	if err != nil {
		return false
	}
	v := strings.TrimPrefix(out, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 1 {
		return false
	}
	major, _ := strconv.Atoi(parts[0])
	return major >= 23
}

func checkHermesCLI() bool {
	return wslExit0("which hermes 2>/dev/null && hermes --version 2>/dev/null")
}

func checkAPIServerConfig() bool {
	py := `python3 -c "
import yaml
cfg = yaml.safe_load(open('/root/.hermes/profiles/deepseek/config.yaml'))
api = cfg.get('platforms',{}).get('api_server',{})
if not api.get('enabled'):
    exit(1)
port = str(api.get('extra',{}).get('port',''))
host = str(api.get('extra',{}).get('host',''))
key = str(api.get('key',''))
if not port or not host:
    exit(1)
print(f'{port}|{host}|{key}')
" 2>/dev/null`
	out, err := wsl(py)
	if err != nil {
		return false
	}
	return strings.Contains(out, "|")
}

func fixAPIServerConfig() bool {
	py := `python3 -c "
import yaml
path = '/root/.hermes/profiles/deepseek/config.yaml'
with open(path) as f:
    cfg = yaml.safe_load(f)
if 'platforms' not in cfg:
    cfg['platforms'] = {}
if 'api_server' not in cfg['platforms']:
    cfg['platforms']['api_server'] = {}
api = cfg['platforms']['api_server']
api['enabled'] = True
if 'extra' not in api:
    api['extra'] = {}
if 'port' not in api['extra'] or not api['extra']['port']:
    api['extra']['port'] = 8642
if 'host' not in api['extra'] or not api['extra']['host']:
    api['extra']['host'] = '127.0.0.1'
if 'key' not in api:
    api['key'] = ''
api['cors_origins'] = '*'
with open(path, 'w') as f:
    yaml.dump(cfg, f, default_flow_style=False)
print('OK')
" 2>/dev/null`
	out, err := wsl(py)
	return err == nil && strings.HasPrefix(out, "OK")
}

func checkGateWayEnv() bool {
	out, err := wsl("grep -c 'GATEWAY_ALLOW_ALL_USERS=true' /root/.hermes/profiles/deepseek/.env 2>/dev/null")
	if err != nil {
		return false
	}
	n, _ := strconv.Atoi(out)
	return n > 0
}

func fixGateWayEnv() bool {
	_, err := wsl("echo 'GATEWAY_ALLOW_ALL_USERS=true' >> /root/.hermes/profiles/deepseek/.env")
	return err == nil
}

func checkActiveProfile() bool {
	ok1 := wslExit0("test -f /root/.hermes/active_profile && grep -q deepseek /root/.hermes/active_profile")
	ok2 := wslExit0("test -f /root/.hermes/profiles/deepseek/active_profile && grep -q deepseek /root/.hermes/profiles/deepseek/active_profile")
	return ok1 && ok2
}

func fixActiveProfile() bool {
	wsl("mkdir -p /root/.hermes/profiles/deepseek")
	wsl("echo 'deepseek' > /root/.hermes/active_profile")
	wsl("echo 'deepseek' > /root/.hermes/profiles/deepseek/active_profile")
	return checkActiveProfile()
}

func checkSQLite() bool {
	hasSHM := wslExit0("test -f /root/.hermes-web-ui/hermes-web-ui.db-shm")
	hasWAL := wslExit0("test -f /root/.hermes-web-ui/hermes-web-ui.db-wal")
	if hasSHM || hasWAL {
		return false
	}
	hasDB := wslExit0("test -f /root/.hermes-web-ui/hermes-web-ui.db")
	webUIRunning := wslExit0("pgrep -f 'hermes-web-ui/dist/server' >/dev/null 2>&1")
	if hasDB && webUIRunning {
		listening := wslExit0("ss -tlnp 2>/dev/null | grep -q 8648")
		if !listening {
			return false
		}
	}
	return true
}

func fixSQLite() bool {
	wsl("rm -f /root/.hermes-web-ui/hermes-web-ui.db-shm /root/.hermes-web-ui/hermes-web-ui.db-wal")
	webUIProc := wslExit0("pgrep -f 'hermes-web-ui/dist/server' >/dev/null 2>&1")
	listening := wslExit0("ss -tlnp 2>/dev/null | grep -q 8648")
	if webUIProc && !listening {
		fmt.Print("(Web UI 进程挂起, 清理数据库...) ")
		wsl("pkill -9 -f 'hermes-web-ui' 2>/dev/null; sleep 1; rm -f /root/.hermes-web-ui/hermes-web-ui.db")
		return true
	}
	return !wslExit0("test -f /root/.hermes-web-ui/hermes-web-ui.db-shm") &&
		!wslExit0("test -f /root/.hermes-web-ui/hermes-web-ui.db-wal")
}

func checkLoginLock() bool {
	return !wslExit0("test -f /root/.hermes-web-ui/.login-lock.json")
}

func fixLoginLock() bool {
	wsl("rm -f /root/.hermes-web-ui/.login-lock.json")
	return checkLoginLock()
}

func checkGlobalYAML() bool {
	return wslExit0(`python3 -c "import yaml; yaml.safe_load(open('/root/.hermes/config.yaml'))" 2>/dev/null`)
}

func fixGlobalYAML() bool {
	wsl(`python3 -c "
import yaml, sys
path = '/root/.hermes/config.yaml'
with open(path) as f:
    data = f.read()
lines = data.split('\n')
seen = {}
new_lines = []
for line in lines:
    key = line.split(':')[0].strip() if ':' in line else ''
    if key in ('plugins',) and key in seen:
        continue
    if key:
        seen[key] = True
    new_lines.append(line)
with open(path, 'w') as f:
    f.write('\n'.join(new_lines))
print('OK')
" 2>/dev/null`)
	return checkGlobalYAML()
}

func checkDefaultProfile() bool {
	return !wslExit0("test -d /root/.hermes/profiles/default")
}

func fixDefaultProfile() bool {
	wsl("hermes profile delete default 2>/dev/null; rm -rf /root/.hermes/profiles/default 2>/dev/null")
	return checkDefaultProfile()
}

func getActualPort() string {
	out, err := wsl(`python3 -c "
import yaml
cfg = yaml.safe_load(open('/root/.hermes/profiles/deepseek/config.yaml'))
print(cfg.get('platforms',{}).get('api_server',{}).get('extra',{}).get('port',8642))
" 2>/dev/null`)
	if err != nil {
		return "8642"
	}
	return strings.TrimSpace(out)
}

func checkGateway() bool {
	port := getActualPort()
	return wslExit0(fmt.Sprintf("ss -tlnp 2>/dev/null | grep -q ':%s'", port))
}

func fixGateway() bool {
	wsl(`pkill -9 -f "hermes.*gateway run" 2>/dev/null; sleep 1;
rm -f /root/.hermes/gateway.lock /root/.hermes/profiles/deepseek/gateway.lock
rm -f /root/.hermes/gateway_state.json /root/.hermes/profiles/deepseek/gateway_state.json
rm -f /root/.hermes/gateway/gateway.pid /root/.hermes/profiles/deepseek/gateway.pid`)
	// Start gateway with nohup, poll up to 40 seconds
	port := getActualPort()
	_, err := wsl(fmt.Sprintf(`nohup hermes gateway run --replace --accept-hooks > /dev/null 2>&1 &
disown
for i in 1 2 3 4 5 6 7 8; do
  ss -tlnp 2>/dev/null | grep -q ':%s' && break
  sleep 5
done
ss -tlnp 2>/dev/null | grep -q ':%s'`, port, port))
	return err == nil
}

func checkWebUI() bool {
	// Web UI 始终在 8648，不受 api_server 端口影响
	return wslExit0("ss -tlnp 2>/dev/null | grep -q ':8648'")
}

func fixWebUI() bool {
	wsl(`pkill -9 -f "hermes-web-ui" 2>/dev/null; sleep 1;
rm -f /root/.hermes-web-ui/server.pid
rm -f /root/.hermes-web-ui/hermes-web-ui.db-shm /root/.hermes-web-ui/hermes-web-ui.db-wal`)
	listening := wslExit0("ss -tlnp 2>/dev/null | grep -q 8648")
	if !listening {
		wsl("rm -f /root/.hermes-web-ui/hermes-web-ui.db")
	}
	_, err := wsl(`nohup env HOME=/root hermes-web-ui start 8648 > /root/.hermes-web-ui/server.log 2>&1 &
disown
for i in 1 2 3 4 5 6 7 8; do
  ss -tlnp 2>/dev/null | grep -q ':8648' && break
  sleep 5
done
ss -tlnp 2>/dev/null | grep -q ':8648'`)
	return err == nil
}

func checkPortConsistency() bool {
	configPort := getActualPort()
	return configPort == "8642"
}

func fixPortConsistency() bool {
	_, err := wsl(`python3 -c "
import yaml
path = '/root/.hermes/profiles/deepseek/config.yaml'
with open(path) as f:
    cfg = yaml.safe_load(f)
api = cfg.get('platforms',{}).get('api_server',{})
if api.get('extra',{}).get('port',8642) != 8642:
    api['extra']['port'] = 8642
    with open(path, 'w') as f:
        yaml.dump(cfg, f, default_flow_style=False)
    print('fixed')
else:
    print('ok')
" 2>/dev/null`)
	return err == nil
}

func checkGatewayHealth() bool {
	port := getActualPort()
	out, err := wsl(fmt.Sprintf("curl -s --connect-timeout 3 http://127.0.0.1:%s/health 2>/dev/null", port))
	if err != nil {
		return false
	}
	return strings.Contains(out, `"status": "ok"`)
}

func checkAccessInfo() bool {
	port := getActualPort()
	out, err := wsl(fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' --connect-timeout 3 http://127.0.0.1:%s/ 2>/dev/null", port))
	if err != nil {
		return false
	}
	return out == "200"
}

func getToken() string {
	out, err := wsl("cat /root/.hermes-web-ui/.token 2>/dev/null")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
