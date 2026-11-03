import React, { useState, useRef, useEffect } from 'react';
import { HelpCircle, X } from 'lucide-react';

export interface InfoTooltipProps {
  title: string;
  desc: string;
  principle?: string;
  recommend?: string;
}

export const InfoTooltip: React.FC<InfoTooltipProps> = ({
  title,
  desc,
  principle,
  recommend,
}) => {
  const [visible, setVisible] = useState(false);
  const [position, setPosition] = useState<'top' | 'bottom'>('top');
  const [align, setAlign] = useState<'center' | 'left' | 'right'>('center');
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const timerRef = useRef<any>(null);

  // 视口边界碰撞检测，确保浮层不被屏幕边缘裁切
  const updatePosition = () => {
    if (!triggerRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    const spaceAbove = rect.top;
    const spaceBelow = window.innerHeight - rect.bottom;

    // 默认上方，如果上方空间小于 160px 且下方空间充裕，自动向下翻转
    if (spaceAbove < 160 && spaceBelow > spaceAbove) {
      setPosition('bottom');
    } else {
      setPosition('top');
    }

    // 水平方向：靠右边界时向左对齐，靠左边界时向右对齐
    if (rect.left < 140) {
      setAlign('left');
    } else if (window.innerWidth - rect.right < 140) {
      setAlign('right');
    } else {
      setAlign('center');
    }
  };

  const showTooltip = () => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      updatePosition();
      setVisible(true);
    }, 100);
  };

  const hideTooltip = () => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      setVisible(false);
    }, 200);
  };

  const toggleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!visible) updatePosition();
    setVisible(prev => !prev);
  };

  // 点击外部自动关闭
  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      if (triggerRef.current && !triggerRef.current.contains(e.target as Node)) {
        setVisible(false);
      }
    };
    if (visible) {
      document.addEventListener('mousedown', handleOutsideClick);
    }
    return () => {
      document.removeEventListener('mousedown', handleOutsideClick);
    };
  }, [visible]);

  return (
    <span style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', marginLeft: '4px' }}>
      <button
        ref={triggerRef}
        type="button"
        onClick={toggleClick}
        onMouseEnter={showTooltip}
        onMouseLeave={hideTooltip}
        aria-label={`说明: ${title}`}
        style={{
          background: 'transparent',
          border: 'none',
          padding: '2px',
          margin: 0,
          cursor: 'pointer',
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: visible ? 'var(--text-primary)' : 'var(--text-secondary)',
          opacity: visible ? 1 : 0.65,
          transition: 'all 0.15s ease',
          verticalAlign: 'middle',
          borderRadius: '50%'
        }}
      >
        <HelpCircle size={12} strokeWidth={2} />
      </button>

      {visible && (
        <div
          onMouseEnter={() => clearTimeout(timerRef.current)}
          onMouseLeave={hideTooltip}
          style={{
            position: 'absolute',
            [position === 'top' ? 'bottom' : 'top']: 'calc(100% + 6px)',
            ...(align === 'center'
              ? { left: '50%', transform: 'translateX(-50%)' }
              : align === 'left'
              ? { left: 0 }
              : { right: 0 }),
            width: '270px',
            background: 'var(--surface-0)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border-black)',
            boxShadow: '0 4px 18px rgba(0, 0, 0, 0.16)',
            padding: '10px 12px',
            fontSize: '11px',
            lineHeight: 1.5,
            zIndex: 1000,
            pointerEvents: 'auto',
            textAlign: 'left'
          }}
        >
          {/* 标题 */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border)', paddingBottom: '6px', marginBottom: '6px' }}>
            <span style={{ fontWeight: 600, fontSize: '12px', color: 'var(--text-primary)' }}>
              {title}
            </span>
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); setVisible(false); }}
              style={{ background: 'transparent', border: 'none', padding: '1px', cursor: 'pointer', color: 'var(--text-secondary)' }}
            >
              <X size={11} />
            </button>
          </div>

          {/* 1. 功能作用 */}
          <div style={{ marginBottom: '6px', color: 'var(--text-primary)' }}>
            <span style={{ fontWeight: 500, color: 'var(--text-secondary)' }}>【作用】</span>
            {desc}
          </div>

          {/* 2. 技术原理 */}
          {principle && (
            <div style={{ marginBottom: '6px', color: 'var(--text-secondary)', fontSize: '10.5px' }}>
              <span style={{ fontWeight: 500 }}>【原理】</span>
              {principle}
            </div>
          )}

          {/* 3. 建议配置 */}
          {recommend && (
            <div style={{
              background: 'var(--surface-1)',
              padding: '4px 6px',
              borderLeft: '2px solid var(--border-black)',
              fontSize: '10.5px',
              color: 'var(--text-primary)',
              marginTop: '4px'
            }}>
              <span style={{ fontWeight: 500 }}>【建议】</span>
              {recommend}
            </div>
          )}
        </div>
      )}
    </span>
  );
};
