import React, { useState, useEffect } from 'react';
import { AnalysisReportResponse, getAnalysisReport, SubRankingItem } from '../api/client';
import { RefreshCw, Globe, ShieldCheck, Activity, Award, AlertTriangle, Copy, Clock, Zap, Database } from 'lucide-react';
import { useToast } from '../components/Toast';
import { WorldMap } from '../components/WorldMap';

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

  const copyUrl = (url: string) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(url).then(() => showToast("订阅地址已复制"));
    } else {
      const inp = document.createElement("input");
      inp.value = url;
      document.body.appendChild(inp);
      inp.select();
      document.execCommand("copy");
      inp.remove();
      showToast("订阅地址已复制");
    }
  };

  const rd = report?.report_data;
  const checkInfo = rd?.check_info;
  const globalAnalysis = rd?.global_analysis;

  // 节点总数与可用数
  const totalNodes = checkInfo?.check_count_raw || (checkInfo?.check_count ? parseInt(checkInfo.check_count) : 0);
  const aliveNodes = globalAnalysis?.alive_count || 0;
  const aliveRate = totalNodes > 0 ? ((aliveNodes / totalNodes) * 100).toFixed(2) : '0.00';

  // 国家分布（降序排列）
  const geoEntries = Object.entries(globalAnalysis?.geography_distribution || {})
    .sort((a, b) => b[1] - a[1]);

  // 协议分布（降序排列）
  const protoEntries = Object.entries(globalAnalysis?.protocol_distribution || {})
    .sort((a, b) => b[1] - a[1]);

  // 优选订阅源排行榜
  const subsRanking = rd?.subs_ranking || [];
  // 失效订阅源
  const subsRankingBad = rd?.subs_ranking_bad || [];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 顶部标题区 */}
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
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <h2 style={{ fontSize: '18px', fontWeight: 500, color: 'var(--text-primary)' }}>
              节点质检分析报告 (Node Quality Analytics)
            </h2>
            {checkInfo?.check_time && (
              <span className="badge" style={{ fontSize: '11px' }}>
                <Clock size={11} style={{ marginRight: '3px' }} />
                完成于 {checkInfo.check_time}
              </span>
            )}
          </div>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            客观分析代理节点的网络可用率、地理分布画像、流媒体/AI解锁与各大订阅源表现
          </p>
        </div>

        <button onClick={loadData} className="btn-ghost" disabled={loading}>
          <RefreshCw size={13} className={loading ? "animate-spin" : ""} /> 重新加载
        </button>
      </div>

      {/* 全景执行摘要卡片 */}
      {rd?.summary && (
        <div style={{
          background: 'var(--surface-2)',
          border: '1px solid var(--border)',
          borderLeft: '4px solid var(--text-primary)',
          padding: '16px 20px'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
            <Zap size={14} color="var(--text-primary)" />
            <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>
              全景质检执行摘要 (Execution Summary)
            </span>
          </div>
          <div style={{
            fontSize: '13px',
            color: 'var(--text-secondary)',
            lineHeight: 1.6,
            whiteSpace: 'pre-line'
          }}>
            {rd.summary.trim()}
          </div>
        </div>
      )}

      {/* 4 维核心指标大盘 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '16px' }}>
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>探测节点总数</span>
            <Database size={13} color="var(--text-secondary)" />
          </div>
          <div style={{ fontSize: '24px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {totalNodes.toLocaleString()}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            总计拉取池容量 {checkInfo?.total_subs_size || '-'}
          </div>
        </div>

        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>可用节点总数</span>
            <Activity size={13} color="var(--text-secondary)" />
          </div>
          <div style={{ fontSize: '24px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {aliveNodes.toLocaleString()}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            速度门槛 {checkInfo?.check_min_speed ? `≥ ${checkInfo.check_min_speed} KB/s` : '无限制'}
          </div>
        </div>

        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>节点有效存活率</span>
            <ShieldCheck size={13} color="var(--text-secondary)" />
          </div>
          <div style={{ fontSize: '24px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {aliveRate}%
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            已过滤丢弃所有离线节点
          </div>
        </div>

        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>质检耗时与总流量</span>
            <Clock size={13} color="var(--text-secondary)" />
          </div>
          <div style={{ fontSize: '20px', fontWeight: 500, fontFamily: 'var(--font-mono)', marginTop: '6px' }}>
            {checkInfo?.check_duration || '-'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            消耗流量 {checkInfo?.check_traffic_total || '-'}
          </div>
        </div>
      </div>

      {/* 全球节点真实雷达航线与地理拓扑地图 */}
      <WorldMap distribution={globalAnalysis?.geography_distribution || {}} />

      {/* 国家与协议分布双栏极简看板 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(420px, 1fr))', gap: '16px' }}>
        {/* 国家地区画像 */}
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border)', paddingBottom: '10px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Globe size={15} color="var(--text-secondary)" />
              <h3 style={{ fontSize: '15px', fontWeight: 500 }}>国家与地区分布画像</h3>
            </div>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
              覆盖 {geoEntries.length} 个地区
            </span>
          </div>

          <div style={{ marginTop: '14px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
            {geoEntries.length > 0 ? (
              geoEntries.map(([code, count], idx) => {
                const pct = aliveNodes > 0 ? ((count / aliveNodes) * 100).toFixed(1) : '0';
                return (
                  <div key={idx} style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '13px' }}>
                      <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 500 }}>
                        {code}
                      </span>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>{pct}%</span>
                        <span className="badge">{count} 节点</span>
                      </div>
                    </div>
                    {/* 单像素进度条 */}
                    <div style={{ width: '100%', height: '4px', background: 'var(--surface-1)', border: '1px solid var(--border)' }}>
                      <div style={{ width: `${pct}%`, height: '100%', background: 'var(--text-primary)' }} />
                    </div>
                  </div>
                );
              })
            ) : (
              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', padding: '24px 0', textAlign: 'center' }}>
                暂无国家地区画像数据
              </div>
            )}
          </div>
        </div>

        {/* 协议分布画像 */}
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border)', paddingBottom: '10px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <ShieldCheck size={15} color="var(--text-secondary)" />
              <h3 style={{ fontSize: '15px', fontWeight: 500 }}>代理协议分布比例</h3>
            </div>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
              共 {protoEntries.length} 种协议
            </span>
          </div>

          <div style={{ marginTop: '14px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
            {protoEntries.length > 0 ? (
              protoEntries.map(([proto, count], idx) => {
                const pct = aliveNodes > 0 ? ((count / aliveNodes) * 100).toFixed(1) : '0';
                return (
                  <div key={idx} style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '13px' }}>
                      <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 500 }}>
                        {proto.toUpperCase()}
                      </span>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>{pct}%</span>
                        <span className="badge">{count} 节点</span>
                      </div>
                    </div>
                    {/* 单像素进度条 */}
                    <div style={{ width: '100%', height: '4px', background: 'var(--surface-1)', border: '1px solid var(--border)' }}>
                      <div style={{ width: `${pct}%`, height: '100%', background: 'var(--text-primary)' }} />
                    </div>
                  </div>
                );
              })
            ) : (
              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', padding: '24px 0', textAlign: 'center' }}>
                暂无协议分布数据
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 订阅源质量排行榜（优选有效源红榜） */}
      <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border)', paddingBottom: '10px', marginBottom: '14px' }}>
          <Award size={16} color="var(--text-primary)" />
          <h3 style={{ fontSize: '15px', fontWeight: 500, color: 'var(--text-primary)' }}>
            高可用订阅源排行榜 (Top Working Sources)
          </h3>
          <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            按可用节点数与成功率优选排列
          </span>
        </div>

        {subsRanking.length > 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            {subsRanking.map((sub, idx) => (
              <div key={idx} style={{
                background: 'var(--surface-1)',
                border: '1px solid var(--border)',
                padding: '12px 16px',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                flexWrap: 'wrap',
                gap: '10px'
              }}>
                <div style={{ flex: 1, minWidth: '280px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '11px',
                      background: 'var(--surface-2)',
                      padding: '2px 6px',
                      border: '1px solid var(--border)'
                    }}>
                      TOP {idx + 1}
                    </span>
                    <span style={{
                      fontSize: '13px',
                      fontFamily: 'var(--font-mono)',
                      color: 'var(--text-primary)',
                      wordBreak: 'break-all'
                    }} title={sub.url}>
                      {sub.url.length > 60 ? sub.url.slice(0, 58) + '...' : sub.url}
                    </span>
                    <button
                      onClick={() => copyUrl(sub.url)}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '2px', color: 'var(--text-secondary)' }}
                      title="复制订阅源地址"
                    >
                      <Copy size={12} />
                    </button>
                  </div>
                  {sub.top_locations && sub.top_locations.length > 0 && (
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      主力地区：{sub.top_locations.join(' ')} · 体积: {sub.file_size || '-'}
                    </div>
                  )}
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                  {sub.avg_speed && sub.avg_speed !== '0.0 MB/s' && (
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>均速</div>
                      <div style={{ fontSize: '13px', fontFamily: 'var(--font-mono)', fontWeight: 500 }}>{sub.avg_speed}</div>
                    </div>
                  )}
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>存活率</div>
                    <div style={{ fontSize: '13px', fontFamily: 'var(--font-mono)', fontWeight: 500 }}>
                      {sub.stats?.rate || '0%'}
                    </div>
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>可用节点</div>
                    <span className="badge" style={{ fontSize: '12px', fontWeight: 500 }}>
                      {sub.stats?.success || 0} / {sub.stats?.total || 0}
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div style={{ color: 'var(--text-secondary)', fontSize: '13px', padding: '20px 0', textAlign: 'center' }}>
            暂无有效订阅源上榜数据
          </div>
        )}
      </div>

      {/* 失效/全挂订阅源预警列表（黑榜） */}
      {subsRankingBad.length > 0 && (
        <div style={{ background: 'var(--surface-2)', border: '1px solid var(--border)', padding: '16px 20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border)', paddingBottom: '10px', marginBottom: '14px' }}>
            <AlertTriangle size={15} color="var(--text-secondary)" />
            <h3 style={{ fontSize: '15px', fontWeight: 500 }}>
              失效订阅源预警 (Zero Availability Sources)
            </h3>
            <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
              以下 {subsRankingBad.length} 条订阅源在最近一次检测中 0 节点存活，建议排查或移除
            </span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {subsRankingBad.map((bad, idx) => (
              <div key={idx} style={{
                background: 'var(--surface-1)',
                border: '1px solid var(--border)',
                padding: '10px 14px',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                fontSize: '12px'
              }}>
                <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)', wordBreak: 'break-all' }}>
                  {bad.url}
                </span>
                <span className="badge" style={{ opacity: 0.7 }}>
                  0 / {bad.stats?.total || 0} 可用 (0%)
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
