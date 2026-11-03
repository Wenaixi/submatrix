import React, { useEffect, useRef, useState } from 'react';

// 国家中心坐标 [lon, lat]
const GEO_COUNTRY_COORDS: Record<string, [number, number]> = {
  CN: [104.20, 35.86], HK: [114.17, 22.32], TW: [120.96, 23.70],
  JP: [138.25, 36.20], KR: [127.77, 35.91], MN: [103.85, 46.86], MO: [113.54, 22.20],
  SG: [103.82, 1.35], TH: [100.99, 15.87], MY: [109.70, 4.21], ID: [113.92, -0.79],
  VN: [108.28, 14.06], PH: [122.88, 12.88], IN: [78.96, 20.59], PK: [69.34, 30.38],
  KZ: [66.92, 48.02], TR: [35.24, 38.96], IL: [34.85, 31.05], AE: [53.85, 23.42],
  SA: [45.08, 23.89], IR: [53.69, 32.43], GB: [-3.44, 55.38], FR: [2.21, 46.23],
  DE: [10.45, 51.17], NL: [5.29, 52.13], BE: [4.47, 50.50], LU: [6.13, 49.82],
  CH: [8.23, 46.82], AT: [14.55, 47.52], IE: [-8.24, 53.41], PT: [-8.22, 39.40],
  ES: [-3.75, 40.46], IT: [12.57, 41.87], GR: [21.82, 39.07], SE: [18.64, 60.13],
  NO: [8.47, 60.47], DK: [9.50, 56.26], FI: [25.75, 61.92], IS: [-18.14, 64.96],
  RU: [105.32, 61.52], UA: [31.17, 48.38], PL: [19.15, 51.92], CZ: [15.47, 49.82],
  RO: [24.97, 45.94], HU: [19.50, 47.16], US: [-95.71, 37.09], CA: [-96.80, 60.00],
  MX: [-102.55, 23.63], BR: [-51.93, -14.24], AR: [-63.62, -38.42], CL: [-71.54, -35.68],
  AU: [133.77, -25.27], NZ: [174.88, -40.90], ZA: [22.93, -30.55], EG: [30.80, 26.82],
  CM: [12.35, 7.36]
};

// 大区颜色（黑白高对比与夜景璀璨双模兼容）
const REGION_COLORS: Record<string, string> = {
  US: '#3b82f6', NL: '#10b981', SG: '#f59e0b', DE: '#ec4899',
  HK: '#8b5cf6', JP: '#ef4444', GB: '#06b6d4', FR: '#6366f1',
  DEFAULT: '#94a3b8'
};

interface WorldMapProps {
  distribution: Record<string, number>;
}

