package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 仓库自带的 config.example.json 含 // 注释，README 让人直接 cp 成 config.json；
// 带注释的配置必须能加载，否则快起步就断在第一步。
func TestLoadToleratesExampleConfigComments(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.example.json"))
	if err != nil {
		t.Fatalf("read config.example.json: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("带注释的示例配置应能加载: %v", err)
	}
	if cfg.Listen != ":7866" || cfg.AuthDir != "./auths" || cfg.DefaultModel != "snap-chat" {
		t.Fatalf("示例配置字段未生效: %+v", cfg)
	}
	if cfg.Cooldown.SoftRate != "60s" || cfg.MaxConcurrent != 5 || cfg.Watch.PollMinutes != 30 {
		t.Fatalf("示例配置嵌套字段未生效: %+v", cfg)
	}
	if cfg.KeepaliveWindow != "10m" {
		t.Errorf("keepalive_window 未生效: %q", cfg.KeepaliveWindow)
	}
}

func TestStripJSONCommentsKeepsStrings(t *testing.T) {
	raw := []byte("{\n" +
		"  // 行注释\n" +
		"  \"oauth_callback_host\": \"https://oneapi.example.com/codearts\", /* 块注释 */\n" +
		"  \"note\": \"含 // 与 /* 的字符串\",\n" +
		"  \"n\": 1\n" +
		"}")
	var v map[string]any
	if err := json.Unmarshal(stripJSONComments(raw), &v); err != nil {
		t.Fatalf("剥离注释后应是合法 JSON: %v", err)
	}
	if v["oauth_callback_host"] != "https://oneapi.example.com/codearts" {
		t.Errorf("URL 值被破坏: %v", v["oauth_callback_host"])
	}
	if v["note"] != "含 // 与 /* 的字符串" {
		t.Errorf("字符串内容被破坏: %v", v["note"])
	}
	if v["n"] != float64(1) {
		t.Errorf("n=%v", v["n"])
	}

	// 无注释的配置保持原样。
	plain := []byte(`{"listen":":7866"}`)
	if got := string(stripJSONComments(plain)); got != string(plain) {
		t.Errorf("无注释配置被改动: %q", got)
	}
}
