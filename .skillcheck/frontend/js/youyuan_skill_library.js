async function yyOpenSkillLibrary() {
  const id = yyWorkspaceActive.id, r = await A("GET", "/api/youyuan/agent/skills?id=" + encodeURIComponent(id));
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  if (yyWorkspaceActive.id !== id) return;
  const host = yyAgentProfileModal("\u6280\u80FD\u52A0\u8F7D\u5E93 \xB7 " + yyLanguageNames[r.language]);
  host.innerHTML = '<p class="yy-hint">\u52FE\u9009\u591A\u4EFD\u6280\u80FD\uFF0C\u70B9\u51FB\u201C\u4FDD\u5B58\u6240\u9009\u6280\u80FD\u201D\u540E\u7EDF\u4E00\u751F\u6548\u3002\u52A0\u8F7D\u662F\u5141\u8BB8 AI \u6309\u9700\u8BFB\u53D6\uFF0C\u5185\u5BB9\u4E0D\u4F1A\u4E00\u6B21\u6027\u6CE8\u5165\u3002\u65B0\u5DE5\u4F5C\u533A\u9ED8\u8BA4\u5F00\u653E\u6280\u80FD\uFF0C\u52FE\u9009\u8868\u793A AI \u53EF\u6309\u9700\u8BFB\u53D6\uFF0C\u6B63\u6587\u5C1A\u672A\u81EA\u52A8\u8BFB\u5165\u5BF9\u8BDD\u3002\u5176\u4ED6\u5DE5\u4F5C\u533A\u4E92\u4E0D\u5F71\u54CD\u3002</p><div class="yy-actions"><button class="btn" id="yy-skill-save">\u4FDD\u5B58\u6240\u9009\u6280\u80FD</button><button class="btn-s" id="yy-skill-all">\u5168\u9009</button><button class="btn-s" id="yy-skill-none">\u6E05\u7A7A\u9009\u62E9</button><span id="yy-skill-selection"></span></div><input id="yy-skill-search" placeholder="\u641C\u7D22\u6280\u80FD\u540D\u79F0\u6216\u529F\u80FD"><div id="yy-skill-cards"></div><p class="yy-hint">\u53D6\u6D88\u9009\u62E9\u5E76\u4FDD\u5B58\u4F1A\u963B\u6B62\u540E\u7EED\u8BFB\u53D6\uFF1B\u5BF9\u8BDD\u4E2D\u5DF2\u7ECF\u8BFB\u8FC7\u7684\u5185\u5BB9\u4ECD\u5C5E\u4E8E\u5386\u53F2\u8BB0\u5F55\u3002\u9700\u8981\u5B8C\u5168\u5E72\u51C0\u7684\u4E0A\u4E0B\u6587\u65F6\uFF0C\u65B0\u5EFA\u4E00\u4E2A\u5DE5\u4F5C\u533A\u3002</p>';
  const selected = new Set(r.skills);
  const status = () => {
    host.querySelector("#yy-skill-selection").textContent = "\u5DF2\u9009 " + selected.size + " \u4EFD \xB7 \u5C1A\u672A\u4FDD\u5B58";
  };
  const render = () => {
    const search = host.querySelector("#yy-skill-search").value.toLowerCase(), cards = host.querySelector("#yy-skill-cards");
    cards.replaceChildren();
    for (const sk of r.catalog.filter((s) => (s.name + " " + s.id + " " + s.description).toLowerCase().includes(search))) {
      const card = document.createElement("section");
      card.className = "card yy-multi-choice-card";
      card.dataset.skillId = sk.id;
      card.classList.toggle("is-selected", selected.has(sk.id));
      const head = document.createElement("strong");
      head.textContent = sk.name;
      head.onclick = () => {
        checkbox.checked = !checkbox.checked;
        checkbox.onchange();
      };
      const desc = document.createElement("p");
      desc.className = "yy-hint";
      desc.textContent = sk.description + " \xB7 " + sk.sections.length + " \u4E2A\u7AE0\u8282";
      const actions = document.createElement("div");
      actions.className = "yy-actions";
      const toggle = document.createElement("label");
      toggle.className = "yy-skill-select";
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = selected.has(sk.id);
      checkbox.setAttribute("aria-label", "\u9009\u62E9\u6280\u80FD " + sk.name);
      checkbox.onchange = () => {
        checkbox.checked ? selected.add(sk.id) : selected.delete(sk.id);
        card.classList.toggle("is-selected", checkbox.checked);
        status();
      };
      toggle.append(checkbox, document.createTextNode("\u9009\u62E9\u8FD9\u4EFD\u6280\u80FD \xB7 \u53EF\u591A\u9009"));
      const preview = document.createElement("button");
      preview.className = "btn-s";
      preview.textContent = "\u67E5\u770B\u7AE0\u8282";
      const details = document.createElement("div");
      details.hidden = true;
      preview.onclick = () => {
        details.hidden = !details.hidden;
      };
      const select = document.createElement("select");
      select.setAttribute("aria-label", sk.name + " \u7AE0\u8282");
      sk.sections.forEach((s) => {
        const o = document.createElement("option");
        o.value = s;
        o.textContent = s;
        select.appendChild(o);
      });
      const pre = document.createElement("pre");
      pre.className = "yy-readout yy-doc";
      pre.style.maxHeight = "300px";
      pre.style.overflow = "auto";
      let offset = 0;
      const read = async (reset) => {
        if (reset) offset = 0;
        const res = await A("GET", "/api/youyuan/skills/read?skill=" + encodeURIComponent(sk.id) + "&section=" + encodeURIComponent(select.value) + "&offset=" + offset);
        if (!res.ok) {
          T(res.error, "e");
          return;
        }
        pre.textContent = res.content;
        offset += 3e3;
        pre.scrollTop = 0;
      };
      select.onchange = () => read(true);
      const start = document.createElement("button");
      start.className = "btn-s";
      start.textContent = "\u8BFB\u53D6\u8FD9\u4E00\u7AE0";
      start.onclick = () => read(true);
      const next = document.createElement("button");
      next.className = "btn-s";
      next.textContent = "\u4E0B\u4E00\u9875";
      next.onclick = () => read(false);
      const download = document.createElement("button");
      download.className = "btn-s";
      download.textContent = "\u5BFC\u51FA\u72EC\u7ACB ZIP";
      download.onclick = async () => {
        var _a;
        try {
          const path = "/api/youyuan/skills/export?skill=" + encodeURIComponent(sk.id);
          if (window.android && android.exportFile) {
            const result = await A("POST", path, {});
            if (!result.ok) throw Error(result.error);
            android.exportFile(result.src_path, sk.id + ".zip");
            return;
          }
          const res = await fetch(path);
          if (!res.ok || !((_a = res.headers.get("Content-Type")) == null ? void 0 : _a.includes("application/zip"))) throw Error("\u5BFC\u51FA\u5931\u8D25");
          const url = URL.createObjectURL(await res.blob()), a = document.createElement("a");
          a.href = url;
          a.download = sk.id + ".zip";
          a.click();
          setTimeout(() => URL.revokeObjectURL(url), 6e4);
        } catch (e) {
          T(e.message, "e");
        }
      };
      actions.append(toggle, preview, download);
      details.append(select, start, next, pre);
      card.append(head, desc, actions, details);
      cards.appendChild(card);
    }
  };
  host.querySelector("#yy-skill-search").oninput = render;
  host.querySelector("#yy-skill-all").onclick = () => {
    r.catalog.forEach((sk) => selected.add(sk.id));
    render();
    status();
  };
  host.querySelector("#yy-skill-none").onclick = () => {
    selected.clear();
    render();
    status();
  };
  host.querySelector("#yy-skill-save").onclick = async () => {
    const button = host.querySelector("#yy-skill-save");
    button.disabled = true;
    try {
      const res = await A("POST", "/api/youyuan/agent/skills", { id, skills: Array.from(selected) });
      if (!res.ok) throw Error(res.error);
      r.skills = res.skills;
      host.querySelector("#yy-skill-selection").textContent = "\u5DF2\u4FDD\u5B58 " + r.skills.length + " \u4EFD\u6280\u80FD";
      T("\u6280\u80FD\u9009\u62E9\u5DF2\u7EDF\u4E00\u4FDD\u5B58", "o");
    } catch (e) {
      T(e.message, "e");
    } finally {
      button.disabled = false;
    }
  };
  render();
  host.querySelector("#yy-skill-selection").textContent = "\u5F53\u524D\u5DF2\u4FDD\u5B58 " + selected.size + " \u4EFD\u6280\u80FD";
}
