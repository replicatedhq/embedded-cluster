import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ConfigProvider } from "./contexts/ConfigContext";
import { WizardModeProvider } from "./contexts/WizardModeContext";
import { BrandingProvider } from "./contexts/BrandingContext";
import InstallWizard from "./components/wizard/InstallWizard";
import { QueryClientProvider } from "@tanstack/react-query";
import { getQueryClient } from "../query-client";

function App() {
  const queryClient = getQueryClient();
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider>
        <BrandingProvider>
          <div className="min-h-screen bg-gray-50 text-gray-900 font-sans">
            <BrowserRouter>
              <Routes>
                <Route
                  path="/"
                  element={
                    <WizardModeProvider mode="install">
                      <InstallWizard />
                    </WizardModeProvider>
                  }
                />

                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </BrowserRouter>
          </div>
        </BrandingProvider>
      </ConfigProvider>
    </QueryClientProvider>
  );
}

export default App;
