import React from "react";
import { WizardContext, getTextVariations, WizardModeContextType } from "../definitions/WizardModeContext";
import { useInitialState } from "../hooks/useInitialState";

export const WizardProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { title, installTarget } = useInitialState();
  const mode = "install"; // TODO: get mode from initial state
  const isLinux = installTarget === "linux";
  const text = getTextVariations(isLinux, title)[mode];

  const value: WizardModeContextType = {
    target: installTarget,
    mode,
    text,
  };

  return <WizardContext.Provider value={value}>{children}</WizardContext.Provider>;
};
