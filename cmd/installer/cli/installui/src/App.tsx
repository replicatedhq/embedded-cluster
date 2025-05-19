import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider } from './contexts/ConfigContext';
import { WizardModeProvider } from './contexts/WizardModeContext';
import InstallWizard from './components/wizard/InstallWizard';
import PrototypeSettings from './components/prototype/PrototypeSettings';

function App() {
  return (
    <ConfigProvider>
      <div className="min-h-screen bg-gray-50 text-gray-900 font-sans">
        <BrowserRouter>
          <Routes>
            <Route path="/prototype" element={<PrototypeSettings />} />
            <Route 
              path="/" 
              element={
                <WizardModeProvider mode="install">
                  <InstallWizard />
                </WizardModeProvider>
              } 
            />
            <Route 
              path="/upgrade" 
              element={
                <WizardModeProvider mode="upgrade">
                  <InstallWizard />
                </WizardModeProvider>
              } 
            />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </div>
    </ConfigProvider>
  );
}

export default App;