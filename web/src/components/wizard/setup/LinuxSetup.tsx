import React from 'react';
import Input from '../../common/Input';
import Select from '../../common/Select';
import { ChevronDown, ChevronUp } from 'lucide-react';

interface LinuxSetupProps {
  config: {
    dataDirectory?: string;
    httpProxy?: string;
    httpsProxy?: string;
    noProxy?: string;
    networkInterface?: string;
    globalCidr?: string;
  };
  prototypeSettings: {
    availableNetworkInterfaces?: Array<{
      name: string;
    }>;
  };
  showAdvanced: boolean;
  onShowAdvancedChange: (show: boolean) => void;
  onInputChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onSelectChange: (e: React.ChangeEvent<HTMLSelectElement>) => void;
  availableNetworkInterfaces?: string[];
}

const LinuxSetup: React.FC<LinuxSetupProps> = ({
  config,
  prototypeSettings,
  showAdvanced,
  onShowAdvancedChange,
  onInputChange,
  onSelectChange,
  availableNetworkInterfaces = [],
}) => (
  <div className="space-y-6">
    <div className="space-y-4">
      <h2 className="text-lg font-medium text-gray-900">System Configuration</h2>
      <Input
        id="dataDirectory"
        label="Data Directory"
        value={config.dataDirectory || ''}
        onChange={onInputChange}
        placeholder="/var/lib/gitea"
        helpText="Directory where Gitea will store its data (defaults to /var/lib/gitea)"
      />
    </div>

    <div className="space-y-4">
      <h2 className="text-lg font-medium text-gray-900">Proxy Configuration</h2>
      <div className="space-y-4">
        <Input
          id="httpProxy"
          label="HTTP Proxy"
          value={config.httpProxy || ''}
          onChange={onInputChange}
          placeholder="http://proxy.example.com:3128"
          helpText="HTTP proxy server URL"
        />

        <Input
          id="httpsProxy"
          label="HTTPS Proxy"
          value={config.httpsProxy || ''}
          onChange={onInputChange}
          placeholder="https://proxy.example.com:3128"
          helpText="HTTPS proxy server URL"
        />

        <Input
          id="noProxy"
          label="Proxy Bypass List"
          value={config.noProxy || ''}
          onChange={onInputChange}
          placeholder="localhost,127.0.0.1,.example.com"
          helpText="Comma-separated list of hosts to bypass the proxy"
        />
      </div>
    </div>

    <div className="pt-4">
      <button
        type="button"
        className="flex items-center text-lg font-medium text-gray-900 mb-4"
        onClick={() => onShowAdvancedChange(!showAdvanced)}
      >
        {showAdvanced ? <ChevronDown className="w-4 h-4 mr-1" /> : <ChevronUp className="w-4 h-4 mr-1" />}
        Advanced Settings
      </button>

      {showAdvanced && (
        <div className="space-y-6">
          <Select
            id="networkInterface"
            label="Network Interface"
            value={config.networkInterface || ''}
            onChange={onSelectChange}
            options={[
              { value: '', label: 'Select a network interface' },
              ...(availableNetworkInterfaces.length > 0 
                ? availableNetworkInterfaces.map(iface => ({
                    value: iface,
                    label: iface
                  }))
                : (prototypeSettings.availableNetworkInterfaces || []).map(iface => ({
                    value: iface.name,
                    label: iface.name
                  }))
              )
            ]}
            helpText="Network interface to use for Gitea"
          />
          
          <Input
            id="globalCidr"
            label="Reserved Network Range (CIDR)"
            value={config.globalCidr || ''}
            onChange={onInputChange}
            placeholder="10.244.0.0/16"
            helpText="CIDR notation for the reserved network range (defaults to 10.244.0.0/16; must be /16 or larger)"
          />
        </div>
      )}
    </div>
  </div>
);

export default LinuxSetup;