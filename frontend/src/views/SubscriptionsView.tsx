import React from 'react';
import { Copy, ExternalLink, Check } from 'lucide-react';
import { useToast } from '../components/Toast';

interface SubscriptionsViewProps {
  listenPort: string;
  apiKey: string;
}

export const SubscriptionsView: React.FC<SubscriptionsViewProps> = ({ listenPort, apiKey }) => {
  const { showToast } = useToast();
  const host = location.hostname;
  const protocol = location.protocol;
  const portStr = listenPort ? (listenPort.startsWith(":") ? listenPort : ":" + listenPort) : (location.port ? ":" + location.port : "");
  const tokenQuery = apiKey ? `?token=${encodeURIComponent(apiKey)}` : "";

  const subItems = [
    {
      name: "Clash 全量聚合订阅",
      client: "Clash / Clash Verge / Clash Meta",
      path: "/all.yaml",
      desc: "包含所有经过质检或透传聚合的完整节点配置"
    },
    {
      name: "Mihomo 专享订阅",
      client: "Mihomo Party / Clash.Meta",
      path: "/mihomo.yaml",
      desc: "适配最新 Meta 内核特性的专用 YAML 配置"
    },
    {
      name: "Base64 通用订阅",
      client: "v2rayN / Shadowrocket / QuantumultX",
      path: "/base64.yaml",
      desc: "纯标准 Base64 格式字符串，兼容所有主流客户端"
    },
    {
      name: "历史归档订阅",
      client: "全客户端可用",
      path: "/history.yaml",
      desc: "保留最近多次探测可用节点的历史累积池"
    }
  ];

  const copy = (url: string) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(url).then(() => showToast("订阅地址已复制至剪贴板"));
    } else {
      const inp = document.createElement("input");
      inp.value = url;
      document.body.appendChild(inp);
      inp.select();
      document.execCommand("copy");
      inp.remove();
      showToast("订阅地址已复制至剪贴板");
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <div style={{
        padding: '16px 20px',
        background: 'var(--surface-2)',
        border: '1px solid var(--border)'
      }}>
        <h2 style={{ fontSize: '18px', fontWeight: 500, color: 'var(--text-primary)' }}>
          全量订阅中心 (Subscriptions)
        </h2>
        <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
          支持各大客户端协议直链，内置自动鉴权防盗链机制
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(420px, 1fr))', gap: '16px' }}>
        {subItems.map((item, i) => {
          const fullUrl = `${protocol}//${host}${portStr}${item.path}${tokenQuery}`;
          return (
            <div key={i} style={{
              background: 'var(--surface-2)',
              border: '1px solid var(--border)',
              padding: '18px 20px',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
              gap: '12px'
            }}>
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <h3 style={{ fontSize: '15px', fontWeight: 500, color: 'var(--text-primary)' }}>{item.name}</h3>
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
                <button onClick={() => copy(fullUrl)} className="btn-primary" style={{ flex: 1 }}>
                  <Copy size={13} /> 复制直链
                </button>
                <a href={fullUrl} target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}>
                  <button className="btn-ghost">
                    <ExternalLink size={13} /> 预览
                  </button>
                </a>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
