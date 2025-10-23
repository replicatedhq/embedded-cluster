import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useMutation } from '@tanstack/react-query';
import Card from '../../common/Card';
import Button from '../../common/Button';
import Input from '../../common/Input';
import Textarea from '../../common/Textarea';
import Checkbox from '../../common/Checkbox';
import Radio from '../../common/Radio';
import Select from '../../common/Select';
import Label from '../../common/Label';
import Markdown from '../../common/Markdown';
import FileInput from '../../common/file';
import { useWizard } from '../../../contexts/WizardModeContext';
import { useAuth } from '../../../contexts/AuthContext';
import { useSettings } from '../../../contexts/SettingsContext';
import { ChevronRight, Loader2 } from 'lucide-react';
import { useDebouncedFetch } from '../../../api/debouncedFetch';
import { ApiError } from '../../../api/error';
import { createAuthedClient, getWizardBasePath, getApiBasePath } from '../../../api/client';
import { handleUnauthorized } from '../../../utils/auth';

import type { components } from "../../../types/api";
import type { ConfigGroup as AppConfigGroup, ConfigItem as AppConfigItem, AppConfig } from "../../../types/api-overrides";
type AppConfigValues = components["schemas"]["types.AppConfigValues"];


interface ConfigurationStepProps {
  onNext: () => void;
}


