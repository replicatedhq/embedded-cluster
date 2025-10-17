import React from "react";
import Card from "../../common/Card";
import Button from "../../common/Button";
import { useInitialState } from "../../../contexts/InitialStateContext";
import { useSettings } from "../../../contexts/SettingsContext";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "../../../contexts/AuthContext";
import { CheckCircle, ExternalLink, Loader2, XCircle } from "lucide-react";
import { getApiBase } from '../../../utils/api-base';
import { LinuxConfigResponse } from "../../../types";
import { ApiError } from '../../../utils/api-error';

const LinuxCompletionStep: React.FC = () => {
  const { title, installTarget, mode } = useInitialState();
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const { token } = useAuth();
  const apiBase = getApiBase(installTarget, mode);
  // Query for fetching install configuration
  const { isLoading, error, data: config } = useQuery<LinuxConfigResponse, Error>({
    queryKey: ["installConfig"],
    queryFn: async () => {
      const response = await fetch(`${apiBase}/installation/config`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to fetch install configuration")
      }
      const configResponse = await response.json() as LinuxConfigResponse;
      return configResponse;
    },
  });

  // Loading state
  if (isLoading) {
    return (
      <div className="space-y-6" data-testid="linux-completion-loading">
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
      <div className="space-y-6" data-testid="linux-completion-error">
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
            <p className="text-gray-600 mt-2" data-testid="completion-message">
              Visit the Admin Console to configure and install {title}
            </p>
            <Button
              className="mt-4"
              dataTestId="admin-console-button"
              onClick={() => window.open(`http://${window.location.hostname}:${config?.resolved.adminConsolePort}`, "_blank")}
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

export default LinuxCompletionStep;
