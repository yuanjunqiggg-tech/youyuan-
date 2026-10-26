package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PluginStatus 插件状态（供前端列表展示）。
type PluginStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Type        string `json:"type"`
	Description string `json:"description"`
	State       string `json:"state"`           // unloaded | loaded | error
	Running     bool   `json:"running"`         // 开关：启用/禁用
	Disabled    bool   `json:"disabled"`        // 持久禁用（重连不自动加载）
	Error       string `json:"error,omitempty"` // 加载错误信息（State=error 时）
	Docs        string `json:"docs,omitempty"`  // Markdown 操作文档
}

// PluginController 管理内置插件定义与运行状态。
type PluginController struct {
	mu           sync.Mutex
	defs         map[string]*PluginDef
	running      map[string]bool
	loading      map[string]bool // 正在加载中（防并发 Run 重复加载）
	state        map[string]string
	errs         map[string]string  // 加载错误信息（State=error 时）
	removeTree   func(string) error // nil uses the real filesystem; testable IO failure boundary
	privateDir   string             // 插件存储私有根目录（插件文件/ 扫描与磁盘加载用）
	disabled     map[string]bool    // 持久禁用的插件 id（重连不自动加载）
	disabledPath string             // 禁用集合的持久化文件路径
}

// NewPluginController 初始化控制器（内置插件已移除，插件由用户安装后 AddDef 注册）。
func NewPluginController() *PluginController {
	return &PluginController{
		defs:     map[string]*PluginDef{},
		running:  map[string]bool{},
		loading:  map[string]bool{},
		state:    map[string]string{},
		errs:     map[string]string{},
		disabled: map[string]bool{},
	}
}

// SetPluginDirs 设置插件存储根目录（私有 + 公有）。publicDir 当前仅预留，
// 扫描与磁盘加载均以私有目录为准。
func (pc *PluginController) SetPluginDirs(privateDir, publicDir string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.privateDir = privateDir
	_ = publicDir
	pc.disabledPath = filepath.Join(privateDir, "disabled.json")
	pc.loadDisabledLocked()
}

// loadDisabledLocked 从磁盘加载禁用集合（调用方需持锁）。
func (pc *PluginController) loadDisabledLocked() {
	pc.disabled = map[string]bool{}
	if pc.disabledPath == "" {
		return
	}
	data, err := os.ReadFile(pc.disabledPath)
	if err != nil {
		return
	}
	var list []string
	if json.Unmarshal(data, &list) != nil {
		return
	}
	for _, id := range list {
		pc.disabled[id] = true
	}
}

// saveDisabledLocked 把禁用集合写回磁盘（调用方需持锁）。
func (pc *PluginController) saveDisabledLocked() {
	if pc.disabledPath == "" {
		return
	}
	list := make([]string, 0, len(pc.disabled))
	for id := range pc.disabled {
		list = append(list, id)
	}
	data, _ := json.MarshalIndent(list, "", "  ")
	_ = os.WriteFile(pc.disabledPath, data, 0644)
}

// IsDisabled 判断插件是否被持久禁用。
func (pc *PluginController) IsDisabled(id string) bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.disabled[id]
}

// SetDisabled 设置插件持久禁用状态并落盘。
func (pc *PluginController) SetDisabled(id string, disabled bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if disabled {
		pc.disabled[id] = true
	} else {
		delete(pc.disabled, id)
	}
	pc.saveDisabledLocked()
}

// AutoLoadPlugins 加载所有未被持久禁用且未在运行的插件（主机器人连接并取得 OP 后调用）。
// 失败只记日志，不中断其它插件。
func (pc *PluginController) AutoLoadPlugins(bm *BotManager) {
	pc.mu.Lock()
	var pending []string
	for id := range pc.defs {
		if pc.disabled[id] || pc.running[id] || pc.loading[id] {
			continue
		}
		pending = append(pending, id)
	}
	pc.mu.Unlock()
	for _, id := range pending {
		if err := pc.Run(bm, id); err != nil {
			debugLog("[插件] 自动加载失败 id=%s err=%v", id, err)
		} else {
			debugLog("[插件] 自动加载成功 id=%s", id)
		}
	}
}

