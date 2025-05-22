import React from 'react';

export const GiteaLogo: React.FC<{ className?: string }> = ({ className = 'w-6 h-6' }) => {
  return (
    <img 
      src="https://upload.wikimedia.org/wikipedia/commons/b/bb/Gitea_Logo.svg" 
      alt="Gitea Logo"
      className={className}
    />
  );
};