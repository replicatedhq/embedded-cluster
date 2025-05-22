import React from 'react';
import Card from '../common/Card';
import Select from '../common/Select';
import { GiteaLogo } from '../common/Logo';
import { useConfig } from '../../contexts/ConfigContext';

const PrototypeSettings: React.FC = () => {
  const { prototypeSettings, updatePrototypeSettings } = useConfig();

  const handleSkipValidationChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ skipValidation: e.target.checked });
  };

  const handleFailPreflightsChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ failPreflights: e.target.checked });
  };

  const handleFailHostPreflightsChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ failHostPreflights: e.target.checked });
  };

  const handleFailInstallationChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ failInstallation: e.target.checked });
  };

  const handleThemeColorChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ themeColor: e.target.value });
  };

  const handleSelfSignedCertChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ useSelfSignedCert: e.target.checked });
  };

  const handleSkipNodeValidationChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ skipNodeValidation: e.target.checked });
  };

  const handleMultiNodeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    updatePrototypeSettings({ enableMultiNode: e.target.checked });
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center py-4">
            <div className="flex items-center space-x-3">
              <GiteaLogo className="h-10 w-10" />
              <div>
                <h1 className="text-xl font-semibold text-gray-900">Prototype Settings</h1>
                <p className="text-sm text-gray-500">Configure prototype behavior</p>
              </div>
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Card>
          <div className="space-y-6">
            <div className="border-t border-gray-200 pt-6">
              <h2 className="text-lg font-medium text-gray-900 mb-4">Theme Settings</h2>
              <div className="space-y-2">
                <label htmlFor="themeColor" className="block text-sm font-medium text-gray-700">
                  Theme Color
                </label>
                <input
                  type="text"
                  id="themeColor"
                  value={prototypeSettings.themeColor}
                  onChange={handleThemeColorChange}
                  placeholder="#609926"
                  className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#609926]"
                  pattern="^#[0-9A-Fa-f]{6}$"
                />
                <p className="text-sm text-gray-500">Enter a hex color code (e.g., #609926)</p>
              </div>
            </div>

            <div className="border-t border-gray-200 pt-6">
              <h2 className="text-lg font-medium text-gray-900 mb-4">Security Settings</h2>
              <div className="space-y-4">
                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="useSelfSignedCert"
                    checked={prototypeSettings.useSelfSignedCert}
                    onChange={handleSelfSignedCertChange}
                    className="h-4 w-4 text-[#609926] focus:ring-[#609926] border-gray-300 rounded"
                  />
                  <label htmlFor="useSelfSignedCert" className="text-sm text-gray-700">
                    Use self-signed certificate for installer
                  </label>
                </div>
              </div>
            </div>

            <div className="border-t border-gray-200 pt-6">
              <h2 className="text-lg font-medium text-gray-900 mb-4">Linux Installation Settings</h2>
              <div className="space-y-4">
                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="enableMultiNode"
                    checked={prototypeSettings.enableMultiNode}
                    onChange={handleMultiNodeChange}
                    className="h-4 w-4 text-[#609926] focus:ring-[#609926] border-gray-300 rounded"
                  />
                  <label htmlFor="enableMultiNode" className="text-sm text-gray-700">
                    Enable multi-node support for Linux installations
                  </label>
                </div>
              </div>
            </div>

            <div className="border-t border-gray-200 pt-6">
              <h2 className="text-lg font-medium text-gray-900 mb-4">Validation Settings</h2>
              
              <div className="space-y-4">
                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="skipValidation"
                    checked={prototypeSettings.skipValidation}
                    onChange={handleSkipValidationChange}
                    className="h-4 w-4 text-[#609926] focus:ring-[#609926] border-gray-300 rounded"
                  />
                  <label htmlFor="skipValidation" className="text-sm text-gray-700">
                    Skip required field validation on configuration page
                  </label>
                </div>

                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="skipNodeValidation"
                    checked={prototypeSettings.skipNodeValidation}
                    onChange={handleSkipNodeValidationChange}
                    className="h-4 w-4 text-[#609926] focus:ring-[#609926] border-gray-300 rounded"
                  />
                  <label htmlFor="skipNodeValidation" className="text-sm text-gray-700">
                    Allow proceeding without all required hosts
                  </label>
                </div>
                
                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="failPreflights"
                    checked={prototypeSettings.failPreflights}
                    onChange={handleFailPreflightsChange}
                    className="h-4 w-4 text-[#609926] focus:ring-[#609926] border-gray-300 rounded"
                  />
                  <label htmlFor="failPreflights" className="text-sm text-gray-700">
                    Force preflight checks to fail
                  </label>
                </div>

                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="failHostPreflights"
                    checked={prototypeSettings.failHostPreflights}
                    onChange={handleFailHostPreflightsChange}
                    className="h-4 w-4 text-[#609926] focus:ring-[#609926] border-gray-300 rounded"
                  />
                  <label htmlFor="failHostPreflights" className="text-sm text-gray-700">
                    Force host preflight checks to fail
                  </label>
                </div>

                <div className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    id="failInstallation"
                    checked={prototypeSettings.failInstallation}
                    onChange={handleFailInstallationChange}
                    className="h-4 w-4 text-[#609926] focus:ring-[#609926] border-gray-300 rounded"
                  />
                  <label htmlFor="failInstallation" className="text-sm text-gray-700">
                    Simulate installation failure
                  </label>
                </div>
              </div>
              
              <p className="text-sm text-gray-500 mt-4">
                These settings affect how the installer behaves during development and testing.
              </p>
            </div>
          </div>
        </Card>
      </main>
    </div>
  );
};

export default PrototypeSettings;