package config

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/WASIDJ/rsi/pkg/api"
	"github.com/WASIDJ/rsi/pkg/hy2"
	"gopkg.in/yaml.v3"
)

const (
	BaseDir         = "/jffs/mihomo"
	RunDir          = "/tmp/mihomo"
	ConfigFile      = BaseDir + "/config.yaml"
	SubFile         = BaseDir + "/subscription.yaml"
	CustomNodesFile = BaseDir + "/custom_nodes.yaml"
	SubURLFile      = BaseDir + "/sub.url"
	CandidateFile   = BaseDir + "/config.yaml.new"
	SymlinkConfig   = RunDir + "/config.yaml"
	CoreBinary      = RunDir + "/mihomo"
)

// LoadCustomNodes loads custom nodes from a YAML file.
func LoadCustomNodes(path string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw []interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		var dict struct {
			Proxies     []interface{} `yaml:"proxies"`
			CustomNodes []interface{} `yaml:"custom_nodes"`
		}
		if err2 := yaml.Unmarshal(data, &dict); err2 == nil {
			if len(dict.Proxies) > 0 {
				raw = dict.Proxies
			} else {
				raw = dict.CustomNodes
			}
		} else {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
	}

	var nodes []map[string]interface{}
	for _, item := range raw {
		switch v := item.(type) {
		case string:
			v = strings.TrimSpace(v)
			if strings.HasPrefix(v, "hysteria2://") || strings.HasPrefix(v, "hy2://") {
				node, err := hy2.ParseURL(v)
				if err != nil {
					return nil, fmt.Errorf("failed to parse URI %q: %w", v, err)
				}
				nodes = append(nodes, node)
			}
		case map[string]interface{}:
			nodes = append(nodes, v)
		}
	}
	return nodes, nil
}

