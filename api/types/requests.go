package types

import kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"

// SetAppConfigValuesRequest represents the request when setting the app config values
type SetAppConfigValuesRequest struct {
	Values map[string]kotsv1beta1.ConfigValue `json:"values"`
}
