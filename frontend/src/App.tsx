import React, { useState, useEffect } from 'react';
import {
  GroupItem,
  TelemetryStatus,
  getGroups,
  getStatus,
  triggerCheck,
  getApiKey,
  setApiKey
} from './api/client';
import { GroupsView } from './views/GroupsView';
import { SubscriptionsView } from './views/SubscriptionsView';
import { MonitorView } from './views/MonitorView';
import { FilesView } from './views/FilesView';
import { AnalysisView } from './views/AnalysisView';
import { ConfigView } from './views/ConfigView';
import { useToast } from './components/Toast';
import {
  Layers,
  Link2,
  Activity,
  FolderClosed,
  BarChart2,
  Settings,
  ExternalLink,
  Play,
  KeyRound,
  Github,
  Sun,
  Moon
} from 'lucide-react';

export const App: React.FC = () => {
  const { showToast } = useToast();
  
  const parseTab = (): 'groups' | 'subs' | 'monitor' | 'files' | 'analysis' | 'config' => {
    const p = window.location.pathname;
    const h = window.location.hash;
    if (p.includes('/files') || h === '#files') return 'files';
    if (p.includes('/analysis') || h === '#analysis') return 'analysis';
    if (h === '#subs') return 'subs';
    if (h === '#monitor') return 'monitor';
    if (h === '#config') return 'config';
    return 'groups';
  };

  const [activeTab, setActiveTab] = useState<'groups' | 'subs' | 'monitor' | 'files' | 'analysis' | 'config'>(parseTab);

  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return (localStorage.getItem('submatrix_theme') as 'light' | 'dark') || 'light';
  });

  const [groups, setGroups] = useState<GroupItem[]>([]);
  const [telemetry, setTelemetry] = useState<TelemetryStatus | null>(null);
  const [listenPort, setListenPort] = useState('8199');
  const [apiKey, setKey] = useState(getApiKey());
  const [keyModalOpen, setKeyModalOpen] = useState(false);
  const [tempKey, setTempKey] = useState(apiKey);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('submatrix_theme', theme);
  }, [theme]);

  // 监听浏览器前进/后退/Hash变化
  useEffect(() => {
    const handlePop = () => {
      setActiveTab(parseTab());
    };
    window.addEventListener('popstate', handlePop);
    window.addEventListener('hashchange', handlePop);
    return () => {
      window.removeEventListener('popstate', handlePop);
      window.removeEventListener('hashchange', handlePop);
    };
  }, []);

  const switchTab = (tabId: 'groups' | 'subs' | 'monitor' | 'files' | 'analysis' | 'config') => {
    setActiveTab(tabId);
    window.location.hash = tabId === 'groups' ? '' : tabId;
  };

  const toggleTheme = () => {
    setTheme(prev => prev === 'light' ? 'dark' : 'light');
    showToast(theme === 'light' ? "已切换至全黑深色模式" : "已切换至纯白极简模式");
  };

  const loadData = async () => {
    try {
      const gRes = await getGroups();
      setGroups(gRes.groups || []);
      if (gRes.listenPort) setListenPort(gRes.listenPort);
      if (gRes.apiKey) setKey(gRes.apiKey);
    } catch (e: any) {
      if (e.message === 'UNAUTHORIZED') {
        setKeyModalOpen(true);
      }
    }

    try {
      const s = await getStatus();
      setTelemetry(s);
    } catch {
      // 静默待命
    }
  };

  useEffect(() => {
    loadData();
    const interval = setInterval(() => {
      getStatus().then(setTelemetry).catch(() => {});
    }, 3000);
    return () => clearInterval(interval);
  }, []);

  const handleQuickCheck = async () => {
    try {
      await triggerCheck();
      showToast("已启动节点测活任务");
      const s = await getStatus();
      setTelemetry(s);
    } catch (e: any) {
      showToast(`检测启动失败: ${e.message}`, true);
    }
  };

  const handleSaveKey = () => {
    setApiKey(tempKey.trim());
    setKey(tempKey.trim());
    setKeyModalOpen(false);
    showToast("访问凭证已保存");
    loadData();
  };

  // 人性化重塑后的导航项 (通俗自然，无空洞排比)
  const navItems = [
    { id: 'groups', label: '节点分组', sub: 'Groups', icon: Layers },
    { id: 'subs', label: '订阅链接', sub: 'Subscriptions', icon: Link2 },
    { id: 'monitor', label: '运行日志', sub: 'Logs', icon: Activity },
    { id: 'files', label: '本地文件', sub: 'Files', icon: FolderClosed },
    { id: 'analysis', label: '节点统计', sub: 'Analytics', icon: BarChart2 },
    { id: 'config', label: '系统设置', sub: 'Settings', icon: Settings },
  ] as const;

  return (
    <div style={{ display: 'flex', minHeight: '100vh', width: '100vw', background: 'var(--surface-1)' }}>
      {/* 1. 左侧纯白/全黑侧边栏 */}
      <aside style={{
        width: '260px',
        flexShrink: 0,
        background: 'var(--surface-0)',
        borderRight: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'space-between',
        height: '100vh',
        position: 'sticky',
        top: 0
      }}>
        <div>
          {/* 品牌标识 */}
          <div style={{ padding: '24px 20px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: '12px' }}>
            <svg width="26" height="26" viewBox="0 0 100 100" fill="none">
              <rect width="100" height="100" fill="#000000" />
              <path d="M25 25 H75 M25 50 H75 M25 75 H75 M25 25 V75 M50 25 V75 M75 25 V75" stroke="#FFFFFF" strokeWidth="5" strokeLinecap="square"/>
              <rect x="17" y="17" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="42" y="17" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="67" y="17" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="17" y="42" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="42" y="42" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="67" y="42" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="17" y="67" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="42" y="67" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
              <rect x="67" y="67" width="16" height="16" fill="#000000" stroke="#FFFFFF" strokeWidth="5" />
            </svg>
            <div>
              <h1 style={{ fontSize: '18px', fontWeight: 500, color: 'var(--text-primary)', letterSpacing: '0.04em' }}>SubMatrix</h1>
              <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>节点分组与订阅管理</div>
            </div>
          </div>

          {/* 导航菜单 */}
          <nav style={{ padding: '12px 10px', display: 'flex', flexDirection: 'column', gap: '3px' }}>
            {navItems.map(item => {
              const Icon = item.icon;
              const isActive = activeTab === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => switchTab(item.id)}
                  style={{
                    width: '100%',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'flex-start',
                    gap: '10px',
                    padding: '9px 12px',
                    background: isActive ? 'var(--btn-primary-bg)' : 'transparent',
                    color: isActive ? 'var(--btn-primary-fg)' : 'var(--text-secondary)',
                    borderColor: 'transparent',
                    textAlign: 'left'
                  }}
                >
                  <Icon size={16} strokeWidth={1.7} />
                  <div style={{ display: 'flex', flexDirection: 'column' }}>
                    <span style={{ fontSize: '13px', fontWeight: 500, color: isActive ? 'var(--btn-primary-fg)' : 'var(--text-primary)' }}>{item.label}</span>
                    <span style={{ fontSize: '10px', color: isActive ? 'var(--text-muted)' : 'var(--text-secondary)', opacity: 0.8 }}>{item.sub}</span>
                  </div>
                </button>
              );
            })}

            {/* Sub-Store 链接 */}
            <a
              href="/sub-store/"
              target="_blank"
              rel="noopener noreferrer"
              style={{ textDecoration: 'none' }}
            >
              <button
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '9px 12px',
                  background: 'transparent',
                  color: 'var(--text-secondary)',
                  borderColor: 'transparent',
                  marginTop: '8px'
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <ExternalLink size={15} />
                  <span style={{ fontSize: '13px' }}>Sub-Store 转换</span>
                </div>
                <span className="badge">:8199</span>
              </button>
            </a>
          </nav>
        </div>

        {/* 底部信息 */}
        <div style={{ padding: '16px 20px', borderTop: '1px solid var(--border)', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '12px' }}>
            <span style={{ color: 'var(--text-secondary)' }}>端口: {listenPort.replace(':', '')}</span>
            <button onClick={() => setKeyModalOpen(true)} className="btn-ghost" style={{ padding: '2px 6px', fontSize: '11px' }}>
              <KeyRound size={12} /> API 密钥
            </button>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', alignItems: 'center' }}>
            <a href="https://github.com/Wenaixi/submatrix" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--text-secondary)' }}>
              <Github size={15} />
            </a>
          </div>
        </div>
      </aside>

      {/* 2. 主工作区 */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100vh', overflowY: 'auto' }}>
        {/* 顶栏状态监控 */}
        <header style={{
          position: 'sticky',
          top: 0,
          zIndex: 10,
          background: 'var(--surface-0)',
          borderBottom: '1px solid var(--border)',
          padding: '12px 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: '12px'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '20px', fontSize: '13px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <span style={{ width: '7px', height: '7px', background: telemetry?.checking ? '#000000' : '#10b981', display: 'inline-block' }} />
              <span style={{ color: 'var(--text-secondary)' }}>状态:</span>
              <span style={{ fontWeight: 500 }}>{telemetry?.checking ? `检测中 (${telemetry.stepName || '处理中'})` : '空闲'}</span>
            </div>
            <div>
              <span style={{ color: 'var(--text-secondary)' }}>总节点: </span>
              <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 500 }}>{telemetry?.proxyCount || telemetry?.lastCheck?.total || 0}</span>
            </div>
            <div>
              <span style={{ color: 'var(--text-secondary)' }}>可用: </span>
              <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 500 }}>{telemetry?.available || telemetry?.lastCheck?.available || 0}</span>
            </div>
            {telemetry?.lastCheck?.time && (
              <div style={{ color: 'var(--text-secondary)', fontSize: '12px' }}>
                上次检测: {telemetry.lastCheck.time}
              </div>
            )}
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button onClick={toggleTheme} className="btn-ghost" title="切换纯白/全黑模式" style={{ padding: '6px 10px' }}>
              {theme === 'light' ? <Moon size={15} /> : <Sun size={15} />}
            </button>
            <button onClick={handleQuickCheck} className="btn-primary" disabled={telemetry?.checking}>
              <Play size={13} /> {telemetry?.checking ? '正在测活...' : '开始检测'}
            </button>
          </div>
        </header>

        {/* 主内容区域 */}
        <main style={{ padding: '24px', flex: 1 }}>
          {activeTab === 'groups' && (
            <GroupsView
              groups={groups}
              setGroups={setGroups}
              listenPort={listenPort}
              apiKey={apiKey}
              onRefresh={loadData}
            />
          )}
          {activeTab === 'subs' && (
            <SubscriptionsView
              listenPort={listenPort}
              apiKey={apiKey}
            />
          )}
          {activeTab === 'monitor' && <MonitorView />}
          {activeTab === 'files' && <FilesView />}
          {activeTab === 'analysis' && <AnalysisView />}
          {activeTab === 'config' && <ConfigView />}
        </main>
      </div>

      {/* 密钥修改微型弹窗 */}
      {keyModalOpen && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, width: '100vw', height: '100vh',
          background: 'rgba(0,0,0,0.3)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 100000,
          padding: '20px'
        }}>
          <div style={{
            width: '100%',
            maxWidth: '400px',
            background: 'var(--surface-0)',
            border: '1px solid var(--border-black)',
            padding: '24px',
            display: 'flex',
            flexDirection: 'column',
            gap: '16px'
          }}>
            <h3 style={{ fontSize: '16px', fontWeight: 500, color: 'var(--text-primary)' }}>管理访问密钥 (API Key)</h3>
            <p style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>
              请输入 config.yaml 中配置的 API Key 凭证：
            </p>
            <input
              type="text"
              value={tempKey}
              onChange={(e) => setTempKey(e.target.value)}
              placeholder="请输入 API Key..."
            />
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '8px' }}>
              <button onClick={() => setKeyModalOpen(false)} className="btn-ghost">取消</button>
              <button onClick={handleSaveKey} className="btn-primary">保存凭证</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
