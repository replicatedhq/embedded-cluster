import React from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { useConfig } from "../../contexts/ConfigContext";
import { useBranding } from "../../contexts/BrandingContext";
import { CheckCircle, ExternalLink } from "lucide-react";

const CompletionStep: React.FC = () => {
  const { config } = useConfig();
  const { title } = useBranding();

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-6">
          <div className="flex flex-col items-center justify-center mb-6">
            <div className="w-16 h-16 rounded-full flex items-center justify-center mb-4">
              <CheckCircle className="w-10 h-10" style={{ color: "blue" }} />
            </div>
            <p className="text-gray-600 mt-2">
              Visit the Admin Console to configure and install {title}
            </p>
            <Button
              className="mt-4"
              onClick={() => window.open(`http://${window.location.hostname}:${config.adminConsolePort}`, "_blank")}
              icon={<ExternalLink className="ml-2 mr-1 h-5 w-5" />}
            >
              Visit Admin Console
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
};

export default CompletionStep;
