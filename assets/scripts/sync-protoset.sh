#!/bin/bash
#
# Script to download the correct events.protoset file based on the version
# specified in go.mod, with direct asset URL access.
#

# Set the file paths for the version files and the asset
events_protoset_version_file="./assets/events/events.protoset.version"
events_protoset_file="./assets/events/events.protoset"
go_mod_file="./go.mod"

# Set the repository and asset name
repo="superpowerdotcom/events"
asset_name="events.protoset"

# Check if GITHUB_TOKEN is set
if [[ -z "$GITHUB_TOKEN" ]]; then
    echo "Error: GITHUB_TOKEN is not set."
    exit 1
fi

# Read desired version from go.mod
desired_version=$(grep "superpowerdotcom/events" "$go_mod_file" | awk '{print $2}')

# Function to download asset using API URL
download_asset() {
    echo "Fetching release information for version $desired_version from GitHub API..."
    release_info=$(curl -sH "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$repo/releases/tags/$desired_version")

    echo "Our release info: $release_info"

    if echo "$release_info" | jq -e '.message' >/dev/null; then
        echo "GitHub API error: $(echo "$release_info" | jq -r '.message')"
        exit 1
    fi

    asset_url=$(echo "$release_info" | jq -r '.assets[] | select(.name == "'$asset_name'") | .url')
    if [[ -z "$asset_url" || "$asset_url" == "null" ]]; then
        echo "Error: No valid API URL found for the asset."
        exit 1
    fi

    echo "Downloading $asset_name from $asset_url..."
    # Use the asset API URL to download the asset, passing Accept header for octet-stream
    curl -L -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/octet-stream" -o "$events_protoset_file" "$asset_url"
    if [[ $? -eq 0 ]]; then
        echo "Downloaded $asset_name version $desired_version successfully."
        echo "$desired_version" > "$events_protoset_version_file"
    else
        echo "Failed to download $asset_name. Check if the GitHub token has the required permissions or if the asset URL is correct."
        exit 1
    fi
}

# Check if events.protoset file exists
if [[ ! -f "$events_protoset_file" ]]; then
    echo "events.protoset does not exist. Initiating download..."
    download_asset
else
    local_version=$(cat "$events_protoset_version_file")
    # Compare versions - use sort to compare versions
    highest_version=$(printf "%s\n%s" "$local_version" "$desired_version" | sort -V | tail -n 1)
    if [[ "$highest_version" == "$local_version" && "$local_version" != "$desired_version" ]]; then
        echo "Local version ($local_version) is newer than the desired version ($desired_version). No update necessary."
    elif [[ "$local_version" != "$desired_version" ]]; then
        echo "Local version does not match desired version. Updating..."
        download_asset
    else
        echo "Local version matches the desired version. No update necessary."
    fi
fi
