package hy2

import (
	"testing"
)

func TestParseURL(t *testing.T) {
	raw := "hysteria2://mypassword@example.com:8443/?sni=sni.example.com&insecure=1&alpn=h3#MyNode"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if node["name"] != "MyNode" {
		t.Errorf("expected name MyNode, got %v", node["name"])
	}
	if node["server"] != "example.com" {
		t.Errorf("expected server example.com, got %v", node["server"])
	}
	if node["port"] != 8443 {
		t.Errorf("expected port 8443, got %v", node["port"])
	}
	if node["password"] != "mypassword" {
		t.Errorf("expected password mypassword, got %v", node["password"])
	}
	if node["sni"] != "sni.example.com" {
		t.Errorf("expected sni sni.example.com, got %v", node["sni"])
	}
	if node["skip-cert-verify"] != true {
		t.Errorf("expected skip-cert-verify true, got %v", node["skip-cert-verify"])
	}
}
