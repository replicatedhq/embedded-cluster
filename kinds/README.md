# embedded-cluster/kinds

This directory contains the definitions and clientsets for the embeddedcluster.replicated.com kinds.
These aren't CRDs and controllers, but are implemented as normal Kubernetes objects.
This allows us to use the client-go and other functionality to parse and ensure conformance.

## Publishing kinds

To publish a new version of the github.com/replicatedhq/embedded-cluster/kinds package, tag and push to with the format `kinds/v1.0.0` with a leading `v`.
This is a Go feature. See the docs on [Multi-Module Repositories](https://go.dev/wiki/Modules#faqs--multi-module-repositories) for more information.

```bash
git tag -a kinds/v1.0.0 -m "Release v1.0.0"
git push origin kinds/v1.0.0
```
