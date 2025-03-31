## E2E Tests

### Buildting the release for E2E tests

```bash
./scripts/build-and-release.sh ARCH=amd64 \
   APP_VERSION="appver-dev-local-$USER"
```

### Running individual CMX tests locally

You can run a single test with:

```bash
export SHORT_SHA="dev-local-$USER"
export LICENSE_ID="$EC_SMOKE_TEST_LICENSE_ID"
make e2e-test TEST_NAME=TestSomething
```

TestSomething is the name of the test function you want to run.

### Running individual Docker tests locally

You can run a single test with:

```bash
export SHORT_SHA="dev-local-$USER"
export LICENSE_ID="$EC_SMOKE_TEST_LICENSE_ID"
make e2e-test TEST_NAME=TestSomething
```

TestSomething is the name of the test function you want to run.

### Adding more tests

Tests are added as Go tests in the e2e/ directory.
Tests must be added to the ci.yaml and release-prod.yaml GitHub workflows to be run in CI.

### Kots test application

During end to end tests we embed a license for a smoke test kots app.
This app can be found under the 'Replicated, Inc.' team on staging:

https://vendor.staging.replicated.com/apps/embedded-cluster-smoke-test-staging-app

New releases are created using the corresponding YAML files in the e2e/kots-release-* directories.

### Playwright

We use [Playwright](https://playwright.dev/) to run end to end tests on the UI.
The tests live in the `playwright` directory.

For more details on how to write tests with Playwright, refer to the [Playwright documentation](https://playwright.dev/docs/writing-tests).


## DEPRECATED: E2E tests

Integration tests depends on LXD. The following procedure shows
how to get everything installed on Ubuntu 22.04.

### Installing LXD

You can install LXD using snap:

```
$ snap install lxd
$ lxd init --auto
```
Once LXD is configured you need to set up ovn so we can create isolated
networks:

```
$ sudo apt install ovn-host ovn-central -y
$ sudo ovs-vsctl set open_vswitch . \
   external_ids:ovn-remote=unix:/var/run/ovn/ovnsb_db.sock \
   external_ids:ovn-encap-type=geneve \
   external_ids:ovn-encap-ip=127.0.0.1
```

If you have problems setting up ovn you can check their docs:
https://documentation.ubuntu.com/lxd/en/latest/howto/network_ovn_setup/

### Test workflow

Scripts inside the `scripts` directory are copied to all nodes.
If you have a new test you want to add then start by creating a
shell script to execute it and save it under the `scripts` dir.
You can then call the script from your Go code.
