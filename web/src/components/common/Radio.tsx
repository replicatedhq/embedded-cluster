import React from 'react';
import { useSettings } from '../../contexts/SettingsContext';
import { AppConfigChildItem } from '../../types';

interface RadioProps {
  id: string;
  label: string;
  helpText?: string;
  error?: string;
  required?: boolean;
  value: string;
  options: AppConfigChildItem[];
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  disabled?: boolean;
  className?: string;
  labelClassName?: string;
}

const Radio: React.FC<RadioProps> = ({
  id,
  label,
  helpText,
  error,
  required,
  value,
  options,
  onChange,
  disabled = false,
  className = '',
  labelClassName = '',
}) => {
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  return (
    <div className="mb-4">
      <label className={`block text-sm font-medium text-gray-700 mb-2 ${labelClassName}`}>
        {label}
        {required && <span className="text-red-500 ml-1">*</span>}
      </label>
      <div className="space-y-2">
        {options.map(option => (
          <div key={option.name} className="flex items-center">
            <input
              type="radio"
              id={option.name}
              name={id}
              value={option.name}
              checked={value === option.name}
              onChange={onChange}
              disabled={disabled}
              className={`h-4 w-4 focus:ring-offset-2 border-gray-300 ${className}`}
              data-testid={`radio-input-${option.name}`}
              style={{
                color: themeColor,
                '--tw-ring-color': themeColor,
                accentColor: themeColor,
              } as React.CSSProperties}
            />
            <label htmlFor={option.name} className="ml-3 text-sm text-gray-700">
              {option.title}
            </label>
          </div>
        ))}
      </div>
      {error && <p className="mt-1 text-sm text-red-500">{error}</p>}
      {helpText && !error && <p className="mt-1 text-sm text-gray-500">{helpText}</p>}
    </div>
  );
};

export default Radio;
