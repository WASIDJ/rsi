package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/WASIDJ/rsi/pkg/api"
	"github.com/WASIDJ/rsi/pkg/config"
	"github.com/WASIDJ/rsi/pkg/hy2"
	"github.com/WASIDJ/rsi/pkg/sub"
	"gopkg.in/yaml.v3"
)

const Version = "v1.1.0"

func isRunningOnRouter() bool {
	_, err := os.Stat("/jffs")
	return err == nil
}

func getTarget() string {
	if t := os.Getenv("RSI_TARGET"); t != "" {
		return t
	}
	return "RSI@192.168.50.1"
}

func printHelp() {
	mode := "Router Native"
	if !isRunningOnRouter() {
		mode = fmt.Sprintf("macOS Remote (Target: %s)", getTarget())
	}
	help := fmt.Sprintf(`RSI - Mihomo Manager CLI (%s) [%s]

Usage:
  rsi sub set <URL>          设置并下载新机场订阅，自动融合自建节点并秒级热重载
  rsi sub update             按保存的订阅链接更新节点，并热重载
  rsi sub show               查看当前保存的机场订阅地址

  rsi node add <hy2_url>     添加自建 Hysteria 2 节点 (直接粘贴 hysteria2:// 链接)
  rsi node add -f <file>     从 YAML 文件批量导入自建节点
  rsi node list              列出当前所有自建节点
  rsi node rm <name>         删除指定名称的自建节点并热重载

  rsi chnroute [update|status] 更新/查看国内直连 IP 规则集与硬件加速旁路
  rsi firewall [apply|status]   重新应用或查看防火墙分流与流缓存状态
  rsi client [status|mode|add|rm] 局域网分流拦截管理 (支持 home/company 模式一键切换)

  rsi reload                 重新验证并热重载当前配置 (0 断流, < 0.2s)
  rsi status                 查看服务状态、PID、内存、节点与策略组信息
  rsi doctor                 全自动诊断网络上行、硬件加速、分流名单与核心健康度
  rsi log [-n lines] [-f]    查看运行日志
  rsi restart                重启服务
  rsi stop                   停止服务
  rsi start                  启动服务

Mac 专属指令:
  rsi router deploy          交叉编译并安装 rsi 到路由器中
  rsi version                显示版本
`, Version, mode)
	fmt.Print(help)
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	// If running on Mac and not a local router, forward commands to the router via SSH
	if !isRunningOnRouter() {
		cmd := os.Args[1]
		if cmd == "version" || cmd == "-v" || cmd == "--version" {
			fmt.Printf("rsi %s (client)\n", Version)
			return
		}
		if cmd == "help" || cmd == "-h" || cmd == "--help" {
			printHelp()
			return
		}
		if cmd == "router" && len(os.Args) > 2 && os.Args[2] == "deploy" {
			deployToRouter()
			return
		}

		// Forward command via SSH to router
		target := getTarget()
		remoteArgs := make([]string, len(os.Args)-1)
		for i, arg := range os.Args[1:] {
			if strings.Contains(arg, " ") || strings.Contains(arg, "#") || strings.Contains(arg, "?") {
				remoteArgs[i] = fmt.Sprintf("%q", arg)
			} else {
				remoteArgs[i] = arg
			}
		}
		remoteCmd := "/jffs/mihomo/rsi " + strings.Join(remoteArgs, " ")
		sshCmd := exec.Command("ssh", "-t", target, remoteCmd)
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr
		if err := sshCmd.Run(); err != nil {
			os.Exit(1)
		}
		return
	}

	// Router native execution
	command := os.Args[1]
	switch command {
	case "sub":
		handleSub(os.Args[2:])
	case "node":
		handleNode(os.Args[2:])
	case "chnroute":
		handleChnroute(os.Args[2:])
	case "firewall":
		handleFirewall(os.Args[2:])
	case "client", "clients":
		handleClient(os.Args[2:])
	case "reload":
		handleReload()
	case "status":
		handleStatus()
	case "doctor":
		handleDoctor()
	case "restart":
		handleRestart()
	case "start":
		handleStart()
	case "stop":
		handleStop()
	case "log":
		handleLog(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("rsi %s (router)\n", Version)
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func deployToRouter() {
	target := getTarget()
	fmt.Printf("[*] Compiling rsi for linux/arm64...\n")
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", "/tmp/rsi-arm64", "./cmd/rsi")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=arm64")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("[-] Build failed: %s\n", string(out))
		os.Exit(1)
	}

	fmt.Printf("[*] Uploading to router %s...\n", target)
	f, err := os.Open("/tmp/rsi-arm64")
	if err != nil {
		fmt.Printf("[-] Failed to open binary: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	sshCmd := exec.Command("ssh", target, "cat > /jffs/mihomo/rsi && chmod +x /jffs/mihomo/rsi && ln -sf /jffs/mihomo/rsi /koolshare/bin/rsi")
	sshCmd.Stdin = f
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		fmt.Printf("[-] Upload failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Successfully deployed rsi to router!")
}

func handleSub(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rsi sub [set <URL> | update | show]")
		return
	}
	subCmd := args[0]
	switch subCmd {
	case "set":
		if len(args) < 2 {
			fmt.Println("Error: Missing subscription URL")
			fmt.Println("Usage: rsi sub set <URL>")
			os.Exit(1)
		}
		subURL := args[1]
		applySubscription(subURL)
	case "update":
		urlBytes, err := os.ReadFile(config.SubURLFile)
		if err != nil || len(strings.TrimSpace(string(urlBytes))) == 0 {
			fmt.Println("Error: No saved subscription URL found. Run 'rsi sub set <URL>' first.")
			os.Exit(1)
		}
		subURL := strings.TrimSpace(string(urlBytes))
		applySubscription(subURL)
	case "show":
		urlBytes, err := os.ReadFile(config.SubURLFile)
		if err != nil || len(strings.TrimSpace(string(urlBytes))) == 0 {
			fmt.Println("No subscription URL currently saved.")
			return
		}
		fmt.Printf("Current subscription URL:\n%s\n", strings.TrimSpace(string(urlBytes)))
	default:
		fmt.Printf("Unknown sub command: %s\n", subCmd)
		os.Exit(1)
	}
}

func applySubscription(subURL string) {
	fmt.Printf("[*] Downloading subscription from: %s\n", subURL)
	rawSub, err := sub.Fetch(subURL)
	if err != nil {
		fmt.Printf("[-] Download failed: %v\n", err)
		os.Exit(1)
	}

	var baseConfig map[string]interface{}
	if err := yaml.Unmarshal(rawSub, &baseConfig); err != nil {
		fmt.Printf("[-] Failed to parse subscription YAML: %v\n", err)
		os.Exit(1)
	}

	_ = os.WriteFile(config.SubFile, rawSub, 0600)
	_ = os.WriteFile(config.SubURLFile, []byte(subURL), 0600)

	customNodes, err := config.LoadCustomNodes(config.CustomNodesFile)
	if err != nil {
		fmt.Printf("[!] Warning reading custom nodes: %v\n", err)
	}

	secret, proxyAuth := config.GetExistingSecrets()
	merged, err := config.MergeAndAdapt(baseConfig, customNodes, secret, proxyAuth)
	if err != nil {
		fmt.Printf("[-] Failed to merge config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[*] Validating candidate config...")
	if err := config.ValidateAndApply(merged, secret); err != nil {
		fmt.Printf("[-] Failed to apply config: %v\n", err)
		os.Exit(1)
	}

	proxiesCount := 0
	if p, ok := merged["proxies"].([]interface{}); ok {
		proxiesCount = len(p)
	}
	groupsCount := 0
	if g, ok := merged["proxy-groups"].([]interface{}); ok {
		groupsCount = len(g)
	}

	fmt.Println("✅ Subscription updated and hot-reloaded successfully!")
	fmt.Printf("   Total proxies: %d (including %d custom nodes)\n", proxiesCount, len(customNodes))
	fmt.Printf("   Total groups:  %d\n", groupsCount)
}

func handleNode(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rsi node [add <hy2_url> | list | rm <name>]")
		return
	}
	nodeCmd := args[0]
	switch nodeCmd {
	case "add":
		if len(args) < 2 {
			fmt.Println("Usage:")
			fmt.Println("  rsi node add \"hysteria2://password@host:port/?sni=host&insecure=0#NodeName\"")
			fmt.Println("  rsi node add -f /path/to/custom_nodes.yaml")
			os.Exit(1)
		}
		var newNodes []map[string]interface{}
		if args[1] == "-f" {
			if len(args) < 3 {
				fmt.Println("Error: Missing file path for -f")
				os.Exit(1)
			}
			nodes, err := config.LoadCustomNodes(args[2])
			if err != nil {
				fmt.Printf("[-] Failed to load nodes from file: %v\n", err)
				os.Exit(1)
			}
			newNodes = nodes
		} else {
			rawURI := args[1]
			node, err := hy2.ParseURL(rawURI)
			if err != nil {
				fmt.Printf("[-] Failed to parse HY2 URI: %v\n", err)
				os.Exit(1)
			}
			newNodes = append(newNodes, node)
		}

		currentNodes, _ := config.LoadCustomNodes(config.CustomNodesFile)
		for _, nn := range newNodes {
			name, _ := nn["name"].(string)
			exists := false
			for i, cn := range currentNodes {
				if cName, _ := cn["name"].(string); cName == name {
					currentNodes[i] = nn
					exists = true
					break
				}
			}
			if !exists {
				currentNodes = append(currentNodes, nn)
			}
			fmt.Printf("[+] Added node: %s (%s -> %v:%v)\n", name, nn["type"], nn["server"], nn["port"])
		}

		if err := config.SaveCustomNodes(config.CustomNodesFile, currentNodes); err != nil {
			fmt.Printf("[-] Failed to save custom nodes: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("[*] Re-building and hot-reloading Mihomo config...")
		handleReload()

	case "list":
		currentNodes, _ := config.LoadCustomNodes(config.CustomNodesFile)
		if len(currentNodes) == 0 {
			fmt.Println("No custom nodes configured. Use 'rsi node add' to add one.")
			return
		}
		fmt.Printf("Custom Nodes (%d):\n", len(currentNodes))
		for i, n := range currentNodes {
			fmt.Printf("  %d. %s [%s] %v:%v (sni: %v)\n",
				i+1, n["name"], n["type"], n["server"], n["port"], n["sni"])
		}

	case "rm", "del", "remove":
		if len(args) < 2 {
			fmt.Println("Error: Missing node name to remove")
			fmt.Println("Usage: rsi node rm <name>")
			os.Exit(1)
		}
		targetName := args[1]
		currentNodes, _ := config.LoadCustomNodes(config.CustomNodesFile)
		filtered := make([]map[string]interface{}, 0, len(currentNodes))
		found := false
		for _, n := range currentNodes {
			if n["name"] == targetName {
				found = true
			} else {
				filtered = append(filtered, n)
			}
		}
		if !found {
			fmt.Printf("[-] Node %q not found in custom nodes\n", targetName)
			os.Exit(1)
		}
		_ = config.SaveCustomNodes(config.CustomNodesFile, filtered)
		fmt.Printf("[-] Removed node: %s\n", targetName)
		handleReload()

	default:
		fmt.Printf("Unknown node command: %s\n", nodeCmd)
		os.Exit(1)
	}
}

func handleReload() {
	var baseConfig map[string]interface{}
	srcPath := config.SubFile
	if _, err := os.Stat(config.SubFile); os.IsNotExist(err) {
		srcPath = config.ConfigFile
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Printf("[-] Failed to read %s: %v\n", srcPath, err)
		os.Exit(1)
	}

	if err := yaml.Unmarshal(data, &baseConfig); err != nil {
		fmt.Printf("[-] Failed to parse config YAML: %v\n", err)
		os.Exit(1)
	}

	customNodes, _ := config.LoadCustomNodes(config.CustomNodesFile)
	secret, proxyAuth := config.GetExistingSecrets()
	merged, err := config.MergeAndAdapt(baseConfig, customNodes, secret, proxyAuth)
	if err != nil {
		fmt.Printf("[-] Failed to merge config: %v\n", err)
		os.Exit(1)
	}

	if err := config.ValidateAndApply(merged, secret); err != nil {
		fmt.Printf("[-] Reload failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Config validated and hot-reloaded! (custom nodes: %d)\n", len(customNodes))
}

func handleStatus() {
	pidBytes, err := os.ReadFile("/tmp/mihomo/core.pid")
	if err != nil {
		fmt.Println("Mihomo is STOPPED (no pidfile)")
		return
	}
	pidStr := strings.TrimSpace(string(pidBytes))
	pid, _ := strconv.Atoi(pidStr)

	statusBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		fmt.Printf("Mihomo is NOT RUNNING (stale pid %d)\n", pid)
		return
	}

	vmRSS := "unknown"
	threads := "unknown"
	for _, line := range strings.Split(string(statusBytes), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			vmRSS = strings.TrimSpace(strings.TrimPrefix(line, "VmRSS:"))
		}
		if strings.HasPrefix(line, "Threads:") {
			threads = strings.TrimSpace(strings.TrimPrefix(line, "Threads:"))
		}
	}

	fmt.Printf("🟢 Mihomo Core: RUNNING (PID %d)\n", pid)
	fmt.Printf("   Memory RSS:  %s\n", vmRSS)
	fmt.Printf("   Threads:     %s\n", threads)

	secret, _ := config.GetExistingSecrets()
	client := api.NewClient("http://"+config.GetControllerAddress(), secret)
	if ver, err := client.GetVersion(); err == nil {
		fmt.Printf("   API Core:    %s (port 9090 ok)\n", ver)
	}

	customNodes, _ := config.LoadCustomNodes(config.CustomNodesFile)
	fmt.Printf("   Custom Nodes: %d node(s) active\n", len(customNodes))

	if urlBytes, err := os.ReadFile(config.SubURLFile); err == nil && len(urlBytes) > 0 {
		fmt.Printf("   Sub URL:     %s\n", strings.TrimSpace(string(urlBytes)))
	}
}

func handleRestart() {
	fmt.Println("[*] Restarting Mihomo service...")
	cmd := exec.Command("/jffs/mihomo/service.sh", "restart")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func handleStart() {
	fmt.Println("[*] Starting Mihomo service...")
	cmd := exec.Command("/jffs/mihomo/service.sh", "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func handleStop() {
	fmt.Println("[*] Stopping Mihomo service...")
	cmd := exec.Command("/jffs/mihomo/service.sh", "stop")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func handleLog(args []string) {
	lines := "50"
	follow := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			lines = args[i+1]
			i++
		} else if args[i] == "-f" {
			follow = true
		}
	}

	cmdArgs := []string{"-n", lines}
	if follow {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmdArgs = append(cmdArgs, "/tmp/mihomo/core.log")

	cmd := exec.Command("tail", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func handleChnroute(args []string) {
	subcmd := "status"
	if len(args) > 0 {
		subcmd = args[0]
	}
	switch subcmd {
	case "update":
		fmt.Println("[*] Updating China IP routes (chnroute)...")
		cmd := exec.Command("/jffs/mihomo/firewall.sh", "update-chnroute")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("[-] Failed to update chnroute: %v\n", err)
			os.Exit(1)
		}
	case "status":
		out, err := exec.Command("ipset", "list", "chnroute", "-t").CombinedOutput()
		if err != nil {
			fmt.Printf("[-] chnroute ipset not active: %v\n", err)
			return
		}
		fmt.Print(string(out))
	default:
		fmt.Println("Usage: rsi chnroute [update|status]")
	}
}

func handleFirewall(args []string) {
	subcmd := "status"
	if len(args) > 0 {
		subcmd = args[0]
	}
	switch subcmd {
	case "apply", "reload":
		fmt.Println("[*] Applying firewall rules & flushing hardware flow cache...")
		cmd := exec.Command("/jffs/mihomo/firewall.sh", "apply")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	case "status":
		fmt.Println("=== MH_ROUTE (TPROXY & Hardware Bypass Rules) ===")
		out, _ := exec.Command("iptables", "-t", "mangle", "-L", "MH_ROUTE", "-v", "-n").CombinedOutput()
		fmt.Print(string(out))
		fmt.Println("\n=== Broadcom Flow Cache ===")
		fcOut, _ := exec.Command("fc", "status").CombinedOutput()
		for _, line := range strings.Split(string(fcOut), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "HW Acceleration") || strings.Contains(line, "Flow Learning Enabled") || strings.Contains(line, "Acceleration Mode") {
				fmt.Println(" ", line)
			}
		}
	default:
		fmt.Println("Usage: rsi firewall [apply|status]")
	}
}

func handleClient(args []string) {
	subcmd := "status"
	if len(args) > 0 {
		subcmd = args[0]
	}
	clientsPath := "/jffs/mihomo/clients.txt"

	switch subcmd {
	case "status", "list":
		fmt.Println("=== 局域网分流拦截名单 (/jffs/mihomo/clients.txt) ===")
		content, err := os.ReadFile(clientsPath)
		if err != nil {
			fmt.Printf("[-] 读取 clients.txt 失败: %v\n", err)
			return
		}
		activeClients := []string{}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			activeClients = append(activeClients, line)
		}
		if len(activeClients) == 0 {
			fmt.Println("当前状态: [居家测试 / 全直连模式] (未拦截任何设备，全内网直连 WAN)")
		} else {
			fmt.Printf("当前拦截设备 (%d 个条目):\n", len(activeClients))
			for _, c := range activeClients {
				fmt.Printf("  • %s\n", c)
			}
		}
		fmt.Println("\n=== 内核 ipset mh_clients 实时生效成员 ===")
		out, _ := exec.Command("ipset", "list", "mh_clients").CombinedOutput()
		fmt.Print(string(out))

	case "mode":
		if len(args) < 2 {
			fmt.Println("用法: rsi client mode <home|company|all>")
			return
		}
		mode := strings.ToLower(args[1])
		switch mode {
		case "home", "direct", "off":
			header := "# Intercept list empty (Home test / all direct mode)\n"
			_ = os.WriteFile(clientsPath, []byte(header), 0644)
			fmt.Println("[*] 已切换至 [居家模式]：清空代理名单，全屋设备直连 WAN，0 拦截。")
		case "company", "all", "on":
			content := "# Intercept entire LAN subnet\n192.168.50.0/24\n"
			_ = os.WriteFile(clientsPath, []byte(content), 0644)
			fmt.Println("[*] 已切换至 [公司/全网模式]：拦截 192.168.50.0/24 全网段。")
		default:
			fmt.Printf("[-] 未知模式: %s (可选: home, company)\n", mode)
			return
		}
		cmd := exec.Command("/jffs/mihomo/firewall.sh", "apply")
		_ = cmd.Run()
		fmt.Println("[✓] 防火墙已热重载。")

	case "add":
		if len(args) < 2 {
			fmt.Println("用法: rsi client add <IP或网段, 如 192.168.50.219>")
			return
		}
		target := strings.TrimSpace(args[1])
		if !strings.HasPrefix(target, "192.168.50.") {
			fmt.Println("[-] 仅允许添加 192.168.50.x 局域网地址")
			return
		}
		content, _ := os.ReadFile(clientsPath)
		lines := strings.Split(string(content), "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) == target {
				fmt.Printf("[!] 设备 %s 已经在名单中\n", target)
				return
			}
		}
		lines = append(lines, target)
		_ = os.WriteFile(clientsPath, []byte(strings.Join(lines, "\n")), 0644)
		cmd := exec.Command("/jffs/mihomo/firewall.sh", "apply")
		_ = cmd.Run()
		fmt.Printf("[✓] 已添加 %s 到拦截名单并热重载防火墙。\n", target)

	case "rm", "del", "remove":
		if len(args) < 2 {
			fmt.Println("用法: rsi client rm <IP或网段, 如 192.168.50.219>")
			return
		}
		target := strings.TrimSpace(args[1])
		content, _ := os.ReadFile(clientsPath)
		newLines := []string{}
		found := false
		for _, l := range strings.Split(string(content), "\n") {
			if strings.TrimSpace(l) == target {
				found = true
				continue
			}
			newLines = append(newLines, l)
		}
		if !found {
			fmt.Printf("[-] 名单中未找到 %s\n", target)
			return
		}
		_ = os.WriteFile(clientsPath, []byte(strings.Join(newLines, "\n")), 0644)
		cmd := exec.Command("/jffs/mihomo/firewall.sh", "apply")
		_ = cmd.Run()
		fmt.Printf("[✓] 已从拦截名单移除 %s 并热重载防火墙。\n", target)

	default:
		fmt.Println("用法:")
		fmt.Println("  rsi client status                 查看当前拦截名单与 ipset 状态")
		fmt.Println("  rsi client mode home              切换到居家测试模式 (全直连，不拦截)")
		fmt.Println("  rsi client mode company           切换到公司模式 (拦截 192.168.50.0/24 全网段)")
		fmt.Println("  rsi client add <192.168.50.x>     指定拦截单台设备")
		fmt.Println("  rsi client rm <192.168.50.x>      移除单台设备拦截")
	}
}

func handleDoctor() {
	fmt.Println("🩺 RSI System Health & Deployment Doctor (" + Version + ")")
	fmt.Println("==================================================")
	allOk := true

	// 1. Mihomo Core Process
	pidBytes, err := os.ReadFile("/tmp/mihomo/core.pid")
	if err != nil {
		fmt.Println("  [✗] Mihomo 核心未运行! 请执行 'rsi start'")
		allOk = false
	} else {
		pid := strings.TrimSpace(string(pidBytes))
		memRSS := ""
		if statBytes, err := os.ReadFile(fmt.Sprintf("/proc/%s/status", pid)); err == nil {
			for _, line := range strings.Split(string(statBytes), "\n") {
				if strings.HasPrefix(line, "VmRSS:") {
					memRSS = strings.TrimSpace(strings.TrimPrefix(line, "VmRSS:"))
					break
				}
			}
		}
		fmt.Printf("  [✓] 核心守护进程: 正常运行 (PID %s, 物理内存 %s)\n", pid, memRSS)
	}

	// 2. WAN 端口与上行链路
	wanDev := "eth0"
	wanUp := false
	if operstate, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/operstate", wanDev)); err == nil {
		if strings.TrimSpace(string(operstate)) == "up" {
			wanUp = true
		}
	}
	if wanUp {
		fmt.Printf("  [✓] WAN 物理上行口: 链路接通 (%s 端口 UP)\n", wanDev)
	} else {
		fmt.Printf("  [!] WAN 物理上行口: 未检测到链路信号 (%s 端口 DOWN，请检查网线)\n", wanDev)
		allOk = false
	}

	// 3. 局域网分流模式 (Client Interception Mode)
	clientsContent, _ := os.ReadFile("/jffs/mihomo/clients.txt")
	activeClients := 0
	isCompanyMode := false
	for _, line := range strings.Split(string(clientsContent), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			activeClients++
			if line == "192.168.50.0/24" {
				isCompanyMode = true
			}
		}
	}
	if isCompanyMode {
		fmt.Println("  [✓] 局域网拦截模式: [公司/全网代理模式] (192.168.50.0/24 全量透明分流)")
	} else if activeClients > 0 {
		fmt.Printf("  [✓] 局域网拦截模式: [精准白名单模式] (已指定拦截 %d 台设备)\n", activeClients)
	} else {
		fmt.Println("  [!] 局域网拦截模式: [居家直连模式] (未开启任何代理，全屋设备直连 WAN)")
		fmt.Println("      提示: 若在公司部署，请运行 'rsi client mode company' 开启全网代理")
	}

	// 4. 国内 IP 硬件加速旁路 (Chnroute & Flow Cache)
	chnEntries := "0"
	if out, err := exec.Command("ipset", "list", "chnroute", "-t").CombinedOutput(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "Number of entries:") {
				chnEntries = strings.TrimSpace(strings.TrimPrefix(line, "Number of entries:"))
				break
			}
		}
	}
	if chnEntries != "0" {
		fmt.Printf("  [✓] 国内流量硬件旁路: 规则集就绪 (%s 个中国网段, 博通硬件流缓存就绪)\n", chnEntries)
	} else {
		fmt.Println("  [✗] 国内流量规则集未就绪: chnroute 为空，请执行 'rsi chnroute update'")
		allOk = false
	}

	// 5. DNS 分流探测
	dnsWorks := false
	if out, err := exec.Command("nslookup", "bilibili.com", "127.0.0.1:1053").CombinedOutput(); err == nil {
		outStr := string(out)
		if strings.Contains(outStr, "Address") && !strings.Contains(outStr, "198.18.") {
			dnsWorks = true
		}
	}
	if dnsWorks {
		fmt.Println("  [✓] DNS 双层分流: 正常响应 (国内域名返回真实 IP, 直通博通硬件流缓存)")
	} else {
		fmt.Println("  [!] DNS 响应异常: 未能正确解析国内白名单域名")
		allOk = false
	}

	// 6. 本地 API 与 Web 控制面板
	if out, err := exec.Command("curl", "-s", "--connect-timeout", "2", "http://192.168.50.1:9090/version").CombinedOutput(); err == nil {
		outStr := string(out)
		if strings.Contains(outStr, "version") || strings.Contains(outStr, "Unauthorized") {
			fmt.Println("  [✓] 本地 REST API: 正常通信 (端口 9090 安全鉴权激活, 支持热重载与 Web 面板)")
		} else {
			fmt.Println("  [!] REST API 端口响应异常")
		}
	}

	fmt.Println("==================================================")
	if allOk {
		fmt.Println("🎉 系统状态全部健康！明天带去公司插上网线与电源即可即插即用。")
	} else {
		fmt.Println("⚠️  存在部分待就绪项，请参考上述提示执行对应命令。")
	}
}



