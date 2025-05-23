import React, { useState, useEffect } from 'react';
import Card from '../common/Card';
import Button from '../common/Button';
import { useConfig } from '../../contexts/ConfigContext';
import { useWizardMode } from '../../contexts/WizardModeContext';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import LinuxSetup from './setup/LinuxSetup';
import KubernetesSetup from './setup/KubernetesSetup';

interface SetupStepProps {
  onNext: () => void;
  onBack: () => void;
}

const SetupStep: React.FC<SetupStepProps> = ({ onNext, onBack }) => {
  const { config, updateConfig, prototypeSettings } = useConfig();
  const { text } = useWizardMode();
  const [showAdvanced, setShowAdvanced] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [availableNetworkInterfaces, setAvailableNetworkInterfaces] = useState<string[]>([]);

  useEffect(() => {
    const fetchData = async () => {
      try {
        // Fetch cluster config
        const installResponse = await fetch('/api/install', {
          headers: {
            ...(localStorage.getItem('auth') && {
              'Authorization': `${localStorage.getItem('auth')}`,
            }),
          },
        });

        if (installResponse.ok) {
          const install = await installResponse.json();
          updateConfig(install.config);

          // TODO: remove this once it's a dropdown
          setAvailableNetworkInterfaces([install.config.networkInterface]);
        }

        // TODO: make this a
        // Fetch network interfaces
        // const interfacesResponse = await fetch('/api/host-network-interfaces', {
        //   headers: {
        //     ...(localStorage.getItem('auth') && {
        //       'Authorization': `${localStorage.getItem('auth')}`,
        //     }),
        //   },
        // });

        // if (interfacesResponse.ok) {
        //   const interfacesData = await interfacesResponse.json();
        //   setAvailableNetworkInterfaces(interfacesData.networkInterfaces || []);
        // }
      } catch (err) {
        console.error('Failed to fetch data:', err);
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, value } = e.target;
    if (id === 'adminConsolePort') {
      updateConfig({ adminConsolePort: parseInt(value) });
    } else if (id === 'localArtifactMirrorPort') {
      updateConfig({ localArtifactMirrorPort: parseInt(value) });
    } else {
      updateConfig({ [id]: value });
    }
  };

  const handleSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const { id, value } = e.target;
    updateConfig({ [id]: value });
  };

  const handleNext = async () => {
    setIsSubmitting(true);
    setError(null);

    try {
      // Make the POST request to the cluster-setup endpoint
      const response = await fetch('/api/install/config', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          // Include auth credentials if available from localStorage or another source
          ...(localStorage.getItem('auth') && {
            'Authorization': `${localStorage.getItem('auth')}`,
          }),
        },
        body: JSON.stringify(config),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || `Server responded with ${response.status}`);
      }

      // Call the original onNext function to proceed to the next step
      onNext();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to setup cluster');
      console.error('Cluster setup failed:', err);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.setupTitle}</h2>
          <p className="text-gray-600 mt-1">
            {prototypeSettings.clusterMode === 'embedded' 
              ? 'Configure the installation settings.'
              : text.setupDescription}
          </p>
        </div>

        {isLoading ? (
          <div className="py-4 text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-2 text-gray-600">Loading configuration...</p>
          </div>
        ) : prototypeSettings?.clusterMode === 'embedded' ? (
          <LinuxSetup
            config={config}
            prototypeSettings={prototypeSettings}
            showAdvanced={showAdvanced}
            onShowAdvancedChange={setShowAdvanced}
            onInputChange={handleInputChange}
            onSelectChange={handleSelectChange}
            availableNetworkInterfaces={availableNetworkInterfaces}
          />
        ) : (
          <KubernetesSetup
            config={config}
            onInputChange={handleInputChange}
          />
        )}

        {error && (
          <div className="mt-4 p-3 bg-red-50 text-red-700 rounded-md">
            {error}
          </div>
        )}
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={onBack} icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button 
          onClick={handleNext}
          icon={<ChevronRight className="w-5 h-5" />}
          disabled={isSubmitting || isLoading}
        >
          {isSubmitting ? 'Setting up...' : text.nextButtonText}
        </Button>
      </div>
    </div>
  );
};

export default SetupStep;