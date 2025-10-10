import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { LinuxConfigProvider } from "./providers/LinuxConfigProvider";
import { KubernetesConfigProvider } from "./providers/KubernetesConfigProvider";
import { SettingsProvider } from "./providers/SettingsProvider";
import { WizardProvider } from "./providers/WizardProvider";
import { InitialStateProvider } from "./providers/InitialStateProvider";
import { AuthProvider } from "./providers/AuthProvider";
import { InstallationProgressProvider } from "./providers/InstallationProgressProvider";
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
            <InstallationProgressProvider>
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
            </InstallationProgressProvider>
          </SettingsProvider>
        </AuthProvider>
        <ConnectionMonitor />
      </QueryClientProvider>
    </InitialStateProvider>
  );
}

export default App;
