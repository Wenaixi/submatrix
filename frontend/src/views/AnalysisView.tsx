import React, { useState, useEffect } from 'react';
import { AnalysisReportResponse, getAnalysisReport } from '../api/client';
import { BarChart3, RefreshCw, Activity, Globe, ShieldCheck } from 'lucide-react';
import { useToast } from '../components/Toast';

export const AnalysisView: React.FC = () => {
  const { showToast } = useToast();
  const [report, setReport] = useState<AnalysisReportResponse | null>(null);
  const [loading, setLoading] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const data = await getAnalysisReport();
      setReport(data);
    } catch (e: any) {
      showToast(`加载分析报告失败: ${e.message}`, true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const total = report?.total || 0;
  const available = report?.available || 0;
  const rate = total > 0 ? ((available / total) * 100).toFixed(1) : '0';

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
            节点质检分析报告 (Node Quality Analytics)
          </h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            客观分析代理节点的网络可用率、地理分布画像与协议比例
          </p>
        </div>

        <button onClick={loadData} className="btn-ghost" disabled={loading}>
          <RefreshCw size={13} className={loading ? "animate-spin" : ""} /> 重新加载
        </button>
      </div>

      {/* 4 维指标卡片 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '16px' }}>
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>探测节点总数</div>
          <div style={{ fontSize: '24px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '4px' }}>{total}</div>
        </div>
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>可用节点总数</div>
          <div style={{ fontSize: '24px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '4px' }}>{available}</div>
        </div>
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>节点存活率</div>
          <div style={{ fontSize: '24px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '4px' }}>{rate}%</div>
        </div>
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>平均下载带宽</div>
          <div style={{ fontSize: '24px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {report?.avg_speed ? `${(report.avg_speed / 1024 / 1024).toFixed(1)} MB/s` : '未开启测速'}
          </div>
        </div>
      </div>

      {/* 国家分布与协议分布双栏极简表格 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(420px, 1fr))', gap: '16px' }}>
        {/* 地区分布 */}
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border)', paddingBottom: '10px' }}>
            <Globe size={15} color="var(--text-secondary)" />
            <h3 style={{ fontSize: '15px', fontWeight: 500 }}>国家与地区分布</h3>
          </div>
          <div style={{ marginTop: '12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {report?.countries && Object.keys(report.countries).length > 0 ? (
              Object.entries(report.countries).map(([k, v], i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '13px' }}>
                  <span style={{ fontFamily: 'var(--font-mono)' }}>{k}</span>
                  <span className="badge">{v} 节点</span>
                </div>
              ))
            ) : (
              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', padding: '16px 0' }}>暂无国家地区画像数据</div>
            )}
          </div>
        </div>

        {/* 协议分布 */}
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border)', paddingBottom: '10px' }}>
            <ShieldCheck size={15} color="var(--text-secondary)" />
            <h3 style={{ fontSize: '15px', fontWeight: 500 }}>代理协议分布</h3>
          </div>
          <div style={{ marginTop: '12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {report?.protocols && Object.keys(report.protocols).length > 0 ? (
              Object.entries(report.protocols).map(([k, v], i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '13px' }}>
                  <span style={{ fontFamily: 'var(--font-mono)' }}>{k.toUpperCase()}</span>
                  <span className="badge">{v} 节点</span>
                </div>
              ))
            ) : (
              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', padding: '16px 0' }}>暂无协议分布数据</div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
