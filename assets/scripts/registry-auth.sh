#!/usr/bin/env bash
#

function create_auth() {
    local namespace=$1
    kubectl config use-context minikube

    eval "$(minikube docker-env)"

    aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 554179259394.dkr.ecr.us-east-1.amazonaws.com

    kubectl create secret docker-registry regcred \
        --docker-server=554179259394.dkr.ecr.us-east-1.amazonaws.com \
        --docker-username=AWS \
        --docker-password=$(aws ecr get-login-password --region us-east-1) \
        --docker-email=your-email@example.com \
        -n "$namespace" \
        --dry-run=client -o yaml | kubectl apply -f -
}

for i in "default" "medplum"; do
    echo "Creating registry creds in namespace ${i}"
    create_auth $i
done
