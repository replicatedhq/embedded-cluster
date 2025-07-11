import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { LinuxConfigProvider } from "./contexts/LinuxConfigContext";
import { KubernetesConfigProvider } from "./contexts/KubernetesConfigContext";
import { SettingsProvider } from "./contexts/SettingsContext";
import { WizardProvider } from "./contexts/WizardModeContext";
import { InitialStateProvider } from "./contexts/InitialStateContext";
import { AuthProvider } from "./contexts/AuthContext";
import ConnectionMonitor from "./components/common/ConnectionMonitor";
import InstallWizard from "./components/wizard/InstallWizard";
import { QueryClientProvider } from "@tanstack/react-query";
import { getQueryClient } from "./query-client";

function App() {
  const queryClient = getQueryClient();
  return (
    <InitialStateProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <SettingsProvider>
            <LinuxConfigProvider>
              <KubernetesConfigProvider>
                <div className="min-h-screen bg-gray-50 text-gray-900 font-sans">
                  <BrowserRouter>
                    <Routes>
                      <Route
                        path="/"
                        element={
                          <WizardProvider>
                            <InstallWizard />
                          </WizardProvider>
                        }
                      />
                      <Route path="*" element={<Navigate to="/" replace />} />
                    </Routes>
                  </BrowserRouter>
                </div>
              </KubernetesConfigProvider>
            </LinuxConfigProvider>
          </SettingsProvider>
        </AuthProvider>
        <ConnectionMonitor />
      </QueryClientProvider>
    </InitialStateProvider>
  );
}

export default App;
