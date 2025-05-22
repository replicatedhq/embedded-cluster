import React, { useState } from 'react';
import Card from '../common/Card';
import Button from '../common/Button';
import { useConfig } from '../../contexts/ConfigContext';
import { CheckCircle, ExternalLink, Copy, ClipboardCheck } from 'lucide-react';

const CompletionStep: React.FC = () => {
  const { config, prototypeSettings } = useConfig();
  const [copied, setCopied] = useState(false);
  const themeColor = prototypeSettings.themeColor;

  const baseUrl = `${config.useHttps ? 'https' : 'http'}://${config.domain}`;
  const urls = [
    { name: 'Web Interface', url: baseUrl, description: 'Access the main Gitea interface' },
    { name: 'API Documentation', url: `${baseUrl}/api/swagger`, description: 'Browse and test the Gitea API' }
  ];

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-8">
          <div className="w-16 h-16 rounded-full flex items-center justify-center mb-6" style={{ backgroundColor: `${themeColor}1A` }}>
            <CheckCircle className="w-10 h-10" style={{ color: themeColor }} />
          </div>
          <h2 className="text-3xl font-bold text-gray-900 mb-4">Installation Complete!</h2>
          <p className="text-xl text-gray-600 max-w-2xl mb-8">
            Gitea Enterprise is installed successfully.
          </p>
          
          <Button
            size="lg"
            onClick={() => window.open(`${baseUrl}/admin`, '_blank')}
            className="mb-8"
          >
            Access Admin Dashboard
          </Button>

          <div className="w-full max-w-2xl space-y-4">
            {urls.map((item, index) => (
              <div key={index} className="bg-gray-50 rounded-lg border border-gray-200 p-4">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-sm font-medium text-gray-700">{item.name}</h3>
                  <Button
                    variant="outline"
                    size="sm"
                    className="py-1 px-2 text-xs"
                    icon={copied ? <ClipboardCheck className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                    onClick={() => copyToClipboard(item.url)}
                  >
                    {copied ? 'Copied!' : 'Copy URL'}
                  </Button>
                </div>
                <a
                  href={item.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center justify-between w-full p-2 bg-white rounded border border-gray-300 text-blue-600 hover:bg-blue-50 hover:border-blue-300 transition-colors"
                >
                  <span className="font-mono text-sm">{item.url}</span>
                  <ExternalLink className="w-4 h-4 ml-2" />
                </a>
                <p className="text-sm text-gray-500 mt-2">{item.description}</p>
              </div>
            ))}
          </div>
        </div>
      </Card>

      <Card>
        <div className="space-y-4">
          <h3 className="text-lg font-medium text-gray-900">Next Steps</h3>
          
          <div className="space-y-4">
            <div className="flex items-start">
              <div className="flex-shrink-0 mt-1">
                <div className="w-6 h-6 rounded-full flex items-center justify-center text-white font-medium" style={{ backgroundColor: themeColor }}>
                  1
                </div>
              </div>
              <div className="ml-3">
                <h4 className="text-base font-medium text-gray-900">Log in to your Gitea Enterprise instance</h4>
                <p className="text-sm text-gray-600 mt-1">
                  Use the administrator credentials you provided during setup to log in to your Gitea Enterprise instance.
                </p>
              </div>
            </div>

            <div className="flex items-start">
              <div className="flex-shrink-0 mt-1">
                <div className="w-6 h-6 rounded-full flex items-center justify-center text-white font-medium" style={{ backgroundColor: themeColor }}>
                  2
                </div>
              </div>
              <div className="ml-3">
                <h4 className="text-base font-medium text-gray-900">Configure additional settings</h4>
                <p className="text-sm text-gray-600 mt-1">
                  Visit the Admin Dashboard to configure additional settings such as authentication providers, 
                  webhooks, and other integrations.
                </p>
              </div>
            </div>

            <div className="flex items-start">
              <div className="flex-shrink-0 mt-1">
                <div className="w-6 h-6 rounded-full flex items-center justify-center text-white font-medium" style={{ backgroundColor: themeColor }}>
                  3
                </div>
              </div>
              <div className="ml-3">
                <h4 className="text-base font-medium text-gray-900">Create your first organization</h4>
                <p className="text-sm text-gray-600 mt-1">
                  Set up an organization for your team and invite members to collaborate on repositories.
                </p>
              </div>
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
};

export default CompletionStep;