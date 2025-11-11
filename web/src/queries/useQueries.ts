import { useQuery } from "@tanstack/react-query";
import { useWizard } from "../contexts/WizardModeContext";
import { useAuth } from "../contexts/AuthContext";
import { getWizardBasePath, createAuthedClient } from "../api/client";

/**
 * Query hook to fetch airgap status
 * Used to poll airgap processing status during bundle processing
 */
export function useAirgapStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
}) {
  const { token } = useAuth();
  const { mode } = useWizard();

  return useQuery({
    queryKey: ["airgapStatus", mode],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", mode);

      const { data } = await client.GET(`${apiBase}/airgap/status`);

      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
  });
}

/**
 * Query hook to fetch host preflight status
 * Used to poll host preflight check status during validation
 */
export function useHostPreflightStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
  gcTime?: number;
}) {
  const { token } = useAuth();

  return useQuery({
    queryKey: ["hostPreflightStatus"],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", "install");

      const { data } = await client.GET(`${apiBase}/host-preflights/status`);

      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    gcTime: options?.gcTime,
  });
}

/**
 * Query hook to fetch app preflight status
 * Used to poll app preflight check status during validation
 */
export function useAppPreflightStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
}) {
  const { token } = useAuth();
  const { target, mode } = useWizard();

  return useQuery({
    queryKey: ["appPreflightStatus", target, mode],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath(target, mode);

      const { data } = await client.GET(`${apiBase}/app-preflights/status`);

      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
  });
}

/**
 * Query hook to fetch linux infra status
 * Used to poll infrastructure installation/upgrade status
 */
export function useLinuxInfraStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
}) {
  const { token } = useAuth();
  const { mode } = useWizard();

  return useQuery({
    queryKey: ["infraStatus", "linux", mode],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", mode);

      const { data } = await client.GET(`${apiBase}/infra/status`);

      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
  });
}

/**
 * Query hook to fetch kubernetes infra status
 * Used to poll infrastructure installation/upgrade status
 */
export function useKubernetesInfraStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
}) {
  const { token } = useAuth();

  return useQuery({
    queryKey: ["infraStatus", "kubernetes", "install"],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("kubernetes", "install");

      const { data } = await client.GET(`${apiBase}/infra/status`);

      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
  });
}

/**
 * Query hook to fetch app installation status
 * Used to poll app installation status
 */
export function useAppInstallStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
}) {
  const { token } = useAuth();
  const { target, mode } = useWizard();

  return useQuery({
    queryKey: ["appInstallationStatus", target, mode],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath(target, mode);

      const { data } = await client.GET(`${apiBase}/app/status`);

      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
  });
}

/**
 * Query hook to fetch linux installation config
 * Used to get installation configuration for setup forms
 */
export function useLinuxInstallConfig() {
  const { token } = useAuth();

  return useQuery({
    queryKey: ["installConfig", "linux"],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("linux", "install");

      const { data } = await client.GET(`${apiBase}/installation/config`);

      return data;
    },
  });
}

/**
 * Query hook to fetch kubernetes installation config
 * Used to get installation configuration for setup forms
 */
export function useKubernetesInstallConfig() {
  const { token } = useAuth();

  return useQuery({
    queryKey: ["installConfig", "kubernetes"],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath("kubernetes", "install");

      const { data } = await client.GET(`${apiBase}/installation/config`);

      return data;
    },
  });
}

/**
 * Query hook to fetch installation status
 * Used to poll installation status after configuration submission
 */
export function useInstallationStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
  gcTime?: number;
}) {
  const { token } = useAuth();
  const { target } = useWizard();

  return useQuery({
    queryKey: ["installationStatus", target],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getWizardBasePath(target, "install");

      const { data } = await client.GET(`${apiBase}/installation/status`);

      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    gcTime: options?.gcTime,
  });
}

/**
 * Query hook to fetch available network interfaces
 * Used in setup steps to populate network interface dropdown
 */
export function useNetworkInterfaces() {
  const { token } = useAuth();

  return useQuery({
    queryKey: ["networkInterfaces"],
    queryFn: async () => {
      const client = createAuthedClient(token);

      const { data } = await client.GET(
        "/console/available-network-interfaces",
      );

      return data;
    },
  });
}
