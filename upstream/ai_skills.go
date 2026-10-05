package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// aiSkillsFS 内嵌技能知识文件（打包进二进制，本地加载，无需网络）。
//
//go:embed skills
var aiSkillsFS embed.FS

// AISkill 一个可加载到 AI 上下文的技能（知识包），内容随 App 打包。
type AISkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Sources     []string `json:"-"` // 内嵌文件相对路径（不暴露给前端）
}

// aiSkills 内置技能列表。内容来自 MiemieMethod/bedrock-docs（基岩版中文文档），打包在 skills/ 下。
var aiSkills = []*AISkill{
	{ID: "building-transfer", Name: "建筑导入导出与存储位置", Description: "宿主/游戏服务器/市场的职责、实际计数、部分导出与文件下载", Sources: []string{"skills/building-transfer/index.md"}},
	{ID: "lua-runtime", Name: "Lua 常驻开发", Description: "原版兼容接口、游戏操作、常驻事件与资源调度，按功能章节读取", Sources: []string{"skills/lua-runtime/index.md"}},
	{ID: "packet-reference", Name: "游戏协议包与玩法", Description: "当前内核215种协议包、字段、监听/发包与玩法边界，支持单包读取", Sources: []string{"skills/packet-reference/index.md", "skills/packet-reference/catalog.md"}},
	{ID: "mcbe-commands", Name: "网易基岩版指令系统", Description: "execute、rawtext、状态机、战斗、地皮、网易差异与性能，按章节读取", Sources: []string{"skills/mcbe-commands/SKILL.md", "skills/mcbe-commands/references/01-execute.md", "skills/mcbe-commands/references/02-rawtext.md", "skills/mcbe-commands/references/03-菜单与状态机.md", "skills/mcbe-commands/references/04-实体与战斗.md", "skills/mcbe-commands/references/05-坐标与地皮.md", "skills/mcbe-commands/references/06-网易扩展与差异.md", "skills/mcbe-commands/references/07-性能与调试.md", "skills/mcbe-commands/references/08-速查表.md", "skills/mcbe-commands/references/09-粒子作图-分叉取样与大数圆规.md", "skills/mcbe-commands/references/10-官方文档摘录-execute与坐标.md"}},
	{ID: "mcbe-execute", Name: "指令 · execute", Description: "两代语法、执行者与位置、条件、上下文继承与分叉", Sources: []string{"skills/mcbe-commands/references/01-execute.md", "skills/mcbe-commands/references/10-官方文档摘录-execute与坐标.md"}},
	{ID: "mcbe-rawtext", Name: "指令 · rawtext", Description: "tellraw / titleraw、组件、占位符与条件显示", Sources: []string{"skills/mcbe-commands/references/02-rawtext.md"}},
	{ID: "mcbe-systems", Name: "菜单、战斗与地皮", Description: "状态机、冷却、实体战斗、坐标与地皮方案", Sources: []string{"skills/mcbe-commands/references/03-菜单与状态机.md", "skills/mcbe-commands/references/04-实体与战斗.md", "skills/mcbe-commands/references/05-坐标与地皮.md"}},
	{ID: "mcbe-netease", Name: "网易扩展与性能调试", Description: "版本差异、权限、每刻预算、调试与速查", Sources: []string{"skills/mcbe-commands/references/06-网易扩展与差异.md", "skills/mcbe-commands/references/07-性能与调试.md", "skills/mcbe-commands/references/08-速查表.md"}},
	{ID: "mcbe-particles", Name: "粒子作图与坐标变换", Description: "分叉取样、大数圆规、坐标变换案例", Sources: []string{"skills/mcbe-commands/references/09-粒子作图-分叉取样与大数圆规.md"}},

	{ID: "mcp-connection", Name: "MCP接入教程", Description: "地址、密钥、临时公网、外部工具与排查", Sources: []string{"skills/mcp-connection/index.md"}},
	{ID: "self-auth", Name: "游园独立游戏认证", Description: "自己的网易登录、游客注册、游戏令牌、加密保存和刷新", Sources: []string{"skills/self-auth/index.md"}},
	{ID: "qq-assistant", Name: "群服互通助手", Description: ".通、绑定、日志、角色授权、资料卡与按权限调用AI/MCP", Sources: []string{"skills/qq-assistant/index.md"}},
	{ID: "public-tunnel", Name: "APK 临时公网 MCP 隧道", Description: "开启、关闭、连接密钥、实际临时域名与故障恢复", Sources: []string{"skills/public-tunnel/index.md"}},
	{ID: "command-chain-api", Name: "插件命令链管理", Description: "Lua/Python/C++ 调用文件、配置、启停与游戏聊天菜单", Sources: []string{"skills/command-chain-api/index.md"}},
	{ID: "physical-actions", Name: "机器人物理动作", Description: "挖掘、点击放置、近战攻击、移动与服务器确认", Sources: []string{"skills/physical-actions/index.md"}},
	{ID: "block-media", Name: "方块视频与转换", Description: "选定平面、逐帧差分、TXT执行与格式转换", Sources: []string{"skills/block-media/index.md"}},
	{ID: "command-execution", Name: "插件命令执行", Description: "通道、实例默认参数、逐条确认与流水线批量执行", Sources: []string{"skills/command-execution/index.md"}},
	{ID: "toolbox-live-development", Name: "游园实际开发与验收", Description: "当前能力、常驻实例、示例、公网 MCP 与实际验证范围", Sources: []string{"skills/toolbox-live-development/index.md", "skills/toolbox-live-development/examples.md", "skills/toolbox-live-development/verification.md", "skills/toolbox-live-development/workspace-files.md", "skills/toolbox-live-development/python_seed.py", "skills/toolbox-live-development/cpp_seed.cpp", "skills/toolbox-live-development/python_observer.py", "skills/toolbox-live-development/cpp_observer.cpp"}},
	{ID: "python-runtime", Name: "Python 常驻开发", Description: "Python SDK 完整手册", Sources: []string{"skills/python-runtime/index.md"}},
	{ID: "cpp-runtime", Name: "C++ 常驻开发", Description: "C++ SDK 完整手册与 ABI 要求", Sources: []string{"skills/cpp-runtime/index.md"}},
	{ID: "command-chain", Name: "命令链开发", Description: "TXT 状态、跨链调用、条件等待与有界循环", Sources: []string{"skills/command-chain/index.md", "skills/command-chain/control-flow.md", "skills/toolbox-live-development/workspace-files.md"}},
	{
		ID:          "bedrock-commands",
		Name:        "基岩版指令",
		Description: "Minecraft 基岩版命令/选择器/坐标/参数权威参考（已内置）",
		Sources: []string{
			"skills/bedrock-commands/index.md",
			"skills/bedrock-commands/command-list.md",
			"skills/bedrock-commands/selector-parameter.md",
			"skills/bedrock-commands/command-argument-type.md",
		},
	},
	{
		ID:          "fmbe-display",
		Name:        "FMbe 显示实体",
		Description: "FMbe（狐狸方块实体）：用骨骼动画 playanimation 制作展示物品/方块动画（已内置）",
		Sources: []string{
			"skills/fmbe-display/display-entities.md",
		},
	},
}

