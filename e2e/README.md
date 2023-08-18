## E2E tests

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

### Running all the tests

You can run the tests from within this directory:

```
$ make e2e-tests
```

### Running individual tests

You an run a single test with:

```
$ make e2e-test TEST_NAME=TestSomething
```

TestSomething is the name of the test function you want to run.

### Adding more tests

To add more tests you just need to create one inside this directory
and then run:

```
$ make create-e2e-workflows
```

