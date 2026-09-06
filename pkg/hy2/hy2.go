package hy2

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseURL parses a hysteria2:// or hy2:// URL into a Mihomo proxy map.
func ParseURL(rawURL string) (map[string]interface{}, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "hysteria2" && u.Scheme != "hy2" {
		return nil, fmt.Errorf("unsupported scheme: %s (expected hysteria2:// or hy2://)", u.Scheme)
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("HY2-%s", u.Hostname())
	} else {
		name, _ = url.QueryUnescape(name)
	}

	password := ""
	if u.User != nil {
		password = u.User.Username()
		if pass, ok := u.User.Password(); ok && password == "" {
			password = pass
		}
	}

	port := 443
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	}

	node := map[string]interface{}{
		"name":     name,
		"type":     "hysteria2",
		"server":   u.Hostname(),
		"port":     port,
		"password": password,
	}

	q := u.Query()
	if sni := q.Get("sni"); sni != "" {
		node["sni"] = sni
	}
	if insecure := q.Get("insecure"); insecure == "1" || strings.ToLower(insecure) == "true" {
		node["skip-cert-verify"] = true
	}
	if obfs := q.Get("obfs"); obfs != "" {
		node["obfs"] = obfs
	}
	if obfsPass := q.Get("obfs-password"); obfsPass != "" {
		node["obfs-password"] = obfsPass
	}
	if alpn := q.Get("alpn"); alpn != "" {
		node["alpn"] = strings.Split(alpn, ",")
	}
	if up := q.Get("up"); up != "" {
		node["up"] = up
	}
	if down := q.Get("down"); down != "" {
		node["down"] = down
	}
	if ports := q.Get("ports"); ports != "" {
		node["ports"] = ports
	}

	return node, nil
}
