import React, { useState, useEffect } from "react";
import { AuthContext, AuthContextType } from "../definitions/AuthContext";
import { handleUnauthorized } from "../../utils/auth";
import { useInitialState } from "../hooks/useInitialState";

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [token, setTokenState] = useState<string | null>(() => {
    return localStorage.getItem("auth");
  });
  const [isLoading, setIsLoading] = useState(true);

  const setToken = (newToken: string | null) => {
    if (newToken) {
      localStorage.setItem("auth", newToken);
    } else {
      localStorage.removeItem("auth");
    }
    setTokenState(newToken);
  };
  // Get the installation target from initial state
  const { installTarget } = useInitialState()
  // Check token validity on mount and when token changes
  useEffect(() => {
    if (token) {
      // Make a request to any authenticated endpoint to check token validity
      // Use the correct target-specific endpoint based on installation target
      fetch(`/api/${installTarget}/install/installation/config`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })
        .then((response) => {
          if (!response.ok) {
            const err = new Error(`HTTP error! status: ${response.status}`);
            (err as Error & { status?: number }).status = response.status;
            throw err;
          }
          setIsLoading(false);
        })
        .catch(() => {
          // If the request fails, assume the token is invalid
          const err = new Error("Request failed");
          (err as Error & { status?: number }).status = 401;
          handleUnauthorized(err);
          setIsLoading(false);
        });
    } else {
      setIsLoading(false);
    }
  }, [token, installTarget]);

  useEffect(() => {
    // Listen for storage events to sync token state across tabs
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === "auth") {
        setTokenState(e.newValue);
      }
    };

    window.addEventListener("storage", handleStorageChange);
    return () => window.removeEventListener("storage", handleStorageChange);
  }, []);

  const value: AuthContextType = {
    token,
    setToken,
    isAuthenticated: !!token,
    isLoading,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};
