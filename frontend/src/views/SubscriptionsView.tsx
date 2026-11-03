import React, { useState } from 'react';
import { Copy, Eye, X, Smartphone, Monitor } from 'lucide-react';
import { useToast } from '../components/Toast';

interface SubscriptionsViewProps {
  listenPort: string;
  apiKey: string;
}

export const SubscriptionsView: React.FC<SubscriptionsViewProps> = ({ listenPort, apiKey }) => {
  const { showToast } = useToast();
  const [previewContent, setPreviewContent] = useState<{ title: string; text: string } | null>(null);
  const [selectedHostMode, setSelectedHostMode] = useState<'local' | 'lan'>('local');

  const protocol = location.protocol;
  const portStr = listenPort ? (listenPort.startsWith(":") ? listenPort : ":" + listenPort) : (location.port ? ":" + location.port : "");
  const tokenQuery = apiKey ? `?token=${encodeURIComponent(apiKey)}` : "";

  // 宿主机局域网真实 IP（支持手机、小火箭、同一 Wi-Fi 下设备访问）
  const lanHost = "192.168.1.141";
  const activeHost = selectedHostMode === 'lan' ? lanHost : location.hostname;

  const subItems = [
    {
      name: "Clash / Verge / Mihomo 全能完整订阅",
      client: "Clash Verge / Mihomo / CFW / FlClash 通用",
      path: "/mihomo.yaml",
      desc: "已自动配置好 126 个精细策略组与全量分流规则，客户端直接导入即用（强烈推荐）",
      highlight: true
    },
    {
      name: "通用 Base64 订阅",
      client: "小火箭 / v2rayN / Shadowrocket / QuantumultX",
      path: "/base64.txt",
      desc: "标准 Base64 格式，适合移动端小火箭、v2rayN 一键订阅，无 404 故障",
      highlight: false
    },
    {
      name: "纯节点池 (Proxy Provider 专用)",
      client: "供第三方配置文件外部引用",
      path: "/all.yaml",
      desc: "仅包含纯 proxies 节点列表，适合高端玩家放在自己配置的 proxy-providers 节点池中",
      highlight: false
    },
    {
      name: "历史归档可用池",
      client: "全客户端可用",
      path: "/history.yaml",
      desc: "保留最近多次探测均健康的累计优质节点",
      highlight: false
    }
  ];

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
            已为您自动配置好全量分流策略组与规则，支持直接导入客户端。
          </p>
        </div>

        {/* 客户端使用场景切换：本机回路 vs 手机局域网 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>使用设备：</span>
          <button
            type="button"
            onClick={() => setSelectedHostMode('local')}
            style={{
              padding: '5px 10px',
              fontSize: '12px',
              cursor: 'pointer',
              border: selectedHostMode === 'local' ? '1px solid var(--border-black)' : '1px solid var(--border)',
              background: selectedHostMode === 'local' ? 'var(--btn-primary-bg)' : 'var(--surface-1)',
              color: selectedHostMode === 'local' ? 'var(--btn-primary-fg)' : 'var(--text-secondary)',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '4px'
            }}
          >
            <Monitor size={13} /> 本机电脑 (127.0.0.1)
          </button>
          <button
            type="button"
            onClick={() => setSelectedHostMode('lan')}
            style={{
              padding: '5px 10px',
              fontSize: '12px',
              cursor: 'pointer',
              border: selectedHostMode === 'lan' ? '1px solid var(--border-black)' : '1px solid var(--border)',
              background: selectedHostMode === 'lan' ? 'var(--btn-primary-bg)' : 'var(--surface-1)',
              color: selectedHostMode === 'lan' ? 'var(--btn-primary-fg)' : 'var(--text-secondary)',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '4px'
            }}
          >
            <Smartphone size={13} /> 手机/外部设备 ({lanHost})
          </button>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(440px, 1fr))', gap: '16px' }}>
        {subItems.map((item, i) => {
          const fullUrl = `${protocol}//${activeHost}${portStr}${item.path}${tokenQuery}`;
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
