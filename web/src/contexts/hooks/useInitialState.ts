import { useContext } from "react";
import { InitialStateContext } from "../definitions/InitialStateContext";

export const useInitialState = () => {
  const context = useContext(InitialStateContext);
  if (!context) {
    throw new Error("useInitialState must be used within a InitialStateProvider");
  }
  return context;
};