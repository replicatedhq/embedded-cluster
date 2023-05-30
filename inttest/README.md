## Keeping the test env after tests

Sometimes, especially when debugging some test failures, it's good to leave the environment running after the tests have ran. To control that behavior there's an env variable called `K0S_KEEP_AFTER_TESTS`. The value given to that has the following logic:

- no value or `K0S_KEEP_AFTER_TESTS="never"`: The test env is NOT left running regardless of the test results
- `K0S_KEEP_AFTER_TESTS="always"`: The test env is left running regardless of the test results
- `K0S_KEEP_AFTER_TESTS="failure"`: The test env is left running only if the tests have failed

The test output show how to run manual cleanup for the environment, something like:

```shell
TestNetworkSuite: footloosesuite.go:138: footloose cluster left intact for debugging. Needs to be manually cleaned with: footloose delete --config /tmp/afghzzvp-footloose.yaml
```

This allows you to run manual cleanup after you've done the needed debugging.
