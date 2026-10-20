var yyTaskRows = [];
function yyTaskParams(task) {
  if (task.params && typeof task.params === "object") return task.params;
  try {
    return JSON.parse(task.params || "{}");
  } catch {
    return {};
  }
}
function yyTaskStatus(value) {
  return { running: "\u8FD0\u884C\u4E2D", paused: "\u53EF\u7EE7\u7EED", failed: "\u5931\u8D25 \xB7 \u53EF\u91CD\u8BD5", done: "\u5DF2\u5B8C\u6210", cancelled: "\u5DF2\u53D6\u6D88" }[value] || value || "\u672A\u77E5";
}
function yyTaskResumeState(task) {
  if (!["import", "import_multi", "mapart"].includes(task.type)) return { available: false, reason: "\u6B64\u7C7B\u4EFB\u52A1\u4E0D\u63D0\u4F9B\u5EFA\u7B51\u65AD\u70B9\uFF1B\u8BF7\u4F7F\u7528\u5BF9\u5E94\u5DE5\u5177\u91CD\u65B0\u8FD0\u884C\u3002" };
  if (task.status === "done") return { available: false, reason: "\u4EFB\u52A1\u5DF2\u5B8C\u6210\uFF0C\u65E0\u9700\u7EED\u8DD1\uFF1B\u53EF\u6253\u5F00\u539F\u5BFC\u5165\u8BBE\u7F6E\u91CD\u65B0\u521B\u5EFA\u4EFB\u52A1\u3002" };
  if (task.status === "running") return { available: false, reason: "\u4EFB\u52A1\u6B63\u5728\u8FD0\u884C\uFF1B\u5148\u505C\u6B62\u5E76\u7B49\u5F85\u65AD\u70B9\u4FDD\u5B58\u540E\u518D\u7EE7\u7EED\u3002" };
  return { available: true, reason: "\u4F7F\u7528\u5DF2\u4FDD\u5B58\u7684\u8FDB\u5EA6\u4E0E\u539F\u53C2\u6570\u7EE7\u7EED\u3002\u82E5\u539F\u6587\u4EF6\u5DF2\u79FB\u52A8\u6216\u8BB0\u5F55\u7248\u672C\u4E0D\u517C\u5BB9\uFF0C\u4F1A\u663E\u793A\u5177\u4F53\u539F\u56E0\u3002" };
}
function yyTaskButton(text, run, cls = "btn-s") {
  const b = document.createElement("button");
  b.type = "button";
  b.className = cls;
  b.textContent = text;
  b.onclick = async () => {
    b.disabled = true;
    try {
      await run();
    } finally {
      b.disabled = false;
    }
  };
  return b;
}
async function yyTaskList() {
  const r = await A("GET", "/api/task/list");
  if (!r.ok) throw Error(r.error || "\u4EFB\u52A1\u5217\u8868\u8BFB\u53D6\u5931\u8D25");
  yyTaskRows = r.tasks || [];
  return yyTaskRows;
}
function yyDrawTaskRows(host, rows) {
  host.replaceChildren();
  if (!rows.length) {
    host.textContent = "\u8FD8\u6CA1\u6709\u4EFB\u52A1\u8BB0\u5F55";
    return;
  }
  for (const task of rows) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "yy-task-entry";
    const title = document.createElement("strong"), info = document.createElement("span"), progress = document.createElement("progress");
    title.textContent = task.name || task.type;
    info.textContent = yyTaskStatus(task.status) + " \xB7 " + Math.round((task.progress || 0) * 100) + "% \xB7 " + (task.updated_at ? new Date(task.updated_at * 1e3).toLocaleString() : "") + " \xB7 " + (task.message || "");
    progress.max = 1;
    progress.value = Math.max(0, Math.min(1, task.progress || 0));
    b.append(title, info, progress);
    b.onclick = () => yyOpenTask(task.task_id);
    host.append(b);
  }
}
async function yyTaskCenter() {
  delete subPageCache["\u4EFB\u52A1\u8BB0\u5F55"];
  openSubPage("\u4EFB\u52A1\u8BB0\u5F55", yyIntro("\u4F5C\u4E1A\u4E0E\u65AD\u70B9", "\u67E5\u770B\u5F53\u524D\u8D26\u53F7\u7684\u4EFB\u52A1\u3001\u539F\u8BBE\u7F6E\u548C\u6062\u590D\u4F4D\u7F6E\u3002") + '<section class="card" id="yy-task-records">\u6B63\u5728\u8BFB\u53D6\u2026</section>');
  try {
    yyDrawTaskRows(document.getElementById("yy-task-records"), await yyTaskList());
  } catch (e) {
    const h = document.getElementById("yy-task-records");
    if (h) h.textContent = e.message;
  }
}
async function Rtsk() {
  const host = document.getElementById("tab-tasks");
  if (!host) return;
  host.innerHTML = '<div class="content-inner"><section class="card" id="yy-task-records">\u6B63\u5728\u8BFB\u53D6\u2026</section></div>';
  try {
    yyDrawTaskRows(document.getElementById("yy-task-records"), await yyTaskList());
  } catch (e) {
    T(e.message, "e");
  }
}
async function yyOpenTask(id) {
  try {
    const rows = await yyTaskList(), task = rows.find((t) => t.task_id === id);
    if (!task) throw Error("\u6B64\u4EFB\u52A1\u8BB0\u5F55\u4E0D\u5B58\u5728");
    yyTaskDialog(task);
  } catch (e) {
    T(e.message, "e");
  }
}
function yyTaskDialog(task) {
  var _a;
  (_a = document.getElementById("yy-task-dialog")) == null ? void 0 : _a.remove();
  const overlay = document.createElement("div");
  overlay.id = "yy-task-dialog";
  overlay.className = "modal-overlay";
  const box = document.createElement("div");
  box.className = "modal-box yy-task-detail";
  const title = document.createElement("h3");
  title.textContent = task.name || task.type;
  const summary = document.createElement("p");
  summary.textContent = yyTaskStatus(task.status) + " \xB7 " + Math.round((task.progress || 0) * 100) + "%";
  const message = document.createElement("p");
  message.textContent = task.message || "";
  const progress = document.createElement("progress");
  progress.max = 1;
  progress.value = task.progress || 0;
  const params = yyTaskParams(task), dl = document.createElement("dl");
  dl.className = "yy-task-parameters";
  for (const [label, value] of [["\u670D\u52A1\u5668", task.server_code], ["\u6587\u4EF6", params.path], ["\u8D77\u70B9", [params.x, params.y, params.z].join(", ")], ["\u7EF4\u5EA6", params.dimension], ["\u65CB\u8F6C", params.rotation], ["\u901F\u5EA6\uFF08\u65B9\u5757/\u79D2\uFF09", params.speed], ["\u5BFC\u5165\u7C92\u5EA6", params.region_mode]]) {
    if (value === void 0) continue;
    const dt = document.createElement("dt"), dd = document.createElement("dd");
    dt.textContent = label;
    dd.textContent = String(value);
    dl.append(dt, dd);
  }
  const resume = yyTaskResumeState(task), checkpoint = document.createElement("section");
  checkpoint.className = "yy-task-checkpoint";
  const checkpointTitle = document.createElement("h4"), checkpointNote = document.createElement("p");
  checkpointTitle.textContent = resume.available ? "\u65AD\u70B9\u5DF2\u4FDD\u7559 \xB7 \u53EF\u7EE7\u7EED" : "\u65AD\u70B9\u72B6\u6001";
  checkpointNote.textContent = resume.reason;
  checkpoint.append(checkpointTitle, checkpointNote);
  if (resume.available) checkpoint.append(yyTaskButton("\u4ECE\u65AD\u70B9\u7EE7\u7EED", async () => {
    if (await resT(task.task_id)) overlay.remove();
  }, "btn"));
  box.append(title, summary, progress, message, checkpoint, dl);
  const actions = document.createElement("div");
  actions.className = "yy-actions";
  if (task.status === "running") actions.append(yyTaskButton("\u505C\u6B62\u4F5C\u4E1A", async () => {
    await stopT();
    overlay.remove();
  }));
  if (["import", "import_multi"].includes(task.type)) actions.append(yyTaskButton("\u6253\u5F00\u539F\u5BFC\u5165\u8BBE\u7F6E", async () => {
    overlay.remove();
    await yyRestoreImportSettings(task);
  }));
  actions.append(yyTaskButton("\u5220\u9664\u8BB0\u5F55", async () => {
    if (!confirm("\u5220\u9664\u8FD9\u6761\u65AD\u70B9\u8BB0\u5F55\uFF1F\u4E0D\u4F1A\u5220\u9664\u5EFA\u7B51\u6587\u4EF6\u6216\u6E38\u620F\u5185\u65B9\u5757\u3002")) return;
    await delT(task.task_id);
    overlay.remove();
  }), yyTaskButton("\u5173\u95ED", () => overlay.remove()));
  box.append(actions);
  overlay.append(box);
  overlay.onclick = (e) => {
    if (e.target === overlay) overlay.remove();
  };
  document.body.append(overlay);
}
function showTaskDetail(id) {
  return yyOpenTask(id);
}
async function yyRestoreImportSettings(task) {
  var _a, _b;
  const p = yyTaskParams(task);
  delete subPageCache["\u5EFA\u7B51\u5BFC\u5165"];
  openSubPage("\u5EFA\u7B51\u5BFC\u5165", RimpHTML());
  impPath = p.path || "";
  if (!impPath) {
    T("\u8BB0\u5F55\u7F3A\u5C11\u6587\u4EF6\u8DEF\u5F84", "e");
    return;
  }
  await doAnalyze(impPath, impPath.toLowerCase().endsWith(".mcworld"));
  for (const [key, id] of Object.entries({ x: "imp-x", y: "imp-y", z: "imp-z", speed: "imp-spd", dm_id: "imp-dim-id", deny_y_offset: "imp-deny-offset", border_y: "imp-border-y", x1: "imp-x1", y1: "imp-y1", z1: "imp-z1", x2: "imp-x2", y2: "imp-y2", z2: "imp-z2" })) {
    const el = document.getElementById(id);
    if (el && p[key] !== void 0) el.value = p[key];
  }
  for (const [key, id] of Object.entries({ import_commands: "imp-cmds", cmd_disabled: "imp-cmd-disabled", exclude_fluids: "imp-exclude-fluids", exclude_water: "imp-exclude-water", exclude_waterlogged: "imp-exclude-waterlogged", exclude_lava: "imp-exclude-lava", deny_enable: "imp-deny", border_enable: "imp-border", keep_mode: "imp-keep", exclude_nbt: "imp-exclude-nbt" })) {
    const el = document.getElementById(id);
    if (el && p[key] !== void 0) {
      el.checked = !!p[key];
      el.dispatchEvent(new Event("change"));
    }
  }
  for (const [key, id, items] of [["rotation", "imp-rot-picker", ROT_ITEMS], ["dimension", "imp-dim-picker", DIM_ITEMS], ["region_mode", "imp-region-picker", REGION_ITEMS], ["pre_clear_mode", "imp-preclear-picker", PRECLEAR_ITEMS]]) {
    const el = document.getElementById(id);
    if (el && p[key] !== void 0) {
      el.dataset.value = String(p[key]);
      const item = items.find((x) => x.value === String(p[key]));
      if (item) el.querySelector(".picker-label").textContent = item.label;
    }
  }
  setImpMode(task.type === "import_multi" ? "multi" : "import");
  (_a = document.getElementById("imp-adv")) == null ? void 0 : _a.classList.remove("gone");
  const area = (_b = document.getElementById("imp-go")) == null ? void 0 : _b.parentElement;
  if (task.status !== "done" && area) area.prepend(yyTaskButton("\u4FDD\u7559\u539F\u8BBE\u7F6E\uFF0C\u4ECE\u6B64\u65AD\u70B9\u7EE7\u7EED", () => resT(task.task_id), "btn"));
  T("\u5DF2\u6062\u590D\u539F\u8BBE\u7F6E\uFF1B\u4FEE\u6539\u8BBE\u7F6E\u540E\u70B9\u51FB\u5F00\u59CB\u5BFC\u5165\u4F1A\u65B0\u5EFA\u4EFB\u52A1", "s");
}
async function resT(id) {
  const r = await A("POST", "/api/task/resume", { task_id: id });
  T(r.ok ? "\u4EFB\u52A1\u5DF2\u6062\u590D" : r.error, r.ok ? "o" : "e");
  if (typeof refreshHomeTasks === "function") refreshHomeTasks();
  return !!r.ok;
}
async function stopT() {
  const r = await A("POST", "/api/task/stop");
  T(r.ok ? "\u6B63\u5728\u505C\u6B62\u5E76\u4FDD\u5B58\u65AD\u70B9\uFF0C\u8BF7\u7B49\u5F85\u6E05\u7406\u5B8C\u6210" : r.error, r.ok ? "o" : "e");
  if (typeof refreshHomeTasks === "function") refreshHomeTasks();
}
async function delT(id) {
  const r = await A("POST", "/api/task/delete", { task_id: id });
  if (!r.ok) T(r.error, "e");
  if (typeof refreshHomeTasks === "function") refreshHomeTasks();
  return r.ok;
}
