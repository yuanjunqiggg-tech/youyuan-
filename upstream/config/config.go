package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type AppConfig struct {
	Token        string `json:"token"`
	AuthURL      string `json:"auth_url"`
	MarketURL    string `json:"market_url"`
	MarketToken  string `json:"market_token"`
	ServerCode   string `json:"server_code"`
	ServerPass   string `json:"server_pass"`
	BotName      string `json:"bot_name"`
	ImportSpeed  int    `json:"import_speed"`
	ExportSpeed  int    `json:"export_speed"`
	MediaDisplay string `json:"media_display"`
	MediaTarget  string `json:"media_target"`
	ShowProgress bool   `json:"show_progress"`
	DataDir      string `json:"data_dir"`

	ImportCommands     bool `json:"import_commands"`
	CmdDisabled        bool `json:"cmd_disabled"`
	ExcludeFluids      bool `json:"exclude_fluids"`
	ExcludeWater       bool `json:"exclude_water"`
	ExcludeWaterlogged bool `json:"exclude_waterlogged"`
	ExcludeLava        bool `json:"exclude_lava"`
	UseGravityBlocks   bool `json:"use_gravity_blocks"`
	GravityPlatform    bool `json:"gravity_platform"`

	UseNewProtocol bool   `json:"use_new_protocol"` // 1.21.120 新版协议
	RegionMode     string `json:"region_mode"`      // 导入粒度: 1=16×16, 2=32×32, 3=64×64

	PreClearMode int `json:"pre_clear_mode"` // 0=关闭 1=一格 2=竖柱 3=区块

	CustomDNS string `json:"custom_dns"` // 自定义 DNS 服务器，空=默认 223.5.5.5

	TLSSkipVerify *bool `json:"tls_skip_verify,omitempty"` // 跳过 TLS 证书验证，nil=默认 true

	// ── 多机器人（S8） ──
	BotTokens        []string `json:"bot_tokens"` // 子机器人令牌（不包括主令牌），配置即启用多机器人
	AutoConnect      bool     `json:"auto_connect"`
	AutoReconnect    bool     `json:"auto_reconnect"`
	ReconnectSeconds int      `json:"reconnect_seconds"`
	AutoCreative     bool     `json:"auto_creative"` // Default on; can disable for survival/death tests.
}

var cfgPath string
var current *AppConfig
var configMu sync.Mutex

func InitConfig(dataDir string) {
	configMu.Lock()
	defer configMu.Unlock()
	os.MkdirAll(dataDir, 0755)
	cfgPath = filepath.Join(dataDir, "config.json")
	current = nil
}
func configDefaults() *AppConfig {
	return &AppConfig{AuthURL: "https://susu.bot.cd", ImportSpeed: 10000, ExportSpeed: 10000, MediaDisplay: "actionbar", MediaTarget: "@a", ImportCommands: true, ExcludeWater: true, ExcludeWaterlogged: true, ExcludeLava: true, AutoCreative: true}
}
func cloneConfig(cfg *AppConfig) *AppConfig {
	c := *cfg
	c.BotTokens = append([]string(nil), cfg.BotTokens...)
	if cfg.TLSSkipVerify != nil {
		v := *cfg.TLSSkipVerify
		c.TLSSkipVerify = &v
	}
	return &c
}
func loadLocked(reload bool) *AppConfig {
	if current == nil || reload {
		c := configDefaults()
		if b, e := os.ReadFile(cfgPath); e == nil {
			if json.Unmarshal(b, c) == nil {
				current = c
			}
		}
		if current == nil {
			current = c
		}
	}
	return cloneConfig(current)
}
func LoadConfig() *AppConfig   { configMu.Lock(); defer configMu.Unlock(); return loadLocked(false) }
func ReloadConfig() *AppConfig { configMu.Lock(); defer configMu.Unlock(); return loadLocked(true) }
func SaveConfig(cfg *AppConfig) error {
	configMu.Lock()
	defer configMu.Unlock()
	data, e := json.MarshalIndent(cfg, "", "  ")
	if e != nil {
		return e
	}
	temp := cfgPath + ".tmp"
	if e = os.WriteFile(temp, data, 0600); e != nil {
		return e
	}
	if e = os.Rename(temp, cfgPath); e != nil {
		return e
	}
	current = cloneConfig(cfg)
	return nil
}
