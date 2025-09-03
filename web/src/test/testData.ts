export const MOCK_LINUX_INSTALL_CONFIG = {
  adminConsolePort: 8800,
  localArtifactMirrorPort: 8801,
  networkInterface: "eth0",
};

export const MOCK_LINUX_INSTALL_CONFIG_RESPONSE = {
  values: {
    adminConsolePort: 8800,
    localArtifactMirrorPort: 8801,
    networkInterface: "eth0",
    dataDirectory: "/custom/data/dir",
  },
  defaults: {
    adminConsolePort: 30000,
    localArtifactMirrorPort: 50000,
    dataDirectory: "/var/lib/embedded-cluster",
    globalCidr: "10.244.0.0/16",
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
    networkInterface: "eth0",
  },
  resolved: {
    adminConsolePort: 8800,
    localArtifactMirrorPort: 8801,
    networkInterface: "eth0",
    dataDirectory: "/custom/data/dir",
    globalCidr: "10.244.0.0/16",
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
};

export const MOCK_LINUX_INSTALL_CONFIG_RESPONSE_WITH_ZEROS = {
  values: {
    adminConsolePort: 0,
    localArtifactMirrorPort: 0,
    networkInterface: "eth0",
    dataDirectory: "/custom/data/dir",
  },
  defaults: {
    adminConsolePort: 30000,
    localArtifactMirrorPort: 50000,
    dataDirectory: "/var/lib/embedded-cluster",
    globalCidr: "10.244.0.0/16",
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
    networkInterface: "eth0",
  },
  resolved: {
    adminConsolePort: 30000,
    localArtifactMirrorPort: 50000,
    networkInterface: "eth0",
    dataDirectory: "/custom/data/dir",
    globalCidr: "10.244.0.0/16",
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
};

export const MOCK_LINUX_INSTALL_CONFIG_RESPONSE_EMPTY = {
  values: {
    dataDirectory: "",
  },
  defaults: {
    adminConsolePort: 30000,
    localArtifactMirrorPort: 50000,
    dataDirectory: "/var/lib/embedded-cluster",
    globalCidr: "10.244.0.0/16",
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
    networkInterface: "eth0",
  },
  resolved: {
    adminConsolePort: 30000,
    localArtifactMirrorPort: 50000,
    dataDirectory: "/var/lib/embedded-cluster",
    globalCidr: "10.244.0.0/16",
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
    networkInterface: "eth0",
  },
};

export const MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE = {
  values: {
    adminConsolePort: 8800,
  },
  defaults: {
    adminConsolePort: 30000,
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
  resolved: {
    adminConsolePort: 8800,
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
};

export const MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE_WITH_ZEROS = {
  values: {
    adminConsolePort: 0,
  },
  defaults: {
    adminConsolePort: 30000,
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
  resolved: {
    adminConsolePort: 30000,
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
};

export const MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE_EMPTY = {
  values: {},
  defaults: {
    adminConsolePort: 30000,
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
  resolved: {
    adminConsolePort: 30000,
    httpProxy: "",
    httpsProxy: "",
    noProxy: "localhost,127.0.0.1",
  },
};

export const MOCK_NETWORK_INTERFACES = {
  networkInterfaces: ["eth0", "eth1"]
};

export const MOCK_PROTOTYPE_SETTINGS = {
  installTarget: "linux",
  title: "Test Cluster",
  description: "Test cluster configuration",
}; 
