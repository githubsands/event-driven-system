#!/bin/zsh

# Set the release file path
RELEASE_FILE=".release"

# Read the release version from the file and increment it
if [ -f "$RELEASE_FILE" ]; then
    RELEASE_VERSION=$(cat "$RELEASE_FILE")
    RELEASE_VERSION=$((RELEASE_VERSION + 1))
else
    RELEASE_VERSION=1
fi

# Write the new release version back to the file
echo $RELEASE_VERSION > "$RELEASE_FILE"

# Set the Helm release name, chart path, and any required values
HELM_RELEASE_NAME="io-batch-reader" # NOTE: Some helm bug isn't recognizing old releases so we have "3" here
HELM_CHART_PATH="/Users/ryanvacek/rust/src/axelar-core/devops/io-batch-reader"

# Run helm upgrade with the new release version
helm upgrade --install "$HELM_RELEASE_NAME" "$HELM_CHART_PATH" --set "releaseVersion=$RELEASE_VERSION" -n=demo
sleep 8
kubectl logs --selector app=io-batch-reader -n=demo -f --max-log-requests 10
