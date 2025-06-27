export const MOCK_INSTALL_CONFIG = {
  adminConsolePort: 8800,
  localArtifactMirrorPort: 8801,
  networkInterface: "eth0",
  installTarget: "linux",
};

export const MOCK_NETWORK_INTERFACES = {
  networkInterfaces: [
    { name: "eth0", address: "192.168.1.1" },
    { name: "eth1", address: "192.168.1.2" },
  ],
};

export const MOCK_PROTOTYPE_SETTINGS = {
  installTarget: "linux",
  title: "Test Cluster",
  description: "Test cluster configuration",
}; 