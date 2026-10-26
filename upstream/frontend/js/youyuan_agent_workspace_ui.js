/* ═══ 游园 · AI 帮写工作区（任务书第十二部分）═══
 *
 * 目标：手机上一屏就是「当前任务」，不是把桌面三栏塞进手机。
 *  - 顶部：返回 / 任务名（页面头）+ 运行状态 + 更多操作
 *  - 中间只有对话
 *  - 底部输入区：附件行 / 输入框 / 一行「模型入口 + 动作按钮」
 *
 * 具体做了六件事：
 *  1. 技术信息收纳：Token / 上下文估算那行不再常显，收进「更多 → 任务详情」。
 *  2. 删掉「对话 / 展开对话 / 完整记录 / 最新」这条中间条：完整记录与展开对话进「更多」，
 *     最新消息沿用已有的浮动按钮（#yy-agent-latest，仅在有新消息时出现）。
 *  3. 模型选择从输入框下方的整行，改成动作行左侧的紧凑入口（12.4 模型入口清晰但不占屏）。
 *  4. 发送 / 停止 / 继续 跟随真实任务状态：运行中给「停止」，暂停/中断给「继续」，空闲只给「发送」。
 *     运行中发送 = 引导当前任务（后端 steering 语义），文案写清楚。
 *  5. 输入框随内容增高（到上限内部滚动）。
 *  6. 软键盘弹出时页面跟着可视视口收缩，保证发送按钮不被键盘盖住（12.3 / 14.3）。
 *
 * 不改业务：按钮仍调用原来的 yyAgentTask / yyFullConversation / yyAgentPreferences 等，
 * 模型 select 仍是 #yy-agent-model（yyAgentTask 读它）。
 */
