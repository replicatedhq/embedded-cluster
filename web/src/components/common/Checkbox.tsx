import React from 'react';
import { useSettings } from '../../contexts/hooks/useSettings';
import HelpText from './HelpText';

interface CheckboxProps {
  id: string;
  label: string;
  helpText?: string;
  error?: string;
  required?: boolean;
  checked: boolean;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  disabled?: boolean;
  className?: string;
  labelClassName?: string;
  dataTestId?: string;
}

const Checkbox: React.FC<CheckboxProps> = ({
  id,
  label,
  helpText,
  error,
  required,
  checked,
  onChange,
  disabled = false,
  className = '',
  labelClassName = '',
  dataTestId,
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
          {required && <span className="text-red-500 ml-1">*</span>}
        </label>
      </div>
      {error && <p className="mt-1 text-sm text-red-500">{error}</p>}
      <HelpText helpText={helpText} error={error} />
    </div>
  );
};

export default Checkbox;
