package cli

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli/installui"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type API struct {
	installFlags    InstallCmdFlags
	metricsReporter preflights.MetricsReporter

	isInstalling     bool
	isInstallSuccess bool
	installError     error
}

// ClusterSetupRequest represents the JSON request for cluster setup
type ClusterSetupRequest struct {
	ClusterConfig ClusterConfig `json:"clusterConfig"`
}

func NewAPI(ctx context.Context, installFlags InstallCmdFlags, metricsReporter preflights.MetricsReporter) (*API, error) {
	return &API{installFlags: installFlags, metricsReporter: metricsReporter}, nil
}

func (a *API) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	uiFs := http.FileServer(http.FS(installui.Fs()))
	// uiFs := http.FileServer(http.FS(os.DirFS("./cmd/installer/cli/installui/dist")))
	mux.Handle("/", uiFs)

	up := userProvider{
		username: "admin",
		password: a.installFlags.adminConsolePassword,
	}

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/api/login", authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}, up))

	mux.HandleFunc("/api/host-network-interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"networkInterfaces": []string{"eth0", "eth1"},
		})
	})

	mux.HandleFunc("/api/cluster-config", authenticate(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			a.getClusterConfig(w, r)
		case http.MethodPost:
			a.postClusterConfig(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}, up))

	mux.HandleFunc("/api/install-cluster", authenticate(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			a.postInstallCluster(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}, up))

	return http.ListenAndServe(fmt.Sprintf(":%d", a.installFlags.guidedExperiencePort), mux)
}

type ClusterConfig struct {
	DataDirectory        string `json:"dataDirectory"`
	HTTPProxy            string `json:"httpProxy"`
	HTTPSProxy           string `json:"httpsProxy"`
	NoProxy              string `json:"noProxy"`
	HostNetworkInterface string `json:"hostNetworkInterface"`
	ClusterNetworkCIDR   string `json:"clusterNetworkCIDR"`
}

func (a *API) getClusterConfig(w http.ResponseWriter, r *http.Request) {
	clusterNetworkCIDR := ecv1beta1.DefaultNetworkCIDR
	if a.installFlags.cidrCfg.GlobalCIDR != nil {
		clusterNetworkCIDR = *a.installFlags.cidrCfg.GlobalCIDR
	}

	var proxy ecv1beta1.ProxySpec
	if a.installFlags.proxy != nil {
		proxy = *a.installFlags.proxy
	}

	clusterConfig := ClusterConfig{
		DataDirectory:        a.installFlags.dataDir,
		HTTPProxy:            proxy.HTTPProxy,
		HTTPSProxy:           proxy.HTTPSProxy,
		NoProxy:              proxy.NoProxy,
		HostNetworkInterface: a.installFlags.networkInterface,
		ClusterNetworkCIDR:   clusterNetworkCIDR,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clusterConfig)
}

func (a *API) postClusterConfig(w http.ResponseWriter, r *http.Request) {
	var clusterConfig ClusterConfig
	err := json.NewDecoder(r.Body).Decode(&clusterConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if clusterConfig.DataDirectory != "" {
		a.installFlags.dataDir = clusterConfig.DataDirectory
	} else {
		a.installFlags.dataDir = ecv1beta1.DefaultDataDir
	}
	runtimeconfig.SetDataDir(a.installFlags.dataDir)

	os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
	os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

	if clusterConfig.HTTPProxy != "" || clusterConfig.HTTPSProxy != "" || clusterConfig.NoProxy != "" {
		if a.installFlags.proxy == nil {
			a.installFlags.proxy = &ecv1beta1.ProxySpec{}
		}
		a.installFlags.proxy.HTTPProxy = clusterConfig.HTTPProxy
		a.installFlags.proxy.HTTPSProxy = clusterConfig.HTTPSProxy
		a.installFlags.proxy.NoProxy = clusterConfig.NoProxy
	}

	if clusterConfig.HostNetworkInterface != "" {
		a.installFlags.networkInterface = clusterConfig.HostNetworkInterface
	}

	if clusterConfig.ClusterNetworkCIDR != "" {
		a.installFlags.cidrCfg.GlobalCIDR = &clusterConfig.ClusterNetworkCIDR
	}

	a.getClusterConfig(w, r)
}

func (a *API) postInstallCluster(w http.ResponseWriter, r *http.Request) {
	if a.isInstalling {
		http.Error(w, "Cluster is already being installed", http.StatusConflict)
		return
	}

	if !a.isInstallSuccess && a.installError == nil {
		a.isInstalling = true
		defer func() {
			a.isInstalling = false
		}()

		err := reallyRunInstall(context.Background(), a.installFlags, a.metricsReporter)
		if err != nil {
			a.installError = err
		}
		a.isInstallSuccess = true
	}

	if a.installError != nil {
		http.Error(w, a.installError.Error(), http.StatusInternalServerError)
		return
	}

	adminConsoleURL := getAdminConsoleURL(a.installFlags.networkInterface, a.installFlags.adminConsolePort)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":          "success",
		"message":         "Cluster installation completed successfully",
		"adminConsoleUrl": adminConsoleURL,
	})
}

type userProvider struct {
	username string
	password string
}

func (u *userProvider) credsMatch(username, password string) bool {
	return username == u.username && password == u.password
}
func authenticate(h http.HandlerFunc, userProvider userProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		doAuthentication(w, r, h, userProvider)
	}
}

func doAuthentication(w http.ResponseWriter, r *http.Request, innerHandler func(w http.ResponseWriter, r *http.Request), userProvider userProvider) {
	w.Header().Set("WWW-Authenticate", `Basic realm="pprof"`)

	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 || s[0] != "Basic" {
		http.Error(w, "Invalid authorization header", 401)
		return
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}

	credentials := strings.SplitN(string(b), ":", 2)
	if len(credentials) != 2 {
		http.Error(w, "Invalid authorization header", 401)
		return
	}

	if !userProvider.credsMatch(credentials[0], credentials[1]) {
		http.Error(w, "Not authorized", 401)
		return
	}

	innerHandler(w, r)
}