// SaveCustomNodes writes custom nodes back to file.
func SaveCustomNodes(path string, nodes []map[string]interface{}) error {
	data, err := yaml.Marshal(nodes)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// MergeAndAdapt merges subscription and custom nodes, applying router network settings.
func MergeAndAdapt(baseConfig map[string]interface{}, customNodes []map[string]interface{}, secret, proxyAuth string) (map[string]interface{}, error) {
	cfg := make(map[string]interface{})
	for k, v := range baseConfig {
		cfg[k] = v
	}

	// 1. Proxies
	var proxies []interface{}
	if p, ok := cfg["proxies"].([]interface{}); ok {
		proxies = append(proxies, p...)
	}

	customNames := make([]string, 0, len(customNodes))
	for _, cn := range customNodes {
		name, _ := cn["name"].(string)
		if name != "" {
			customNames = append(customNames, name)
		}
	}

	filteredProxies := make([]interface{}, 0, len(proxies))
	for _, p := range proxies {
		if pMap, ok := p.(map[string]interface{}); ok {
			pName, _ := pMap["name"].(string)
			isDup := false
			for _, cn := range customNames {
				if pName == cn {
					isDup = true
					break
				}
			}
			if !isDup {
				filteredProxies = append(filteredProxies, p)
			}
		} else {
			filteredProxies = append(filteredProxies, p)
		}
	}
	for _, cn := range customNodes {
		filteredProxies = append(filteredProxies, cn)
	}
	cfg["proxies"] = filteredProxies

	// 2. Proxy groups
	var groups []interface{}
	if g, ok := cfg["proxy-groups"].([]interface{}); ok {
		groups = append(groups, g...)
	}

	if len(customNames) > 0 {
		customGroup := map[string]interface{}{
			"name":    "⚡ 自建节点",
			"type":    "select",
			"proxies": customNames,
		}
		cleanGroups := make([]interface{}, 0, len(groups))
		for _, g := range groups {
			if gMap, ok := g.(map[string]interface{}); ok {
				if gMap["name"] != "⚡ 自建节点" {
					cleanGroups = append(cleanGroups, g)
				}
			}
		}
		groups = append([]interface{}{customGroup}, cleanGroups...)

		for _, g := range groups {
			gMap, ok := g.(map[string]interface{})
			if !ok || gMap["name"] == "⚡ 自建节点" {
				continue
			}
			gType, _ := gMap["type"].(string)
			if gType == "select" || gType == "fallback" || gType == "url-test" || gType == "load-balance" {
				gProxies, ok := gMap["proxies"].([]interface{})
				if !ok {
					continue
				}
				newProxies := make([]interface{}, 0, len(gProxies)+len(customNames))
				for _, cn := range customNames {
					newProxies = append(newProxies, cn)
				}
				for _, gp := range gProxies {
					gpStr, _ := gp.(string)
					isDup := false
					for _, cn := range customNames {
						if gpStr == cn {
							isDup = true
							break
						}
					}
					if !isDup {
						newProxies = append(newProxies, gp)
					}
				}
				gMap["proxies"] = newProxies
			}
		}
	}
	cfg["proxy-groups"] = groups

	// 3. Router adaptations
	cfg["mixed-port"] = 7890
	cfg["tproxy-port"] = 7893
	cfg["allow-lan"] = true
	cfg["bind-address"] = "*"
	cfg["lan-allowed-ips"] = []string{"127.0.0.0/8", "192.168.50.0/24"}
	cfg["external-controller"] = "192.168.50.1:9090"
	if secret != "" {
		cfg["secret"] = secret
	}
	if proxyAuth != "" {
		cfg["authentication"] = []string{"router:" + proxyAuth}
	}
	cfg["external-ui"] = "/tmp/mihomo/ui"
	cfg["skip-auth-prefixes"] = []string{"127.0.0.1/32"}
	cfg["ipv6"] = false
	cfg["find-process-mode"] = "off"
	cfg["interface-name"] = "eth0"
	cfg["log-level"] = "warning"
	cfg["geodata-mode"] = false
	cfg["geo-auto-update"] = false
	cfg["profile"] = map[string]interface{}{
		"store-selected": true,
		"store-fake-ip":  true,
	}

	// 4. DNS
	dnsMap := make(map[string]interface{})
	if d, ok := cfg["dns"].(map[string]interface{}); ok {
		for k, v := range d {
			dnsMap[k] = v
		}
	}
	dnsMap["listen"] = "0.0.0.0:1053"
	dnsMap["ipv6"] = false

	var filters []interface{}
	if f, ok := dnsMap["fake-ip-filter"].([]interface{}); ok {
		filters = append(filters, f...)
	}
	for _, pat := range []string{"*.lan", "*.local", "localhost", "*.ts.net", "router.asus.com", "www.asusrouter.com"} {
		has := false
		for _, ex := range filters {
			if ex == pat {
				has = true
				break
			}
		}
		if !has {
			filters = append(filters, pat)
		}
	}
	dnsMap["fake-ip-filter"] = filters
	cfg["dns"] = dnsMap

	for _, k := range []string{"cfw-bypass", "clash-for-android", "tun", "external-controller-unix", "external-controller-pipe", "external-controller-tls", "external-ui-url"} {
		delete(cfg, k)
	}

	return cfg, nil
}

// ValidateAndApply checks syntax and atomically replaces config.yaml, then reloads API.
func ValidateAndApply(cfg map[string]interface{}, secret string) error {
	_ = os.MkdirAll(RunDir, 0700)
	_ = os.MkdirAll(BaseDir, 0700)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode YAML: %w", err)
	}

	if err := os.WriteFile(CandidateFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write candidate config: %w", err)
	}

	// Syntax validation
	cmd := exec.Command(CoreBinary, "-t", "-d", RunDir, "-f", CandidateFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("syntax validation failed:\n%s", string(output))
	}

	if _, err := os.Stat(ConfigFile); err == nil {
		_ = os.Rename(ConfigFile, ConfigFile+".previous")
	}

	if err := os.Rename(CandidateFile, ConfigFile); err != nil {
		return fmt.Errorf("failed to install config.yaml: %w", err)
	}
	_ = os.Chmod(ConfigFile, 0600)

	_ = os.Remove(SymlinkConfig)
	_ = os.Symlink(ConfigFile, SymlinkConfig)

	controller := GetControllerAddress()
	client := api.NewClient("http://"+controller, secret)
	if err := client.Reload(SymlinkConfig); err != nil {
		fmt.Printf("[!] API reload failed (%v); restarting service gracefully...\n", err)
		cmd := exec.Command("/jffs/mihomo/service.sh", "restart")
		_ = cmd.Run()
	}

	return nil
}

// GetControllerAddress extracts controller address.
func GetControllerAddress() string {
	data, err := os.ReadFile(ConfigFile)
	if err == nil {
		var raw map[string]interface{}
		if err := yaml.Unmarshal(data, &raw); err == nil {
			if ctrl, ok := raw["external-controller"].(string); ok && ctrl != "" {
				if strings.HasPrefix(ctrl, "0.0.0.0:") {
					return "127.0.0.1:" + strings.TrimPrefix(ctrl, "0.0.0.0:")
				}
				return ctrl
			}
		}
	}
	return "192.168.50.1:9090"
}

// GetExistingSecrets extracts secret and proxyAuth from existing config.yaml if available.
func GetExistingSecrets() (string, string) {
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		return "", ""
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", ""
	}
	secret, _ := raw["secret"].(string)
	proxyAuth := ""
	if auths, ok := raw["authentication"].([]interface{}); ok && len(auths) > 0 {
		if authStr, ok := auths[0].(string); ok {
			parts := strings.Split(authStr, ":")
			if len(parts) == 2 {
				proxyAuth = parts[1]
			}
		}
	}
	return secret, proxyAuth
}
