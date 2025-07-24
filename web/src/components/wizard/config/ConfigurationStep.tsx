import React, { useState, useEffect, useCallback } from 'react';
import { useMutation } from '@tanstack/react-query';
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
import { useDebouncedFetch } from '../../../utils/debouncedFetch';
import { AppConfig, AppConfigGroup, AppConfigItem, AppConfigValues } from '../../../types';

// Constants for configuration
const FOCUS_DELAY_CROSS_TAB_MS = 100;
const FOCUS_DELAY_SAME_TAB_MS = 10;

interface ConfigurationStepProps {
  onNext: () => void;
}

const ConfigurationStep: React.FC<ConfigurationStepProps> = ({ onNext }) => {
  const { text, target } = useWizard();
  const { token } = useAuth();
  const { settings } = useSettings();
  const [activeTab, setActiveTab] = useState<string>('');
  const [appConfig, setAppConfig] = useState<AppConfig | null>(null);
  const [changedValues, setChangedValues] = useState<AppConfigValues>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const { debouncedFetch } = useDebouncedFetch({ debounceMs: 250 });
  
  // Add HEAD branch validation features
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const themeColor = settings.themeColor;

  const templateConfig = useCallback(async (values: AppConfigValues) => {
    try {
      setGeneralError(null); // Clear any existing errors

      const response = await debouncedFetch(`/api/${target}/install/app/config/template`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ values }),
      });

      // If no response, the request was cancelled/aborted - just return silently
      if (!response) {
        return;
      }

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error('Session expired. Please log in again.');
        }
        throw new Error(errorData.message || 'Failed to template configuration');
      }

      const config = await response.json();
      setAppConfig(config);
    } catch (error) {
      setGeneralError(error instanceof Error ? error.message : String(error));
    }
  }, [target, token, debouncedFetch]);

  // Fetch initial config on mount
  useEffect(() => {
    const fetchInitialConfig = async () => {
      setIsLoading(true);
      await templateConfig({});
      setIsLoading(false);
    };
    fetchInitialConfig();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Helper function to find the first field with field error in DOM order
  const findFirstFieldWithError = (fieldErrors: Record<string, string>): AppConfigItem | null => {
    if (!appConfig?.groups || Object.keys(fieldErrors).length === 0) {
      return null;
    }
    
    // Iterate through groups and items in DOM order to find first field with error
    for (const group of appConfig.groups) {
      for (const item of group.items) {
        if (fieldErrors[item.name]) {
          return item;
        }
      }
    }
    
    return null;
  };

  // Helper function to find which group/tab contains a specific field
  const findGroupForField = (fieldName: string): AppConfigGroup | null => {
    if (!appConfig?.groups) return null;
    
    return appConfig.groups.find(group => 
      group.items.some(item => item.name === fieldName)
    ) || null;
  };

  // Helper function to focus on a field with tab switching support
  const focusFieldWithTabSupport = (fieldItem: AppConfigItem): void => {
    const targetGroup = findGroupForField(fieldItem.name);
    
    if (!targetGroup) {
      console.warn(`Could not find group for field: ${fieldItem.name}`);
      return;
    }
    
    // Switch to the correct tab if field is in a different tab
    if (targetGroup.name !== activeTab) {
      setActiveTab(targetGroup.name);
    }
    
    // Focus the field after a brief delay to ensure DOM updates complete
    // This is especially important when switching tabs
    const focusDelay = targetGroup.name !== activeTab ? FOCUS_DELAY_CROSS_TAB_MS : FOCUS_DELAY_SAME_TAB_MS;
    setTimeout(() => {
      let fieldElement: HTMLElement | null = null;
      
      // Special handling for file inputs - focus the upload button instead of hidden input
      if (fieldItem.type === 'file') {
        // For file inputs, focus the upload button using data-testid (follows pattern: file-input-{name}-button)
        fieldElement = document.querySelector(`[data-testid="file-input-${fieldItem.name}-button"]`) as HTMLElement;
      }
      // Special handling for radio buttons - focus the first radio option  
      else if (fieldItem.type === 'radio' && fieldItem.items && fieldItem.items.length > 0) {
        // For radio buttons, focus the first radio option using its ID
        fieldElement = document.getElementById(fieldItem.items[0].name);
      }
      // Special handling for checkboxes - focus the checkbox input
      else if (fieldItem.type === 'bool') {
        // For checkboxes, focus using the field ID
        fieldElement = document.getElementById(fieldItem.name);
      }
      // Default case - use the field name as element ID
      else {
        fieldElement = document.getElementById(fieldItem.name);
      }
      
      if (fieldElement) {
        fieldElement.focus();
        // Scroll the element into view to ensure it's visible
        if (fieldElement.scrollIntoView) {
          fieldElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
      } else {
        console.warn(`Could not find DOM element for field: ${fieldItem.name} (type: ${fieldItem.type})`);
      }
    }, focusDelay);
  };

  // Helper function to parse server validation errors from API response
  const parseServerErrors = (error: unknown): Record<string, string> => {
    const fieldErrors: Record<string, string> = {};
    
    // Check if error has structured field errors
    if (error && typeof error === 'object' && 'errors' in error && Array.isArray(error.errors)) {
      error.errors.forEach((fieldError: unknown) => {
        if (fieldError && typeof fieldError === 'object' && 'field' in fieldError && 'message' in fieldError && 
            typeof fieldError.field === 'string' && typeof fieldError.message === 'string') {
          // Pass through server error message directly - no client-side enhancement
          fieldErrors[fieldError.field] = fieldError.message;
        }
      });
    }
    
    return fieldErrors;
  };

  // Mutation to save config values
  const { mutate: submitConfigValues } = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/${target}/install/app/config/values`, {
        method: 'PATCH',
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
        // Re-throw with full error data for parsing in onError
        const error = new Error(errorData.message || 'Failed to save configuration') as Error & { errorData: unknown };
        error.errorData = errorData;
        throw error;
      }
    },
    onSuccess: () => {
      setGeneralError(null);
      setFieldErrors({}); // Clear field errors
      setChangedValues({}); // Clear changed values after successful submission
      onNext();
    },
    onError: (error: Error & { errorData?: unknown }) => {
      // HEAD branch sophisticated error parsing
      const parsedFieldErrors = parseServerErrors(error?.errorData);
      setFieldErrors(parsedFieldErrors);
      setGeneralError(error?.message || 'Failed to save configuration');
      
      // HEAD branch field focusing
      const firstErrorField = findFirstFieldWithError(parsedFieldErrors);
      if (firstErrorField) {
        focusFieldWithTabSupport(firstErrorField);
      }
    },
  });

  // Set active tab when config loads
  useEffect(() => {
    if (appConfig?.groups && appConfig.groups.length > 0 && !activeTab) {
      setActiveTab(appConfig.groups[0].name);
    }
  }, [appConfig, activeTab]);

  // Helper function to get the display value for a config item (no defaults)
  const getDisplayValue = (item: AppConfigItem): string => {
    // First check user value, then config item value (use ?? to allow empty strings from the user)
    return changedValues?.[item.name]?.value ?? (item.value || '');
  };

  // Helper function to get the effective value for a config item (includes defaults)
  const getEffectiveValue = (item: AppConfigItem): string => {
    // First check user value, then config item value, then default (use ?? to allow empty strings from the user)
    return changedValues?.[item.name]?.value ?? (item.value || item.default || '');
  };


  const updateConfigValue = (itemName: string, value: string, filename?: string) => {
    // Update the config values map
    const newChangedValues = { ...changedValues, [itemName]: { value, filename } };
    setChangedValues(newChangedValues);

    // Clear field error for this field when user modifies it (HEAD)
    if (fieldErrors[itemName]) {
      setFieldErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[itemName];
        return newErrors;
      });
    }

    // Template the config with the new values (main)
    templateConfig(newChangedValues);
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { id, value } = e.target;
    updateConfigValue(id, value);
  };

  const handlePasswordFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    e.target.select();
  };

  const handlePasswordKeyDown = (itemName: string, e: React.KeyboardEvent<Element>) => {
    if (!changedValues[itemName] && e.key.length === 1) {
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
    // CORRECT priority: fieldErrors (server validation) > item.error (initial config errors)
    const displayError = fieldErrors[item.name] || item.error;
    
    const sharedProps = {
      id: item.name,
      label: item.title,
      helpText: item.help_text,
      error: displayError,
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
            filename={changedValues[item.name]?.filename}
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

  if (isLoading) {
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

  if (generalError && !appConfig) {
    return (
      <div className="space-y-6" data-testid="configuration-step-error">
        <Card>
          <div className="flex flex-col items-center justify-center py-12">
            <p className="text-red-600 mb-4">Failed to load configuration</p>
            <p className="text-gray-600 text-sm">{generalError}</p>
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

        {generalError && (
          <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md" data-testid="config-submit-error">
            {generalError}
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