// StopAllPlugins 停止所有正在运行的插件（主机器人断开时调用，仅瞬态停用，不改禁用状态）。
func (pc *PluginController) StopAllPlugins(bm *BotManager) {
	pc.mu.Lock()
	var running []string
	for id, r := range pc.running {
		if r {
			running = append(running, id)
		}
	}
	pc.mu.Unlock()
	for _, id := range running {
		if err := pc.Stop(bm, id); err != nil {
			debugLog("[插件] 断开停用失败 id=%s err=%v", id, err)
		}
	}
	debugLog("[插件] 断开已停用 %d 个插件", len(running))
}

// ScanPlugins 扫描私有目录 插件文件/ 下的插件文件（.lua 与词库 .json），
// 为每个文件构建 PluginDef 并注册（已注册的跳过）。目录不存在视为空，不报错。
func (pc *PluginController) ScanPlugins() error {
	pc.mu.Lock()
	dir := filepath.Join(pc.privateDir, "插件文件")
	pc.mu.Unlock()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 插件目录尚未创建 → 空扫描
		}
		return err
	}
	var conflicts []string
	for _, e := range entries {
		if e.IsDir() {
			// 每插件独立目录：插件文件/<id>/<id>.lua（或 main.lua / <id>.json 词库）
			sub := filepath.Join(dir, e.Name())
			codePath := ""
			for _, cand := range []string{e.Name() + ".lua", "main.lua", e.Name() + ".json", e.Name() + "词库.json"} {
				if _, err := os.Stat(filepath.Join(sub, cand)); err == nil {
					codePath = filepath.Join(sub, cand)
					break
				}
			}
			if codePath == "" {
				continue
			}
			if e := registerScannedDef(pc, codePath); e != nil {
				conflicts = append(conflicts, e.Error())
			}
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext != ".lua" && ext != ".json" {
			continue
		}
		// 跳过元数据文件（manifest 不是插件本体；docs 是 .md，已被上面的扩展名过滤排除）
		trimmed := strings.TrimSuffix(name, ext)
		if strings.HasSuffix(trimmed, ".manifest") {
			continue
		}
		if e := registerScannedDef(pc, filepath.Join(dir, name)); e != nil {
			conflicts = append(conflicts, e.Error())
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("插件扫描冲突：%s", strings.Join(conflicts, "；"))
	}
	return nil
}

// registerScannedDef 从代码文件路径构建 def 并去重注册。
func registerScannedDef(pc *PluginController, path string) error {
	def, err := BuildDefFromFile(path)
	if err != nil {
		debugLog("[插件] 扫描跳过 %s err=%v", path, err)
		return err
	}
	pc.mu.Lock()
	previous, exists := pc.defs[def.ID]
	pc.mu.Unlock()
	if exists {
		if previous.SourceFile != "" && filepath.Clean(previous.SourceFile) != filepath.Clean(def.SourceFile) {
			return fmt.Errorf("插件ID %s 对应多个入口：%s 与 %s；保留既有归属，未覆盖", def.ID, previous.SourceFile, def.SourceFile)
		}
		return nil
	}
	if err := pc.AddDef(def); err != nil {
		debugLog("[插件] 注册失败 %s err=%v", def.ID, err)
		return err
	}
	return nil
}

// LoadPluginFromDisk 从磁盘加载插件（走 Run 路径，状态流转到 loaded/running）。
func (pc *PluginController) LoadPluginFromDisk(bm *BotManager, id string) error {
	debugLog("[插件] LoadPluginFromDisk id=%s", id)
	return pc.Run(bm, id)
}

// AddDef 注册一个插件定义（供用户安装的插件使用）。
func (pc *PluginController) AddDef(def *PluginDef) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if _, ok := pc.defs[def.ID]; ok {
		return fmt.Errorf("插件 %s 已存在", def.ID)
	}
	pc.defs[def.ID] = def
	return nil
}

