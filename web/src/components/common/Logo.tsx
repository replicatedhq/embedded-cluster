import React from 'react';

import { useBranding } from '../../contexts/BrandingContext';

export const AppIcon: React.FC<{ className?: string }> = ({ className = 'w-6 h-6' }) => {
  const { branding } = useBranding();
  if (!branding?.appIcon) {
    return <div className="h-6 w-6 bg-gray-200 rounded"></div>;
  }
  return (
    <img
      src={branding?.appIcon}
      alt="App Icon"
      className={className}
    />
  );
};
