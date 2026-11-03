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
        AppendOriginalName: false,
        TagIPType: true,
        MediaCheck: false,
        DropOnFail: true,
        MinSpeed: 0,
        Timeout: 3000,
        OverwriteURL: "",
        RulePreset: "pro_full",
        FilterProtocols: ["http"],
        Proxy: "",
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
          const appendOriginalName = Boolean(g.AppendOriginalName ?? g.append_original_name ?? g.appendOriginalName ?? false);
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

              {/* 一级总控：连通测活总控横幅 (1A 独立指挥条) */}
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '12px 14px',
                background: checkAlive ? 'var(--surface-1)' : 'var(--surface-0)',
                border: checkAlive ? '1px solid var(--border-black)' : '1px dashed var(--border)',
                transition: 'all 0.2s ease'
              }}>
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-primary)' }}>
                      连通测活总控 (Check Alive)
                    </span>
                    <span className="badge" style={{ fontSize: '10px' }}>
                      {checkAlive ? '已激活全握手质检' : '纯免检直通模式'}
                    </span>
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '3px' }}>
                    {checkAlive
                      ? '开启后将对所有节点发起真实 TCP/TLS 握手探测与可用性甄别。'
                      : '关闭后进入零 I/O 纯免检直通模式，节点全量放行，测速与画像自动收起。'}
                  </div>
                </div>
                <Switch
                  checked={checkAlive}
                  onChange={(c) => updateGroup(idx, { CheckAlive: c, check_alive: c })}
                />
              </div>

              {/* 阶梯 1 & 2：智能探测与质检开关（测活关闭时彻底收起 2A/3A） */}
              {checkAlive ? (
                <>
                  {/* 基础探测开关 */}
                  <div style={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(3, 1fr)',
                    gap: '8px',
                    padding: '10px 12px',
                    background: 'var(--surface-1)',
                    border: '1px solid var(--border)'
                  }}>
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

                  {/* 高阶打标与画像开关 (含智能改名伴生的保留原名 3A) */}
                  <div style={{
                    display: 'grid',
                    gridTemplateColumns: renameNode ? 'repeat(3, 1fr)' : 'repeat(2, 1fr)',
                    gap: '8px',
                    padding: '10px 12px',
                    background: 'var(--surface-1)',
                    border: '1px solid var(--border)'
                  }}>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                      <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }} title="标记 |家宽、|机房 或 |移动">IP画像打标</span>
                      <Switch checked={tagIPType} onChange={(c) => updateGroup(idx, { TagIPType: c, tag_ip_type: c })} />
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                      <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }} title="检测 Netflix/GPT/Gemini/YouTube 解锁">流媒体AI检测</span>
                      <Switch checked={mediaCheck} onChange={(c) => updateGroup(idx, { MediaCheck: c, media_check: c })} />
                    </div>
                    {renameNode && (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                        <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }} title="在重命名后的节点末尾追加 (原名: xxx)">保留原名末尾</span>
                        <Switch checked={appendOriginalName} onChange={(c) => updateGroup(idx, { AppendOriginalName: c, append_original_name: c })} />
                      </div>
                    )}
                  </div>
                </>
              ) : (
                /* 测活关闭：除智能改名外全部收起 (2A & 3A) */
                <div style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '10px 12px',
                  background: 'var(--surface-1)',
                  border: '1px solid var(--border)'
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span style={{ fontSize: '12px', color: 'var(--text-primary)' }}>智能改名</span>
                      <Switch checked={renameNode} onChange={(c) => updateGroup(idx, { RenameNode: c, rename_node: c })} />
                    </div>
                    {renameNode && (
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderLeft: '1px solid var(--border)', paddingLeft: '16px' }}>
                        <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>保留原名末尾</span>
                        <Switch checked={appendOriginalName} onChange={(c) => updateGroup(idx, { AppendOriginalName: c, append_original_name: c })} />
                      </div>
                    )}
                  </div>
                  <span style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>
                    免检模式仅提供文本正则智能改名，其余探测与丢弃功能已自动收起
                  </span>
                </div>
              )}

              {/* 核心名称与超时参数 (5A 智能联动简化) */}
              <div style={{ display: 'grid', gridTemplateColumns: checkAlive ? '3fr 1fr' : '1fr', gap: '10px' }}>
                <div>
                  <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                    分组名称 (Display Name)
                  </label>
                  <input
                    type="text"
                    placeholder="例如：公网免费专选、亚太自建"
                    value={displayName}
                    onChange={(e) => {
                      const val = e.target.value;
                      const patch: Partial<GroupItem> = {
                        DisplayName: val, displayName: val, display_name: val
                      };
                      if (!name || name.startsWith('group-') || name === 'default') {
                        const slug = val.toLowerCase().replace(/[^a-z0-9_-]/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '');
                        if (slug) { patch.Name = slug; patch.name = slug; }
                      }
                      updateGroup(idx, patch);
                    }}
                  />
                </div>
                {checkAlive && (
                  <div>
                    <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                      测活超时 (ms)
                    </label>
                    <input
                      type="number"
                      value={timeout}
                      onChange={(e) => updateGroup(idx, { Timeout: parseInt(e.target.value) || 3000, timeout: parseInt(e.target.value) || 3000 })}
                    />
                  </div>
                )}
              </div>

              {/* 阶梯 4：高级过滤、分流规则模板与单组参数 (折叠区) */}
              <details style={{ border: '1px solid var(--border)', background: 'var(--surface-1)' }}>
                <summary style={{ padding: '8px 12px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center', userSelect: 'none' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    <Sliders size={13} color="var(--text-secondary)" />
                    <span style={{ fontSize: '13px', color: 'var(--text-primary)' }}>高级过滤、分流规则模板与单组参数</span>
                  </div>
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>点击展开 / 收起</span>
                </summary>
                <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '14px', background: 'var(--surface-0)' }}>
                  {/* 第 1 模块：规则集预设选择器 */}
                  <div>
                    <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '6px' }}>
                      分流规则集模板 (Rule Preset，客户端导入订阅时生效)
                    </label>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                      {[
                        { id: 'pro_full', name: '最详分流矩阵', desc: 'PRO全功能 · 126策略组 + 流媒体 + AI全分流 (默认推荐)' },
                        { id: 'pro_lite', name: '精简分流矩阵', desc: 'PRO-Lite 核心轻量化策略组与分流' },
                        { id: 'acl4ssr', name: 'ACL4SSR 经典', desc: '全量在线 ACL4SSR 经典规则集' },
                        { id: 'direct_pure', name: '纯节点直通', desc: '仅纯节点列表，不注入任何策略组与分流' },
                        { id: 'custom', name: '自定义外部直链', desc: '指定外部远程模板或本地规则文件' },
                      ].map((preset) => {
                        const cur = g.RulePreset || g.rule_preset || 'pro_full';
                        const isSel = cur === preset.id;
                        return (
                          <button
                            key={preset.id}
                            type="button"
                            onClick={() => updateGroup(idx, { RulePreset: preset.id, rule_preset: preset.id })}
                            style={{
                              padding: '4px 10px',
                              fontSize: '12px',
                              cursor: 'pointer',
                              fontFamily: 'var(--font-mono)',
                              border: isSel ? '1px solid var(--border-black)' : '1px solid var(--border)',
                              background: isSel ? 'var(--btn-primary-bg)' : 'var(--surface-1)',
                              color: isSel ? 'var(--btn-primary-fg)' : 'var(--text-secondary)',
                              fontWeight: isSel ? 500 : 400
                            }}
                            title={preset.desc}
                          >
                            {preset.name} {isSel ? '✓' : ''}
                          </button>
                        );
                      })}
                    </div>
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      {((cur) => {
                        switch (cur) {
                          case 'pro_lite': return 'PRO-Lite 核心轻量化策略组，内存占用更低';
                          case 'acl4ssr': return '全量在线 ACL4SSR 经典规则集';
                          case 'direct_pure': return '纯节点列表模式，不注入任何策略组与规则';
                          case 'custom': return '自定义外部/本地直链，需在下方填写直链地址';
                          default: return '包含 126 个策略组、流媒体与 AI 全分流的主力推荐规则库';
                        }
                      })(g.RulePreset || g.rule_preset || 'pro_full')}
                    </div>
                  </div>

                  {/* 如果选择自定义模板，展开重写链接输入框 */}
                  {(g.RulePreset === 'custom' || g.rule_preset === 'custom') && (
                    <div>
                      <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                        自定义规则模板链接 (Overwrite URL)
                      </label>
                      <input
                        type="text"
                        placeholder="https://raw.githubusercontent.com/.../clash.yaml"
                        value={overwriteURL}
                        onChange={(e) => updateGroup(idx, { OverwriteURL: e.target.value.trim(), overwrite_url: e.target.value.trim() })}
                      />
                    </div>
                  )}

                  {/* 测速专属微调参数 (8A)：仅当测活与测速都开启时展现 */}
                  {checkAlive && checkSpeed && (
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                      <div>
                        <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                          最低下载速度门槛 (KB/s，低于此速率的节点自动淘汰)
                        </label>
                        <input
                          type="number"
                          placeholder="0 表示不过滤速率"
                          value={minSpeed || ''}
                          onChange={(e) => updateGroup(idx, { MinSpeed: parseInt(e.target.value) || 0, min_speed: parseInt(e.target.value) || 0 })}
                        />
                      </div>
                      <div>
                        <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                          单节点测速流量上限 (MB，8A 独立控制，防止跑爆流量)
                        </label>
                        <input
                          type="number"
                          placeholder="默认 20 MB"
                          value={g.DownloadMB || g.download_mb || ''}
                          onChange={(e) => updateGroup(idx, { DownloadMB: parseInt(e.target.value) || 0, download_mb: parseInt(e.target.value) || 0 })}
                        />
                      </div>
                    </div>
                  )}

                  {/* 6A 节点保留数量上限 (SuccessLimit，默认不限) */}
                  <div>
                    <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                      单组最大保留节点数量 (Success Limit，留空或 0 表示不限)
                    </label>
                    <input
                      type="number"
                      placeholder="0 表示保留所有有效节点，建议填写 50~200 防止客户端卡顿"
                      value={g.SuccessLimit || g.success_limit || ''}
                      onChange={(e) => updateGroup(idx, { SuccessLimit: parseInt(e.target.value) || 0, success_limit: parseInt(e.target.value) || 0 })}
                    />
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '2px' }}>
                      测活和排序完成后，将自动截取前 N 个最优质节点导出，避免订阅体积过大。
                    </div>
                  </div>

                  {/* 5A 底层机器标识与节点标签微调 */}
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                    <div>
                      <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                        组底层 URL 标识 (Name，用于目录名与 /group/xxx/ 订阅路径)
                      </label>
                      <input
                        type="text"
                        placeholder="all-in-one / free-a"
                        value={name}
                        onChange={(e) => updateGroup(idx, { Name: e.target.value.trim(), name: e.target.value.trim() })}
                      />
                    </div>
                    <div>
                      <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                        节点后缀标签 (Tag，改名后缀如 🇭🇰HK_01|Tag)
                      </label>
                      <input
                        type="text"
                        placeholder="建议 2~4 个汉字"
                        value={tag}
                        onChange={(e) => updateGroup(idx, { Tag: e.target.value.trim(), tag: e.target.value.trim() })}
                      />
                    </div>
                  </div>

                  {/* 组专用前置代理 */}
                  <div>
                    <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '3px' }}>
                      组专用前置代理 (Group Proxy，选填)
                    </label>
                    <input
                      type="text"
                      placeholder="http://127.0.0.1:10808 或 socks5://127.0.0.1:10808"
                      value={g.Proxy || g.proxy || ''}
                      onChange={(e) => updateGroup(idx, { Proxy: e.target.value.trim(), proxy: e.target.value.trim() })}
                    />
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '3px' }}>
                      配置后，此组节点的所有连通测活、带宽测速与订阅拉取流量均穿透该代理建立连接。
                    </div>
                  </div>

                  {/* 协议过滤配置（默认过滤 HTTP） */}
                  <div>
                    <label style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'block', marginBottom: '6px' }}>
                      协议过滤配置 (Protocol Filter，点击可切换排除状态)
                    </label>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                      {['http', 'socks5', 'vless', 'ss', 'anytls', 'trojan', 'ssr', 'hysteria2', 'tuic'].map((proto) => {
                        const filterList = g.FilterProtocols || g.filter_protocols || ['http'];
                        const isFiltered = filterList.map(p => p.toLowerCase()).includes(proto.toLowerCase());
                        return (
                          <button
                            key={proto}
                            type="button"
                            onClick={() => {
                              const cur = filterList.map(p => p.toLowerCase());
                              const next = isFiltered
                                ? cur.filter(p => p !== proto.toLowerCase())
                                : [...cur, proto.toLowerCase()];
                              updateGroup(idx, { FilterProtocols: next, filter_protocols: next });
                            }}
                            style={{
                              padding: '3px 8px',
                              fontSize: '11px',
                              cursor: 'pointer',
                              fontFamily: 'var(--font-mono)',
                              border: isFiltered ? '1px solid var(--border-black)' : '1px solid var(--border)',
                              background: isFiltered ? 'var(--btn-primary-bg)' : 'var(--surface-1)',
                              color: isFiltered ? 'var(--btn-primary-fg)' : 'var(--text-secondary)',
                              fontWeight: isFiltered ? 500 : 400
                            }}
                            title={isFiltered ? `已排除 ${proto.toUpperCase()} 协议节点` : `点击排除 ${proto.toUpperCase()} 协议节点`}
                          >
                            {proto.toUpperCase()} {isFiltered ? '(已过滤)' : ''}
                          </button>
                        );
                      })}
                    </div>
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      已过滤协议的节点在拉取时会被直接丢弃（默认排除明文 HTTP 废节点），绝不进入后续测活与订阅导出。
                    </div>
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
