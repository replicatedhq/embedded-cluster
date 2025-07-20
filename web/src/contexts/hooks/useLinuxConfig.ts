import { useContext } from "react";
import { LinuxConfigContext, LinuxConfigContextType } from "../definitions/LinuxConfigContext";

export const useLinuxConfig = (): LinuxConfigContextType => {
  const context = useContext(LinuxConfigContext);
  if (context === undefined) {
    throw new Error("useLinuxConfig must be used within a LinuxConfigProvider");
  }
  return context;
};