// List 返回所有插件状态。
func (pc *PluginController) List() []PluginStatus {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	out := make([]PluginStatus, 0, len(pc.defs))
	for _, def := range pc.defs {
		st := pc.defStatus(def)
		out = append(out, *st)
	}
	return out
}

// defStatus 根据 def 构建状态（调用方需持锁）。
func (pc *PluginController) defStatus(def *PluginDef) *PluginStatus {
	id := def.ID
	st := "unloaded"
	if stid, ok := pc.state[id]; ok {
		st = stid
	}
	if pc.running[id] && st == "unloaded" {
		st = "loaded"
	}
	return &PluginStatus{
		ID:          def.ID,
		Name:        def.Name,
		Version:     def.Version,
		Type:        def.Type,
		Description: def.Description,
		State:       st,
		Running:     pc.running[id],
		Disabled:    pc.disabled[id],
		Error:       pc.errs[id],
		Docs:        def.Docs,
	}
}

// status 返回单个插件状态；不存在返回 nil。
func (pc *PluginController) status(id string) *PluginStatus {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	def, ok := pc.defs[id]
	if !ok {
		return nil
	}
	return pc.defStatus(def)
}

// Run 加载并运行插件。
func (pc *PluginController) Run(bm *BotManager, id string) error {
	debugLog("[插件] Run id=%s", id)
	pc.mu.Lock()
	if pc.running[id] {
		pc.mu.Unlock()
		return fmt.Errorf("插件 %s 已在运行", id)
	}
	if pc.loading[id] {
		pc.mu.Unlock()
		return fmt.Errorf("插件 %s 正在加载中，请稍候", id)
	}
	def, ok := pc.defs[id]
	if !ok {
		pc.mu.Unlock()
		return fmt.Errorf("未知插件 %s", id)
	}
	pc.loading[id] = true
	pc.mu.Unlock()
	defer func() {
		pc.mu.Lock()
		delete(pc.loading, id)
		pc.mu.Unlock()
	}()
	// 先解析前置依赖（递归加载，保证被依赖插件先运行）
	if err := pc.ensureDependencies(bm, def, map[string]bool{}); err != nil {
		pc.markError(id, err)
		return err
	}
	// 确保插件配置/数据文件存在
	pc.EnsurePluginFiles(id)
	if err := def.Load(bm); err != nil {
		debugLog("[插件] Run 失败 id=%s err=%v", id, err)
		// 回滚：Load 可能半途注册了部分触发项/事件/定时器，失败时全部清理，
		// 避免插件处于 error 状态但其触发项仍在运行（且 Stop 会报"未在运行"无法清理）。
		bm.RemovePluginTriggers(id)
		RemovePluginEvents(id, bm)
		RemovePluginSchedules(id, bm)
		RemovePluginWeb(id)
		pc.markError(id, err)
		return err
	}
	pc.mu.Lock()
	pc.state[id] = "loaded"
	pc.running[id] = true
	delete(pc.errs, id)
	pc.mu.Unlock()
	debugLog("[插件] Run 成功 id=%s", id)
	return nil
}