(function () {
  "use strict";

  var baseOpen = yyOpenWorkspace;

  yyOpenWorkspace = async function (id, pane) {
    await baseOpen(id, pane);
    if (!yyWorkspaceActive || yyWorkspaceActive.id !== id) return;
    var page = document.getElementById("sub-page");
    if (!page || !page.classList.contains("show")) return;
    if (!document.getElementById("yy-workspace")) return;
    mount(page);
  };

  function btn(text, fn, cls) {
    var b = document.createElement("button");
    b.type = "button";
    b.textContent = text;
    b.className = cls || "yy-ws-menu-item";
    b.onclick = fn;
    return b;
  }
  function closeMore(more) { if (more) more.open = false; }

  function mount(page) {
    var agent = document.querySelector(".yy-agent-pane");
    if (!agent) return;
    page.classList.add("yy-agent-ws");

    var header = agent.querySelector(".yy-agent-header");
    var compose = agent.querySelector(".yy-agent-compose");
    var actions = compose && compose.querySelector(".yy-agent-compose-actions");
    var modelBar = agent.querySelector(".yy-model-toolbar");
    var usage = document.getElementById("yy-agent-usage");

    /* ── 1. 顶部：右侧「更多」菜单 ───────────────────────────── */
    if (header && !header.querySelector(".yy-ws-more")) {
      var more = document.createElement("details");
      more.className = "yy-ws-more";
      var sum = document.createElement("summary");
      sum.setAttribute("aria-label", "更多操作");
      sum.setAttribute("role", "button");
      // 不用裸图标：手机上「⋯」看不出是什么，直接写「更多」
      sum.textContent = "更多";
      var menu = document.createElement("div");
      menu.className = "yy-ws-menu";
      more.append(sum, menu);
      header.append(more);

      if (typeof yyFullConversation === "function") {
        menu.append(btn("完整记录", function () { closeMore(more); yyFullConversation(); }));
      }
      menu.append(btn("展开对话", function () {
        closeMore(more);
        var on = page.classList.toggle("yy-agent-focus");
        sum.title = on ? "返回工作区" : "展开对话";
      }));
      if (typeof yyAgentPreferences === "function") {
        menu.append(btn("提示词 / 技能 / 工具", function () { closeMore(more); yyAgentPreferences(); }));
      }
      if (typeof yyAgentModels === "function") {
        menu.append(btn("模型与服务商", function () { closeMore(more); yyAgentModels(); }));
      }
      if (typeof yyWorkspacePane === "function") {
        menu.append(btn("MCP 连接与工具", function () { closeMore(more); yyWorkspacePane("context"); }));
      }
      if (typeof yyAgentRunBudgetDialog === "function") {
        menu.append(btn("运行预算", function () { closeMore(more); yyAgentRunBudgetDialog(); }));
      }
      if (usage) {
        var det = document.createElement("details");
        det.className = "yy-ws-detail";
        var ds = document.createElement("summary");
        ds.textContent = "任务详情（Token · 上下文 · 模型）";
        det.append(ds, usage);
        menu.append(det);
        usage.classList.add("yy-ws-usage");
      }
    } else if (header) {
      // 重新打开工作区时 profiles 会重建 usage 节点，这里再收进「任务详情」
      var exist = header.querySelector(".yy-ws-detail");
      if (exist && usage && usage.parentNode !== exist) {
        exist.append(usage);
        usage.classList.add("yy-ws-usage");
      }
    }

    /* ── 2. 去掉中间那条「对话 / 完整记录 / 最新」───────────────── */
    agent.querySelectorAll(".yy-agent-scroll-controls").forEach(function (el) { el.remove(); });

    /* ── 3. 动作行：左「模型入口」，右「停止 / 继续 / 发送」──────── */
    if (actions && modelBar && !actions.classList.contains("yy-ws-actions")) {
      var stopBtn = null, resumeBtn = null;
      var sendBtn = actions.querySelector(".yy-primary") || actions.querySelector(".yy-ws-send > button:last-child");
      actions.querySelectorAll("button").forEach(function (b) {
        if (b === sendBtn) return;
        var t = (b.textContent || "").trim();
        if (/停止/.test(t)) stopBtn = b;
        else if (/继续/.test(t)) resumeBtn = b;
      });
      var left = document.createElement("div");
      left.className = "yy-ws-tools";
      var right = document.createElement("div");
      right.className = "yy-ws-send";
      left.append(modelBar);
      if (stopBtn) { stopBtn.classList.add("yy-ws-stop"); right.append(stopBtn); }
      if (resumeBtn) { resumeBtn.classList.add("yy-ws-resume"); right.append(resumeBtn); }
      if (sendBtn) right.append(sendBtn);
      actions.replaceChildren(left, right);
      actions.classList.add("yy-ws-actions");
    }

    /* ── 4. 状态关联：按钮跟着 #yy-agent-state 走 ─────────────── */
    var textarea = document.getElementById("yy-agent-message");
    var sendEl = actions && actions.querySelector(".yy-ws-send > button:last-child");

    function syncState() {
      var badge = document.getElementById("yy-agent-state");
      var st = ((badge && badge.textContent) || "").trim();
      var running = st.indexOf("运行中") >= 0;
      var resumable = /已暂停|等待继续|已中断|出错/.test(st);
      var stop = actions && actions.querySelector(".yy-ws-stop");
      var resume = actions && actions.querySelector(".yy-ws-resume");
      if (stop) stop.hidden = !running;
      if (resume) resume.hidden = running || !resumable;
      if (sendEl) {
        // 运行中：发送退为「引导」，停止才是主操作
        sendEl.classList.toggle("yy-primary", !running);
        sendEl.classList.toggle("yy-secondary", running);
        sendEl.textContent = running ? "引导" : "发送";
        if (!running) {
          var arrow = document.createElement("span");
          arrow.className = "yy-ws-arrow";
          arrow.textContent = "→";
          sendEl.append(" ", arrow);
        }
        sendEl.title = running ? "把这条消息作为引导发给正在运行的任务" : "发送并开始执行";
      }
      if (textarea) {
        textarea.placeholder = running
          ? "补充说明会作为引导发给正在运行的任务，不会中断执行…"
          : "描述要完成的目标，或补充当前任务…";
      }
      page.dataset.agentState = st || "待开始";
    }
    syncState();
    var badgeEl = document.getElementById("yy-agent-state");
    if (badgeEl && !badgeEl.dataset.wsObserved) {
      badgeEl.dataset.wsObserved = "1";
      new MutationObserver(syncState).observe(badgeEl, { childList: true, characterData: true, subtree: true });
    }

    /* ── 5. 输入框随内容增高 ─────────────────────────────────── */
    if (textarea && !textarea.dataset.wsAuto) {
      textarea.dataset.wsAuto = "1";
      var grow = function () {
        textarea.style.height = "auto";
        textarea.style.height = Math.max(44, Math.min(textarea.scrollHeight, 132)) + "px";
      };
      textarea.addEventListener("input", grow);
      setTimeout(grow, 0);
    }

    /* ── 6. 软键盘 ───────────────────────────────────────────── */
    bindViewport();
  }

  // 点菜单外部收起
  document.addEventListener("click", function (e) {
    document.querySelectorAll(".yy-ws-more[open]").forEach(function (d) {
      if (!d.contains(e.target)) d.open = false;
    });
  }, true);

  var vvBound = false;
  function bindViewport() {
    if (vvBound || !window.visualViewport) return;
    vvBound = true;
    var sync = function () {
      var p = document.getElementById("sub-page");
      if (!p || !p.classList.contains("yy-agent-page")) return;
      var vv = window.visualViewport;
      var kb = Math.round(window.innerHeight - vv.height);
      // 键盘占掉超过 100px 才算真的弹出（地址栏收缩只差几十像素）
      if (kb > 100) {
        p.classList.add("yy-kb-open");
        p.style.top = Math.round(vv.offsetTop) + "px";
        p.style.height = Math.round(vv.height) + "px";
        p.style.bottom = "auto";
      } else {
        p.classList.remove("yy-kb-open");
        p.style.top = "";
        p.style.height = "";
        p.style.bottom = "";
      }
    };
    window.visualViewport.addEventListener("resize", sync);
    window.visualViewport.addEventListener("scroll", sync);
    document.addEventListener("focusin", function (e) {
      if (e.target && e.target.id === "yy-agent-message") setTimeout(sync, 260);
    });
    document.addEventListener("focusout", function () { setTimeout(sync, 120); });
  }
})();
