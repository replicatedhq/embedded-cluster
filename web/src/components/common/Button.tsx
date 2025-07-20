import React from 'react';
import { useSettings } from '../../contexts/hooks/useSettings';

interface ButtonProps {
  children: React.ReactNode;
  onClick?: () => void;
  type?: 'button' | 'submit' | 'reset';
  variant?: 'primary' | 'secondary' | 'outline' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  className?: string;
  icon?: React.ReactNode;
  dataTestId?: string;
}

const Button: React.FC<ButtonProps> = ({
  children,
  onClick,
  type = 'button',
  variant = 'primary',
  size = 'md',
  disabled = false,
  className = '',
  icon,
  dataTestId
}) => {
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const baseStyles = 'inline-flex items-center justify-center font-medium transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-offset-2 rounded-md';

  const variantStyles = {
    primary: `bg-[${themeColor}] hover:bg-[${themeColor}] text-white focus:ring-[${themeColor}]`,
    secondary: 'bg-[#3498DB] hover:bg-[#2980B9] text-white focus:ring-[#3498DB]',
    outline: `border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 focus:ring-[${themeColor}]`,
    danger: 'bg-red-500 hover:bg-red-600 text-white focus:ring-red-500',
  };

  const sizeStyles = {
    sm: 'px-3 py-1.5 text-sm',
    md: 'px-4 py-2 text-sm',
    lg: 'px-5 py-2.5 text-base',
  };

  const disabledStyles = disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer';

  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={`${baseStyles} ${variantStyles[variant]} ${sizeStyles[size]} ${disabledStyles} ${className}`}
      style={{
        '--theme-color': themeColor,
        backgroundColor: variant === 'primary' ? themeColor : undefined,
        borderColor: variant === 'outline' ? 'currentColor' : undefined,
      } as React.CSSProperties}
      data-testid={dataTestId}
    >
      {icon && <span className="mr-2">{icon}</span>}
      {children}
    </button>
  );
};

export default Button;