// ensureDependencies 递归加载 def 的前置插件；前置缺失/版本不足/加载失败则返回明确错误。
// chain 记录当前递归链，检测依赖环（否则 A↔B 互相依赖会无限递归栈溢出）。
func (pc *PluginController) ensureDependencies(bm *BotManager, def *PluginDef, chain map[string]bool) error {
	if chain[def.ID] {
		return fmt.Errorf("插件依赖成环: %s", def.ID)
	}
	chain[def.ID] = true
	defer delete(chain, def.ID)
	for depID, minVer := range def.PrePlugins {
		depDef, ok := pc.defs[depID]
		if !ok {
			return fmt.Errorf("缺少前置插件 %s（%s 需要它）", depID, def.ID)
		}
		if minVer != "" && !versionAtLeast(depDef.Version, minVer) {
			cur := depDef.Version
			if cur == "" {
				cur = "未知"
			}
			return fmt.Errorf("前置插件 %s 版本过低：需要 >= %s，当前 %s", depID, minVer, cur)
		}
		if err := pc.ensureDependencies(bm, depDef, chain); err != nil {
			return err
		}
		pc.mu.Lock()
		running := pc.running[depID]
		pc.mu.Unlock()
		if !running {
			if err := pc.Run(bm, depID); err != nil {
				return fmt.Errorf("加载前置插件 %s 失败: %v", depID, err)
			}
		}
	}
	return nil
}

// markError 把插件置为 error 状态并记录错误信息（调用方需已释放锁）。
func (pc *PluginController) markError(id string, err error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.state[id] = "error"
	pc.running[id] = false
	pc.errs[id] = err.Error()
}

// versionAtLeast 判断 cur 是否 >= min（按 . 分段数字比较；非数字段忽略）。
func versionAtLeast(cur, min string) bool {
	curParts, minParts := strings.Split(cur, "."), strings.Split(min, ".")
	for i := 0; i < len(minParts); i++ {
		var cv, mv int
		if i < len(curParts) {
			cv, _ = strconv.Atoi(strings.TrimLeft(curParts[i], "vV"))
		}
		mv, _ = strconv.Atoi(strings.TrimLeft(minParts[i], "vV"))
		if cv > mv {
			return true
		}
		if cv < mv {
			return false
		}
	}
	return true
}

// Stop 停止插件（移除其触发项）。
func (pc *PluginController) Stop(bm *BotManager, id string) error {
	debugLog("[插件] Stop id=%s", id)
	pc.mu.Lock()
	if !pc.running[id] {
		pc.mu.Unlock()
		return fmt.Errorf("插件 %s 未在运行", id)
	}
	pc.mu.Unlock()
	bm.RemovePluginTriggers(id)
	RemovePluginEvents(id, bm)
	RemovePluginSchedules(id, bm)
	RemovePluginWeb(id)
	yyStopResident(bm, id)
	pc.mu.Lock()
	pc.state[id] = "unloaded"
	pc.running[id] = false
	delete(pc.errs, id)
	pc.mu.Unlock()
	debugLog("[插件] Stop 成功 id=%s", id)
	return nil
}

// Reload 热重载插件：先停后起。
// yyRemoveTreeDeferred moves target out of its live path with one rename, then
// deletes the moved tree in the background. A rename is a single directory entry
// update even for a tree with tens of thousands of files, which is what made
// plugin deletion feel frozen while RemoveAll walked it inline.
//
// The rename happens before the caller reports success, so the plugin cannot be
// rescanned into existence after a restart. A tombstone that fails to delete is
// logged: by then the plugin is already gone from the registry, so there is no
// caller left to act on the error.
func yyRemoveTreeDeferred(trashRoot, target string) error {
	if target == "" {
		return nil
	}
	if _, err := os.Lstat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if trashRoot == "" {
		return os.RemoveAll(target)
	}
	if err := os.MkdirAll(trashRoot, 0700); err != nil {
		return os.RemoveAll(target)
	}
	tomb := filepath.Join(trashRoot, filepath.Base(target)+"-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	if err := os.Rename(target, tomb); err != nil {
		// Rename fails across filesystems or on a read-only parent; removing in
		// place is slower but still keeps the plugin from coming back.
		return os.RemoveAll(target)
	}
	go func() {
		if err := os.RemoveAll(tomb); err != nil {
			debugLog("[插件] 后台清理未完成 %s: %v", tomb, err)
		}
	}()
	return nil
}

