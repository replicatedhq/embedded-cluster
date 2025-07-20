import { useContext } from "react";
import { KubernetesConfigContext, KubernetesConfigContextType } from "../definitions/KubernetesConfigContext";

export const useKubernetesConfig = (): KubernetesConfigContextType => {
  const context = useContext(KubernetesConfigContext);
  if (context === undefined) {
    throw new Error("useKubernetesConfig must be used within a KubernetesConfigProvider");
  }
  return context;
};