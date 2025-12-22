#!/bin/bash

set -eo pipefail

DRYRUN_MATCH=${DRYRUN_MATCH:-'Test'}

function plog() {
    local type="$1"
    local message="$2"
    local emoji="$3"
    echo -e "[$(date +'%Y-%m-%d %H:%M:%S')] $emoji [$type] $message"
}

# Build the test container
plog "INFO" "Building test container..." "🔨"
docker build -q -t ec-dryrun ./tests/dryrun > /dev/null

# Compiling the test binary
plog "INFO" "Compiling test binary..." "🔨"
docker run --rm \
    -v "$(pwd)":/ec \
    -w /ec \
    -e GOCACHE=/ec/dev/build/.gocache \
    -e GOMODCACHE=/ec/dev/build/.gomodcache \
    ec-dryrun \
    go test -c -o /ec/dev/build/dryrun.test ./tests/dryrun/...

# Get all test functions
tests=$(grep -o 'func '"$DRYRUN_MATCH"'[^ (]*' ./tests/dryrun/*.go | awk '{print $2}')

# Run tests in separate containers
for test in $tests; do
    plog "INFO" "Starting test: $test" "🚀"
    docker rm -f --volumes "$test" > /dev/null 2>&1 || true
    docker run -d \
        -v "$(pwd)"/dev/build:/ec/dev/build \
        --name "$test" \
        ec-dryrun \
        /ec/dev/build/dryrun.test -test.timeout 1m -test.v -test.run "^$test$" > /dev/null
done

plog "INFO" "Waiting for tests to complete..." "⏳"

# Check test results
failed_tests=()
for test in $tests; do
    exit_code=$(docker wait "$test")
    if [ "$exit_code" -ne 0 ]; then
        failed_tests+=("$test")
        plog "ERROR" "$test failed" "❌"
        docker logs "$test"
    else
        plog "INFO" "$test passed" "✅"
        docker rm -f --volumes "$test" > /dev/null
    fi
done

# Display final summary
if [ ${#failed_tests[@]} -eq 0 ]; then
    plog "SUCCESS" "All tests passed successfully!" "🎉"
    exit 0
else
    plog "FAILURE" "Some tests failed: ${failed_tests[*]}" "🚨"
    exit 1
fi