func init() {
	for _, id := range []string{"lua-runtime", "python-runtime", "cpp-runtime"} {
		sk := skillByID(id)
		for _, section := range []string{"game", "player", "world", "events", "packet", "bot", "chains", "building", "media", "plugin", "util", "api", "http", "system", "web"} {
			sk.Sources = append(sk.Sources, "skills/"+id+"/"+section+".md")
		}
	}
}

// aiSkillStore 技能加载状态 + 内容缓存。
type aiSkillStore struct {
	mu      sync.Mutex
	path    string
	loaded  map[string]bool   // 已加载（落盘持久化）
	content map[string]string // 已拉取内容（内存缓存）
}

// aiSkillStore global 技能存储。路径由 StartServer 里 setPath 初始化。
var skillStore = &aiSkillStore{}

// skillByID 返回内置技能定义。
func skillByID(id string) *AISkill {
	for _, s := range aiSkills {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// resolveSkill 按 id 或名称解析技能（供 AI 的 load_skill 工具用）。
func resolveSkill(idOrName string) *AISkill {
	for _, s := range aiSkills {
		if s.ID == idOrName || s.Name == idOrName {
			return s
		}
	}
	return nil
}

// setPath 设置持久化路径并加载已加载技能 id。
func (s *aiSkillStore) setPath(dataDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = dataDir + "/ai_skills.json"
	s.loaded = map[string]bool{}
	s.content = map[string]string{}
	if data, err := os.ReadFile(s.path); err == nil {
		var ids []string
		if json.Unmarshal(data, &ids) == nil {
			for _, id := range ids {
				if skillByID(id) != nil {
					s.loaded[id] = true
				}
			}
		}
	}
}

// saveLoaded 持久化已加载技能 id（调用方需持锁）。
func (s *aiSkillStore) saveLoaded() {
	if s.path == "" {
		return
	}
	var ids []string
	for id := range s.loaded {
		ids = append(ids, id)
	}
	data, _ := json.MarshalIndent(ids, "", "  ")
	_ = os.WriteFile(s.path, data, 0644)
}

// List 返回所有内置技能及加载状态。
func (s *aiSkillStore) List() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, 0, len(aiSkills))
	for _, sk := range aiSkills {
		out = append(out, map[string]any{
			"id":          sk.ID,
			"name":        sk.Name,
			"description": sk.Description,
			"loaded":      s.loaded[sk.ID],
		})
	}
	return out
}

// Load 从内嵌文件拼装一个技能的知识内容并标记为已加载（本地，无需网络）。
func (s *aiSkillStore) Load(id string) (string, error) {
	sk := skillByID(id)
	if sk == nil {
		return "", fmt.Errorf("未知技能: %s", id)
	}
	var sb strings.Builder
	sb.WriteString("# 技能：" + sk.Name + "\n")
	for _, src := range sk.Sources {
		text, err := aiSkillsFS.ReadFile(src)
		if err != nil {
			return "", fmt.Errorf("读取技能文件 %s 失败: %v", src, err)
		}
		sb.WriteString("\n--- 来源: " + src + " ---\n")
		sb.WriteString(string(text))
	}
	s.mu.Lock()
	s.loaded[id] = true
	s.content[id] = sb.String()
	s.saveLoaded()
	s.mu.Unlock()
	return sb.String(), nil
}

// Unload 卸载一个技能。
func (s *aiSkillStore) Unload(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.loaded, id)
	delete(s.content, id)
	s.saveLoaded()
}

// BuildContext includes only player-selected skill metadata; read content through tools.
func (s *aiSkillStore) BuildContext() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var sb strings.Builder
	for _, sk := range aiSkills {
		if s.loaded[sk.ID] {
			sb.WriteString("\n" + sk.ID + "：" + sk.Description)
		}
	}
	if sb.Len() == 0 {
		return "\n未加载额外技能；请玩家在技能库主动选择。"
	}
	return "\n可按需用 load_skill 分页读取的离线技能目录：" + sb.String()
}
