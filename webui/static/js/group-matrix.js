// group-matrix.js - SubMatrix 拓扑矩阵可视化交互控制中心
let currentGroups = [];
let currentApiKey = localStorage.getItem("apiKey") || "";
let currentPort = location.port || "8199";

function showToast(msg, isErr = false) {
  const toast = document.createElement("div");
  toast.className = "matrix-toast";
  toast.style.cssText = `
    position: fixed; bottom: 24px; right: 24px; z-index: 999999;
    background: ${isErr ? "#ef4444" : "#18181b"};
    color: ${isErr ? "#fff" : "#00ffcc"};
    border: 1px solid ${isErr ? "#dc2626" : "rgba(0, 255, 204, 0.4)"};
    padding: 10px 18px; font-size: 0.82rem; font-weight: 600;
    box-shadow: 0 4px 16px rgba(0,0,0,0.5);
    border-radius: 0; display: flex; align-items: center; gap: 8px;
    font-family: monospace; transition: opacity 0.3s;
  `;
  toast.textContent = (isErr ? "❌ " : "⚡ ") + msg;
  document.body.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = "0";
    setTimeout(() => toast.remove(), 300);
  }, 2600);
}

export async function fetchGroups() {
  try {
    const key = localStorage.getItem("apiKey") || currentApiKey;
    const res = await fetch(`/api/groups?token=${encodeURIComponent(key)}`, {
      headers: { "X-API-Key": key }
    });
    if (!res.ok) throw new Error("加载分组失败: " + res.status);
    const data = await res.json();
    currentGroups = data.groups || [];
    if (data.apiKey) currentApiKey = data.apiKey;
    if (data.listenPort) currentPort = data.listenPort.replace(":", "");
    renderGroupMatrixPanel();
  } catch (e) {
    console.error("加载矩阵分组失败:", e);
    const container = document.getElementById("panel-groups");
    if (container) {
      container.innerHTML = `<div style="color:#ef4444;padding:20px;">加载矩阵分组失败，请确认认证状态: ${e.message}</div>`;
    }
  }
}

