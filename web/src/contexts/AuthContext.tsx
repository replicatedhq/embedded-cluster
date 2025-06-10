import React, { createContext, useContext, useState, useEffect } from "react";

interface AuthContextType {
  token: string | null;
  setToken: (token: string | null) => void;
  isAuthenticated: boolean;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [token, setTokenState] = useState<string | null>(() => {
    return localStorage.getItem("auth");
  });

  const setToken = (newToken: string | null) => {
    if (newToken) {
      localStorage.setItem("auth", newToken);
    } else {
      localStorage.removeItem("auth");
    }
    setTokenState(newToken);
  };

  useEffect(() => {
    // Listen for storage events to sync token state across tabs
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === "auth") {
        setTokenState(e.newValue);
      }
    };

    window.addEventListener("storage", handleStorageChange);
    return () => {
      window.removeEventListener("storage", handleStorageChange);
    };
  }, []);

  const value = {
    token,
    setToken,
    isAuthenticated: !!token,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};
