#!/usr/bin/env bash
#
# Helper script for forwarding deps ports to localhost
#

set -e

# Fetch pod names
RABBIT_POD=$(kubectl get pods -n rabbit -l app=rabbitmq -o jsonpath="{.items[0].metadata.name}")
REDIS_POD=$(kubectl get pods -n redis -l app=redis -o jsonpath="{.items[0].metadata.name}")

# Set up port forwarding
kubectl port-forward -n rabbit $RABBIT_POD 5672:5672 & \
kubectl port-forward -n rabbit $RABBIT_POD 15672:15672 & \
kubectl port-forward -n redis $REDIS_POD 6379:6379

