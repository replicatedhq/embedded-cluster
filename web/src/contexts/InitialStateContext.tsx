import React, { createContext, useContext } from "react";
import { InitialState } from "../types";
import { InstallationTarget, isInstallationTarget } from "../types/installation-target";

export const InitialStateContext = createContext<InitialState>({ title: "My App", installTarget: "linux" });

export const useInitialState = () => {
  const context = useContext(InitialStateContext);
  if (!context) {
    throw new Error("useInitialState must be used within a InitialStateProvider");
  }
  return context;
};

function parseInstallationTarget(target: string): InstallationTarget {
  if (isInstallationTarget(target)) {
    return target;
  }
  throw new Error(`Invalid installation target: ${target}`);
}

export const InitialStateProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  // __INITIAL_STATE__ is a global variable that can be set by the server-side rendering process
  // as a way to pass initial data to the client.
  const initialState = window.__INITIAL_STATE__ || {};

  const state = {
    title: initialState.title || "My App",
    icon: initialState.icon,
    installTarget: parseInstallationTarget(initialState.installTarget || "linux"), // default to "linux" if not provided
  };

  return (
    <InitialStateContext.Provider value={state}>
      {children}
    </InitialStateContext.Provider>
  );
};
