import { useMutation } from "@tanstack/react-query";
import { useWizard } from "../contexts/WizardModeContext";
import { useAuth } from "../contexts/AuthContext";
import { getApiBase } from "../utils/api-base";
import { ApiError } from "../utils/api-error";

export function useProcessAirgap() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async () => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/airgap/process`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ isUi: true }),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start airgap processing");
      }
      return response.json();
    },
  });
}

export function useUpgradeInfra() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async (args?: { ignoreHostPreflights?: boolean }) => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/infra/upgrade`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          isUi: true,
          ignoreHostPreflights: args?.ignoreHostPreflights || false,
        }),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start infrastructure upgrade");
      }
      return response.json();
    },
  });
}

export function useRunHostPreflights() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async () => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/host-preflights/run`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ isUi: true }),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start host preflight checks");
      }
      return response.json();
    },
  });
}

export function useRunAppPreflights() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async () => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/app-preflights/run`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ isUi: true }),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start app preflight checks");
      }
      return response.json();
    },
  });
}

export function useStartInfraSetup() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async (args?: { ignoreHostPreflights?: boolean }) => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/infra/setup`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          ignoreHostPreflights: args?.ignoreHostPreflights || false,
        }),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start infrastructure setup");
      }
      return response.json();
    },
  });
}

export function useStartAppInstallation() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async (args?: { ignoreAppPreflights?: boolean }) => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/app/${mode}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          ignoreAppPreflights: args?.ignoreAppPreflights || false,
        }),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start application installation");
      }
      return response.json();
    },
  });
}
