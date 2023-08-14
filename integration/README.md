## Integration tests

Integration tests depends on LXD. The following procedure shows
how to get everything installed on Ubuntu 22.04. Make sure you
are using `root` when following these steps.

### Installing LXD

You can install LXD using snap:

```
# snap install lxd
# lxd init
```
For sake of simplicity during `lxd init` you can select `dir` as
the default storage backend. Once LXD is configured you need to
set up ovn so we can create isolated networks:

```
# apt install ovn-host ovn-central -y
# ovs-vsctl set open_vswitch . \
   external_ids:ovn-remote=unix:/var/run/ovn/ovnsb_db.sock \
   external_ids:ovn-encap-type=geneve \
   external_ids:ovn-encap-ip=127.0.0.1
```

If you have problems setting up ovn you can check their docs:
https://documentation.ubuntu.com/lxd/en/latest/howto/network_ovn_setup/
Now we can pull the needed images:

```
$ lxc image copy images:99db9953a091 local: --alias ubuntu-22.04
```

**For now we only need Ubuntu 22.04.**

### Test workflow

Integration test cases tests expect a few things:

1. The `helmvm` binary you want to test is in `/usr/local/bin/helmvm`.
2. You are running the tests as root.
3. Your SSH keys for testing are in `/root/.ssh/id_rsa` and `/root/.ssh/id_rsa.pub`.

Scripts inside the `scripts` directory are copied to all nodes.
If you have a new test you want to add then start by creating a
shell script to execute it and save it under the `scripts` dir.
You can then call the script from your Go code.

### Running the tests

You can run the tests from within this directory:

```
# go test -v ./.
```
