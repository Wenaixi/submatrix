import React, { useState } from 'react';
import { GroupItem, saveGroups, toggleGroup } from '../api/client';
import { Switch } from '../components/Switch';
import { useToast } from '../components/Toast';
import { Copy, Plus, Trash2, Save, ChevronDown, ChevronRight, Sliders } from 'lucide-react';

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

  // 复制直链
  const copyLink = (url: string) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(url).then(() => showToast("订阅链接已复制"));
    } else {
      const inp = document.createElement("input");
      inp.value = url;
      document.body.appendChild(inp);
      inp.select();
      document.execCommand("copy");
      inp.remove();
      showToast("订阅链接已复制");
    }
  };

  // 单组属性局部更新
  const updateGroup = (idx: number, patch: Partial<GroupItem>) => {
    setGroups(prev => {
      const copy = [...prev];
      copy[idx] = { ...copy[idx], ...patch };
      return copy;
    });
  };

  // 单组主开关即时持久化
  const handleToggle = async (idx: number, checked: boolean) => {
    updateGroup(idx, { Enabled: checked, enabled: checked });
    const g = groups[idx];
    const name = g.Name || g.name || "";
    try {
      await toggleGroup(name, checked);
      showToast(`分组 [${name}] 状态已更新`);
    } catch (err: any) {
      showToast(`切换失败: ${err.message}`, true);
    }
  };

  // 批量启停
  const setAllEnabled = (enabled: boolean) => {
    setGroups(prev => prev.map(g => ({ ...g, Enabled: enabled, enabled })));
    showToast(enabled ? "所有分组已设为启用" : "所有分组已设为停用");
  };

  // 新增分组
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
        AppendOriginalName: true,
        TagIPType: true,
        MediaCheck: false,
        DropOnFail: true,
        MinSpeed: 0,
        Timeout: 3000,
        OverwriteURL: "",
        URLs: []
      }
    ]);
    showToast("已创建新分组模板，编辑后请保存");
  };

  // 删除分组
  const deleteGroup = (idx: number) => {
    const g = groups[idx];
    const name = g.Name || g.name || "";
    if (confirm(`确定删除分组 [${name}] 吗？`)) {
      setGroups(prev => prev.filter((_, i) => i !== idx));
      showToast("分组已移除，点击右上角保存即可写入配置");
    }
  };

  // 全量保存配置
  const handleSave = async () => {
    setSaving(true);
    try {
      await saveGroups(groups);
      showToast("配置已保存并立即生效");
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
      {/* 顶部操作条：文案全面人性化，彻底去除生冷口号与 AI 味 */}
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
            节点分组 (Groups)
          </h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            管理节点分组与测活改名规则，分流或聚合订阅。
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

      {/* 分组卡片列表 */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(480px, 1fr))',
        gap: '16px'
      }}>
        {groups.map((g, idx) => {
          // 健全回退解构 (修复审查 BUG-01)
          const name = g.Name ?? g.name ?? "未命名分组";
          const displayName = g.DisplayName ?? g.displayName ?? g.display_name ?? "";
          const tag = g.Tag ?? g.tag ?? "TAG";
          const isEnabled = (g.Enabled ?? g.enabled) !== false;

          // 基础 4 项开关
          const checkAlive = (g.CheckAlive ?? g.check_alive ?? g.checkAlive) !== false;
          const checkSpeed = (g.CheckSpeed ?? g.check_speed ?? g.checkSpeed) === true;
          const renameNode = (g.RenameNode ?? g.rename_node ?? g.renameNode) !== false;
          const dropOnFail = (g.DropOnFail ?? g.drop_on_fail ?? g.dropOnFail) !== false;

          // 高阶 3 项质检打标开关 (前作核心回归)
          const appendOriginalName = (g.AppendOriginalName ?? g.append_original_name ?? g.appendOriginalName) !== false;
          const tagIPType = (g.TagIPType ?? g.tag_ip_type ?? g.tagIPType) !== false;
          const mediaCheck = (g.MediaCheck ?? g.media_check ?? g.mediaCheck) === true;

          // 参数与门槛
          const timeout = g.Timeout ?? g.timeout ?? 3000;
          const minSpeed = g.MinSpeed ?? g.min_speed ?? g.minSpeed ?? 0;
          const overwriteURL = g.OverwriteURL ?? g.overwrite_url ?? g.overwriteUrl ?? "";

          // 订阅源
          const urls = g.URLs ?? g.urls ?? [];
          const urlsText = Array.isArray(urls) ? urls.join("\n") : "";
          const baseLink = `${protocol}//${host}${portStr}/group/${encodeURIComponent(name)}`;

          return (
            <div
              key={idx}
              style={{
                background: 'var(--surface-2)',
                border: '1px solid var(--border)',
                padding: '18px 20px',
                display: 'flex',
                flexDirection: 'column',
                gap: '14px',
                opacity: isEnabled ? 1 : 0.6,
                borderStyle: isEnabled ? 'solid' : 'dashed'
              }}
            >
              {/* 卡片头部：标识与主开关 */}
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

              {/* 阶梯 1：基础探测质检开关 (4 列平铺) */}
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
                  <Switch checked={checkAlive} onChange={(c) => updateGroup(idx, { CheckAlive: c, check_alive: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>带宽测速</span>
                  <Switch checked={checkSpeed} onChange={(c) => updateGroup(idx, { CheckSpeed: c, check_speed: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>智能改名</span>
                  <Switch checked={renameNode} onChange={(c) => updateGroup(idx, { RenameNode: c, rename_node: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>失败丢弃</span>
                  <Switch checked={dropOnFail} onChange={(c) => updateGroup(idx, { DropOnFail: c, drop_on_fail: c })} />
                </div>
              </div>

              {/* 阶梯 2：高阶质检与打标开关 (3 列平铺，包含原名保留与客观画像) */}
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(3, 1fr)',
                gap: '8px',
                padding: '10px 12px',
                background: 'var(--surface-1)',
                border: '1px solid var(--border)'
              }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }} title="在重命名后的节点末尾追加 (原名: xxx)">保留原名末尾</span>
                  <Switch checked={appendOriginalName} onChange={(c) => updateGroup(idx, { AppendOriginalName: c, append_original_name: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }} title="标记 |家宽、|机房 或 |移动">IP画像打标</span>
                  <Switch checked={tagIPType} onChange={(c) => updateGroup(idx, { TagIPType: c, tag_ip_type: c })} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }} title="检测 Netflix/GPT/Gemini/YouTube 解锁">流媒体AI检测</span>
                  <Switch checked={mediaCheck} onChange={(c) => updateGroup(idx, { MediaCheck: c, media_check: c })} />
                </div>
              </div>

              {/* 阶梯 3：核心标识与超时参数 (4 列紧凑输入) */}
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
                    onChange={(e) => updateGroup(idx, { DisplayName: e.target.value, displayName: e.target.value, display_name: e.target.value })}
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
                    onChange={(e) => updateGroup(idx, { Timeout: parseInt(e.target.value) || 3000, timeout: parseInt(e.target.value) || 3000 })}
                  />
                </div>
              </div>

              {/* 阶梯 4：高级过滤与重写折叠区 */}
              <details style={{ border: '1px solid var(--border)', background: 'var(--surface-1)' }}>
                <summary style={{ padding: '8px 12px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center', userSelect: 'none' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    <Sliders size={13} color="var(--text-secondary)" />
                    <span style={{ fontSize: '13px', color: 'var(--text-primary)' }}>高级过滤与链接重写</span>
                  </div>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>点击展开 / 收起</span>
                </summary>
                <div style={{ padding: '12px', display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '12px', background: 'var(--surface-0)' }}>
                  <div>
                    <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>最低速度限制 (KB/s)</label>
                    <input
                      type="number"
                      placeholder="0 表示不过滤"
                      value={minSpeed || ''}
                      onChange={(e) => updateGroup(idx, { MinSpeed: parseInt(e.target.value) || 0, min_speed: parseInt(e.target.value) || 0 })}
                    />
                  </div>
                  <div>
                    <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>外部重写链接 (Overwrite URL)</label>
                    <input
                      type="text"
                      placeholder="留空默认使用内部聚合直链"
                      value={overwriteURL}
                      onChange={(e) => updateGroup(idx, { OverwriteURL: e.target.value.trim(), overwrite_url: e.target.value.trim() })}
                    />
                  </div>
                </div>
              </details>

              {/* 阶梯 5：折叠式订阅源清单 */}
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
                    placeholder="每行一个订阅链接（支持外部 HTTP 订阅或内部子组聚合链接）"
                    onChange={(e) => {
                      const lines = e.target.value.split("\n").map(s => s.trim()).filter(Boolean);
                      updateGroup(idx, { URLs: lines, urls: lines });
                    }}
                  />
                </div>
              </details>

              {/* 底部直链复制与删除操作栏 */}
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
