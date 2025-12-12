package types

// AppUpgrade represents the current upgrade status
type AppUpgrade struct {
	Status Status `json:"status"`
	Logs   string `json:"logs"`
}

// UpgradeAppRequest represents the request to upgrade an app
type UpgradeAppRequest struct {
	IgnoreAppPreflights bool `json:"ignoreAppPreflights"`
}

// LinuxInfraUpgradeRequest represents a request to upgrade infrastructure
type LinuxInfraUpgradeRequest struct {
	IgnoreHostPreflights bool `json:"ignoreHostPreflights"`
}
