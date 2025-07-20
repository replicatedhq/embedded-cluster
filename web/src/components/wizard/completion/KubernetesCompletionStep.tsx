import React, { useState } from "react";
import Card from "../../common/Card";
import Button from "../../common/Button";
import { useKubernetesConfig } from "../../../contexts/hooks/useKubernetesConfig";
import { useInitialState } from "../../../contexts/hooks/useInitialState";
import { useSettings } from "../../../contexts/hooks/useSettings";
import { CheckCircle, ClipboardCheck, Copy, Terminal } from "lucide-react";

const KubernetesCompletionStep: React.FC = () => {
  const [copied, setCopied] = useState(false);
  const { config } = useKubernetesConfig();
  const { title } = useInitialState();
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const copyInstallCommand = () => {
    if (config.installCommand) {
      navigator.clipboard.writeText(config.installCommand).then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      });
    }
  };

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
                  <Terminal className="w-4 h-4 text-gray-400 mt-0.5 flex-shrink-0" />
                  <code className="font-mono text-sm text-left">
                    {config.installCommand}
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
