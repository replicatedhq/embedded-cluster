import React, { useState, useEffect } from "react";
import { handleUnauthorized } from "../utils/auth";
import { useInitialState } from "../contexts/InitialStateContext";
import { AuthContext } from "../contexts/AuthContext";

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
            // If we get a 401, handle it
            if (response.status === 401) {
              const error = new Error("Unauthorized");
              (error as Error & { status?: number }).status = 401;
              handleUnauthorized(error);
            }
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
    return () => {
      window.removeEventListener("storage", handleStorageChange);
    };
  }, []);

  const value = {
    token,
    setToken,
    isAuthenticated: !!token,
    isLoading,
  };

  if (isLoading) {
    return null; // Don't render anything while checking token validity
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};