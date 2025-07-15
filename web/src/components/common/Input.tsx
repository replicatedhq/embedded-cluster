import React from 'react';
import { useSettings } from '../../contexts/SettingsContext';

interface InputProps {
  id: string;
  label: string;
  renderedHelpText?: React.ReactNode;
  error?: string;
  required?: boolean;
  type?: string;
  value: string;
  icon?: React.ReactNode;
  placeholder?: string;
  onKeyDown?: (e: React.KeyboardEvent) => void;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onFocus?: (e: React.FocusEvent<HTMLInputElement>) => void;
  disabled?: boolean;
  className?: string;
  labelClassName?: string;
  dataTestId?: string;
}

const Input: React.FC<InputProps> = ({
  id,
  label,
  renderedHelpText,
  error,
  required,
  type = 'text',
  value,
  icon,
  placeholder = '',
  onKeyDown,
  onChange,
  onFocus,
  disabled = false,
  className = '',
  labelClassName = '',
  dataTestId,
}) => {
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  return (
    <div className="mb-4">
      <label htmlFor={id} className={`block text-sm font-medium text-gray-700 mb-1 ${labelClassName}`}>
        {label}
        {required && <span className="text-red-500 ml-1">*</span>}
      </label>
      <div className="relative">
        {icon && (
          <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-gray-400">
            {icon}
          </div>
        )}
        <input
          id={id}
          type={type}
          value={value}
          onChange={onChange}
          onKeyDown={onKeyDown}
          onFocus={onFocus}
          placeholder={placeholder}
          disabled={disabled}
          required={required}
          className={`w-full px-3 py-2 ${icon ? 'pl-10' : ''} border ${
            error ? 'border-red-500' : 'border-gray-300'
          } rounded-md shadow-sm focus:outline-none ${
            disabled ? 'bg-gray-100 text-gray-500' : 'bg-white'
          } ${className}`}
          style={{
            '--tw-ring-color': themeColor,
            '--tw-ring-offset-color': themeColor,
          } as React.CSSProperties}
          data-testid={dataTestId}
        />
      </div>
      {error && <p className="mt-1 text-sm text-red-500">{error}</p>}
      {renderedHelpText}
    </div>
  );
};

export default Input;