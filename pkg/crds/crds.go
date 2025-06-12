package crds

// this package is used to embed the installation CRD file into the binary

import _ "embed"

//go:embed resources.yaml
var InstallationCRDFile string
