package template

import (
	"maps"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// Mode defines the operating mode of the template engine
type Mode string

const (
	// ModeConfig is for processing and templating the KOTS config itself
	ModeConfig Mode = "config"
	// ModeGeneric is for processing and templating generic manifests
	ModeGeneric Mode = "generic"
)

type Engine struct {
	mode                       Mode
	config                     *kotsv1beta1.Config
	license                    *kotsv1beta1.License
	releaseData                *release.ReleaseData
	privateCACertConfigMapName string // ConfigMap name for private CA certificates, empty string if not available
	isAirgapInstallation       bool   // Whether the installation is an airgap installation

	// ExecOptions
	proxySpec        *ecv1beta1.ProxySpec    // Proxy spec for the proxy template functions, if applicable
	registrySettings *types.RegistrySettings // Registry settings for registry template functions, if applicable

	// Internal state
	configValues     types.AppConfigValues
	prevConfigValues types.AppConfigValues
	tmpl             *template.Template
	funcMap          template.FuncMap
	cache            map[string]resolvedConfigItem
	depsTree         map[string][]string
	stack            []string
	mtx              sync.Mutex
}

type EngineOption func(*Engine)

func WithMode(mode Mode) EngineOption {
	return func(e *Engine) {
		e.mode = mode
	}
}

func WithLicense(license *kotsv1beta1.License) EngineOption {
	return func(e *Engine) {
		e.license = license
	}
}

func WithReleaseData(releaseData *release.ReleaseData) EngineOption {
	return func(e *Engine) {
		e.releaseData = releaseData
	}
}

func WithPrivateCACertConfigMapName(configMapName string) EngineOption {
	return func(e *Engine) {
		e.privateCACertConfigMapName = configMapName
	}
}

func WithIsAirgap(isAirgap bool) EngineOption {
	return func(e *Engine) {
		e.isAirgapInstallation = isAirgap
	}
}

func NewEngine(config *kotsv1beta1.Config, opts ...EngineOption) *Engine {
	engine := &Engine{
		mode:             ModeGeneric, // default to generic mode
		config:           config,
		configValues:     make(types.AppConfigValues),
		prevConfigValues: make(types.AppConfigValues),
		cache:            make(map[string]resolvedConfigItem),
		depsTree:         make(map[string][]string),
		stack:            []string{},
		mtx:              sync.Mutex{},
	}

	for _, opt := range opts {
		opt(engine)
	}

	// Initialize funcMap once
	engine.funcMap = sprig.TxtFuncMap()
	maps.Copy(engine.funcMap, engine.getFuncMap())

	return engine
}
