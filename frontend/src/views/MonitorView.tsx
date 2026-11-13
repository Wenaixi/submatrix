import React, { useState, useEffect, useRef } from 'react';
import { TelemetryStatus, getStatus, getLogs, clearLogs, triggerCheck, forceClose } from '../api/client';
import { Play, Square, Trash2, Copy, RefreshCw } from 'lucide-react';
import { useToast } from '../components/Toast';

export const MonitorView: React.FC = () => {
  const { showToast } = useToast();
  const [status, setStatus] = useState<TelemetryStatus | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [autoScroll, setAutoScroll] = useState(true);
  const [filter, setFilter] = useState('');
  const logBoxRef = useRef<HTMLDivElement>(null);

  // 轮询状态与日志
  useEffect(() => {
    let timer: any = null;
    const poll = async () => {
      try {
        const s = await getStatus();
        setStatus(s);
        const l = await getLogs();
        setLogs(l.logs || []);
      } catch (e) {
        // 静默
      }
    };
    poll();
    timer = setInterval(poll, 2000);
    return () => clearInterval(timer);
  }, []);

  // 自动滚屏
  useEffect(() => {
    if (autoScroll && logBoxRef.current) {
      logBoxRef.current.scrollTop = logBoxRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const handleTrigger = async () => {
    try {
      await triggerCheck();
      showToast("已触发节点检测任务");
    } catch (e: any) {
      showToast(`触发失败: ${e.message}`, true);
    }
  };

  const handleStop = async () => {
    try {
      await forceClose();
      showToast("已发送强制终止信号");
    } catch (e: any) {
      showToast(`终止失败: ${e.message}`, true);
    }
  };

  const handleClear = async () => {
    try {
      await clearLogs();
      setLogs([]);
      showToast("控制台日志已清空");
    } catch (e: any) {
      showToast(`清空失败: ${e.message}`, true);
    }
  };

  const copyLogs = () => {
    const text = logs.join("\n");
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text).then(() => showToast("全部日志已复制"));
    }
  };

  const filteredLogs = filter
    ? logs.filter(l => l.toLowerCase().includes(filter.toLowerCase()))
    : logs;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      {/* 遥测 HUD 状态卡片 */}
      <div style={{
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        padding: '16px 20px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        gap: '12px'
      }}>
        <div style={{ display: 'flex', gap: '24px', flexWrap: 'wrap' }}>
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>检测状态</div>
            <div style={{ fontSize: '15px', fontWeight: 500, color: status?.checking ? '#000000' : 'var(--text-secondary)' }}>
              {status?.checking ? `检测中: ${status.stepName || '处理中'}` : '空闲待命'}
            </div>
          </div>
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>总节点数</div>
            <div style={{ fontSize: '15px', fontWeight: 500, fontFamily: 'var(--font-mono)' }}>
              {status?.proxyCount || status?.lastCheck?.total || 0}
            </div>
          </div>
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>可用节点</div>
            <div style={{ fontSize: '15px', fontWeight: 500, fontFamily: 'var(--font-mono)' }}>
              {status?.available || status?.lastCheck?.available || 0}
            </div>
          </div>
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>当前进度</div>
            <div style={{ fontSize: '15px', fontWeight: 500, fontFamily: 'var(--font-mono)' }}>
              {(() => {
                // 后端 progress 字段是“已完成个数”，总数在 proxyCount，须在此换算成百分比。
                // 阶段切换时两者会同步变化（测活阶段是总订阅数，测速阶段是存活数），因此分母一致。
                const done = status?.progress || 0;
                const total = status?.proxyCount || 0;
                if (total <= 0) return '0%';
                const pct = Math.min(100, Math.max(0, (done / total) * 100));
                return `${pct.toFixed(1)}%`;
              })()}
            </div>
          </div>
          {status?.eta !== undefined && status.eta > 0 && (
            <div>
              <div style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>预估剩余</div>
              <div style={{ fontSize: '15px', fontWeight: 500, fontFamily: 'var(--font-mono)' }}>
                {status.eta} 秒
              </div>
            </div>
          )}
        </div>

        <div style={{ display: 'flex', gap: '8px' }}>
          {status?.checking ? (
            <button onClick={handleStop} className="btn-danger">
              <Square size={13} /> 强制停止
            </button>
          ) : (
            <button onClick={handleTrigger} className="btn-primary">
              <Play size={13} /> 手动触发检测
            </button>
          )}
        </div>
      </div>

      {/* 终端控制栏与日志容器 */}
      <div style={{
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column'
      }}>
        {/* 工具条 */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 16px',
          borderBottom: '1px solid var(--border)',
          background: 'var(--surface-1)',
          flexWrap: 'wrap',
          gap: '8px'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <span style={{ fontSize: '13px', fontWeight: 500 }}>运行日志 (Terminal Logs)</span>
            <span className="badge">{logs.length} 行</span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <input
              type="text"
              placeholder="过滤日志..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              style={{ width: '160px', padding: '4px 8px', fontSize: '12px' }}
            />
            <button
              onClick={() => setAutoScroll(!autoScroll)}
              className="btn-ghost"
              style={{ fontSize: '12px', padding: '4px 8px' }}
            >
              {autoScroll ? "自动滚屏: 开" : "自动滚屏: 关"}
            </button>
            <button onClick={copyLogs} className="btn-ghost" style={{ fontSize: '12px', padding: '4px 8px' }}>
              <Copy size={12} /> 复制
            </button>
            <button onClick={handleClear} className="btn-ghost" style={{ fontSize: '12px', padding: '4px 8px' }}>
              <Trash2 size={12} /> 清屏
            </button>
          </div>
        </div>

        {/* 纯白/深灰高对比日志输出区域 */}
        <div
          ref={logBoxRef}
          style={{
            height: '480px',
            overflowY: 'auto',
            padding: '12px 16px',
            background: '#ffffff',
            fontFamily: 'var(--font-mono)',
            fontSize: '12px',
            lineHeight: '1.6',
            color: '#18181b',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all'
          }}
        >
          {filteredLogs.length === 0 ? (
            <div style={{ color: 'var(--text-secondary)' }}>暂无日志输出...</div>
          ) : (
            filteredLogs.map((line, i) => (
              <div key={i} style={{ borderBottom: '1px solid #f4f4f5', padding: '1px 0' }}>
                {line}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
};
