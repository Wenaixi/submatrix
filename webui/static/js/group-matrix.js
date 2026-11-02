// group-matrix.js - SubMatrix 拓扑矩阵控制中心 (Claude Design System)
let currentGroups = [];
let currentApiKey = localStorage.getItem("apiKey") || "wenxi-admin";
let currentPort = location.port || "8199";

const OUTLINE_ICONS = {
  copy: '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="square"><rect x="9" y="9" width="13" height="13"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>',
  trash: '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="square"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>',
  check: '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="square"><polyline points="20 6 9 17 4 12"/></svg>',
  alert: '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="square"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>'
};

function showToast(msg, isErr = false) {
  document.querySelectorAll(".matrix-toast-center").forEach(t => t.remove());
  const toast = document.createElement("div");
  toast.className = "matrix-toast-center";
  toast.style.background = isErr ? "#1c1c1f" : "#ffffff";
  toast.style.color = isErr ? "#ffffff" : "#000000";
  toast.style.borderColor = isErr ? "#ef4444" : "#ffffff";
  toast.innerHTML = `<span style="display:inline-flex;align-items:center;gap:8px;">${isErr ? OUTLINE_ICONS.alert : OUTLINE_ICONS.check} ${msg}</span>`;
  document.body.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = "0";
    setTimeout(() => toast.remove(), 250);
  }, 2200);
}

export async function fetchGroups() {
  try {
    const key = localStorage.getItem("apiKey") || currentApiKey;
    const res = await fetch(`/api/groups`, {
      headers: { "X-API-Key": key }
    });
    if (!res.ok) throw new Error("HTTP " + res.status);
    const data = await res.json();
    currentGroups = data.groups || [];
    if (data.apiKey) currentApiKey = data.apiKey;
    if (data.listenPort) currentPort = data.listenPort.replace(":", "");
    renderAllViews();
  } catch (e) {
    console.error("加载矩阵分组失败:", e);
    const container = document.getElementById("matrixModalCards");
    if (container) {
      container.innerHTML = `<div style="color:#a1a1aa;padding:24px;font-family:monospace;font-size:13px;">无法连接到分组 API 端点: ${e.message}</div>`;
    }
  }
}

function updateStatsBar() {
  const total = currentGroups.length;
  let active = 0;
  let urlsCount = 0;
  currentGroups.forEach(g => {
    const isEnabled = (g.Enabled ?? g.enabled) !== false;
    if (isEnabled) active++;
    const urls = g.URLs ?? g.urls ?? [];
    urlsCount += urls.length;
  });

  const elTotal = document.getElementById("mStatTotal");
  const elActive = document.getElementById("mStatActive");
  const elUrls = document.getElementById("mStatUrls");
  const elPort = document.getElementById("mStatPort");
  if (elTotal) elTotal.textContent = total;
  if (elActive) elActive.textContent = active;
  if (elUrls) elUrls.textContent = urlsCount;
  if (elPort) elPort.textContent = ":" + currentPort;
}

export function renderAllViews() {
  updateStatsBar();
  const modalCards = document.getElementById("matrixModalCards");
  const drawerPanel = document.getElementById("panel-groups");
  const cardsHTML = generateCardsHTML();

  if (modalCards) modalCards.innerHTML = cardsHTML;
  if (drawerPanel) {
    drawerPanel.innerHTML = `
      <div style="padding:14px;border-bottom:1px solid rgba(255,255,255,0.08);display:flex;justify-content:space-between;align-items:center;">
        <div>
          <div style="font-size:14px;font-weight:500;color:#ffffff;">分组矩阵调度</div>
          <div style="font-size:12px;color:#71717a;">在侧边栏点击可直接唤出大屏中心</div>
        </div>
        <button id="drawerOpenModalBtn" class="btn-matrix-white" style="padding:4px 10px;font-size:12px;">全景视图</button>
      </div>
      <div style="padding:12px;">${cardsHTML}</div>
    `;
    const openBtn = document.getElementById("drawerOpenModalBtn");
    if (openBtn) openBtn.addEventListener("click", openMatrixModal);
  }

  bindGroupCardEvents();
}