export function renderGroupMatrixPanel() {
  const panel = document.getElementById("panel-groups");
  if (!panel) return;

  let html = `
    <div class="matrix-header-bar">
      <div>
        <div class="matrix-header-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#00ffcc" stroke-width="2">
            <rect x="3" y="3" width="7" height="7"></rect>
            <rect x="14" y="3" width="7" height="7"></rect>
            <rect x="14" y="14" width="7" height="7"></rect>
            <rect x="3" y="14" width="7" height="7"></rect>
          </svg>
          <span>订阅拓扑矩阵管理 (Groups Matrix)</span>
        </div>
        <div class="matrix-header-subtitle">多组级联继承、独立并发测活、失败沉底保护与纯透传聚合调度</div>
      </div>
      <div class="matrix-actions-bar">
        <button id="btnAddNewGroup" class="btn-matrix-sharp">+ 新增分组</button>
        <button id="btnSaveGroups" class="btn-matrix-save">💾 保存矩阵</button>
      </div>
    </div>
    <div id="groupsCardsList">
  `;

  currentGroups.forEach((g, idx) => {
    const isEnabled = g.enabled !== false;
    const checkAlive = g["check-alive"] !== false;
    const checkSpeed = g["check-speed"] === true;
    const renameNode = g["rename-node"] !== false;
    const dropOnFail = g["drop-on-fail"] !== false;
    const timeout = g.timeout || 3000;
    const urlsText = (g.urls || []).join("\n");
    const host = location.hostname;
    const protocol = location.protocol;
    const portStr = currentPort ? (":" + currentPort) : (location.port ? ":" + location.port : "");
    const baseLink = `${protocol}//${host}${portStr}/group/${encodeURIComponent(g.name || "group")}`;
    const tokenQuery = currentApiKey ? `?token=${encodeURIComponent(currentApiKey)}` : "";

    html += `
      <div class="group-matrix-card ${isEnabled ? "" : "disabled-group"}" data-index="${idx}">
        <div class="group-card-header">
          <div class="group-identity">
            <span class="group-name-tag">${g.name || "未命名分组"}</span>
            <span class="group-badge">${g.tag || "TAG"}</span>
            <span style="font-size:0.75rem;color:#71717a;">${g["display-name"] || ""}</span>
          </div>
          <div class="sharp-toggle-wrap">
            <span class="sharp-toggle-label">${isEnabled ? "已启用" : "已禁用"}</span>
            <label class="matrix-switch">
              <input type="checkbox" class="grp-enable-toggle" data-index="${idx}" ${isEnabled ? "checked" : ""}>
              <span class="matrix-slider"></span>
            </label>
          </div>
        </div>

        <!-- 4 维独立工业级开关网格 -->
        <div class="group-switches-grid">
          <div class="switch-item">
            <span class="switch-title">连通测活</span>
            <label class="matrix-switch">
              <input type="checkbox" class="sw-check-alive" data-index="${idx}" ${checkAlive ? "checked" : ""}>
              <span class="matrix-slider"></span>
            </label>
          </div>
          <div class="switch-item">
            <span class="switch-title">带宽测速</span>
            <label class="matrix-switch">
              <input type="checkbox" class="sw-check-speed" data-index="${idx}" ${checkSpeed ? "checked" : ""}>
              <span class="matrix-slider"></span>
            </label>
          </div>
          <div class="switch-item">
            <span class="switch-title">智能改名</span>
            <label class="matrix-switch">
              <input type="checkbox" class="sw-rename-node" data-index="${idx}" ${renameNode ? "checked" : ""}>
              <span class="matrix-slider"></span>
            </label>
          </div>
          <div class="switch-item">
            <span class="switch-title">失败丢弃</span>
            <label class="matrix-switch">
              <input type="checkbox" class="sw-drop-on-fail" data-index="${idx}" ${dropOnFail ? "checked" : ""}>
              <span class="matrix-slider"></span>
            </label>
          </div>
        </div>

        <!-- 参数编辑输入行 -->
        <div class="group-inputs-row">
          <div class="matrix-form-group">
            <label class="matrix-label">组英文标识 (Name)</label>
            <input type="text" class="matrix-input inp-name" data-index="${idx}" value="${g.name || ""}" placeholder="例如: free-a">
          </div>
          <div class="matrix-form-group">
            <label class="matrix-label">组显示名称 (Display Name)</label>
            <input type="text" class="matrix-input inp-display-name" data-index="${idx}" value="${g["display-name"] || ""}" placeholder="例如: 公网免费A组">
          </div>
          <div class="matrix-form-group">
            <label class="matrix-label">节点自定义标签 (Tag)</label>
            <input type="text" class="matrix-input inp-tag" data-index="${idx}" value="${g.tag || ""}" placeholder="例如: 免费A">
          </div>
          <div class="matrix-form-group">
            <label class="matrix-label">探测超时 (ms)</label>
            <input type="number" class="matrix-input inp-timeout" data-index="${idx}" value="${timeout}" placeholder="3000">
          </div>
        </div>

        <!-- 订阅链接文本域 -->
        <div class="matrix-form-group" style="margin-bottom:10px;">
          <label class="matrix-label">该组订阅源 URLs (每行一个，支持子组 http 聚合链接)</label>
          <textarea class="matrix-textarea inp-urls" data-index="${idx}" placeholder="https://...&#10;http://127.0.0.1:8199/group/free-a/all.yaml?token=...">${urlsText}</textarea>
        </div>

        <!-- 聚合直链快速复制与操作 -->
        <div class="group-footer-bar">
          <div class="direct-copy-group">
            <span style="font-size:0.72rem;color:#71717a;">组专属订阅:</span>
            <button class="btn-direct-link btn-copy-link" data-url="${baseLink}/all.yaml${tokenQuery}">📋 复制 Clash</button>
            <button class="btn-direct-link btn-copy-link" data-url="${baseLink}/mihomo.yaml${tokenQuery}">📋 复制 Mihomo</button>
            <button class="btn-direct-link btn-copy-link" data-url="${baseLink}/base64.txt${tokenQuery}">📋 复制 Base64</button>
          </div>
          <button class="btn-group-delete btn-del-group" data-index="${idx}">🗑️ 删除此分组</button>
        </div>
      </div>
    `;
  });

  html += "</div>";
  panel.innerHTML = html;
  bindGroupEvents();
}

