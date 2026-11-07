import React, { useState, useEffect, useMemo } from 'react';
import { getConfig, saveConfig } from '../api/client';
import { Save, RefreshCw, Code2, Settings, ChevronDown, ChevronRight } from 'lucide-react';
import { useToast } from '../components/Toast';

// 常用设置项定义：key 对应 config.yaml 的顶层键，type 决定输入控件
interface QuickSetting {
  key: string;
  label: string;
  hint: string;
  type: 'text' | 'number' | 'boolean';
  unit?: string;
}

// 分组展示常用项，避免一次性铺满
const QUICK_GROUPS: { title: string; items: QuickSetting[] }[] = [
  {
    title: '服务与访问',
    items: [
      { key: 'public-url', label: '公网访问地址', hint: '组间引用与订阅直链的基准地址，必须填写完整 https 域名', type: 'text' },
      { key: 'api-key', label: 'API 密钥', hint: '所有接口与订阅链接的鉴权令牌', type: 'text' },
      { key: 'listen-port', label: '监听端口', hint: 'WebUI 与订阅服务的端口', type: 'text', unit: '端口' },
      { key: 'enable-web-ui', label: '启用 WebUI', hint: '关闭后只能通过 CLI 操作', type: 'boolean' },
    ],
  },
  {
    title: '检测参数',
    items: [
      { key: 'min-speed', label: '最低测速门槛', hint: '低于此速度的节点判定为不合格；单位 KB/s', type: 'number', unit: 'KB/s' },
      { key: 'concurrent', label: '全局并发数', hint: '同时检测的节点数，过大会拖垮本机网络', type: 'number', unit: '个' },
      { key: 'timeout', label: '连接超时', hint: '单节点握手超时时间；单位毫秒', type: 'number', unit: 'ms' },
      { key: 'download-mb', label: '测速下载上限', hint: '每个节点最多下载多少数据用于测速', type: 'number', unit: 'MB' },
      { key: 'download-timeout', label: '测速超时', hint: '下载测速的最长等待时间', type: 'number', unit: '秒' },
    ],
  },
  {
    title: '调度与订阅',
    items: [
      { key: 'cron-expression', label: '定时检测计划', hint: '标准 cron 表达式；留空则用检测间隔', type: 'text' },
      { key: 'check-interval', label: '检测间隔', hint: '未设置 cron 时按此分钟数循环检测', type: 'number', unit: '分钟' },
      { key: 'speed-test-url', label: '测速地址', hint: '下载测速使用的文件地址，填 random 则自动挑选', type: 'text' },
    ],
  },
  {
    title: '命名与打标',
    items: [
      { key: 'rename-node', label: '按地区重命名', hint: '用 IP 定位结果重写节点名（国旗+地区+序号）', type: 'boolean' },
      { key: 'enhanced-tag', label: '增强位置角标', hint: '显示 CF 状态角标：HK²=直连、HK¹⁺=CF同区、HK¹-US⁰=CF跨区', type: 'boolean' },
      { key: 'node-prefix', label: '节点名前缀', hint: '给所有重命名后的节点加统一前缀', type: 'text' },
      { key: 'media-check', label: '流媒体解锁检测', hint: '检测 Netflix/Disney/OpenAI 等平台可用性', type: 'boolean' },
      { key: 'isp-check', label: 'ISP 类型检测', hint: '识别机房/住宅/移动等出口类型（需要 API key）', type: 'boolean' },
    ],
  },
];

