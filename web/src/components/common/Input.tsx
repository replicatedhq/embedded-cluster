import React, { useState } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { useSettings } from '../../contexts/SettingsContext';
import HelpText from './HelpText';

interface InputProps {
  id: string;
  label: string;
  helpText?: string;
  defaultValue?: string;
  error?: string;
  required?: boolean;
  type?: string;
  allowShowPassword?: boolean; // property to control password visibility toggle
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

const Input = React.forwardRef<HTMLInputElement, InputProps>(({
  id,
  label,
  helpText,
  defaultValue,
  error,
  required,
  type = 'text',
  allowShowPassword = true,
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
}, ref) => {
  const { settings } = useSettings();
  const [showPassword, setShowPassword] = useState(false);
  const themeColor = settings.themeColor;
  const isPasswordField = type === 'password';
  // For password fields, toggle between 'text' and 'password' types when showPassword is enabled
  const inputType = isPasswordField && showPassword ? 'text' : type;

  const togglePasswordVisibility = () => {
    setShowPassword(!showPassword);
  }

  return (
    <div className={`mb-4 ${className}`}>
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
          ref={ref}
          id={id}
          type={inputType}
          value={value}
          onChange={onChange}
          onKeyDown={onKeyDown}
          onFocus={onFocus}
          placeholder={placeholder}
          disabled={disabled}
          required={required}
          className={`w-full px-3 py-2 ${icon ? 'pl-10' : ''} border ${error ? 'border-red-500' : 'border-gray-300'
            } rounded-md shadow-sm focus:outline-none ${disabled ? 'bg-gray-100 text-gray-500' : 'bg-white'
            }`}
          style={{
            '--tw-ring-color': themeColor,
            '--tw-ring-offset-color': themeColor,
          } as React.CSSProperties}
          data-testid={dataTestId}
        />
        {isPasswordField && allowShowPassword &&
          <button
            type="button"
            onClick={togglePasswordVisibility}
            className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600 transition-colors"
            tabIndex={-1}
            data-testid={dataTestId ? `password-visibility-toggle-${dataTestId}` : "password-visibility-toggle"}
          >
            {showPassword ? (
              <EyeOff className="w-5 h-5" data-testid={dataTestId ? `eye-off-icon-${dataTestId}` : "eye-off-icon"} />
            ) : (
              <Eye className="w-5 h-5" data-testid={dataTestId ? `eye-icon-${dataTestId}` : "eye-icon"} />
            )}
          </button>
        }
      </div>
      {error && <p className="mt-1 text-sm text-red-500">{error}</p>}
      <HelpText helpText={helpText} defaultValue={defaultValue} error={error} />
    </div>
  );
});

Input.displayName = 'Input';

export default Input;
