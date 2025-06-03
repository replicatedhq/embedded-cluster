import React, { useState } from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { useConfig } from "../../contexts/ConfigContext";
import { useWizardMode } from "../../contexts/WizardModeContext";
import { ChevronLeft, ChevronRight } from "lucide-react";
import LinuxSetup from "./setup/LinuxSetup";
import KubernetesSetup from "./setup/KubernetesSetup";
import LinuxPreflightCheck from "./setup/LinuxPreflightCheck";
import { useQuery, useMutation } from "@tanstack/react-query";

interface SetupStepProps {
  onNext: () => void;
  onBack: () => void;
}

type SetupPhase = "configuration" | "validation";

const SetupStep: React.FC<SetupStepProps> = ({ onNext, onBack }) => {
  const { config, updateConfig, prototypeSettings } = useConfig();
  const { text } = useWizardMode();
  const [showAdvanced, setShowAdvanced] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [preflightComplete, setPreflightComplete] = useState(false);
  const [preflightSuccess, setPreflightSuccess] = useState(false);
  const [phase, setPhase] = useState<SetupPhase>("configuration");

  // Query for fetching install configuration
  const { isLoading: isConfigLoading } = useQuery({
    queryKey: ["installConfig"],
    queryFn: async () => {
      const response = await fetch("/api/install/installation/config", {
        headers: {
          ...(localStorage.getItem("auth") && {
            Authorization: `Bearer ${localStorage.getItem("auth")}`,
          }),
        },
      });
      if (!response.ok) {
        throw new Error("Failed to fetch install configuration");
      }
      const config = await response.json();
      updateConfig(config);
      return config;
    },
  });

  // Query for fetching network interfaces
  const { data: networkInterfacesData, isLoading: isInterfacesLoading } =
    useQuery({
      queryKey: ["networkInterfaces"],
      queryFn: async () => {
        const response = await fetch(
          "/api/console/available-network-interfaces",
          {
            headers: {
              ...(localStorage.getItem("auth") && {
                Authorization: `Bearer ${localStorage.getItem("auth")}`,
              }),
            },
          }
        );
        if (!response.ok) {
          throw new Error("Failed to fetch network interfaces");
        }
        return response.json();
      },
    });

  // Mutation for submitting the configuration
  const {
    mutate: submitConfig,
    isPending: isSubmitting,
    error: submitError,
  } = useMutation({
    mutationFn: async (configData: typeof config) => {
      const response = await fetch("/api/install/installation/configure", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(localStorage.getItem("auth") && {
            Authorization: `Bearer ${localStorage.getItem("auth")}`,
          }),
        },
        body: JSON.stringify(configData),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw errorData;
      }
      return response.json();
    },
    onSuccess: () => {
      // Call the original onNext function to proceed to the next step
      if (phase === "configuration") {
        setPhase("validation");
      } else {
        onNext();
      }
    },
    onError: (err: any) => {
      setError(err.message || "Failed to setup cluster");
      return err;
    },
  });

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, value } = e.target;
    if (id === "adminConsolePort" || id === "localArtifactMirrorPort") {
      // Only update if the value is empty or a valid number
      if (value === "" || !isNaN(Number(value))) {
        updateConfig({ [id]: value === "" ? undefined : Number(value) });
      }
    } else {
      updateConfig({ [id]: value });
    }
  };

  const handleSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const { id, value } = e.target;
    updateConfig({ [id]: value });
  };

  const handleNext = async () => {
    submitConfig(config);
  };

  const handlePreflightComplete = (success: boolean) => {
    setPreflightComplete(true);
    setPreflightSuccess(success);
  };

  const handleBackPhase = () => {
    if (phase === "validation") {
      setPhase("configuration");
    } else {
      onBack();
    }
  };

  const isLoading = isConfigLoading || isInterfacesLoading;
  const availableNetworkInterfaces =
    networkInterfacesData?.networkInterfaces || [];

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">
            {text.setupTitle}
          </h2>
          <p className="text-gray-600 mt-1">
            {prototypeSettings.clusterMode === "embedded" &&
            phase === "configuration"
              ? "Configure the installation settings."
              : text.setupDescription}
          </p>
        </div>

        {isLoading ? (
          <div className="py-4 text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-2 text-gray-600">Loading configuration...</p>
          </div>
        ) : prototypeSettings?.clusterMode === "embedded" ? (
          phase === "configuration" ? (
            <LinuxSetup
              config={config}
              prototypeSettings={prototypeSettings}
              showAdvanced={showAdvanced}
              onShowAdvancedChange={setShowAdvanced}
              onInputChange={handleInputChange}
              onSelectChange={handleSelectChange}
              availableNetworkInterfaces={availableNetworkInterfaces}
              fieldErrors={submitError?.errors || []}
            />
          ) : (
            <LinuxPreflightCheck
              config={config}
              onComplete={handlePreflightComplete}
            />
          )
        ) : (
          <KubernetesSetup config={config} onInputChange={handleInputChange} />
        )}

        {error && (
          <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md">
            Please fix the errors in the form above before proceeding.
          </div>
        )}
      </Card>

      <div className="flex justify-between">
        <Button
          variant="outline"
          onClick={handleBackPhase}
          icon={<ChevronLeft className="w-5 h-5" />}
        >
          Back
        </Button>
        {prototypeSettings?.clusterMode === "embedded" &&
          (phase === "configuration" ? (
            <Button
              onClick={handleNext}
              icon={<ChevronRight className="w-5 h-5" />}
            >
              Next: Validate Host
            </Button>
          ) : (
            <Button
              onClick={handleNext}
              disabled={!preflightComplete || !preflightSuccess}
              icon={<ChevronRight className="w-5 h-5" />}
            >
              {isSubmitting ? "Setting up..." : text.nextButtonText}
            </Button>
          ))}
      </div>
    </div>
  );
};

export default SetupStep;
