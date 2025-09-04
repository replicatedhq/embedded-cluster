import React from "react";
import { InitialStateContext } from "../contexts/InitialStateContext";
import { InstallationTarget, isInstallationTarget } from "../types/installation-target";
import { InitialState } from "../types";

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
  const initialState: Partial<InitialState> = window.__INITIAL_STATE__ || {};

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