func (pc *PluginController) Reload(bm *BotManager, id string) error {
	debugLog("[插件] Reload id=%s", id)
	pc.mu.Lock()
	wasRunning := pc.running[id]
	_, ok := pc.defs[id]
	pc.mu.Unlock()
	if !ok {
		return fmt.Errorf("未知插件 %s", id)
	}
	if wasRunning {
		if err := pc.Stop(bm, id); err != nil {
			return err
		}
	}
	err := pc.Run(bm, id)
	debugLog("[插件] Reload 完成 id=%s err=%v", id, err)
	return err
}

// Delete 删除插件：停止运行、移除列表、清除禁用记录，并删除磁盘上的
// 代码/配置/数据文件（否则重启后扫描会重新注册复活）。不要求机器人连接。
func (pc *PluginController) Delete(bm *BotManager, id string) error {
	if !validPluginID(id) {
		return fmt.Errorf("插件 id 非法")
	}
	codeDir, pathErr := pc.CodeDir(id)
	if pathErr != nil {
		return pathErr
	}
	looseFiles, loose, pathErr := pc.looseCodeFiles(id)
	if pathErr != nil {
		return pathErr
	}
	pc.mu.Lock()
	if pc.loading[id] {
		pc.mu.Unlock()
		return fmt.Errorf("插件 %s 正在加载或清理，请等待本次操作完成", id)
	}
	pc.loading[id] = true
	pc.mu.Unlock()
	defer func() { pc.mu.Lock(); delete(pc.loading, id); pc.mu.Unlock() }()
	// File removal must wait for the resident VM, callbacks and on_stop to exit.
	// A nil owner means every instance of this plugin in this execution process.
	yyLuaMu.Lock()
	keys := []yyLuaKey{}
	for key := range yyLuaStates {
		if key.id == id && (bm == nil || key.bot == bm) {
			keys = append(keys, key)
		}
	}
	yyLuaMu.Unlock()
	for _, key := range keys {
		yyStopResident(key.bot, id)
	}
	bm.RemovePluginTriggers(id)
	if bm == nil {
		RemovePluginEvents(id)
		RemovePluginSchedules(id)
	} else {
		RemovePluginEvents(id, bm)
		RemovePluginSchedules(id, bm)
	}
	RemovePluginWeb(id)
	pc.mu.Lock()
	pc.running[id] = false
	pc.state[id] = "unloaded"
	pc.mu.Unlock()
	// A test that injects removeTree wants a deterministic IO boundary, so it
	// keeps the synchronous path. Production defers the expensive half: a plugin
	// data directory can hold tens of thousands of files, and deleting it inline
	// held the HTTP request open (the frontend waits 300s and disables the row).
	failures := []string{}
	deferred := pc.removeTree == nil
	removeTree := pc.removeTree
	if removeTree == nil {
		removeTree = os.RemoveAll
	}
	type deleteTarget struct{ label, path string }
	targets := []deleteTarget{}
	for _, sub := range []string{"插件文件", "插件配置文件", "插件数据文件"} {
		if sub == "插件文件" && loose {
			for _, file := range looseFiles {
				targets = append(targets, deleteTarget{sub, file})
			}
			continue
		}
		target := filepath.Join(pc.privateDir, sub, id)
		if sub == "插件文件" {
			target = codeDir
		}
		targets = append(targets, deleteTarget{sub, target})
	}
	for _, t := range targets {
		if deferred {
			if err := yyRemoveTreeDeferred(filepath.Join(pc.privateDir, ".trash"), t.path); err != nil {
				failures = append(failures, t.label+": "+err.Error())
			}
			continue
		}
		if err := removeTree(t.path); err != nil {
			failures = append(failures, t.label+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		err := fmt.Errorf("插件已停止，但部分文件未能删除：%s", strings.Join(failures, "；"))
		pc.markError(id, err)
		return err
	}
	pc.mu.Lock()
	delete(pc.defs, id)
	delete(pc.running, id)
	delete(pc.state, id)
	delete(pc.errs, id)
	delete(pc.disabled, id)
	pc.saveDisabledLocked()
	pc.mu.Unlock()
	return nil
}