function generateCardsHTML() {
  if (currentGroups.length === 0) {
    return `<div style="padding:32px;color:#71717a;font-size:13px;text-align:center;">暂无配置分组，请点击右上角 "+ 新增分组"</div>`;
  }

  return currentGroups.map((g, idx) => {
    const name = g.Name ?? g.name ?? "未命名分组";
    const displayName = g.DisplayName ?? g["display-name"] ?? "";
    const tag = g.Tag ?? g.tag ?? "TAG";
    const isEnabled = (g.Enabled ?? g.enabled) !== false;
    const checkAlive = (g.CheckAlive ?? g["check-alive"]) !== false;
    const checkSpeed = (g.CheckSpeed ?? g["check-speed"]) === true;
    const renameNode = (g.RenameNode ?? g["rename-node"]) !== false;
    const dropOnFail = (g.DropOnFail ?? g["drop-on-fail"]) !== false;
    const timeout = g.Timeout ?? g.timeout ?? 3000;
    const urls = g.URLs ?? g.urls ?? [];
    const urlsText = Array.isArray(urls) ? urls.join("\n") : "";
    const host = location.hostname;
    const protocol = location.protocol;
    const portStr = ":" + currentPort;
    const baseLink = `${protocol}//${host}${portStr}/group/${encodeURIComponent(name)}`;
    const tokenQuery = currentApiKey ? `?token=${encodeURIComponent(currentApiKey)}` : "";

    return `
      <div class="group-matrix-card ${isEnabled ? "" : "disabled-group"}" data-index="${idx}">
        <div class="group-card-header">
          <div class="group-identity">
            <span class="group-name-tag">${name}</span>
            <span class="group-badge">${tag}</span>
            <span style="font-size:12px;color:#71717a;">${displayName}</span>
          </div>
          <div class="sharp-toggle-wrap">
            <span class="sharp-toggle-label">${isEnabled ? "已启用" : "已停用"}</span>
            <label class="matrix-switch">
              <input type="checkbox" class="grp-enable-toggle" data-index="${idx}" ${isEnabled ? "checked" : ""}>
              <span class="matrix-slider"></span>
            </label>
          </div>
        </div>

        <div class="group-preset-strip">
          <span>快捷预设:</span>
          <button class="btn-preset-pill btn-set-pass" data-index="${idx}">纯透传模式</button>
          <button class="btn-preset-pill btn-set-audit" data-index="${idx}">完整质检模式</button>
        </div>

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

        <div class="group-inputs-row">
          <div class="matrix-form-group">
            <label class="matrix-label">组标识 (Name)</label>
            <input type="text" class="matrix-input inp-name" data-index="${idx}" value="${name}">
          </div>
          <div class="matrix-form-group">
            <label class="matrix-label">显示名称 (Display Name)</label>
            <input type="text" class="matrix-input inp-display-name" data-index="${idx}" value="${displayName}">
          </div>
          <div class="matrix-form-group">
            <label class="matrix-label">节点标签 (Tag)</label>
            <input type="text" class="matrix-input inp-tag" data-index="${idx}" value="${tag}">
          </div>
          <div class="matrix-form-group">
            <label class="matrix-label">探测超时 (ms)</label>
            <input type="number" class="matrix-input inp-timeout" data-index="${idx}" value="${timeout}">
          </div>
        </div>

        <details class="matrix-urls-accordion">
          <summary class="matrix-urls-summary">
            <span>订阅源清单 (${urls.length} 条有效链接)</span>
            <span style="font-size:12px;color:#71717a;">点击展开 / 收起</span>
          </summary>
          <textarea class="matrix-textarea inp-urls" data-index="${idx}" placeholder="每行一个订阅链接">${urlsText}</textarea>
        </details>

        <div class="group-footer-bar">
          <div class="direct-copy-group">
            <button class="btn-direct-link btn-copy-link" data-url="${baseLink}/all.yaml${tokenQuery}">${OUTLINE_ICONS.copy} 复制 Clash</button>
            <button class="btn-direct-link btn-copy-link" data-url="${baseLink}/mihomo.yaml${tokenQuery}">${OUTLINE_ICONS.copy} 复制 Mihomo</button>
            <button class="btn-direct-link btn-copy-link" data-url="${baseLink}/base64.txt${tokenQuery}">${OUTLINE_ICONS.copy} 复制 Base64</button>
          </div>
          <button class="btn-group-delete btn-del-group" data-index="${idx}">${OUTLINE_ICONS.trash} 删除此组</button>
        </div>
      </div>
    `;
  }).join("");
}

