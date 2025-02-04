package charts

// this package is used to embed the installation CRD file into the binary

import _ "embed"

//go:embed embedded-cluster-operator/charts/crds/templates/resources.yaml
var InstallationCRDFile string
