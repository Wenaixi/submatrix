import React, { useState, useEffect } from 'react';
import { getConfig, saveConfig } from '../api/client';
import { Save, RefreshCw, Code2, AlertCircle } from 'lucide-react';
import { useToast } from '../components/Toast';

export const ConfigView: React.FC = () => {
  const { showToast } = useToast();
  const [yamlContent, setYamlContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await getConfig();
      setYamlContent(res.content || '');
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
      showToast("配置文件已成功保存并即时热重载");
    } catch (e: any) {
      showToast(`保存失败: ${e.message}`, true);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
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
            系统高级配置 (Config Center)
          </h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            直接编辑 config.yaml 底层参数，包括 Cron 定时计划、测速源与通知通道
          </p>
        </div>

        <div style={{ display: 'flex', gap: '8px' }}>
          <button onClick={loadConfig} className="btn-ghost" disabled={loading}>
            <RefreshCw size={13} className={loading ? "animate-spin" : ""} /> 重载
          </button>
          <button onClick={handleSave} className="btn-primary" disabled={saving}>
            <Save size={13} /> {saving ? "保存中..." : "保存配置"}
          </button>
        </div>
      </div>

      {/* 纯白极简 YAML 编辑大屏 */}
      <div style={{
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column'
      }}>
        <div style={{
          padding: '8px 16px',
          background: 'var(--surface-1)',
          borderBottom: '1px solid var(--border)',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          fontSize: '12px',
          color: 'var(--text-secondary)'
        }}>
          <Code2 size={14} />
          <span>config.yaml (UTF-8 纯文本)</span>
        </div>
        <textarea
          rows={24}
          value={yamlContent}
          onChange={(e) => setYamlContent(e.target.value)}
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
            background: '#ffffff',
            color: '#09090b',
            resize: 'vertical'
          }}
        />
      </div>
    </div>
  );
};