function bindGroupCardEvents() {
  document.querySelectorAll(".btn-copy-link").forEach(btn => {
    btn.onclick = () => {
      const url = btn.getAttribute("data-url");
      if (navigator.clipboard) {
        navigator.clipboard.writeText(url).then(() => showToast("订阅直链已复制至剪贴板"));
      } else {
        const inp = document.createElement("input");
        inp.value = url;
        document.body.appendChild(inp);
        inp.select();
        document.execCommand("copy");
        inp.remove();
        showToast("订阅直链已复制至剪贴板");
      }
    };
  });

  document.querySelectorAll(".grp-enable-toggle").forEach(chk => {
    chk.onchange = async (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      const isChecked = e.target.checked;
      const grp = currentGroups[idx];
      if (grp) {
        grp.Enabled = isChecked;
        grp.enabled = isChecked;
        renderAllViews();
        try {
          const key = localStorage.getItem("apiKey") || currentApiKey;
          await fetch(`/api/groups/toggle`, {
            method: "POST",
            headers: { "Content-Type": "application/json", "X-API-Key": key },
            body: JSON.stringify({ name: grp.Name || grp.name, enabled: isChecked })
          });
          showToast(`分组 [${grp.Name || grp.name}] 状态已更新`);
        } catch (err) {
          showToast("状态切换失败: " + err.message, true);
        }
      }
    };
  });

  document.querySelectorAll(".btn-set-pass").forEach(btn => {
    btn.onclick = () => {
      const idx = parseInt(btn.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].CheckAlive = false;
        currentGroups[idx]["check-alive"] = false;
        currentGroups[idx].CheckSpeed = false;
        currentGroups[idx]["check-speed"] = false;
        renderAllViews();
        showToast("已切换为纯透传预设 (关闭测活测速)");
      }
    };
  });

  document.querySelectorAll(".btn-set-audit").forEach(btn => {
    btn.onclick = () => {
      const idx = parseInt(btn.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].CheckAlive = true;
        currentGroups[idx]["check-alive"] = true;
        currentGroups[idx].CheckSpeed = false;
        currentGroups[idx]["check-speed"] = false;
        renderAllViews();
        showToast("已切换为质检预设 (开启测活)");
      }
    };
  });

  document.querySelectorAll(".inp-name").forEach(inp => {
    inp.oninput = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].Name = e.target.value.trim();
        currentGroups[idx].name = e.target.value.trim();
      }
    };
  });
  document.querySelectorAll(".inp-display-name").forEach(inp => {
    inp.oninput = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].DisplayName = e.target.value.trim();
        currentGroups[idx]["display-name"] = e.target.value.trim();
      }
    };
  });
  document.querySelectorAll(".inp-tag").forEach(inp => {
    inp.oninput = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].Tag = e.target.value.trim();
        currentGroups[idx].tag = e.target.value.trim();
      }
    };
  });
  document.querySelectorAll(".inp-timeout").forEach(inp => {
    inp.oninput = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].Timeout = parseInt(e.target.value, 10) || 3000;
        currentGroups[idx].timeout = currentGroups[idx].Timeout;
      }
    };
  });
  document.querySelectorAll(".inp-urls").forEach(inp => {
    inp.oninput = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        const urls = e.target.value.split("\n").map(s => s.trim()).filter(Boolean);
        currentGroups[idx].URLs = urls;
        currentGroups[idx].urls = urls;
        updateStatsBar();
      }
    };
  });

  document.querySelectorAll(".sw-check-alive").forEach(chk => {
    chk.onchange = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].CheckAlive = e.target.checked;
        currentGroups[idx]["check-alive"] = e.target.checked;
      }
    };
  });
  document.querySelectorAll(".sw-check-speed").forEach(chk => {
    chk.onchange = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].CheckSpeed = e.target.checked;
        currentGroups[idx]["check-speed"] = e.target.checked;
      }
    };
  });
  document.querySelectorAll(".sw-rename-node").forEach(chk => {
    chk.onchange = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].RenameNode = e.target.checked;
        currentGroups[idx]["rename-node"] = e.target.checked;
      }
    };
  });
  document.querySelectorAll(".sw-drop-on-fail").forEach(chk => {
    chk.onchange = (e) => {
      const idx = parseInt(e.target.getAttribute("data-index"), 10);
      if (currentGroups[idx]) {
        currentGroups[idx].DropOnFail = e.target.checked;
        currentGroups[idx]["drop-on-fail"] = e.target.checked;
      }
    };
  });

  document.querySelectorAll(".btn-del-group").forEach(btn => {
    btn.onclick = () => {
      const idx = parseInt(btn.getAttribute("data-index"), 10);
      const grp = currentGroups[idx];
      const gName = grp ? (grp.Name || grp.name) : "";
      if (confirm(`确定删除分组 [${gName}] 吗？`)) {
        currentGroups.splice(idx, 1);
        renderAllViews();
        showToast("已移除分组，点击保存即可写回配置");
      }
    };
  });
}

