import React, { useEffect, useState } from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import Input from "../common/Input";
import { AppIcon } from "../common/Logo";
import { ChevronRight, Lock } from "lucide-react";
import { useWizard } from "../../contexts/WizardModeContext";
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../contexts/AuthContext";
import { ApiError } from "../../api/error";
import { createBaseClient } from "../../api/client";
import type { components } from "../../types/api";

interface WelcomeStepProps {
  onNext: () => void;
}

type LoginResponse = components["schemas"]["types.AuthResponse"];

const INCORRECT_PASSWORD_ERROR = "Incorrect password";

const WelcomeStep: React.FC<WelcomeStepProps> = ({ onNext }) => {
  const { text } = useWizard();
  const [password, setPassword] = useState("");
  const [loginError, setLoginError] = useState<string | undefined>();
  const { setToken, isAuthenticated } = useAuth();

  // Automatically redirect to SetupStep if already authenticated
  useEffect(() => {
    if (isAuthenticated) {
      onNext();
    }
  }, [isAuthenticated, onNext]);

  const {
    mutate: login,
    isPending: isLoading,
  } = useMutation<LoginResponse, ApiError, string>({
    retry(failureCount, error) {
      if (error.statusCode === 401) {
        return false; // Don't retry on incorrect password
      }
      // Otherwise retry once, keep the default retry logic
      return failureCount < 1;
    },
    mutationFn: async (password: string) => {
      const apiClient = createBaseClient();
      const { data, error } = await apiClient.POST("/auth/login", {
        body: { password },
      });
      if (error) {
        throw error
      }
      return data
    },
    onSuccess: (data) => {
      setToken(data.token);
      onNext();
    },
    onError: (error) => {
      if (error.statusCode === 401) {
        setLoginError(INCORRECT_PASSWORD_ERROR);
      } else {
        setLoginError(`Login failed with: ${error.details || error.message}`);
      }
    },
  });

  const handlePasswordChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setPassword(e.target.value);
  };

  const handleSubmit = () => {
    // No point in making a request if password is empty
    if (!password) {
      setLoginError(INCORRECT_PASSWORD_ERROR);
      return
    }
    login(password);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSubmit();
    }
  };
  // If already authenticated, don't render the welcome step
  if (isAuthenticated) {
    return null;
  }

  return (
    <div className="space-y-6" data-testid="welcome-step">
      <Card>
        <div className="flex flex-col items-center text-center py-12">
          <AppIcon className="h-20 w-20 mb-6" />
          <h2 className="text-3xl font-bold text-gray-900">{text.welcomeTitle}</h2>
          <p className="text-xl text-gray-600 max-w-2xl mb-8">{text.welcomeDescription}</p>
          <div className="w-full max-w-md mb-8">
            <Input
              id="password"
              label="Enter Password"
              type="password"
              value={password}
              onChange={handlePasswordChange}
              onKeyDown={handleKeyDown}
              error={loginError}
              required
              icon={<Lock className="w-5 h-5" />}
              className="w-full"
              dataTestId="password-input"
            />

            <Button
              onClick={handleSubmit}
              size="lg"
              className="w-full mt-4"
              icon={<ChevronRight className="w-5 h-5" />}
              disabled={isLoading}
              dataTestId="welcome-button-next"
            >
              {text.welcomeButtonText}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
};

export default WelcomeStep;
