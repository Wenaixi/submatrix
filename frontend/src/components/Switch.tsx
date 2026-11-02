import React from 'react';

interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  label?: string;
}

export const Switch: React.FC<SwitchProps> = ({ checked, onChange, disabled, label }) => {
  return (
    <label style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', cursor: disabled ? 'not-allowed' : 'pointer', userSelect: 'none' }}>
      <span className="switch-wrap">
        <input
          type="checkbox"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
        />
        <span className="switch-track">
          <span className="switch-thumb" />
        </span>
      </span>
      {label && <span style={{ fontSize: '13px', color: checked ? 'var(--text-primary)' : 'var(--text-secondary)' }}>{label}</span>}
    </label>
  );
};