const ConfigurationStep: React.FC<ConfigurationStepProps> = ({ onNext }) => {
  const { text, target, mode } = useWizard();
  const { token } = useAuth();
  const { settings } = useSettings();
  const [activeTab, setActiveTab] = useState<string>('');
  const [appConfig, setAppConfig] = useState<AppConfig | null>(null);
  const [changedValues, setChangedValues] = useState<AppConfigValues>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const { debouncedFetch } = useDebouncedFetch({ debounceMs: 250 });

  const [itemErrors, setItemErrors] = useState<Record<string, string>>({});
  const [itemToFocus, setItemToFocus] = useState<AppConfigItem | null>(null);

  // Holds refs to each item by name for focusing
  const itemRefs = useRef<Record<string, HTMLElement | null>>({});

  // Helper function to assign refs dynamically
  const setRef = (name: string) => (el: HTMLElement | null) => {
    itemRefs.current[name] = el;
  };

  const themeColor = settings.themeColor;

  const templateConfig = useCallback(async (values: AppConfigValues) => {
    try {
      setGeneralError(null); // Clear any existing errors

      const apiBase = getApiBasePath(target, mode);
      // TODO consider using this custom fetch in the future together with the openapi-api fetch client in src/api/client.ts
      const response = await debouncedFetch(`${apiBase}/app/config/template`, {
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
        const apiErr = await ApiError.fromResponse(response, 'Failed to template configuration')
        handleUnauthorized(apiErr)
        throw apiErr
      }

      const config = await response.json();
      setAppConfig(config);
    } catch (error) {
      if (error instanceof ApiError) {
        setGeneralError(error.details || error.message);
      } else if (error instanceof Error) {
        setGeneralError(error.message);
      } else {
        setGeneralError(String(error))
      }
    }
  }, [target, mode, token, debouncedFetch]);

  // Fetch initial config on mount
  useEffect(() => {
    const fetchInitialConfig = async () => {
      setIsLoading(true);
      await templateConfig({});
      setIsLoading(false);
    };
    fetchInitialConfig();
  }, []);

  // Helper function to find the first item with error in DOM order
  const findFirstItemWithError = (itemErrors: Record<string, string>): AppConfigItem | null => {
    if (!appConfig?.groups || Object.keys(itemErrors).length === 0) {
      return null;
    }

    // Iterate through groups and items in DOM order to find first item with error
    for (const group of appConfig.groups) {
      for (const item of group.items) {
        if (itemErrors[item.name]) {
          return item;
        }
      }
    }

    return null;
  };

  // Helper function to find which group/tab contains a specific item
  const findGroupForItem = (itemName: string): AppConfigGroup | null => {
    if (!appConfig?.groups) return null;

    return appConfig.groups.find(group =>
      group.items.some(item => item.name === itemName)
    ) || null;
  };

  // Helper function to focus on an item with tab switching support
  const focusItemWithTabSupport = (item: AppConfigItem): void => {
    const targetGroup = findGroupForItem(item.name);

    if (!targetGroup) {
      console.warn(`Could not find group for item: ${item.name}`);
      return;
    }

    // Switch to the correct tab if item is in a different tab
    if (targetGroup.name !== activeTab) {
      setActiveTab(targetGroup.name);
    }

    // Set the item to focus - useEffect will handle the actual focusing
    setItemToFocus(item);
  };

  // Helper function to parse server validation errors from API response
  const parseServerErrors = (error: ApiError): Record<string, string> => {
    const itemErrors: Record<string, string> = {};

    // Check if error has structured item errors
    if (error.fieldErrors) {
      error.fieldErrors.forEach((itemError) => {
        // Pass through server error message directly - no client-side enhancement
        itemErrors[itemError.field] = itemError.message;
      });
    }

    return itemErrors;
  };

  // Mutation to save config values
  const { mutate: submitConfigValues } = useMutation<void, ApiError>({
    mutationFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath(target, mode);

      const { error } = await client.PATCH(`${apiBase}/app/config/values`, {
        body: { values: changedValues },
      });

      if (error) throw error;
    },
    onSuccess: () => {
      setGeneralError(null);
      setItemErrors({}); // Clear item errors
      setChangedValues({}); // Clear changed values after successful submission

      // Proceed to next step
      onNext();
    },
    onError: (error: ApiError) => {
      const parsedItemErrors = parseServerErrors(error);
      setItemErrors(parsedItemErrors);
      setGeneralError(error?.details || error?.message || 'Failed to save configuration');

      // Focus on the first item with validation error
      const firstErrorItem = findFirstItemWithError(parsedItemErrors);
      if (firstErrorItem) {
        focusItemWithTabSupport(firstErrorItem);
      }
    },
  });

  // Set active tab when config loads
  useEffect(() => {
    if (appConfig?.groups && appConfig.groups.length > 0 && !activeTab) {
      setActiveTab(appConfig.groups[0].name);
    }
  }, [appConfig, activeTab]);

  // Handle focusing on item after tab switches or validation errors
  useEffect(() => {
    if (!itemToFocus) return;

    // Use refs to get the focusable element directly
    let itemElement: HTMLElement | null = null;

    // For all inputs including radio, use the main item ref
    // Radio component forwards ref to the first option automatically
    itemElement = itemRefs.current[itemToFocus.name];

    if (itemElement) {
      itemElement.focus();
      // Scroll the element into view to ensure it's visible
      if (itemElement.scrollIntoView) {
        itemElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }

    // Clear the focus request
    setItemToFocus(null);
  }, [itemToFocus]);

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

  // Helper function to get the display filename for a config item (no defaults)
  const getDisplayFilename = (item: AppConfigItem): string => {
    // First check user value, then config item value (use ?? to allow empty strings from the user)
    return changedValues?.[item.name]?.filename ?? (item.filename || '');
  };

  // Helper function for password types to determine if the show password toggle should be enabled
  const allowShowPassword = (item: AppConfigItem): boolean => {
    // Only allow show password if the item is a password type and has a user value set
    return Boolean(item.type === "password" && changedValues?.[item.name]?.value);
  };

  const updateConfigValue = (itemName: string, value: string, filename?: string) => {
    // Update the config values map
    const newChangedValues = { ...changedValues, [itemName]: { value, filename } };
    setChangedValues(newChangedValues);

    // Clear item error for this item when user modifies it
    if (itemErrors[itemName]) {
      setItemErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[itemName];
        return newErrors;
      });
    }

    // Template the config with the new values
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

  const handleDropdownChange = (itemName: string, e: React.ChangeEvent<HTMLSelectElement>) => {
    updateConfigValue(itemName, e.target.value);
  };

  const renderConfigItem = (item: AppConfigItem) => {
    // Display item validation errors with priority over initial config errors
    const displayError = itemErrors[item.name] || item.error;

    const sharedProps = {
      id: item.name,
      label: item.title,
      helpText: item.help_text,
      error: displayError,
      required: item.required,
      disabled: item.readonly,
      ref: setRef(item.name),
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
            allowShowPassword={allowShowPassword(item)}
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
      case 'select_one': // select_one renders as radio for backward compatibility
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
            filename={getDisplayFilename(item)}
            defaultValue={item.default}
            defaultFilename={item.name}
            onChange={(value: string, filename: string) => handleFileChange(item.name, value, filename)}
            dataTestId={`file-input-${item.name}`}
          />
        );

      case 'dropdown':
        if (item.items) {
          const options = item.items.map(child => ({
            value: child.name,
            label: child.title
          }));
          return (
            <Select
              {...sharedProps}
              defaultValue={item.default}
              value={getEffectiveValue(item)}
              options={options}
              placeholder="Select an option"
              onChange={(e) => handleDropdownChange(item.name, e)}
              dataTestId={`dropdown-input-${item.name}`}
              className="w-96"
            />
          );
        }
        return null;

      case 'heading':
        return (
          <div className="pt-4 pb-2 border-b border-gray-200" role="group">
            <h3
              className="text-lg font-semibold text-gray-900"
              data-testid={`heading-${item.name}`}
              role="heading"
              aria-level={3}
            >
              {item.title}
            </h3>
          </div>
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
          <div className="text-gray-600 mb-4 prose max-w-none">
            <Markdown>
              {group.description}
            </Markdown>
          </div>
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
          {/* TODO: centralize the logic for this text */}
          {mode === "upgrade" ? "Next: Validation" : "Next: Setup"}
        </Button>
      </div>
    </div>
  );
};

export default ConfigurationStep;
