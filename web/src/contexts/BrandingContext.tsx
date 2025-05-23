import React, { createContext, useContext, useEffect, useState } from 'react';

interface Branding {
  appTitle: string;
  appIcon?: string;
}

interface BrandingContextType {
  branding: Branding | null;
  isLoading: boolean;
  error: Error | null;
}

const BrandingContext = createContext<BrandingContextType | undefined>(undefined);

export const useBranding = () => {
  const context = useContext(BrandingContext);
  if (!context) {
    throw new Error('useBranding must be used within a BrandingProvider');
  }
  return context;
};

export const BrandingProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [branding, setBranding] = useState<Branding | null>({ appTitle: "App" });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (branding?.appTitle) {
      document.title = branding.appTitle;
    }
  }, [branding?.appTitle]);

  useEffect(() => {
    const fetchBranding = async () => {
      try {
        const response = await fetch('/api/branding', {
          headers: {
            // Include auth credentials if available from localStorage or another source
            ...(localStorage.getItem('auth') && {
              'Authorization': `Bearer ${localStorage.getItem('auth')}`,
            }),
          },
        });
        if (!response.ok) {
          throw new Error('Failed to fetch branding');
        }
        const data = await response.json();
        setBranding(data.branding);
      } catch (err) {
        setError(err instanceof Error ? err : new Error('Failed to fetch branding'));
      } finally {
        setIsLoading(false);
      }
    };

    fetchBranding();
  }, []);

  return (
    <BrandingContext.Provider value={{ branding, isLoading, error }}>
      {children}
    </BrandingContext.Provider>
  );
};
