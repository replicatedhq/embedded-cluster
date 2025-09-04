import React from "react";
import { useSettings } from "../../contexts/SettingsContext";

interface SelectOption {
  value: string;
  label: string;
}

interface SelectProps {
  id: string;
  label: string;
  value: string;
  onChange: (e: React.ChangeEvent<HTMLSelectElement>) => void;
  options: SelectOption[];
  required?: boolean;
  disabled?: boolean;
  error?: string;
  helpText?: string;
  className?: string;
  labelClassName?: string;
  placeholder?: string;
  dataTestId?: string;
}

const Select: React.FC<SelectProps> = ({
  id,
  label,
  value,
  onChange,
  options,
  required = false,
  disabled = false,
  error,
  helpText,
  className = "",
  labelClassName = "",
  placeholder,
  dataTestId,
}) => {
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  return (
    <div className={`mb-4 ${className}`}>
      <label htmlFor={id} className={`block text-sm font-medium text-gray-700 mb-1 ${labelClassName}`}>
        {label}
        {required && <span className="text-red-500 ml-1">*</span>}
      </label>
      <select
        id={id}
        value={value}
        onChange={onChange}
        disabled={disabled}
        required={required}
        className={`w-full px-3 py-2 border ${
          error ? "border-red-500" : "border-gray-300"
        } rounded-md shadow-sm focus:outline-none ${disabled ? "bg-gray-100 text-gray-500" : "bg-white"}`}
        style={
          {
            "--tw-ring-color": themeColor,
            "--tw-ring-offset-color": themeColor,
          } as React.CSSProperties
        }
        data-testid={dataTestId}
      >
        {placeholder && (
          <option value="" disabled>
            {placeholder}
          </option>
        )}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {error && <p className="mt-1 text-sm text-red-500">{error}</p>}
      {helpText && !error && <p className="mt-1 text-sm text-gray-500">{helpText}</p>}
    </div>
  );
};

export default Select;
