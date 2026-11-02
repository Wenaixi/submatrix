import React, { useState } from 'react';
import { GroupItem, saveGroups, toggleGroup } from '../api/client';
import { Switch } from '../components/Switch';
import { useToast } from '../components/Toast';
import { Copy, Plus, Trash2, CheckCircle, Save } from 'lucide-react';

interface GroupsViewProps {
  groups: GroupItem[];
  setGroups: React.Dispatch<React.SetStateAction<GroupItem[]>>;
  listenPort: string;
  apiKey: string;
  onRefresh: () => void;
}

export const GroupsView: React.FC<GroupsViewProps> = ({ groups, setGroups, listenPort, apiKey, onRefresh }) => {
  const { showToast } = useToast();
  const [saving, setSaving] = useState(false);

  // 1. 复制直链
  const copyLink = (url: string) => {
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

  // 2. 字段修改
  const updateGroup = (idx: number, patch: Partial<GroupItem>) => {
    setGroups(prev => {
      const copy = [...prev];
      copy[idx] = { ...copy[idx], ...patch };
      return copy;
    });
  };

  // 3. 单组开关即时持久化
  const handleToggle = async (idx: number, checked: boolean) => {
    updateGroup(idx, { Enabled: checked, enabled: checked });
    const g = groups[idx];
    const name = g.Name || g.name || "";
    try {
      await toggleGroup(name, checked);
      showToast(`分组 [${name}] 状态已更新`);
    } catch (err: any) {
      showToast(`状态切换失败: ${err.message}`, true);
    }
  };

  // 4. 批量操作
  const setAllEnabled = (enabled: boolean) => {
    setGroups(prev => prev.map(g => ({ ...g, Enabled: enabled, enabled })));
    showToast(enabled ? "所有分组已设为启用" : "所有分组已设为停用");
  };

  // 5. 新增分组
  const addGroup = () => {
    const newIdx = groups.length + 1;
    setGroups(prev => [
      ...prev,
      {
        Name: "group-" + newIdx,
        DisplayName: "新分组 " + newIdx,
        Tag: "GRP",
        Enabled: true,
        CheckAlive: true,
        CheckSpeed: false,
        RenameNode: true,
        DropOnFail: true,
        Timeout: 3000,
        URLs: []
      }
    ]);
    showToast("已添加新分组模板，编辑后请保存");
  };

  // 6. 删除分组
  const deleteGroup = (idx: number) => {
    const g = groups[idx];
    const name = g.Name || g.name || "";
    if (confirm(`确定删除分组 [${name}] 吗？`)) {
      setGroups(prev => prev.filter((_, i) => i !== idx));
      showToast("已移除分组，点击右上角保存即可写入配置");
    }
  };

  // 7. 保存全量配置
  const handleSave = async () => {
    setSaving(true);
    try {
      await saveGroups(groups);
      showToast("拓扑矩阵配置已成功持久化并即时热重载");
      onRefresh();
    } catch (err: any) {
      showToast(`保存失败: ${err.message}`, true);
    } finally {
      setSaving(false);
    }
  };

  const portStr = listenPort ? (listenPort.startsWith(":") ? listenPort : ":" + listenPort) : (location.port ? ":" + location.port : "");
  const protocol = location.protocol;
  const host = location.hostname;
  const tokenQuery = apiKey ? `?token=${encodeURIComponent(apiKey)}` : "";

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 顶部操作与状态栏 */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '16px 20px',
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        flexWrap: 'wrap',
        gap: '12px'
      }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 500, color: 'var(--text-primary)', letterSpacing: '0.02em' }}>
            拓扑矩阵管理 (Groups Matrix)
          </h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            级联继承、独立并发测活、客观打标与纯透传聚合调度中心
          </p>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          <button onClick={() => setAllEnabled(true)} className="btn-ghost">全部启用</button>
          <button onClick={() => setAllEnabled(false)} className="btn-ghost">全部停用</button>
          <button onClick={addGroup}>
            <Plus size={14} /> 新增分组
          </button>
          <button onClick={handleSave} className="btn-primary" disabled={saving}>
            <Save size={14} /> {saving ? "正在保存..." : "保存配置"}
          </button>
        </div>
      </div>

      {/* 分组卡片网格 */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(460px, 1fr))',
        gap: '16px'
      }}>
        {groups.map((g, idx) => {
          const name = g.Name ?? g.name ?? "未命名分组";
          const displayName = g.DisplayName ?? g.displayName ?? "";
          const tag = g.Tag ?? g.tag ?? "TAG";
          const isEnabled = (g.Enabled ?? g.enabled) !== false;
          const checkAlive = (g.CheckAlive ?? g.CheckAlive) !== false;
          const checkSpeed = (g.CheckSpeed ?? g.CheckSpeed) === true;
          const renameNode = (g.RenameNode ?? g.RenameNode) !== false;
          const dropOnFail = (g.DropOnFail ?? g.DropOnFail) !== false;
          const timeout = g.Timeout ?? 3000;
          const urls = g.URLs ?? g.urls ?? [];
          const urlsText = Array.isArray(urls) ? urls.join("\n") : "";
          const baseLink = `${protocol}//${host}${portStr}/group/${encodeURIComponent(name)}`;

          return (
            <div
              key={idx}
              style={{
                background: 'var(--surface-2)',
                border: '1px solid var(--border)',
                borderColor: isEnabled ? 'var(--border)' : '#e4e4e7',
                padding: '18px 20px',
                display: 'flex',
                flexDirection: 'column',
                gap: '14px',
                opacity: isEnabled ? 1 : 0.5,
                borderStyle: isEnabled ? 'solid' : 'dashed'
              }}
            >
              {/* 卡片头部 */}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border)', paddingBottom: '12px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '15px', fontWeight: 500, fontFamily: 'var(--font-mono)', color: 'var(--text-primary)' }}>{name}</span>
                  <span className="badge">{tag}</span>
                  {displayName && <span style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>{displayName}</span>}
                </div>
                <Switch
                  checked={isEnabled}
                  onChange={(c) => handleToggle(idx, c)}
                  label={isEnabled ? "已启用" : "已停用"}
                />
              </div>

              {/* 4 维独立开关 (规整 4 列对齐) */}
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(4, 1fr)',
                gap: '8px',
                padding: '10px 12px',
                background: 'var(--surface-1)',
                border: '1px solid var(--border)'
              }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>连通测活</span>
                  <Switch checked={checkAlive} onChange={(c) => updateGroup(idx, { CheckAlive: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>带宽测速</span>
                  <Switch checked={checkSpeed} onChange={(c) => updateGroup(idx, { CheckSpeed: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>智能改名</span>
                  <Switch checked={renameNode} onChange={(c) => updateGroup(idx, { RenameNode: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>失败丢弃</span>
                  <Switch checked={dropOnFail} onChange={(c) => updateGroup(idx, { DropOnFail: c })} />
                </div>
              </div>

              {/* 规整参数输入项 */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '10px' }}>
                <div>
                  <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>组标识 (Name)</label>
                  <input
                    type="text"
                    value={name}
                    onChange={(e) => updateGroup(idx, { Name: e.target.value.trim(), name: e.target.value.trim() })}
                  />
                </div>
                <div>
                  <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>显示名称</label>
                  <input
                    type="text"
                    value={displayName}
                    onChange={(e) => updateGroup(idx, { DisplayName: e.target.value, displayName: e.target.value })}
                  />
                </div>
                <div>
                  <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>节点标签 (Tag)</label>
                  <input
                    type="text"
                    value={tag}
                    onChange={(e) => updateGroup(idx, { Tag: e.target.value, tag: e.target.value })}
                  />
                </div>
                <div>
                  <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>超时 (ms)</label>
                  <input
                    type="number"
                    value={timeout}
                    onChange={(e) => updateGroup(idx, { Timeout: parseInt(e.target.value) || 3000 })}
                  />
                </div>
              </div>

              {/* 折叠式订阅源清单 */}
              <details style={{ border: '1px solid var(--border)', background: 'var(--surface-1)' }}>
                <summary style={{ padding: '8px 12px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center', userSelect: 'none' }}>
                  <span style={{ fontSize: '13px', color: 'var(--text-primary)' }}>
                    订阅源清单 ({urls.length} 条有效源)
                  </span>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>点击展开 / 收起</span>
                </summary>
                <div style={{ padding: '8px' }}>
                  <textarea
                    rows={4}
                    value={urlsText}
                    placeholder="每行一个订阅链接（支持 HTTP/HTTPS 订阅或子组聚合链接）"
                    onChange={(e) => {
                      const lines = e.target.value.split("\n").map(s => s.trim()).filter(Boolean);
                      updateGroup(idx, { URLs: lines, urls: lines });
                    }}
                  />
                </div>
              </details>

              {/* 直链复制与删除操作栏 */}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingTop: '10px', borderTop: '1px solid var(--border)', flexWrap: 'wrap', gap: '8px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                  <button onClick={() => copyLink(`${baseLink}/all.yaml${tokenQuery}`)}>
                    <Copy size={13} /> 复制 Clash
                  </button>
                  <button onClick={() => copyLink(`${baseLink}/mihomo.yaml${tokenQuery}`)}>
                    <Copy size={13} /> 复制 Mihomo
                  </button>
                  <button onClick={() => copyLink(`${baseLink}/base64.txt${tokenQuery}`)}>
                    <Copy size={13} /> 复制 Base64
                  </button>
                </div>
                <button onClick={() => deleteGroup(idx)} className="btn-danger" style={{ padding: '4px 8px' }}>
                  <Trash2 size={13} /> 删除分组
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
