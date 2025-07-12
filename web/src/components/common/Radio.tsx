import React from 'react';
import { useSettings } from '../../contexts/SettingsContext';
import { AppConfigChildItem } from '../../types';

interface RadioProps {
  value: string;
  options: AppConfigChildItem[];
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

const Radio: React.FC<RadioProps> = ({
   value,
   options,
   onChange,
   disabled = false,
   error,
   className = '',
   labelClassName = '',
   id,
   label,
   helpText,
}) => {
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  return (
    <div className="mb-4">
      <label className={`block text-sm font-medium text-gray-700 mb-2 ${labelClassName}`}>
        {label}
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
