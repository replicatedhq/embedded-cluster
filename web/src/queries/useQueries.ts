import { useQuery } from "@tanstack/react-query";
import { useWizard } from "../contexts/WizardModeContext";
import { useAuth } from "../contexts/AuthContext";
import { getApiBasePath, createAuthedClient } from "../api/client";

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
      const apiBase = getApiBasePath("linux", mode);

      const { data, error } = await client.GET(`${apiBase}/airgap/status`);

      if (error) throw error;
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
      const apiBase = getApiBasePath("linux", "install");

      const { data, error } = await client.GET(
        `${apiBase}/host-preflights/status`,
      );

      if (error) throw error;
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
      const apiBase = getApiBasePath(target, mode);

      const { data, error } = await client.GET(
        `${apiBase}/app-preflights/status`,
      );

      if (error) throw error;
      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
  });
}

/**
 * Query hook to fetch infra status
 * Used to poll infrastructure installation/upgrade status
 */
export function useInfraStatus(options?: {
  enabled?: boolean;
  refetchInterval?: number;
}) {
  const { token } = useAuth();
  const { mode } = useWizard();

  return useQuery({
    queryKey: ["infraStatus", mode],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getApiBasePath("linux", mode);

      const { data, error } = await client.GET(`${apiBase}/infra/status`);

      if (error) throw error;
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
      const apiBase = getApiBasePath(target, mode);

      const { data, error } = await client.GET(`${apiBase}/app/status`);

      if (error) throw error;
      return data;
    },
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
  });
}

/**
 * Query hook to fetch installation config
 * Used to get installation configuration for setup forms
 */
export function useInstallConfig() {
  const { token } = useAuth();
  const { target } = useWizard();

  return useQuery({
    queryKey: ["installConfig", target],
    queryFn: async () => {
      const client = createAuthedClient(token);
      const apiBase = getApiBasePath(target, "install");

      const { data, error } = await client.GET(
        `${apiBase}/installation/config`,
      );

      if (error) throw error;
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
      const apiBase = getApiBasePath(target, "install");

      const { data, error } = await client.GET(
        `${apiBase}/installation/status`,
      );

      if (error) throw error;
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

      const { data, error } = await client.GET(
        "/console/available-network-interfaces",
      );

      if (error) throw error;
      return data;
    },
  });
}