// 从 YAML 文本中读取某个顶层标量的当前值（只处理简单 key: value 行）
function readYamlValue(content: string, key: string): string {
  const lines = content.split('\n');
  for (const line of lines) {
    // 顶层键（不能有前导空格），形如 key: value 或 key: "value"
    const m = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (m && m[1] === key) {
      return m[2].replace(/^["']|["']$/g, '').replace(/\s+#.*$/, '').trim();
    }
  }
  return '';
}

// 在 YAML 文本中写入某个顶层标量；不存在则追加到末尾
function writeYamlValue(content: string, key: string, raw: string, type: string): string {
  const lines = content.split('\n');
  const needsQuote = type === 'text' && raw !== '' && !/^\d+$/.test(raw);
  const value = needsQuote ? `"${raw.replace(/"/g, '\\"')}"` : raw;
  let found = false;

  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(/^([A-Za-z0-9_-]+):/);
    if (m && m[1] === key) {
      lines[i] = `${key}: ${value}`;
      found = true;
      break;
    }
  }
  if (!found) {
    lines.push(`${key}: ${value}`);
  }
  return lines.join('\n');
}

export const ConfigView: React.FC = () => {
  const { showToast } = useToast();
  const [yamlContent, setYamlContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [dirty, setDirty] = useState(false);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await getConfig();
      setYamlContent(res.content || '');
      setDirty(false);
    } catch (e: any) {
      showToast(`加载配置失败: ${e.message}`, true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadConfig();
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await saveConfig(yamlContent);
      setDirty(false);
      showToast("配置文件已成功保存并即时热重载");
    } catch (e: any) {
      showToast(`保存失败: ${e.message}`, true);
    } finally {
      setSaving(false);
    }
  };

  // 常用项改动：直接改 YAML 文本，保持单一数据源
  const updateQuick = (s: QuickSetting, value: string) => {
    setYamlContent(prev => writeYamlValue(prev, s.key, value, s.type));
    setDirty(true);
  };

  // 预计算各常用项的当前值（避免每次渲染扫描全文本）
  const quickValues = useMemo(() => {
    const out: Record<string, string> = {};
    for (const g of QUICK_GROUPS) {
      for (const it of g.items) {
        out[it.key] = readYamlValue(yamlContent, it.key);
      }
    }
    return out;
  }, [yamlContent]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 顶部操作条 */}
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
            设置
          </h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            常用项直接修改，进阶项展开底层配置编辑。所有改动点「保存」后立即生效。
          </p>
        </div>

        <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          {dirty && <span style={{ fontSize: '12px', color: 'var(--warning, #d97706)' }}>有未保存的改动</span>}
          <button onClick={loadConfig} className="btn-ghost" disabled={loading}>
            <RefreshCw size={13} className={loading ? "animate-spin" : ""} /> 重载
          </button>
          <button onClick={handleSave} className="btn-primary" disabled={saving || !dirty}>
            <Save size={13} /> {saving ? "保存中..." : "保存配置"}
          </button>
        </div>
      </div>

      {/* 常用设置表单 */}
      {QUICK_GROUPS.map(group => (
        <div key={group.title} style={{
          background: 'var(--surface-2)',
          border: '1px solid var(--border)',
          padding: '18px 20px'
        }}>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            marginBottom: '14px',
            paddingBottom: '10px',
            borderBottom: '1px solid var(--border)'
          }}>
            <Settings size={15} style={{ color: 'var(--text-secondary)' }} />
            <span style={{ fontSize: '14px', fontWeight: 500, color: 'var(--text-primary)' }}>{group.title}</span>
          </div>

          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
            gap: '16px'
          }}>
            {group.items.map(item => {
              const raw = quickValues[item.key] ?? '';
              return (
                <div key={item.key} style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  <label style={{ fontSize: '13px', color: 'var(--text-primary)', display: 'flex', alignItems: 'center', gap: '6px' }}>
                    {item.label}
                    {item.unit && <span style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>({item.unit})</span>}
                  </label>

                  {item.type === 'boolean' ? (
                    <button
                      onClick={() => updateQuick(item, raw === 'true' ? 'false' : 'true')}
                      style={{
                        alignSelf: 'flex-start',
                        padding: '5px 14px',
                        fontSize: '12px',
                        border: '1px solid var(--border)',
                        background: raw === 'true' ? 'var(--accent, #2563eb)' : 'var(--surface-1)',
                        color: raw === 'true' ? '#fff' : 'var(--text-secondary)',
                        cursor: 'pointer'
                      }}
                    >
                      {raw === 'true' ? '已开启' : '已关闭'}
                    </button>
                  ) : (
                    <input
                      type={item.type === 'number' ? 'number' : 'text'}
                      value={raw}
                      onChange={(e) => updateQuick(item, e.target.value)}
                      style={{
                        padding: '7px 10px',
                        fontSize: '13px',
                        fontFamily: 'var(--font-mono)',
                        border: '1px solid var(--border)',
                        background: 'var(--surface-1)',
                        color: 'var(--text-primary)',
                        outline: 'none'
                      }}
                    />
                  )}
                  <span style={{ fontSize: '11px', color: 'var(--text-secondary)', lineHeight: '1.5' }}>{item.hint}</span>
                </div>
              );
            })}
          </div>
        </div>
      ))}

      {/* 进阶：YAML 编辑器折叠区 */}
      <div style={{
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column'
      }}>
        <button
          onClick={() => setShowAdvanced(v => !v)}
          style={{
            padding: '12px 16px',
            background: 'var(--surface-1)',
            border: 'none',
            borderBottom: showAdvanced ? '1px solid var(--border)' : 'none',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            fontSize: '13px',
            color: 'var(--text-secondary)',
            cursor: 'pointer',
            textAlign: 'left'
          }}
        >
          {showAdvanced ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          <Code2 size={14} />
          <span>进阶：直接编辑 config.yaml</span>
          <span style={{ fontSize: '11px', opacity: 0.7 }}>（全部 {yamlContent.split('\n').length} 行）</span>
        </button>

        {showAdvanced && (
          <textarea
            rows={26}
            value={yamlContent}
            onChange={(e) => { setYamlContent(e.target.value); setDirty(true); }}
            spellCheck={false}
            style={{
              width: '100%',
              boxSizing: 'border-box',
              fontFamily: 'var(--font-mono)',
              fontSize: '13px',
              lineHeight: '1.6',
              padding: '16px',
              border: 'none',
              outline: 'none',
              background: 'var(--surface-1)',
              color: 'var(--text-primary)',
              resize: 'vertical'
            }}
          />
        )}
      </div>
    </div>
  );
};