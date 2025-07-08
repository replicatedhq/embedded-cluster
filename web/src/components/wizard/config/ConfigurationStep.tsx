import React, { useState, useEffect } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import Card from '../../common/Card';
import Button from '../../common/Button';
import Input from '../../common/Input';
import { useWizard } from '../../../contexts/WizardModeContext';
import { useAuth } from '../../../contexts/AuthContext';
import { useSettings } from '../../../contexts/SettingsContext';
import { ChevronRight, Loader2 } from 'lucide-react';
import { handleUnauthorized } from '../../../utils/auth';
import { AppConfig, AppConfigItem } from '../../../types';

interface ConfigurationStepProps {
  onNext: () => void;
}


const ConfigurationStep: React.FC<ConfigurationStepProps> = ({ onNext }) => {
  const { text, target } = useWizard();
  const { token } = useAuth();
  const { settings } = useSettings();
  const [activeTab, setActiveTab] = useState<string>('');
  const [appConfig, setAppConfig] = useState<AppConfig | null>(null);
  const [changedValues, setChangedValues] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const themeColor = settings.themeColor;

  // Fetch app config from API
  const { isLoading: isConfigLoading, error: getConfigError } = useQuery<AppConfig>({
    queryKey: ['appConfig', target],
    queryFn: async () => {
      const response = await fetch(`/api/${target}/install/app/config`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error('Session expired. Please log in again.');
        }
        throw new Error(errorData.message || 'Failed to fetch app configuration');
      }
      const config = await response.json();
      setAppConfig(config);
      return config;
    },
  });

  // Mutation to save config values
  const { mutate: submitConfigValues } = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/${target}/install/app/config/values`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ values: changedValues }),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error('Session expired. Please log in again.');
        }
        throw new Error(errorData.message || 'Failed to save configuration');
      }

      return response.json();
    },
    onSuccess: () => {
      setSubmitError(null);
      onNext();
    },
    onError: (error: Error) => {
      setSubmitError(error?.message || 'Failed to save configuration');
    },
  });

  // Set active tab to first group when config loads
  useEffect(() => {
    if (appConfig?.spec?.groups && appConfig.spec.groups.length > 0 && !activeTab) {
      setActiveTab(appConfig.spec.groups[0].name);
    }
  }, [appConfig, activeTab]);

  const updateConfigValue = (itemName: string, value: string) => {
    if (!appConfig) return;

    // Update the app config for display
    setAppConfig({
      ...appConfig,
      spec: {
        ...appConfig.spec,
        groups: appConfig.spec.groups.map(group => ({
          ...group,
          items: group.items.map(item =>
            item.name === itemName ? { ...item, value } : item
          )
        }))
      }
    });

    // Update the changed values map
    setChangedValues(prev => {
      const newValues = { ...prev };

      if (value === '') {
        // Remove the item if it's empty
        delete newValues[itemName];
      } else {
        // Add or update the item with the new value
        newValues[itemName] = value;
      }

      return newValues;
    });
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { id, value } = e.target;
    updateConfigValue(id, value);
  };

  const handleCheckboxChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, checked } = e.target;
    updateConfigValue(id, checked ? '1' : '0');
  };

  const renderConfigItem = (item: AppConfigItem) => {
    const key = item.name;
    const value = item.value || item.default || '';

    const commonProps = {
      id: key,
      label: item.title,
      value: value.toString(),
    };

    switch (item.type) {
      case 'text':
        return (
          <Input
            {...commonProps}
            onChange={handleInputChange}
            placeholder={item.default?.toString() || ''}
          />
        );

      case 'bool':
        return (
          <div className="flex items-center space-x-3">
            <input
              id={key}
              type="checkbox"
              checked={value === '1'}
              onChange={handleCheckboxChange}
              className="h-4 w-4 focus:ring-offset-2 border-gray-300 rounded"
              style={{
                color: themeColor,
                '--tw-ring-color': themeColor,
              } as React.CSSProperties}
            />
            <label htmlFor={key} className="text-sm text-gray-700">
              {item.title}
            </label>
          </div>
        );
    }
  };

  const renderActiveTab = () => {
    if (!appConfig?.spec?.groups) return null;

    const group = appConfig.spec.groups.find(g => g.name === activeTab);
    if (!group) return null;

    return (
      <div className="space-y-6">
        {group.description && (
          <p className="text-gray-600 mb-4">{group.description}</p>
        )}
        {group.items.map(item => (
          <div key={item.name} data-testid={`config-item-${item.name}`}>
            {renderConfigItem(item)}
          </div>
        ))}
      </div>
    );
  };

  if (isConfigLoading) {
    return (
      <div className="space-y-6" data-testid="configuration-step-loading">
        <Card>
          <div className="flex flex-col items-center justify-center py-12">
            <Loader2 className="w-8 h-8 animate-spin text-gray-400 mb-4" />
            <p className="text-gray-600">Loading configuration...</p>
          </div>
        </Card>
      </div>
    );
  }

  if (getConfigError) {
    return (
      <div className="space-y-6" data-testid="configuration-step-error">
        <Card>
          <div className="flex flex-col items-center justify-center py-12">
            <p className="text-red-600 mb-4">Failed to load configuration</p>
            <p className="text-gray-600 text-sm">{getConfigError.message}</p>
          </div>
        </Card>
      </div>
    );
  }

  if (!appConfig?.spec?.groups || appConfig.spec.groups.length === 0) {
    return (
      <div className="space-y-6" data-testid="configuration-step-empty">
        <Card>
          <div className="flex flex-col items-center justify-center py-12">
            <p className="text-gray-600">No configuration available</p>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6" data-testid="configuration-step">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">
            {text.configurationTitle}
          </h2>
          <p className="text-gray-600 mt-1">
            {text.configurationDescription}
          </p>
        </div>

        <div className="mb-6">
          <div className="border-b border-gray-200">
            <nav className="-mb-px flex space-x-6">
              {appConfig.spec.groups.map(group => (
                <button
                  key={group.name}
                  data-testid={`config-tab-${group.name}`}
                  className={`py-4 px-1 border-b-2 font-medium text-sm transition-colors`}
                  onClick={() => setActiveTab(group.name)}
                  style={{
                    borderColor: activeTab === group.name ? themeColor : 'transparent',
                    color: activeTab === group.name ? themeColor : 'rgb(107 114 128)',
                  }}
                >
                  {group.title}
                </button>
              ))}
            </nav>
          </div>
        </div>

        {renderActiveTab()}

        {submitError && (
          <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md" data-testid="config-submit-error">
            {submitError}
          </div>
        )}
      </Card>

      <div className="flex justify-end">
        <Button onClick={submitConfigValues} icon={<ChevronRight className="w-5 h-5" />} dataTestId="config-next-button">
          Next: Setup
        </Button>
      </div>
    </div>
  );
};

export default ConfigurationStep;
