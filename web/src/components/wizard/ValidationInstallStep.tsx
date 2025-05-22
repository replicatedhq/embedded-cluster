import React, { useState, useEffect } from 'react';
import Card from '../common/Card';
import { useConfig } from '../../contexts/ConfigContext';
import { ExternalLink } from 'lucide-react';

const ValidationInstallStep: React.FC = () => {
  const { config } = useConfig();
  const [showAdminLink, setShowAdminLink] = useState(false);
  const [adminConsoleUrl, setAdminConsoleUrl] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const installCluster = async () => {
      try {
        const response = await fetch('/api/install/phase/set-config', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            // Include auth credentials if available from localStorage or another source
            ...(localStorage.getItem('auth') && {
              'Authorization': `${localStorage.getItem('auth')}`,
            }),
          },
        });

        if (!response.ok) {
          throw new Error(`Installation failed: ${response.statusText}`);
        }

        const data = await response.json();
        setAdminConsoleUrl(data.adminConsoleUrl);
        setShowAdminLink(true);
        setIsLoading(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to install cluster');
        setIsLoading(false);
      }
    };

    installCluster();
  }, [config.adminPassword]);

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-12">
          <h2 className="text-2xl font-bold text-gray-900 mb-4">Installing Embedded Cluster</h2>
          
          {isLoading && (
            <p className="text-xl text-gray-600 mb-8">
              Please wait while we complete the installation...
            </p>
          )}

          {error && (
            <div className="text-red-600 mb-8">
              <p className="text-xl">Installation Error</p>
              <p>{error}</p>
            </div>
          )}

          {showAdminLink && adminConsoleUrl && (
            <a
              href={adminConsoleUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              Visit Admin Console
              <ExternalLink className="ml-2 -mr-1 h-5 w-5" />
            </a>
          )}
        </div>
      </Card>
    </div>
  );
};

export default ValidationInstallStep;