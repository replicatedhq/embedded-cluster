import { createContext, useContext } from "react";
import { InitialState } from "../types";

export const defaultInitialState: InitialState = {
  title: "My App",
  installTarget: "linux", // default to "linux" if not provided
  mode: "install", // default to "install" if not provided
  isAirgap: false, // default to false if not provided
  requiresInfraUpgrade: false, // default to false if not provided
};

export const InitialStateContext = createContext<InitialState>(defaultInitialState);

export const useInitialState = () => {
  const context = useContext(InitialStateContext);
  if (!context) {
    throw new Error("useInitialState must be used within a InitialStateProvider");
  }
  return context;
};
