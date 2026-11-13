import React, { useState } from 'react';
import { Copy, Eye, X } from 'lucide-react';
import { useToast } from '../components/Toast';
import { GroupItem } from '../api/client';

interface SubscriptionsViewProps {
  listenPort: string;
  apiKey: string;
  groups: GroupItem[];
}

export const SubscriptionsView: React.FC<SubscriptionsViewProps> = ({ listenPort, apiKey, groups }) => {
  const { showToast } = useToast();
  const [previewContent, setPreviewContent] = useState<{ title: string; text: string } | null>(null);
  // 纯动态自适应当前访问源，无任何硬编码
  const [customOrigin, setCustomOrigin] = useState<string>(() => {
    return localStorage.getItem('submatrix_custom_origin') || '';
  });

  const tokenQuery = apiKey ? `?token=${encodeURIComponent(apiKey)}` : "";

  // 基础源计算：若用户填写了自定义外部域名则使用之，否则自动采用当前浏览器打开的真实源 location.origin
  const getBaseOrigin = () => {
    if (customOrigin.trim()) {
      return customOrigin.trim().replace(/\/+$/, '');
    }
    return location.origin;
  };

  // 按组生成直链：产物统一落在各分组自己的目录下，旧的全局产物路径已随架构调整为按组独立检测而删除
  const enabledGroups = groups.filter(g => (g.Enabled ?? g.enabled) !== false);

  const subItems = enabledGroups.flatMap((g) => {
    const name = g.Name ?? g.name ?? "";
    if (!name) return [];
    const base = `/group/${encodeURIComponent(name)}`;
    const label = g.DisplayName ?? g.displayName ?? g.display_name ?? name;
    return [
      {
        name: `${label} · 完整配置`,
        client: "Clash Verge / Mihomo / CFW",
        path: `${base}/mihomo.yaml`,
        desc: "策略组与分流规则齐全，客户端直接导入即用（推荐）",
        highlight: true
      },
      {
        name: `${label} · 纯节点池`,
        client: "Proxy Provider 外部引用",
        path: `${base}/all.yaml`,
        desc: "仅含 proxies 节点列表，适合放进自建配置的 proxy-providers 中",
        highlight: false
      }
    ];
  });

  const copy = (url: string) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(url).then(() => showToast("订阅地址已复制到剪贴板"));
    } else {
      const inp = document.createElement("input");
      inp.value = url;
      document.body.appendChild(inp);
      inp.select();
      document.execCommand("copy");
      inp.remove();
      showToast("订阅地址已复制到剪贴板");
    }
  };

  const handlePreview = async (name: string, url: string) => {
    setPreviewContent({ title: name, text: "正在加载订阅内容..." });
    try {
      const res = await fetch(url);
      const text = await res.text();
      setPreviewContent({ title: name, text });
    } catch (e: any) {
      setPreviewContent({ title: name, text: `预览加载失败: ${e.message}` });
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 顶部标题与网络地址切换 */}
      <div style={{
        padding: '16px 20px',
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        gap: '12px'
      }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 500, color: 'var(--text-primary)' }}>
            订阅链接
          </h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            每个启用的分组各自生成独立订阅直链，按需选择后导入客户端。
          </p>
        </div>

        {/* 动态域名与自定义外部源 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>导出域名源：</span>
            <input
              type="text"
              placeholder={`默认跟随当前网址 (${location.origin})`}
              value={customOrigin}
              onChange={(e) => {
                const val = e.target.value;
                setCustomOrigin(val);
                localStorage.setItem('submatrix_custom_origin', val);
              }}
              style={{
                padding: '4px 8px',
                fontSize: '12px',
                width: '240px',
                fontFamily: 'var(--font-mono)'
              }}
            />
            {customOrigin && (
              <button
                type="button"
                onClick={() => {
                  setCustomOrigin('');
                  localStorage.removeItem('submatrix_custom_origin');
                  showToast('已恢复自动跟随当前网页网址');
                }}
                className="btn-ghost"
                style={{ padding: '4px 6px', fontSize: '11px' }}
              >
                重置
              </button>
            )}
          </div>
          <span className="badge" style={{ fontSize: '11px' }}>
            {customOrigin ? '自定义域名' : '当前自适应: ' + location.origin}
          </span>
        </div>
      </div>

      {subItems.length === 0 ? (
        <div style={{
          padding: '24px 20px',
          background: 'var(--surface-2)',
          border: '1px solid var(--border)',
          fontSize: '13px',
          color: 'var(--text-secondary)'
        }}>
          暂时没有可用的分组。请先在「节点分组」页新增并启用分组，保存配置后这里会自动出现对应的订阅直链。
        </div>
      ) : (
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(440px, 1fr))', gap: '16px' }}>
        {subItems.map((item, i) => {
          const fullUrl = `${getBaseOrigin()}${item.path}${tokenQuery}`;
          return (
            <div key={i} style={{
              background: 'var(--surface-2)',
              border: item.highlight ? '1px solid var(--border-black)' : '1px solid var(--border)',
              padding: '18px 20px',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
              gap: '12px'
            }}>
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <h3 style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text-primary)' }}>{item.name}</h3>
                    {item.highlight && <span className="badge" style={{ fontSize: '10px' }}>推荐</span>}
                  </div>
                  <span className="badge">{item.client}</span>
                </div>
                <p style={{ fontSize: '12px', color: 'var(--text-secondary)', marginTop: '4px' }}>{item.desc}</p>
                <div style={{
                  marginTop: '10px',
                  padding: '8px 10px',
                  background: 'var(--surface-1)',
                  border: '1px solid var(--border)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '12px',
                  color: 'var(--text-primary)',
                  wordBreak: 'break-all'
                }}>
                  {fullUrl}
                </div>
              </div>

              <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
                <button onClick={() => copy(fullUrl)} className={item.highlight ? "btn-primary" : "btn-secondary"} style={{ flex: 1 }}>
                  <Copy size={13} /> 复制直链
                </button>
                <button onClick={() => handlePreview(item.name, fullUrl)} className="btn-ghost">
                  <Eye size={13} /> 预览
                </button>
              </div>
            </div>
          );
        })}
      </div>
      )}

      {previewContent && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, width: '100vw', height: '100vh',
          background: 'rgba(0,0,0,0.4)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 100000,
          padding: '24px'
        }}>
          <div style={{
            width: '100%',
            maxWidth: '900px',
            maxHeight: '85vh',
            background: 'var(--surface-0)',
            border: '1px solid var(--border-black)',
            display: 'flex',
            flexDirection: 'column'
          }}>
            <div style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '12px 16px',
              borderBottom: '1px solid var(--border)',
              background: 'var(--surface-1)'
            }}>
              <span style={{ fontSize: '14px', fontWeight: 500, fontFamily: 'var(--font-mono)' }}>{previewContent.title}</span>
              <button onClick={() => setPreviewContent(null)} className="btn-ghost" style={{ padding: '4px' }}>
                <X size={16} />
              </button>
            </div>
            <pre style={{
              flex: 1,
              overflowY: 'auto',
              padding: '16px',
              fontFamily: 'var(--font-mono)',
              fontSize: '12px',
              lineHeight: '1.6',
              background: 'var(--surface-1)',
              color: 'var(--text-primary)',
              margin: 0
            }}>
              {previewContent.text}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};
