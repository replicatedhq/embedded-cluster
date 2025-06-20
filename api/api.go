package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/docs"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

//	@title			Embedded Cluster API
//	@version		0.1
//	@description	This is the API for the Embedded Cluster project.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	https://github.com/replicatedhq/embedded-cluster/issues
//	@contact.email	support@replicated.com

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@BasePath	/api

//	@securityDefinitions.bearerauth	bearerauth

// @externalDocs.description	OpenAPI
// @externalDocs.url			https://swagger.io/resources/open-api/
type API struct {
	authController    auth.Controller
	consoleController console.Controller
	installController install.Controller
	rc                runtimeconfig.RuntimeConfig
	releaseData       *release.ReleaseData
	tlsConfig         types.TLSConfig
	license           []byte
	airgapBundle      string
	configValues      string
	endUserConfig     *ecv1beta1.Config
	logger            logrus.FieldLogger
	hostUtils         hostutils.HostUtilsInterface
	metricsReporter   metrics.ReporterInterface
}

type APIOption func(*API)

func WithAuthController(authController auth.Controller) APIOption {
	return func(a *API) {
		a.authController = authController
	}
}

func WithConsoleController(consoleController console.Controller) APIOption {
	return func(a *API) {
		a.consoleController = consoleController
	}
}

func WithInstallController(installController install.Controller) APIOption {
	return func(a *API) {
		a.installController = installController
	}
}

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) APIOption {
	return func(a *API) {
		a.rc = rc
	}
}

func WithLogger(logger logrus.FieldLogger) APIOption {
	return func(a *API) {
		a.logger = logger
	}
}

func WithHostUtils(hostUtils hostutils.HostUtilsInterface) APIOption {
	return func(a *API) {
		a.hostUtils = hostUtils
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) APIOption {
	return func(a *API) {
		a.metricsReporter = metricsReporter
	}
}

func WithReleaseData(releaseData *release.ReleaseData) APIOption {
	return func(a *API) {
		a.releaseData = releaseData
	}
}

func WithTLSConfig(tlsConfig types.TLSConfig) APIOption {
	return func(a *API) {
		a.tlsConfig = tlsConfig
	}
}

func WithLicense(license []byte) APIOption {
	return func(a *API) {
		a.license = license
	}
}

func WithAirgapBundle(airgapBundle string) APIOption {
	return func(a *API) {
		a.airgapBundle = airgapBundle
	}
}

func WithConfigValues(configValues string) APIOption {
	return func(a *API) {
		a.configValues = configValues
	}
}

func WithEndUserConfig(endUserConfig *ecv1beta1.Config) APIOption {
	return func(a *API) {
		a.endUserConfig = endUserConfig
	}
}

func New(password string, opts ...APIOption) (*API, error) {
	api := &API{}

	for _, opt := range opts {
		opt(api)
	}

	if api.rc == nil {
		api.rc = runtimeconfig.New(nil)
	}

	if api.logger == nil {
		l, err := logger.NewLogger()
		if err != nil {
			return nil, fmt.Errorf("create logger: %w", err)
		}
		api.logger = l
	}

	if api.hostUtils == nil {
		api.hostUtils = hostutils.New(
			hostutils.WithLogger(api.logger),
		)
	}

	if api.authController == nil {
		authController, err := auth.NewAuthController(password)
		if err != nil {
			return nil, fmt.Errorf("new auth controller: %w", err)
		}
		api.authController = authController
	}

	if api.consoleController == nil {
		consoleController, err := console.NewConsoleController()
		if err != nil {
			return nil, fmt.Errorf("new console controller: %w", err)
		}
		api.consoleController = consoleController
	}

	// TODO (@team): discuss which of these should / should not be pointers
	if api.installController == nil {
		installController, err := install.NewInstallController(
			install.WithRuntimeConfig(api.rc),
			install.WithLogger(api.logger),
			install.WithHostUtils(api.hostUtils),
			install.WithMetricsReporter(api.metricsReporter),
			install.WithReleaseData(api.releaseData),
			install.WithPassword(password),
			install.WithTLSConfig(api.tlsConfig),
			install.WithLicense(api.license),
			install.WithAirgapBundle(api.airgapBundle),
			install.WithConfigValues(api.configValues),
			install.WithEndUserConfig(api.endUserConfig),
		)
		if err != nil {
			return nil, fmt.Errorf("new install controller: %w", err)
		}
		api.installController = installController
	}

	return api, nil
}

func (a *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", a.getHealth).Methods("GET")

	// Hack to fix issue
	// https://github.com/swaggo/swag/issues/1588#issuecomment-2797801240
	router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	}).Methods("GET")
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	router.HandleFunc("/auth/login", a.postAuthLogin).Methods("POST")

	authenticatedRouter := router.PathPrefix("/").Subrouter()
	authenticatedRouter.Use(a.authMiddleware)

	installRouter := authenticatedRouter.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("/installation/config", a.getInstallInstallationConfig).Methods("GET")
	installRouter.HandleFunc("/installation/configure", a.postInstallConfigureInstallation).Methods("POST")
	installRouter.HandleFunc("/installation/status", a.getInstallInstallationStatus).Methods("GET")

	installRouter.HandleFunc("/host-preflights/run", a.postInstallRunHostPreflights).Methods("POST")
	installRouter.HandleFunc("/host-preflights/status", a.getInstallHostPreflightsStatus).Methods("GET")

	installRouter.HandleFunc("/infra/setup", a.postInstallSetupInfra).Methods("POST")
	installRouter.HandleFunc("/infra/status", a.getInstallInfraStatus).Methods("GET")

	// TODO (@salah): remove this once the cli isn't responsible for setting the install status
	// and the ui isn't polling for it to know if the entire install is complete
	installRouter.HandleFunc("/status", a.getInstallStatus).Methods("GET")
	installRouter.HandleFunc("/status", a.setInstallStatus).Methods("POST")

	consoleRouter := authenticatedRouter.PathPrefix("/console").Subrouter()
	consoleRouter.HandleFunc("/available-network-interfaces", a.getListAvailableNetworkInterfaces).Methods("GET")
}

func (a *API) bindJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		a.logError(r, err, fmt.Sprintf("failed to decode %s %s request", strings.ToLower(r.Method), r.URL.Path))
		a.jsonError(w, r, types.NewBadRequestError(err))
		return err
	}

	return nil
}

func (a *API) json(w http.ResponseWriter, r *http.Request, code int, payload any) {
	response, err := json.Marshal(payload)
	if err != nil {
		a.logError(r, err, "failed to encode response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

func (a *API) jsonError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *types.APIError
	if !errors.As(err, &apiErr) {
		apiErr = types.NewInternalServerError(err)
	}

	response, err := json.Marshal(apiErr)
	if err != nil {
		a.logError(r, err, "failed to encode response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.StatusCode)
	_, _ = w.Write(response)
}

func (a *API) logError(r *http.Request, err error, args ...any) {
	a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).Error(args...)
}

func logrusFieldsFromRequest(r *http.Request) logrus.Fields {
	return logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}
}
