import React from "react";
import Card from "../../common/Card";
import { useInitialState } from "../../../contexts/InitialStateContext";
import { useSettings } from "../../../contexts/SettingsContext";
import { CheckCircle } from "lucide-react";

const KubernetesCompletionStep: React.FC = () => {
  const { title } = useInitialState();
  const { settings: { themeColor } } = useSettings();

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
          </div>
        </div>
      </Card>
    </div>
  );
};

export default KubernetesCompletionStep;
