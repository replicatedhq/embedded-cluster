import React, { useState, useEffect, useCallback } from "react";
import { InstallationProgressContext, StoredInstallState } from "../contexts/InstallationProgressContext";
import { WizardStep, InstallationPhaseId } from "../types";

const STORAGE_KEY = "embedded-cluster-install-progress";

export const InstallationProgressProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  // Initialize state from sessionStorage or defaults
  const [wizardStep, setWizardStepState] = useState<WizardStep>(() => {
    try {
      const stored = sessionStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed: StoredInstallState = JSON.parse(stored);
        return parsed.wizardStep;
      }
    } catch (error) {
      console.error("Failed to restore installation progress:", error);
    }
    return "welcome";
  });

  const [installationPhase, setInstallationPhaseState] = useState<InstallationPhaseId | undefined>(() => {
    try {
      const stored = sessionStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed: StoredInstallState = JSON.parse(stored);
        return parsed.installationPhase;
      }
    } catch (error) {
      console.error("Failed to restore installation progress:", error);
    }
    return undefined;
  });

  // Wrapper functions that update both state and sessionStorage
  const setWizardStep = useCallback((step: WizardStep) => {
    setWizardStepState(step);
  }, []);

  const setInstallationPhase = useCallback((phase: InstallationPhaseId | undefined) => {
    setInstallationPhaseState(phase);
  }, []);

  const clearProgress = useCallback(() => {
    sessionStorage.removeItem(STORAGE_KEY);
  }, []);

  // Save to sessionStorage whenever state changes
  useEffect(() => {
    const state: StoredInstallState = {
      wizardStep,
      installationPhase,
    };
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch (error) {
      console.error("Failed to save installation progress:", error);
    }
  }, [wizardStep, installationPhase]);

  const value = {
    wizardStep,
    setWizardStep,
    installationPhase,
    setInstallationPhase,
    clearProgress,
  };

  return (
    <InstallationProgressContext.Provider value={value}>
      {children}
    </InstallationProgressContext.Provider>
  );
};
