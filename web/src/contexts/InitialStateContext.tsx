import { createContext, useContext } from "react";
import { InitialState } from "../types";

export const InitialStateContext = createContext<InitialState>({ title: "My App", installTarget: "linux" });

export const useInitialState = () => {
  const context = useContext(InitialStateContext);
  if (!context) {
    throw new Error("useInitialState must be used within a InitialStateProvider");
  }
  return context;
};
