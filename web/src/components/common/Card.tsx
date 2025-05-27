import React from 'react';

interface CardProps {
  children: React.ReactNode;
  title?: string;
  className?: string;
  titleClassName?: string;
  contentClassName?: string;
  footer?: React.ReactNode;
  footerClassName?: string;
}

const Card: React.FC<CardProps> = ({
  children,
  title,
  className = '',
  titleClassName = '',
  contentClassName = '',
  footer,
  footerClassName = '',
}) => {
  return (
    <div className={`bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden ${className}`}>
      {title && (
        <div className={`px-6 py-4 border-b border-gray-200 bg-gray-50 ${titleClassName}`}>
          <h3 className="text-lg font-medium text-gray-900">{title}</h3>
        </div>
      )}
      <div className={`px-6 py-5 ${contentClassName}`}>{children}</div>
      {footer && (
        <div className={`px-6 py-4 border-t border-gray-200 bg-gray-50 ${footerClassName}`}>
          {footer}
        </div>
      )}
    </div>
  );
};

export default Card;