function bindGroupEvents() {
  // 1. 复制直链
  document.querySelectorAll(".btn-copy-link").forEach(btn => {
    btn.addEventListener("click", () => {
      const url = btn.getAttribute("data-url");
      if (navigator.clipboard) {
        navigator.clipboard.writeText(url).then(() => showToast("订阅直链已复制至剪贴板！"));
      } else {
        const inp = document.createElement("input");
        inp.value = url;
        document.body.appendChild(inp);
        inp.select();
        document.execCommand("copy");
        inp.remove();
        showToast("订阅直链已复制！");
      }
    });
  });

  // 2. 启用/禁用单组主开关
  document.querySelectorAll(".grp-enable-toggle").forEach(chk => {
    chk.addEventListener("change", async (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      const isChecked = e.target.checked;
      const grp = currentGroups[idx];
      if (grp) {
        grp.enabled = isChecked;
        const card = e.target.closest(".group-matrix-card");
        if (card) {
          card.classList.toggle("disabled-group", !isChecked);
          const lbl = card.querySelector(".sharp-toggle-label");
          if (lbl) lbl.textContent = isChecked ? "已启用" : "已禁用";
        }
        try {
          const key = localStorage.getItem("apiKey") || currentApiKey;
          await fetch(`/api/groups/toggle?token=${encodeURIComponent(key)}`, {
            method: "POST",
            headers: { "Content-Type": "application/json", "X-API-Key": key },
            body: JSON.stringify({ name: grp.name, enabled: isChecked })
          });
          showToast(`分组 [${grp.name}] 已${isChecked ? "启用" : "停用"}`);
        } catch (err) {
          showToast("状态切换失败: " + err.message, true);
        }
      }
    });
  });

  // 3. 字段输入双向绑定
  document.querySelectorAll(".inp-name").forEach(inp => {
    inp.addEventListener("input", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx].name = e.target.value.trim();
    });
  });
  document.querySelectorAll(".inp-display-name").forEach(inp => {
    inp.addEventListener("input", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx]["display-name"] = e.target.value.trim();
    });
  });
  document.querySelectorAll(".inp-tag").forEach(inp => {
    inp.addEventListener("input", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx].tag = e.target.value.trim();
    });
  });
  document.querySelectorAll(".inp-timeout").forEach(inp => {
    inp.addEventListener("input", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx].timeout = parseInt(e.target.value, 10) || 3000;
    });
  });
  document.querySelectorAll(".inp-urls").forEach(inp => {
    inp.addEventListener("input", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].urls = e.target.value.split("\n").map(s => s.trim()).filter(Boolean);
      }
    });
  });

  // 4. 4 项开关双向绑定
  document.querySelectorAll(".sw-check-alive").forEach(chk => {
    chk.addEventListener("change", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx]["check-alive"] = e.target.checked;
    });
  });
  document.querySelectorAll(".sw-check-speed").forEach(chk => {
    chk.addEventListener("change", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx]["check-speed"] = e.target.checked;
    });
  });
  document.querySelectorAll(".sw-rename-node").forEach(chk => {
    chk.addEventListener("change", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx]["rename-node"] = e.target.checked;
    });
  });
  document.querySelectorAll(".sw-drop-on-fail").forEach(chk => {
    chk.addEventListener("change", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) currentGroups[idx]["drop-on-fail"] = e.target.checked;
    });
  });

  // 5. 新增分组
  const btnAdd = document.getElementById("btnAddNewGroup");
  if (btnAdd) {
    btnAdd.addEventListener("click", () => {
      currentGroups.push({
        name: "group-" + (currentGroups.length + 1),
        "display-name": "新分组 " + (currentGroups.length + 1),
        tag: "GRP",
        enabled: true,
        "check-alive": true,
        "check-speed": false,
        "rename-node": true,
        "drop-on-fail": true,
        timeout: 3000,
        urls: []
      });
      renderGroupMatrixPanel();
      showToast("已添加新分组模板，编辑后请点击保存");
    });
  }

  // 6. 删除分组
  document.querySelectorAll(".btn-del-group").forEach(btn => {
    btn.addEventListener("click", (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      const grp = currentGroups[idx];
      if (confirm(`确定要删除分组 [${grp ? grp.name : ""}] 吗？`)) {
        currentGroups.splice(idx, 1);
        renderGroupMatrixPanel();
        showToast("已移除分组，点击保存以写入配置");
      }
    });
  });

  // 7. 保存全量矩阵
  const btnSave = document.getElementById("btnSaveGroups");
  if (btnSave) {
    btnSave.addEventListener("click", async () => {
      try {
        const key = localStorage.getItem("apiKey") || currentApiKey;
        const res = await fetch(`/api/groups?token=${encodeURIComponent(key)}`, {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-API-Key": key },
          body: JSON.stringify({ groups: currentGroups })
        });
        if (!res.ok) {
          const errData = await res.json();
          throw new Error(errData.error || res.statusText);
        }
        showToast("分组矩阵配置已成功持久化并即时热重载！");
      } catch (e) {
        showToast("保存失败: " + e.message, true);
      }
    });
  }
}

// 自动初始化与侧边栏联动
document.addEventListener("DOMContentLoaded", () => {
  // 页面加载预先载入矩阵分组
  setTimeout(fetchGroups, 500);

  // 监听侧边栏分组矩阵按钮
  const navBtn = document.getElementById("groupMatrixNavBtn");
  if (navBtn) {
    navBtn.addEventListener("click", () => {
      const tab = document.querySelector('.cfg-tab[data-tab="groups"]');
      if (tab) {
        tab.click();
        tab.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
      const panel = document.getElementById("panel-groups");
      if (panel) {
        panel.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    });
  }

  // 监听抽屉内的 groups tab 点击
  const groupTab = document.querySelector('.cfg-tab[data-tab="groups"]');
  if (groupTab) {
    groupTab.addEventListener("click", () => {
      document.querySelectorAll(".cfg-panel").forEach(p => p.classList.remove("active"));
      document.querySelectorAll(".cfg-tab").forEach(t => {
        t.classList.remove("active");
        t.setAttribute("aria-selected", "false");
      });
      groupTab.classList.add("active");
      groupTab.setAttribute("aria-selected", "true");

      const panel = document.getElementById("panel-groups");
      if (panel) {
        panel.classList.add("active");
        panel.style.display = "block";
      }
      fetchGroups();
    });
  }
});
