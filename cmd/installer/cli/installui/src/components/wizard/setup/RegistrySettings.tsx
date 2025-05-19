import React from 'react';
import Input from '../../common/Input';
import { CheckCircle, XCircle, Loader2 } from 'lucide-react';
import { ImagePushStatus } from './types';

interface RegistrySettingsProps {
  registryUrl: string;
  registryUsername: string;
  registryPassword: string;
  onInputChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  authError: string | null;
  pushStatus: ImagePushStatus[];
  currentMessage: string;
  pushComplete: boolean;
  isUpgrade?: boolean;
}

const RegistrySettings: React.FC<RegistrySettingsProps> = ({
  registryUrl,
  registryUsername,
  registryPassword,
  onInputChange,
  authError,
  pushStatus,
  currentMessage,
  pushComplete,
  isUpgrade
}) => (
  <div className="space-y-4 pt-4">
    <Input
      id="registryUrl"
      label="Registry URL"
      value={registryUrl}
      onChange={onInputChange}
      placeholder="registry.example.com"
      required
      helpText="The URL of your private container registry"
      error={authError ? ' ' : undefined}
    />

    <Input
      id="registryUsername"
      label="Registry Username"
      value={registryUsername}
      onChange={onInputChange}
      placeholder="username"
      required
      helpText="Username for registry authentication"
      error={authError ? ' ' : undefined}
    />

    <Input
      id="registryPassword"
      label="Registry Password"
      type="password"
      value={registryPassword}
      onChange={onInputChange}
      placeholder="••••••••••••"
      required
      helpText="Password or access token for registry authentication"
      error={authError ? ' ' : undefined}
    />

    {authError && (
      <div className="p-4 bg-red-50 rounded-lg border border-red-200">
        <div className="flex">
          <div className="flex-shrink-0">
            <XCircle className="h-5 w-5 text-red-400" />
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-red-800">Authentication Error</h3>
            <p className="text-sm text-red-700 mt-1">{authError}</p>
          </div>
        </div>
      </div>
    )}

    {pushStatus.length > 0 && (
      <div className="mt-6 space-y-4">
        <h3 className="text-sm font-medium text-gray-900">Image Push Progress</h3>
        {pushStatus.map((status, index) => (
          <div key={index} className="space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-600">{status.image}</span>
              <span className={`font-medium ${
                status.status === 'complete' ? 'text-green-600' :
                status.status === 'failed' ? 'text-red-600' :
                'text-blue-600'
              }`}>
                {status.status === 'complete' ? 'Complete' :
                 status.status === 'failed' ? 'Failed' :
                 status.status === 'pushing' ? 'Pushing' : 'Pending'}
              </span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-1.5">
              <div
                className={`h-1.5 rounded-full transition-all duration-300 ${
                  status.status === 'complete' ? 'bg-green-500' :
                  status.status === 'failed' ? 'bg-red-500' :
                  'bg-blue-500'
                }`}
                style={{ width: `${status.progress}%` }}
              />
            </div>
          </div>
        ))}
        {currentMessage && (
          <p className="text-sm text-gray-600 mt-2">{currentMessage}</p>
        )}
      </div>
    )}

    {pushComplete && (
      <div className="mt-6 p-4 bg-green-50 rounded-lg border border-green-200">
        <div className="flex items-start">
          <div className="flex-shrink-0">
            <CheckCircle className="h-5 w-5 text-green-500" />
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-green-800">Images pushed successfully</h3>
            <p className="text-sm text-green-700 mt-1">
              All required images have been pushed to your private registry. Click "Next" to proceed with the {isUpgrade ? 'upgrade' : 'installation'}.
            </p>
          </div>
        </div>
      </div>
    )}
  </div>
);

export default RegistrySettings;