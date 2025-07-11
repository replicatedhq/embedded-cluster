import React from 'react';
import { useSettings } from '../../contexts/SettingsContext';

interface CheckboxProps {
  checked: boolean;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  disabled?: boolean;
  error?: string;
  className?: string;
  labelClassName?: string;

  // props shared through ConfigItem
  id?: string;
  label?: string;
  dataTestId?: string;
  helpText?: string;
}

const Checkbox: React.FC<CheckboxProps> = ({
  checked,
  onChange,
  disabled = false,
  error,
  className = '',
  labelClassName = '',
  id,
  label,
  dataTestId,
  helpText,
}) => {
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  return (
    <div className="mb-4">
      <div className="flex items-center space-x-3">
        <input
          id={id}
          type="checkbox"
          checked={checked}
          onChange={onChange}
          disabled={disabled}
          className={`h-4 w-4 focus:ring-offset-2 border-gray-300 rounded ${className}`}
          data-testid={dataTestId}
          style={{
            color: themeColor,
            '--tw-ring-color': themeColor,
            accentColor: themeColor,
          } as React.CSSProperties}
        />
        <label htmlFor={id} className={`text-sm text-gray-700 ${labelClassName}`}>
          {label}
        </label>
      </div>
      {error && <p className="mt-1 text-sm text-red-500">{error}</p>}
      {helpText && !error && <p className="mt-1 text-sm text-gray-500">{helpText}</p>}
    </div>
  );
};

export default Checkbox;
