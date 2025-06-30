import React, { createContext, useContext } from "react";

interface Branding {
  title: string;
  icon?: string;
}

export const BrandingContext = createContext<Branding>({ title: "My App" });

export const useBranding = () => {
  const context = useContext(BrandingContext);
  if (!context) {
    throw new Error("useBranding must be used within a BrandingProvider");
  }
  return context;
};

export const BrandingProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  // __INITIAL_STATE__ is a global variable that can be set by the server-side rendering process
  // as a way to pass initial data to the client.
  const initialState = window.__INITIAL_STATE__ || {};

  const branding = {
    title: initialState.title || "My App",
    icon: initialState.icon,
  };

  return (
    <BrandingContext.Provider value={branding}>
      {children}
    </BrandingContext.Provider>
  );
};
