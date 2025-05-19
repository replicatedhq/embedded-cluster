import React from 'react';
import { useConfig } from '../../../contexts/ConfigContext';

interface RegistryChoiceProps {
  usePrivateRegistry: boolean;
  onRegistryChange: (usePrivate: boolean) => void;
}

const RegistryChoice: React.FC<RegistryChoiceProps> = ({ usePrivateRegistry, onRegistryChange }) => {
  const { prototypeSettings } = useConfig();
  const themeColor = prototypeSettings.themeColor;

  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-600 mb-4">
        Pull images from the Gitea Enterprise registry, or have images pushed to your own private registry and pulled from there.
      </p>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div 
          className={`relative rounded-lg border p-4 cursor-pointer transition-colors`}
          onClick={() => onRegistryChange(false)}
          style={{
            borderColor: !usePrivateRegistry ? themeColor : 'rgb(229 231 235)',
            backgroundColor: !usePrivateRegistry ? `${themeColor}10` : undefined,
          }}
        >
          <div className="flex items-start">
            <div className="flex h-6 items-center">
              <input
                type="radio"
                checked={!usePrivateRegistry}
                onChange={() => onRegistryChange(false)}
                className="h-4 w-4 border-gray-300 focus:ring-offset-2"
                style={{
                  color: themeColor,
                  '--tw-ring-color': themeColor,
                } as React.CSSProperties}
              />
            </div>
            <div className="ml-3">
              <h3 className="text-sm font-medium text-gray-900">Gitea Enterprise Registry</h3>
              <p className="text-sm text-gray-500 mt-1">
                Pull images from the Gitea Enterprise registry
              </p>
            </div>
          </div>
        </div>

        <div 
          className={`relative rounded-lg border p-4 cursor-pointer transition-colors`}
          onClick={() => onRegistryChange(true)}
          style={{
            borderColor: usePrivateRegistry ? themeColor : 'rgb(229 231 235)',
            backgroundColor: usePrivateRegistry ? `${themeColor}10` : undefined,
          }}
        >
          <div className="flex items-start">
            <div className="flex h-6 items-center">
              <input
                type="radio"
                checked={usePrivateRegistry}
                onChange={() => onRegistryChange(true)}
                className="h-4 w-4 border-gray-300 focus:ring-offset-2"
                style={{
                  color: themeColor,
                  '--tw-ring-color': themeColor,
                } as React.CSSProperties}
              />
            </div>
            <div className="ml-3">
              <h3 className="text-sm font-medium text-gray-900">Private Registry</h3>
              <p className="text-sm text-gray-500 mt-1">
                Push images to your own private registry and pull from there
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default RegistryChoice;