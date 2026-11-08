import React, { useState, useEffect, useCallback } from 'react';
import { ShareItem, ShareIPHit, getShares, createShare, toggleShare, deleteShare, getShareStats } from '../api/client';
import { useToast } from '../components/Toast';
import { Plus, Trash2, Power, RefreshCw, Link2, ChevronDown, ChevronRight, Eye } from 'lucide-react';

interface SharesViewProps {
  groups: { Name?: string; name?: string }[];
}

export const SharesView: React.FC<SharesViewProps> = ({ groups }) => {
  const { showToast } = useToast();
  const [shares, setShares] = useState<ShareItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [statsDetail, setStatsDetail] = useState<Record<string, ShareIPHit[]>>({});

  // 创建表单
  const [newGroup, setNewGroup] = useState('');
  const [newNote, setNewNote] = useState('');
  const [newExpires, setNewExpires] = useState(''); // 空 = 永不过期

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getShares();
      setShares(res.shares || []);
    } catch (e: any) {
      showToast(`加载分享失败: ${e.message}`, true);
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    if (!newGroup) { showToast('请选择要分享的组', true); return; }
    try {
      await createShare({
        group: newGroup,
        note: newNote || undefined,
        expires_at: newExpires || undefined,
      });
      showToast('分享已创建');
      setShowCreate(false);
      setNewGroup(''); setNewNote(''); setNewExpires('');
      load();
    } catch (e: any) {
      showToast(`创建失败: ${e.message}`, true);
    }
  };

  const handleToggle = async (s: ShareItem) => {
    try {
      await toggleShare(s.id, !s.enabled);
      showToast(s.enabled ? `分享 ${s.id} 已停用` : `分享 ${s.id} 已启用`);
      load();
    } catch (e: any) {
      showToast(`操作失败: ${e.message}`, true);
    }
  };

  const handleDelete = async (s: ShareItem) => {
    if (!confirm(`确定删除分享 [${s.id}]（指向 ${s.group}）？删除后链接立即失效，统计保留。`)) return;
    try {
      await deleteShare(s.id);
      showToast('分享已删除');
      load();
    } catch (e: any) {
      showToast(`删除失败: ${e.message}`, true);
    }
  };

  const handleExpand = async (s: ShareItem) => {
    if (expanded === s.id) { setExpanded(null); return; }
    setExpanded(s.id);
    try {
      const st = await getShareStats(s.id);
      setStatsDetail(prev => ({ ...prev, [s.id]: st.ip_history || [] }));
    } catch (e: any) {
      setStatsDetail(prev => ({ ...prev, [s.id]: [] }));
    }
  };

  const copy = (text: string) => {
    navigator.clipboard.writeText(text).then(() => showToast('链接已复制'));
  };

  const fmtTime = (t: string) => t ? new Date(t).toLocaleString() : '—';
  const fmtExpiry = (s: ShareItem) => s.expires_at && s.expires_at !== "0001-01-01T00:00:00Z" ? fmtTime(s.expires_at) : '永不过期';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 顶部条 */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '16px 20px', background: 'var(--surface-2)', border: '1px solid var(--border)', flexWrap: 'wrap', gap: '12px'
      }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 500 }}>分享管理</h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            按组分发订阅，每个分享独立开关、可删除、记录拉取 IP 与次数。分享码请妥善保管。
          </p>
        </div>
        <div style={{ display: 'flex', gap: '8px' }}>
          <button onClick={load} className="btn-ghost" disabled={loading}>
            <RefreshCw size={13} className={loading ? "animate-spin" : ""} /> 刷新
          </button>
          <button onClick={() => setShowCreate(v => !v)} className="btn-primary">
            <Plus size={14} /> 新建分享
          </button>
        </div>
      </div>

      {/* 创建表单 */}
      {showCreate && (
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '18px 20px' }}>
          <div style={{ fontSize: '14px', fontWeight: 500, marginBottom: '12px' }}>新建分享</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '16px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label style={{ fontSize: '13px' }}>分享的组</label>
              <select
                value={newGroup}
                onChange={e => setNewGroup(e.target.value)}
                style={{ padding: '7px 10px', border: '1px solid var(--border)', background: 'var(--surface-1)', color: 'var(--text-primary)' }}
              >
                <option value="">选择组...</option>
                {(groups || []).map(g => (
                  <option key={g.Name ?? g.name} value={g.Name ?? g.name}>{g.Name ?? g.name}</option>
                ))}
              </select>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label style={{ fontSize: '13px' }}>备注（给谁）</label>
              <input
                value={newNote}
                onChange={e => setNewNote(e.target.value)}
                placeholder="例如：给朋友的免费节点"
                style={{ padding: '7px 10px', border: '1px solid var(--border)', background: 'var(--surface-1)', color: 'var(--text-primary)' }}
              />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label style={{ fontSize: '13px' }}>过期时间（留空 = 永不过期）</label>
              <input
                type="datetime-local"
                value={newExpires}
                onChange={e => setNewExpires(e.target.value)}
                style={{ padding: '7px 10px', border: '1px solid var(--border)', background: 'var(--surface-1)', color: 'var(--text-primary)' }}
              />
            </div>
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: '8px' }}>
              <button onClick={handleCreate} className="btn-primary">创建</button>
              <button onClick={() => setShowCreate(false)} className="btn-ghost">取消</button>
            </div>
          </div>
        </div>
      )}

      {/* 分享列表 */}
      {shares.length === 0 && !loading ? (
        <div style={{ padding: '40px', textAlign: 'center', color: 'var(--text-secondary)', border: '1px dashed var(--border)' }}>
          暂无分享。点击「新建分享」开始分发。
        </div>
      ) : (
        shares.map(s => (
          <div key={s.id} style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
            {/* 头部 */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, fontSize: '15px' }}>{s.id}</span>
                <span className="badge">{s.enabled ? '已启用' : '已停用'}</span>
                <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>组: {s.group}</span>
                {s.note && <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>备注: {s.note}</span>}
              </div>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
                  拉取 {s.total_hits} 次 / 唯一 IP {s.unique_ips}
                </span>
                <button onClick={() => copy(s.link)} className="btn-ghost" title="复制分享链接">
                  <Link2 size={13} /> 复制链接
                </button>
                <button onClick={() => handleExpand(s)} className="btn-ghost" title="查看统计">
                  <Eye size={13} /> {expanded === s.id ? '收起' : '统计'}
                </button>
                <button onClick={() => handleToggle(s)} className="btn-ghost" title={s.enabled ? '停用' : '启用'}>
                  <Power size={13} /> {s.enabled ? '停用' : '启用'}
                </button>
                <button onClick={() => handleDelete(s)} className="btn-ghost" style={{ color: 'var(--danger, #dc2626)' }} title="删除分享">
                  <Trash2 size={13} />
                </button>
              </div>
            </div>

            {/* 详情行 */}
            <div style={{ display: 'flex', gap: '20px', flexWrap: 'wrap', marginTop: '8px', fontSize: '12px', color: 'var(--text-secondary)' }}>
              <span>创建: {fmtTime(s.created_at)}</span>
              <span>过期: {fmtExpiry(s)}</span>
              <span>最近命中: {fmtTime(s.last_hit)}</span>
            </div>

            {/* 展开：IP 历史 */}
            {expanded === s.id && (
              <div style={{ marginTop: '12px', borderTop: '1px solid var(--border)', paddingTop: '12px' }}>
                <div style={{ fontSize: '13px', marginBottom: '8px' }}>拉取历史（IP 已脱敏）</div>
                {(statsDetail[s.id] || []).length === 0 ? (
                  <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>暂无拉取记录</span>
                ) : (
                  <table style={{ width: '100%', fontSize: '12px', borderCollapse: 'collapse' }}>
                    <thead>
                      <tr style={{ textAlign: 'left', color: 'var(--text-secondary)' }}>
                        <th style={{ padding: '4px 8px', borderBottom: '1px solid var(--border)' }}>时间</th>
                        <th style={{ padding: '4px 8px', borderBottom: '1px solid var(--border)' }}>IP</th>
                        <th style={{ padding: '4px 8px', borderBottom: '1px solid var(--border)' }}>文件</th>
                        <th style={{ padding: '4px 8px', borderBottom: '1px solid var(--border)' }}>UA</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(statsDetail[s.id] || []).slice().reverse().map((h, i) => (
                        <tr key={i}>
                          <td style={{ padding: '4px 8px' }}>{fmtTime(h.time)}</td>
                          <td style={{ padding: '4px 8px', fontFamily: 'var(--font-mono)' }}>{h.ip}</td>
                          <td style={{ padding: '4px 8px', fontFamily: 'var(--font-mono)' }}>{h.file}</td>
                          <td style={{ padding: '4px 8px', maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{h.ua || '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            )}
          </div>
        ))
      )}
    </div>
  );
};