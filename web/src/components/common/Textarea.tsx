import React, { ChangeEvent, CSSProperties } from 'react';
import { useSettings } from '../../contexts/SettingsContext';
import HelpText from './HelpText';

interface TextareaProps {
  id: string;
  label: string;
  helpText?: string;
  defaultValue?: string;
  error?: string;
  required?: boolean;
  value: string;
  rows?: number;
  placeholder?: string;
  onChange: (e: ChangeEvent<HTMLTextAreaElement>) => void;
  disabled?: boolean;
  className?: string;
  labelClassName?: string;
  dataTestId?: string;
}

const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(({
  id,
  label,
  helpText,
  defaultValue,
  error,
  required = false,
  value,
  rows = 4,
  placeholder = '',
  onChange,
  disabled = false,
  className = '',
  labelClassName = '',
  dataTestId,
}, ref) => {
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  return (
    <div className={`mb-4 ${className}`}>
      <label htmlFor={id} className={`block text-sm font-medium text-gray-700 mb-1 ${labelClassName}`}>
        {label}
        {required && <span className="text-red-500 ml-1">*</span>}
      </label>
      <textarea
        ref={ref}
        id={id}
        value={value}
        onChange={onChange}
        rows={rows}
        placeholder={placeholder}
        disabled={disabled}
        required={required}
        className={`w-full px-3 py-2 border ${
          error ? 'border-red-500' : 'border-gray-300'
        } rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-offset-2 ${
          disabled ? 'bg-gray-100 text-gray-500' : 'bg-white'
        }`}
        style={{
          '--tw-ring-color': themeColor,
          '--tw-ring-offset-color': themeColor,
        } as CSSProperties}
        data-testid={dataTestId}
      />
      {error && <p className="mt-1 text-sm text-red-500">{error}</p>}
      <HelpText helpText={helpText} defaultValue={defaultValue} error={error} />
    </div>
  );
});

Textarea.displayName = 'Textarea';

export default Textarea;
