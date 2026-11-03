import React, { useEffect, useRef, useState } from 'react';
import worldLandRings from '../assets/world-land.json';

// 国家中心经纬度坐标字典 [lon, lat]
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

// 预先过滤剔除南极极地冰盖（仅保留宜居大陆板块）
const ACTIVE_LAND_RINGS = (worldLandRings as number[][][]).filter((ring: number[][]) => {
  const avgLat = ring.reduce((sum, p) => sum + p[1], 0) / ring.length;
  return avgLat > -58;
});

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
    const dpr = window.devicePixelRatio || 1;

    // 调度中心：中国/香港沿海核心
    const origin = { lon: 114.17, lat: 22.32 };

    const getDimensions = () => {
      const W = container.clientWidth || 960;
      const H = 380;
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

    // 标准国际墨卡托等距经纬度投影
    const proj = (lon: number, lat: number) => {
      const mx = 24, my = 24;
      const x = mx + ((lon + 180) / 360) * (W - 2 * mx);
      const y = my + ((85 - lat) / 143) * (H - 2 * my);
      return [x, y];
    };

    // 目标国家数据与最大值计算
    const entries = Object.entries(distribution || {}).filter(([k]) => GEO_COUNTRY_COORDS[k]);
    const maxCount = Math.max(...entries.map(([, v]) => v), 1);

    // 航线计算
    const [ox, oy] = proj(origin.lon, origin.lat);
    const arcs = entries.map(([code, count], idx) => {
      const [lon, lat] = GEO_COUNTRY_COORDS[code];
      const [dx, dy] = proj(lon, lat);
      const ratio = count / maxCount;
      const dist = Math.hypot(dx - ox, dy - oy);
      const lift = Math.min(dist * 0.35, H * 0.4);
      const cpx = (ox + dx) / 2;
      const cpy = (oy + dy) / 2 - lift;
      return {
        code,
        count,
        ratio,
        ox, oy,
        cpx, cpy,
        dx, dy,
        period: 2600 + idx * 160
      };
    });

    // 鼠标悬停交互
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

    // 绘制真实各大洲版图
    const drawContinents = (isDark: boolean) => {
      ctx.save();
      ctx.beginPath();

      for (const ring of ACTIVE_LAND_RINGS) {
        if (ring.length < 3) continue;
        let lastX = 0;
        for (let i = 0; i < ring.length; i++) {
          const [lon, lat] = ring[i];
          const [px, py] = proj(lon, lat);

          if (i === 0) {
            ctx.moveTo(px, py);
          } else {
            // 防跨日界线撕裂，平滑断笔
            if (Math.abs(px - lastX) > W * 0.45) {
              ctx.moveTo(px, py);
            } else {
              ctx.lineTo(px, py);
            }
          }
          lastX = px;
        }
      }

      // 陆地填充与海岸线高精度描边
      ctx.fillStyle = isDark ? 'rgba(255, 255, 255, 0.08)' : 'rgba(0, 0, 0, 0.05)';
      ctx.fill();
      ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.45)' : 'rgba(0, 0, 0, 0.35)';
      ctx.lineWidth = 0.9;
      ctx.lineJoin = 'round';
      ctx.stroke();

      ctx.restore();
    };

    // 绘制经纬网格线
    const drawGrid = (isDark: boolean) => {
      ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.05)' : 'rgba(0, 0, 0, 0.04)';
      ctx.lineWidth = 1;
      ctx.setLineDash([3, 6]);

      for (let y = 60; y < H; y += 70) {
        ctx.beginPath();
        ctx.moveTo(24, y);
        ctx.lineTo(W - 24, y);
        ctx.stroke();
      }
      for (let x = 80; x < W; x += 120) {
        ctx.beginPath();
        ctx.moveTo(x, 24);
        ctx.lineTo(x, H - 24);
        ctx.stroke();
      }
      ctx.setLineDash([]);
    };

    // 动画主循环
    const render = (time: number) => {
      const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
      ctx.clearRect(0, 0, W, H);

      // 1. 经纬线
      drawGrid(isDark);

      // 2. 真实地球大陆版图
      drawContinents(isDark);

      // 3. 动态贝塞尔航线与光点
      for (const arc of arcs) {
        // 航线弧线
        ctx.beginPath();
        ctx.moveTo(arc.ox, arc.oy);
        ctx.quadraticCurveTo(arc.cpx, arc.cpy, arc.dx, arc.dy);
        ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.25)' : 'rgba(0, 0, 0, 0.18)';
        ctx.lineWidth = 1;
        ctx.stroke();

        // 飞行粒子
        const progress = ((time + arc.period * 0.2) % arc.period) / arc.period;
        const m = 1 - progress;
        const px = m * m * arc.ox + 2 * m * progress * arc.cpx + progress * progress * arc.dx;
        const py = m * m * arc.oy + 2 * m * progress * arc.cpy + progress * progress * arc.dy;

        ctx.beginPath();
        ctx.arc(px, py, 2.5, 0, Math.PI * 2);
        ctx.fillStyle = isDark ? '#ffffff' : '#000000';
        ctx.shadowColor = isDark ? 'rgba(255, 255, 255, 0.9)' : 'rgba(0, 0, 0, 0.6)';
        ctx.shadowBlur = 4;
        ctx.fill();
        ctx.shadowBlur = 0;

        // 目标国家圆点
        ctx.beginPath();
        ctx.arc(arc.dx, arc.dy, 3.5 + arc.ratio * 2.5, 0, Math.PI * 2);
        ctx.fillStyle = isDark ? '#ffffff' : '#000000';
        ctx.fill();

        // 呼吸脉冲光圈
        const pulse = (time % 1600) / 1600;
        ctx.beginPath();
        ctx.arc(arc.dx, arc.dy, (4 + arc.ratio * 2) + pulse * 12, 0, Math.PI * 2);
        ctx.strokeStyle = isDark
          ? `rgba(255, 255, 255, ${0.7 * (1 - pulse)})`
          : `rgba(0, 0, 0, ${0.5 * (1 - pulse)})`;
        ctx.lineWidth = 1.2;
        ctx.stroke();

        // 国家代码标签
        ctx.font = '10px var(--font-mono)';
        ctx.fillStyle = isDark ? 'rgba(255, 255, 255, 0.85)' : 'rgba(0, 0, 0, 0.85)';
        ctx.fillText(`${arc.code} (${arc.count})`, arc.dx + 8, arc.dy + 3);
      }

      // 4. 调度中心基点
      ctx.beginPath();
      ctx.arc(ox, oy, 4.5, 0, Math.PI * 2);
      ctx.fillStyle = isDark ? '#ffffff' : '#000000';
      ctx.fill();

      ctx.beginPath();
      ctx.arc(ox, oy, 9, 0, Math.PI * 2);
      ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.5)' : 'rgba(0, 0, 0, 0.35)';
      ctx.lineWidth = 1;
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
        height: '380px',
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
          全球真实地理地图与节点航线拓扑 (Real World Geography & Radar Arcs)
        </span>
        <span className="badge" style={{ fontSize: '10px' }}>
          真实陆地版图 · 16国连通覆盖
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
        ● 调度中心 · 发射流光光斑表示测活连通链路 · 悬停可查看目标国家详情
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
