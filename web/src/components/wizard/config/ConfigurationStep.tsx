import React, { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Card from '../../common/Card';
import Button from '../../common/Button';
import Input from '../../common/Input';
import Textarea from '../../common/Textarea';
import Checkbox from '../../common/Checkbox';
import Radio from '../../common/Radio';
import Label from '../../common/Label';
import FileInput from '../../common/file';
import { useWizard } from '../../../contexts/WizardModeContext';
import { useAuth } from '../../../contexts/AuthContext';
import { useSettings } from '../../../contexts/SettingsContext';
import { ChevronRight, Loader2 } from 'lucide-react';
import { handleUnauthorized } from '../../../utils/auth';
import { AppConfig, AppConfigItem, AppConfigValues } from '../../../types';

interface ConfigurationStepProps {
  onNext: () => void;
}

const ConfigurationStep: React.FC<ConfigurationStepProps> = ({ onNext }) => {
  const { text, target } = useWizard();
  const { token } = useAuth();
  const { settings } = useSettings();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<string>('');
  const [configValues, setConfigValues] = useState<AppConfigValues>({});
  const [dirtyFields, setDirtyFields] = useState<Set<string>>(new Set());
  const [submitError, setSubmitError] = useState<string | null>(null);
  const themeColor = settings.themeColor;

  // Fetch app config from API
  const { data: appConfig, isLoading: isConfigLoading, error: getConfigError } = useQuery<AppConfig>({
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
      return config;
    },
  });

  // Fetch current config values
  const { data: apiConfigValues, isLoading: isConfigValuesLoading, error: getConfigValuesError } = useQuery<AppConfigValues>({
    queryKey: ['appConfigValues', target],
    queryFn: async () => {
      const response = await fetch(`/api/${target}/install/app/config/values`, {
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
        throw new Error(errorData.message || 'Failed to fetch current config values');
      }
      const data = await response.json();
      return data.values || {};
    },
  });

  // Mutation to save config values
  const { mutate: submitConfigValues } = useMutation<AppConfigValues>({
    mutationFn: async () => {
      // Build payload with only dirty fields
      const dirtyValues: AppConfigValues = {};
      dirtyFields.forEach(fieldName => {
        if (configValues[fieldName] !== undefined) {
          dirtyValues[fieldName] = configValues[fieldName];
        }
      });

      const response = await fetch(`/api/${target}/install/app/config/values`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ values: dirtyValues }),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error('Session expired. Please log in again.');
        }
        throw new Error(errorData.message || 'Failed to save configuration');
      }

      const data = await response.json();
      return data.values || {};
    },
    onSuccess: (appConfigValues) => {
      setSubmitError(null);
      // Clear dirty fields after successful submission
      setDirtyFields(new Set());
      // Update config values with the latest from the API
      setConfigValues(appConfigValues);
      queryClient.setQueryData(['appConfigValues', target], appConfigValues);
      onNext();
    },
    onError: (error: Error) => {
      setSubmitError(error?.message || 'Failed to save configuration');
    },
  });

  // Set active tab to first group when config loads
  useEffect(() => {
    if (appConfig?.groups && appConfig.groups.length > 0 && !activeTab) {
      setActiveTab(appConfig.groups[0].name);
    }
  }, [appConfig, activeTab]);

  // Initialize configValues with initial values when they load
  useEffect(() => {
    if (apiConfigValues && Object.keys(configValues).length === 0) {
      setConfigValues(apiConfigValues);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- configValues is not a dependency of this effect
  }, [apiConfigValues]);

  // Helper function to get the display value for a config item (no defaults)
  const getDisplayValue = (item: AppConfigItem): string => {
    // First check user value, then config item value (use ?? to allow empty strings from the user)
    return configValues?.[item.name]?.value ?? (item.value || '');
  };

  // Helper function to get the effective value for a config item (includes defaults)
  const getEffectiveValue = (item: AppConfigItem): string => {
    // First check user value, then config item value, then default (use ?? to allow empty strings from the user)
    return configValues?.[item.name]?.value ?? (item.value || item.default || '');
  };

  const updateConfigValue = (itemName: string, value: string, filename?: string) => {
    // Update the config values map
    setConfigValues(prev => ({ ...prev, [itemName]: { value, filename } }));

    // Mark field as dirty
    setDirtyFields(prev => new Set(prev).add(itemName));
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { id, value } = e.target;
    updateConfigValue(id, value);
  };

  const handlePasswordFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    // Auto-select entire text for password fields
    e.target.select();
  };

  const handlePasswordKeyDown = (itemName: string, e: React.KeyboardEvent<Element>) => {
    // If field is not dirty and user types a character, clear the field first
    if (!dirtyFields.has(itemName) && e.key.length === 1) {
      // Clear the field before the character is added
      updateConfigValue(itemName, '');
    }
  };

  const handleCheckboxChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, checked } = e.target;
    updateConfigValue(id, checked ? '1' : '0');
  };

  const handleRadioChange = (parentId: string, e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, checked } = e.target;
    if (!checked) return;
    updateConfigValue(parentId, id);
  };

  const handleFileChange = (itemName: string, value: string, filename: string) => {
    updateConfigValue(itemName, value, filename);
  };

  const renderConfigItem = (item: AppConfigItem) => {
    const sharedProps = {
      id: item.name,
      label: item.title,
      helpText: item.help_text,
      error: item.error,
      required: item.required,
    }

    switch (item.type) {
      case 'text':
        return (
          <Input
            {...sharedProps}
            defaultValue={item.default}
            value={getDisplayValue(item)}
            onChange={handleInputChange}
            dataTestId={`text-input-${item.name}`}
            className="w-96"
          />
        );

      case 'password':
        return (
          <Input
            {...sharedProps}
            defaultValue={item.default}
            type="password"
            value={getDisplayValue(item)}
            onChange={handleInputChange}
            onKeyDown={(e) => handlePasswordKeyDown(item.name, e)}
            onFocus={handlePasswordFocus}
            dataTestId={`password-input-${item.name}`}
            className="w-96"
          />
        );

      case 'textarea':
        return (
          <Textarea
            {...sharedProps}
            defaultValue={item.default}
            value={getDisplayValue(item)}
            onChange={handleInputChange}
            dataTestId={`textarea-input-${item.name}`}
            className="w-full max-w-2xl"
          />
        );

      case 'bool':
        return (
          <Checkbox
            {...sharedProps}
            checked={getEffectiveValue(item) === '1'}
            onChange={handleCheckboxChange}
            dataTestId={`bool-input-${item.name}`}
          />
        );

      case 'radio':
        if (item.items) {
          return (
            <Radio
              {...sharedProps}
              value={getEffectiveValue(item)}
              options={item.items}
              onChange={e => handleRadioChange(item.name, e)}
            />
          );
        }
        return null;

      case 'file':
        return (
          <FileInput
            {...sharedProps}
            value={getDisplayValue(item)}
            filename={configValues[item.name]?.filename}
            defaultValue={item.default}
            defaultFilename={item.name}
            onChange={(value: string, filename: string) => handleFileChange(item.name, value, filename)}
            dataTestId={`file-input-${item.name}`}
          />
        );

      case 'label':
        return (
          <Label
            content={item.title}
            dataTestId={`label-${item.name}`}
          />
        );

      default:
        return null;
    }
  };

  const renderActiveTab = () => {
    if (!appConfig?.groups) return null;

    const group = appConfig.groups.find(g => g.name === activeTab);
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

  if (isConfigLoading || isConfigValuesLoading) {
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

  if (getConfigError || getConfigValuesError) {
    const error = getConfigError || getConfigValuesError;
    return (
      <div className="space-y-6" data-testid="configuration-step-error">
        <Card>
          <div className="flex flex-col items-center justify-center py-12">
            <p className="text-red-600 mb-4">Failed to load configuration</p>
            <p className="text-gray-600 text-sm">{error?.message}</p>
          </div>
        </Card>
      </div>
    );
  }

  if (!appConfig?.groups || appConfig.groups.length === 0) {
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
              {appConfig.groups.map(group => (
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
