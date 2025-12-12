import { useMutation } from "@tanstack/react-query";
import { useWizard } from "../contexts/WizardModeContext";
import { useAuth } from "../contexts/AuthContext";
import {
  getWizardBasePath,
  getAppInstallPath,
  createAuthedClient,
} from "../api/client";

export function useProcessAirgap() {
  const { token } = useAuth();
  const { mode } = useWizard();

  return useMutation({
    mutationFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", mode);

      const { data, error } = await client.POST(
        `${apiBase}/airgap/process`,
        {},
      );

      if (error) throw error;
      return data;
    },
  });
}

export function useUpgradeInfra(args?: { ignoreHostPreflights?: boolean }) {
  const { token } = useAuth();

  return useMutation({
    mutationFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", "upgrade");

      const { data, error } = await client.POST(`${apiBase}/infra/upgrade`, {
        body: {
          ignoreHostPreflights: args?.ignoreHostPreflights || false,
        },
      });

      if (error) throw error;
      return data;
    },
  });
}

export function useRunHostPreflights() {
  const { token } = useAuth();

  return useMutation({
    mutationFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", "install");

      const { data, error } = await client.POST(
        `${apiBase}/host-preflights/run`,
        {
          body: { isUi: true },
        },
      );

      if (error) throw error;
      return data;
    },
  });
}

export function useRunAppPreflights() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath(target, mode);

      const { data, error } = await client.POST(
        `${apiBase}/app-preflights/run`,
        {},
      );

      if (error) throw error;
      return data;
    },
  });
}

export function useStartInfraSetup(args?: { ignoreHostPreflights?: boolean }) {
  const { token } = useAuth();

  return useMutation({
    mutationFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", "install");

      const { data, error } = await client.POST(`${apiBase}/infra/setup`, {
        body: {
          ignoreHostPreflights: args?.ignoreHostPreflights || false,
        },
      });

      if (error) throw error;
      return data;
    },
  });
}

export function useStartAppInstallation() {
  const { target, mode } = useWizard();
  const { token } = useAuth();

  return useMutation({
    mutationFn: async (args?: { ignoreAppPreflights?: boolean }) => {
      const client = createAuthedClient(token);
      const path = getAppInstallPath(target, mode);

      const { data, error } = await client.POST(path, {
        body: {
          ignoreAppPreflights: args?.ignoreAppPreflights || false,
        },
      });

      if (error) throw error;
      return data;
    },
  });
}