export const WorldMap: React.FC<WorldMapProps> = ({ distribution }) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [tooltip, setTooltip] = useState<{ x: number; y: number; text: string } | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let animId: number;
    let t0 = performance.now();
    const dpr = window.devicePixelRatio || 1;

    // 起点：默认中国沿海/香港核心 [lon: 114.17, lat: 22.32]
    const origin = { lon: 114.17, lat: 22.32 };

    const getDimensions = () => {
      const W = container.clientWidth || 900;
      const H = 320;
      return { W, H };
    };

    let { W, H } = getDimensions();

    const resize = () => {
      const dims = getDimensions();
      W = dims.W;
      H = dims.H;
      canvas.width = W * dpr;
      canvas.height = H * dpr;
      canvas.style.width = W + 'px';
      canvas.style.height = H + 'px';
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();

    const ro = new ResizeObserver(() => resize());
    ro.observe(container);

    // 墨卡托/经纬度平面映射函数
    const proj = (lon: number, lat: number) => {
      const mx = 12, my = 12;
      const centerLon = origin.lon + 35;
      let dlon = lon - centerLon;
      while (dlon > 180) dlon -= 360;
      while (dlon < -180) dlon += 360;
      const x = mx + (dlon + 180) / 360 * (W - 2 * mx);
      const y = my + (85 - lat) / 145 * (H - 2 * my);
      return [x, y];
    };

    // 提取有效节点目标数据
    const entries = Object.entries(distribution || {}).filter(([k]) => GEO_COUNTRY_COORDS[k]);
    const maxCount = Math.max(...entries.map(([, v]) => v), 1);

    // 贝塞尔航线计算
    const [ox, oy] = proj(origin.lon, origin.lat);
    const arcs = entries.map(([code, count], idx) => {
      const [lon, lat] = GEO_COUNTRY_COORDS[code];
      const [dx, dy] = proj(lon, lat);
      const ratio = count / maxCount;
      const dist = Math.hypot(dx - ox, dy - oy);
      const lift = Math.min(dist * 0.38, H * 0.42);
      const cpx = (ox + dx) / 2;
      const cpy = (oy + dy) / 2 - lift;
      const color = REGION_COLORS[code] || REGION_COLORS.DEFAULT;
      return {
        code,
        count,
        ratio,
        ox, oy,
        cpx, cpy,
        dx, dy,
        color,
        period: 2400 + idx * 180
      };
    });

    // 鼠标移动悬停检测
    const handleMouseMove = (e: MouseEvent) => {
      const rect = canvas.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      let hovered = null;

      for (const arc of arcs) {
        if (Math.hypot(mx - arc.dx, my - arc.dy) < 14) {
          hovered = {
            x: arc.dx,
            y: arc.dy,
            text: `${arc.code}: ${arc.count} 个可用节点`
          };
          break;
        }
      }
      setTooltip(hovered);
      canvas.style.cursor = hovered ? 'pointer' : 'default';
    };

    const handleMouseLeave = () => {
      setTooltip(null);
      canvas.style.cursor = 'default';
    };

    canvas.addEventListener('mousemove', handleMouseMove);
    canvas.addEventListener('mouseleave', handleMouseLeave);

    // 绘制点阵世界底图（高科技雷达感）
    const drawDotGrid = () => {
      const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
      const gridDotColor = isDark ? 'rgba(255, 255, 255, 0.12)' : 'rgba(0, 0, 0, 0.08)';
      const activeDotColor = isDark ? 'rgba(255, 255, 255, 0.4)' : 'rgba(0, 0, 0, 0.35)';

      const destSet = new Set(entries.map(([c]) => c));

      // 渲染世界各国点阵
      for (const [code, [lon, lat]] of Object.entries(GEO_COUNTRY_COORDS)) {
        let d = lon - origin.lon;
        while (d > 180) d -= 360;
        while (d < -180) d += 360;
        const [x, y] = proj(origin.lon + d, lat);

        ctx.beginPath();
        const isTarget = destSet.has(code);
        ctx.arc(x, y, isTarget ? 2 : 1, 0, Math.PI * 2);
        ctx.fillStyle = isTarget ? activeDotColor : gridDotColor;
        ctx.fill();
      }

      // 绘制经纬网格辅助虚线
      ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.04)' : 'rgba(0, 0, 0, 0.03)';
      ctx.lineWidth = 1;
      ctx.setLineDash([3, 6]);
      for (let y = 40; y < H; y += 50) {
        ctx.beginPath();
        ctx.moveTo(0, y);
        ctx.lineTo(W, y);
        ctx.stroke();
      }
      ctx.setLineDash([]);
    };

    // 主动画循环
    const render = (time: number) => {
      const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
      ctx.clearRect(0, 0, W, H);

      // 1. 底图点阵
      drawDotGrid();

      // 2. 绘制静态弧线与动态粒子
      for (const arc of arcs) {
        // 静态航线
        ctx.beginPath();
        ctx.moveTo(arc.ox, arc.oy);
        ctx.quadraticCurveTo(arc.cpx, arc.cpy, arc.dx, arc.dy);
        ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.18)' : 'rgba(0, 0, 0, 0.12)';
        ctx.lineWidth = 1;
        ctx.stroke();

        // 动态飞行光点
        const progress = ((time + arc.period * 0.2) % arc.period) / arc.period;
        const m = 1 - progress;
        const px = m * m * arc.ox + 2 * m * progress * arc.cpx + progress * progress * arc.dx;
        const py = m * m * arc.oy + 2 * m * progress * arc.cpy + progress * progress * arc.dy;

        ctx.beginPath();
        ctx.arc(px, py, 2.5, 0, Math.PI * 2);
        ctx.fillStyle = isDark ? '#ffffff' : '#000000';
        ctx.shadowColor = isDark ? 'rgba(255, 255, 255, 0.8)' : 'rgba(0, 0, 0, 0.5)';
        ctx.shadowBlur = 4;
        ctx.fill();
        ctx.shadowBlur = 0;

        // 目标国家圆点与脉冲光晕
        ctx.beginPath();
        ctx.arc(arc.dx, arc.dy, 3 + arc.ratio * 2.5, 0, Math.PI * 2);
        ctx.fillStyle = isDark ? '#ffffff' : '#000000';
        ctx.fill();

        // 扩散光圈
        const pulse = (time % 1600) / 1600;
        ctx.beginPath();
        ctx.arc(arc.dx, arc.dy, (4 + arc.ratio * 2) + pulse * 10, 0, Math.PI * 2);
        ctx.strokeStyle = isDark ? `rgba(255, 255, 255, ${0.6 * (1 - pulse)})` : `rgba(0, 0, 0, ${0.4 * (1 - pulse)})`;
        ctx.lineWidth = 1;
        ctx.stroke();

        // 国家代码标签
        ctx.font = '10px var(--font-mono)';
        ctx.fillStyle = isDark ? 'rgba(255, 255, 255, 0.7)' : 'rgba(0, 0, 0, 0.65)';
        ctx.fillText(arc.code, arc.dx + 6, arc.dy + 3);
      }

      // 3. 绘制起点（当前调度核心中心）
      ctx.beginPath();
      ctx.arc(ox, oy, 4, 0, Math.PI * 2);
      ctx.fillStyle = isDark ? '#ffffff' : '#000000';
      ctx.fill();

      ctx.beginPath();
      ctx.arc(ox, oy, 8, 0, Math.PI * 2);
      ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.3)' : 'rgba(0, 0, 0, 0.2)';
      ctx.stroke();

      animId = requestAnimationFrame(render);
    };

    animId = requestAnimationFrame(render);

    return () => {
      cancelAnimationFrame(animId);
      ro.disconnect();
      canvas.removeEventListener('mousemove', handleMouseMove);
      canvas.removeEventListener('mouseleave', handleMouseLeave);
    };
  }, [distribution]);

  return (
    <div
      ref={containerRef}
      style={{
        position: 'relative',
        width: '100%',
        height: '320px',
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        overflow: 'hidden'
      }}
    >
      <div style={{
        position: 'absolute',
        top: '12px',
        left: '16px',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        zIndex: 10,
        pointerEvents: 'none'
      }}>
        <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-primary)' }}>
          全球节点雷达与航线拓扑 (Global Node Radar Topology)
        </span>
        <span className="badge" style={{ fontSize: '10px' }}>
          实时经纬度墨卡托投影
        </span>
      </div>

      <div style={{
        position: 'absolute',
        bottom: '10px',
        left: '16px',
        fontSize: '11px',
        color: 'var(--text-secondary)',
        zIndex: 10,
        pointerEvents: 'none'
      }}>
        ● 调度中心 (时区基准) · 发射光斑表示测活航线 · 悬停可查看目标国家详情
      </div>

      <canvas ref={canvasRef} style={{ display: 'block' }} />

      {tooltip && (
        <div style={{
          position: 'absolute',
          left: `${tooltip.x + 12}px`,
          top: `${tooltip.y - 12}px`,
          background: 'var(--surface-0)',
          color: 'var(--text-primary)',
          border: '1px solid var(--border-black)',
          padding: '4px 8px',
          fontSize: '11px',
          fontFamily: 'var(--font-mono)',
          pointerEvents: 'none',
          zIndex: 100,
          whiteSpace: 'nowrap'
        }}>
          {tooltip.text}
        </div>
      )}
    </div>
  );
};
