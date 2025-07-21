import React from "react";
import { InitialStateContext } from "../definitions/InitialStateContext";
import { InitialState } from "../../types";
import { InstallationTarget, isInstallationTarget } from "../../types/installation-target";

function parseInstallationTarget(target: string): InstallationTarget {
  if (isInstallationTarget(target)) {
    return target;
  }
  return "linux"; // fallback
}

export const InitialStateProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const getInitialState = (): InitialState => {
    const dataElement = document.getElementById("initial-state");
    if (dataElement && dataElement.textContent) {
      try {
        const parsed = JSON.parse(dataElement.textContent);
        return {
          title: parsed.title || "My App",
          installTarget: parseInstallationTarget(parsed.installTarget),
        };
      } catch (e) {
        console.error("Failed to parse initial state:", e);
      }
    }
    
    return {
      title: "My App",
      installTarget: "linux",
    };
  };

  const value = getInitialState();

  return <InitialStateContext.Provider value={value}>{children}</InitialStateContext.Provider>;
};
