import React, { useState, useRef } from 'react';
import Card from '../common/Card';
import Button from '../common/Button';
import Input from '../common/Input';
import Select from '../common/Select';
import { useConfig } from '../../contexts/ConfigContext';
import { useWizardMode } from '../../contexts/WizardModeContext';
import { ChevronLeft, ChevronRight, Upload } from 'lucide-react';

interface ConfigurationStepProps {
  onNext: () => void;
  onBack: () => void;
}

interface ValidationErrors {
  clusterName?: string;
  namespace?: string;
  storageClass?: string;
  domain?: string;
  adminUsername?: string;
  adminPassword?: string;
  adminEmail?: string;
  description?: string;
  [key: string]: string | undefined;
}

type TabName = 'cluster' | 'network' | 'admin' | 'database';

const ConfigurationStep: React.FC<ConfigurationStepProps> = ({ onNext, onBack }) => {
  const { config, updateConfig, prototypeSettings } = useConfig();
  const { text } = useWizardMode();
  const [activeTab, setActiveTab] = useState<TabName>('cluster');
  const [errors, setErrors] = useState<ValidationErrors>({});
  const fileInputRef = useRef<HTMLInputElement>(null);
  const themeColor = prototypeSettings.themeColor;

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { id, value } = e.target;
    updateConfig({ [id]: value });
    if (!prototypeSettings.skipValidation) {
      setErrors(prev => ({ ...prev, [id]: undefined }));
    }
  };

  const handleSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const { id, value } = e.target;
    updateConfig({ [id]: value });
  };

  const handleCheckboxChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, checked } = e.target;
    updateConfig({ [id]: checked });
  };

  const handleRadioChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    updateConfig({ [name]: value });
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onload = (event) => {
        const content = event.target?.result;
        updateConfig({ licenseKey: content as string });
      };
      reader.readAsText(file);
    }
  };

  const validateTab = (tab: TabName): boolean => {
    if (prototypeSettings.skipValidation) return true;

    const newErrors: ValidationErrors = {};

    switch (tab) {
      case 'cluster':
        if (!config.clusterName) newErrors.clusterName = 'Cluster name is required';
        if (!config.namespace) newErrors.namespace = 'Namespace is required';
        if (!config.storageClass) newErrors.storageClass = 'Storage class is required';
        if (!config.description) newErrors.description = 'Description is required';
        break;
      case 'network':
        if (!config.domain) newErrors.domain = 'Domain is required';
        break;
      case 'admin':
        if (!config.adminUsername) newErrors.adminUsername = 'Admin username is required';
        if (!config.adminPassword) newErrors.adminPassword = 'Admin password is required';
        if (!config.adminEmail) newErrors.adminEmail = 'Admin email is required';
        break;
      case 'database':
        if (config.databaseType === 'external') {
          if (!config.databaseConfig?.host) newErrors['databaseConfig.host'] = 'Database host is required';
          if (!config.databaseConfig?.username) newErrors['databaseConfig.username'] = 'Database username is required';
          if (!config.databaseConfig?.password) newErrors['databaseConfig.password'] = 'Database password is required';
          if (!config.databaseConfig?.database) newErrors['databaseConfig.database'] = 'Database name is required';
        }
        break;
    }

    setErrors(prev => ({ ...prev, ...newErrors }));
    return Object.keys(newErrors).length === 0;
  };

  const findFirstTabWithErrors = (): TabName | null => {
    const tabs: TabName[] = ['cluster', 'network', 'admin', 'database'];
    for (const tab of tabs) {
      if (!validateTab(tab)) {
        return tab;
      }
    }
    return null;
  };

  const handleNext = () => {
    if (validateTab(activeTab)) {
      const nextTabWithErrors = findFirstTabWithErrors();
      if (nextTabWithErrors) {
        setActiveTab(nextTabWithErrors);
      } else {
        onNext();
      }
    }
  };

  const renderClusterConfig = () => (
    <div className="space-y-6">
      <Input
        id="clusterName"
        label="Cluster Name"
        value={config.clusterName}
        onChange={handleInputChange}
        placeholder="my-gitea"
        required={!prototypeSettings.skipValidation}
        error={errors.clusterName}
        helpText="A unique name for your Gitea Enterprise installation"
      />

      <Select
        id="environment"
        label="Environment"
        value={config.environment}
        onChange={handleSelectChange}
        options={[
          { value: 'development', label: 'Development' },
          { value: 'staging', label: 'Staging' },
          { value: 'production', label: 'Production' },
        ]}
        required={!prototypeSettings.skipValidation}
        helpText="Select the deployment environment"
      />

      <div className="space-y-2">
        <label className="block text-sm font-medium text-gray-700">
          Deployment Mode
        </label>
        <div className="space-y-2">
          <div className="flex items-center">
            <input
              type="radio"
              id="mode-standard"
              name="deploymentMode"
              value="standard"
              checked={config.deploymentMode === 'standard'}
              onChange={handleRadioChange}
              className="h-4 w-4 focus:ring-offset-2 border-gray-300"
              style={{
                color: themeColor,
                '--tw-ring-color': themeColor,
              } as React.CSSProperties}
            />
            <label htmlFor="mode-standard" className="ml-2 text-sm text-gray-700">
              Standard
            </label>
          </div>
          <div className="flex items-center">
            <input
              type="radio"
              id="mode-ha"
              name="deploymentMode"
              value="ha"
              checked={config.deploymentMode === 'ha'}
              onChange={handleRadioChange}
              className="h-4 w-4 focus:ring-offset-2 border-gray-300"
              style={{
                color: themeColor,
                '--tw-ring-color': themeColor,
              } as React.CSSProperties}
            />
            <label htmlFor="mode-ha" className="ml-2 text-sm text-gray-700">
              High Availability
            </label>
          </div>
        </div>
        <p className="text-sm text-gray-500">Choose your deployment configuration</p>
      </div>

      <div className="space-y-1">
        <label htmlFor="description" className="block text-sm font-medium text-gray-700">
          Description
          {!prototypeSettings.skipValidation && <span className="text-red-500 ml-1">*</span>}
        </label>
        <textarea
          id="description"
          value={config.description}
          onChange={handleInputChange}
          rows={4}
          className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-offset-2"
          style={{
            '--tw-ring-color': themeColor,
            '--tw-ring-offset-color': themeColor,
          } as React.CSSProperties}
          placeholder="Describe the purpose of this Gitea Enterprise installation"
        />
        {errors.description && (
          <p className="mt-1 text-sm text-red-500">{errors.description}</p>
        )}
      </div>
    </div>
  );

  const renderNetworkConfig = () => (
    <div className="space-y-6">
      <Input
        id="domain"
        label="Domain"
        value={config.domain}
        onChange={handleInputChange}
        placeholder="gitea.example.com"
        required={!prototypeSettings.skipValidation}
        error={errors.domain}
        helpText="Domain name for accessing Gitea"
      />

      <div className="flex items-center space-x-3">
        <input
          id="useHttps"
          type="checkbox"
          checked={config.useHttps}
          onChange={handleCheckboxChange}
          className="h-4 w-4 focus:ring-offset-2 border-gray-300 rounded"
          style={{
            color: themeColor,
            '--tw-ring-color': themeColor,
          } as React.CSSProperties}
        />
        <label htmlFor="useHttps" className="text-sm text-gray-700">
          Enable HTTPS
        </label>
      </div>
    </div>
  );

  const renderAdminConfig = () => (
    <div className="space-y-6">
      <Input
        id="adminUsername"
        label="Admin Username"
        value={config.adminUsername}
        onChange={handleInputChange}
        placeholder="giteaadmin"
        required={!prototypeSettings.skipValidation}
        error={errors.adminUsername}
        helpText="Username for the administrator account"
      />

      <Input
        id="adminEmail"
        label="Admin Email"
        type="email"
        value={config.adminEmail}
        onChange={handleInputChange}
        placeholder="admin@example.com"
        required={!prototypeSettings.skipValidation}
        error={errors.adminEmail}
        helpText="Email address for the administrator"
      />

      <Input
        id="adminPassword"
        label="Admin Password"
        type="password"
        value={config.adminPassword}
        onChange={handleInputChange}
        placeholder="••••••••••••"
        required={!prototypeSettings.skipValidation}
        error={errors.adminPassword}
        helpText="Password must be at least 8 characters"
      />

      <div className="space-y-1">
        <label className="block text-sm font-medium text-gray-700">
          License Key File
        </label>
        <div className="mt-1 flex items-center">
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileChange}
            accept=".key,.txt"
            className="hidden"
          />
          <Button
            variant="outline"
            onClick={() => fileInputRef.current?.click()}
            icon={<Upload className="w-4 h-4" />}
          >
            Upload License Key
          </Button>
        </div>
        <p className="text-sm text-gray-500">Upload your Gitea Enterprise license key</p>
      </div>
    </div>
  );

  const renderDatabaseConfig = () => (
    <div className="space-y-6">
      <Select
        id="databaseType"
        label="Database Type"
        value={config.databaseType}
        onChange={handleSelectChange}
        options={[
          { value: 'internal', label: 'Internal (PostgreSQL)' },
          { value: 'external', label: 'External Database' },
        ]}
        required={!prototypeSettings.skipValidation}
        helpText="Choose between managed internal database or connect to your existing database"
      />

      {config.databaseType === 'external' && (
        <>
          <Input
            id="databaseConfig.host"
            label="Database Host"
            value={config.databaseConfig?.host || ''}
            onChange={handleInputChange}
            placeholder="db.example.com"
            required={!prototypeSettings.skipValidation}
            error={errors['databaseConfig.host']}
          />

          <Input
            id="databaseConfig.port"
            label="Database Port"
            type="number"
            value={config.databaseConfig?.port?.toString() || '5432'}
            onChange={handleInputChange}
            placeholder="5432"
            required={!prototypeSettings.skipValidation}
          />

          <Input
            id="databaseConfig.username"
            label="Database Username"
            value={config.databaseConfig?.username || ''}
            onChange={handleInputChange}
            placeholder="postgres"
            required={!prototypeSettings.skipValidation}
            error={errors['databaseConfig.username']}
          />

          <Input
            id="databaseConfig.password"
            label="Database Password"
            type="password"
            value={config.databaseConfig?.password || ''}
            onChange={handleInputChange}
            placeholder="••••••••••••"
            required={!prototypeSettings.skipValidation}
            error={errors['databaseConfig.password']}
          />

          <Input
            id="databaseConfig.database"
            label="Database Name"
            value={config.databaseConfig?.database || ''}
            onChange={handleInputChange}
            placeholder="gitea"
            required={!prototypeSettings.skipValidation}
            error={errors['databaseConfig.database']}
          />
        </>
      )}
    </div>
  );

  const renderActiveTab = () => {
    switch (activeTab) {
      case 'cluster':
        return renderClusterConfig();
      case 'network':
        return renderNetworkConfig();
      case 'admin':
        return renderAdminConfig();
      case 'database':
        return renderDatabaseConfig();
      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.configurationTitle}</h2>
          <p className="text-gray-600 mt-1">
            {text.configurationDescription}
          </p>
        </div>

        <div className="mb-6">
          <div className="border-b border-gray-200">
            <nav className="-mb-px flex space-x-6">
              <button
                className={`py-4 px-1 border-b-2 font-medium text-sm transition-colors`}
                onClick={() => setActiveTab('cluster')}
                style={{
                  borderColor: activeTab === 'cluster' ? themeColor : 'transparent',
                  color: activeTab === 'cluster' ? themeColor : 'rgb(107 114 128)',
                }}
              >
                Cluster Settings
              </button>
              <button
                className={`py-4 px-1 border-b-2 font-medium text-sm transition-colors`}
                onClick={() => setActiveTab('network')}
                style={{
                  borderColor: activeTab === 'network' ? themeColor : 'transparent',
                  color: activeTab === 'network' ? themeColor : 'rgb(107 114 128)',
                }}
              >
                Network
              </button>
              <button
                className={`py-4 px-1 border-b-2 font-medium text-sm transition-colors`}
                onClick={() => setActiveTab('admin')}
                style={{
                  borderColor: activeTab === 'admin' ? themeColor : 'transparent',
                  color: activeTab === 'admin' ? themeColor : 'rgb(107 114 128)',
                }}
              >
                Admin Account
              </button>
              <button
                className={`py-4 px-1 border-b-2 font-medium text-sm transition-colors`}
                onClick={() => setActiveTab('database')}
                style={{
                  borderColor: activeTab === 'database' ? themeColor : 'transparent',
                  color: activeTab === 'database' ? themeColor : 'rgb(107 114 128)',
                }}
              >
                Database
              </button>
            </nav>
          </div>
        </div>

        {renderActiveTab()}
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={onBack} icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button onClick={handleNext} icon={<ChevronRight className="w-5 h-5" />}>
          Next: Setup
        </Button>
      </div>
    </div>
  );
};

export default ConfigurationStep;