export function openMatrixModal() {
  const modal = document.getElementById("matrixModalOverlay");
  if (modal) {
    modal.style.display = "flex";
    fetchGroups();
  }
}

export function closeMatrixModal() {
  const modal = document.getElementById("matrixModalOverlay");
  if (modal) {
    modal.style.display = "none";
  }
}

document.addEventListener("DOMContentLoaded", () => {
  setTimeout(fetchGroups, 500);

  const navBtn = document.getElementById("groupMatrixNavBtn");
  if (navBtn) {
    navBtn.addEventListener("click", () => {
      openMatrixModal();
    });
  }

  const closeBtn = document.getElementById("matrixModalCloseBtn");
  if (closeBtn) closeBtn.onclick = closeMatrixModal;

  const modalOverlay = document.getElementById("matrixModalOverlay");
  if (modalOverlay) {
    modalOverlay.onclick = (e) => {
      if (e.target === modalOverlay) closeMatrixModal();
    };
  }

  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") closeMatrixModal();
  });

  const btnBatchPass = document.getElementById("modalBtnBatchPass");
  if (btnBatchPass) {
    btnBatchPass.onclick = () => {
      currentGroups.forEach(g => {
        g.CheckAlive = false;
        g["check-alive"] = false;
        g.CheckSpeed = false;
        g["check-speed"] = false;
      });
      renderAllViews();
      showToast("所有分组已设为纯透传模式 (跳过测活测速)");
    };
  }

  const btnBatchCheck = document.getElementById("modalBtnBatchCheck");
  if (btnBatchCheck) {
    btnBatchCheck.onclick = () => {
      currentGroups.forEach(g => {
        g.CheckAlive = true;
        g["check-alive"] = true;
        g.CheckSpeed = false;
        g["check-speed"] = false;
      });
      renderAllViews();
      showToast("所有分组已开启测活质检");
    };
  }

  const btnEnableAll = document.getElementById("modalBtnEnableAll");
  if (btnEnableAll) {
    btnEnableAll.onclick = () => {
      currentGroups.forEach(g => {
        g.Enabled = true;
        g.enabled = true;
      });
      renderAllViews();
      showToast("已全部启用");
    };
  }

  const btnDisableAll = document.getElementById("modalBtnDisableAll");
  if (btnDisableAll) {
    btnDisableAll.onclick = () => {
      currentGroups.forEach(g => {
        g.Enabled = false;
        g.enabled = false;
      });
      renderAllViews();
      showToast("已全部停用");
    };
  }

  const btnAdd = document.getElementById("modalBtnAddGroup");
  if (btnAdd) {
    btnAdd.onclick = () => {
      currentGroups.push({
        Name: "group-" + (currentGroups.length + 1),
        DisplayName: "新分组 " + (currentGroups.length + 1),
        Tag: "GRP",
        Enabled: true,
        CheckAlive: true,
        CheckSpeed: false,
        RenameNode: true,
        DropOnFail: true,
        Timeout: 3000,
        URLs: []
      });
      renderAllViews();
      showToast("已添加新分组模板");
    };
  }

  const btnSave = document.getElementById("modalBtnSave");
  if (btnSave) {
    btnSave.onclick = async () => {
      try {
        const key = localStorage.getItem("apiKey") || currentApiKey;
        const res = await fetch(`/api/groups`, {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-API-Key": key },
          body: JSON.stringify({ groups: currentGroups })
        });
        if (!res.ok) {
          const errData = await res.json();
          throw new Error(errData.error || res.statusText);
        }
        showToast("拓扑矩阵配置已成功持久化并即时热重载");
      } catch (e) {
        showToast("保存失败: " + e.message, true);
      }
    };
  }
});
