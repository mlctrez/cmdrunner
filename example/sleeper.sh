#!/usr/bin/env bash

# usage sleeper.sh <sleep_seconds> <exitCode>

echo "sleeping for $1 seconds"
sleep $1
echo "exiting after $1 seconds"

exit $2