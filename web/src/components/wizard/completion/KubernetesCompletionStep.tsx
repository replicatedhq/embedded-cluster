import React, { useState } from "react";
import Card from "../../common/Card";
import Button from "../../common/Button";
import { useInitialState } from "../../../contexts/InitialStateContext";
import { useSettings } from "../../../contexts/SettingsContext";
import { CheckCircle, ClipboardCheck, Copy, Terminal, Loader2, XCircle } from "lucide-react";
import { ApiError } from '../../../api/error';
import { useInstallConfig } from '../../../queries/useQueries';

const KubernetesCompletionStep: React.FC = () => {
  const [copied, setCopied] = useState(false);
  const { title } = useInitialState();
  const { settings: { themeColor } } = useSettings();

  // Query for fetching install configuration
  const { isLoading, error, data: config } = useInstallConfig();

  const copyInstallCommand = () => {
    if (config?.resolved.installCommand) {
      navigator.clipboard.writeText(config?.resolved.installCommand).then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      });
    }
  };

  // Loading state
  if (isLoading) {
    return (
      <div className="space-y-6" data-testid="kubernetes-completion-loading">
        <Card>
          <div className="flex flex-col items-center justify-center py-12">
            <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
            <p className="text-gray-600">Loading installation configuration...</p>
          </div>
        </Card>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="space-y-6" data-testid="kubernetes-completion-error">
        <Card>
          <div className="flex flex-col items-center justify-center py-12">
            <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
              <XCircle className="w-6 h-6 text-red-600" />
            </div>
            <p className="text-lg font-medium text-gray-900">Failed to load installation configuration</p>
            <p className="text-sm text-red-600 mt-2">
              {error instanceof ApiError ? error.details || error.message : error.message}
            </p>
          </div>
        </Card>
      </div>
    );
  }

  // Success state
  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-6">
          <div className="flex flex-col items-center justify-center mb-6">
            <div className="w-16 h-16 rounded-full flex items-center justify-center mb-4">
              <CheckCircle className="w-10 h-10" style={{ color: themeColor }} />
            </div>
            <p className="text-gray-600 mt-2 mb-6" data-testid="completion-message">
              Visit the Admin Console to configure and install {title}
            </p>
            <div className="w-full max-w-2xl space-y-6">
              <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-sm font-medium text-gray-700">Installation Command</h3>
                  <Button
                    variant="outline"
                    size="sm"
                    className="py-1 px-2 text-xs"
                    icon={copied ? <ClipboardCheck className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                    onClick={copyInstallCommand}
                  >
                    {copied ? 'Copied!' : 'Copy Command'}
                  </Button>
                </div>
                <div className="flex items-start space-x-2 p-2 bg-white rounded border border-gray-300">
                  <Terminal className="w-4 h-4 text-gray-400 mt-0.5 shrink-0" />
                  <code className="font-mono text-sm text-left">
                    {config?.resolved.installCommand}
                  </code>
                </div>
              </div>
              <p className="text-sm text-gray-500">
                Run this command to access the Admin Console
              </p>
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
};

export default KubernetesCompletionStep;
