#!/usr/bin/env bash
#

set -e

# Install tools
brew install jq
brew install kubectl
brew install streamdal/public/plumber
npm install -g dotenv-cli

# Setup minikube
make util/minikube/start
make util/minikube/create-namespaces
make util/ecr/auth

echo "Setup complete!"
