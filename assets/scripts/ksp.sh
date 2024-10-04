#!/usr/bin/env bash
#

set -e

KSP_DEFAULT_NAMESPACE="superpower"

function show_help {
    echo "ksp - k8s cheat tool for superpower"
    echo ""
    echo "To set a default namespace, 'export KSP_NAMESPACE=superpower'"
    echo ""
    echo "Usage:"
    echo "  ksp -h"
    echo "  ksp pods"
    echo "  ksp pod <partial pod name>"
    echo "  ksp image <partial pod name>"
    echo "  ksp describe <partial pod name>"
    echo "  ksp watch <partial pod name>"
    echo "  ksp exec <partial pod name>"
}

if [ "$#" -eq 0 ] || [ "$1" == "-h" ]; then
    show_help
    exit 0
fi

# If set, overwrite KSP_NAMESPACE
if [ -z "$KSP_NAMESPACE" ]; then
    KSP_NAMESPACE=$KSP_DEFAULT_NAMESPACE
fi

case "$1" in
    image)
        if [ -z "$2" ]; then
            echo "Error: Missing part of a pod name"
            show_help
            exit 1
        else
            POD=$(kubectl get pods -n $KSP_NAMESPACE | grep $2 | awk {'print $1'})

            if [ -z "$POD" ]; then
                echo "Error: Pod not found"
                exit 1
            fi

            kubectl describe pod $POD -n $KSP_NAMESPACE | grep Image: | awk {'print $2'}
        fi
        ;;
    describe)
        if [ -z "$2" ]; then
            echo "Error: Missing part of a pod name"
            show_help
            exit 1
        else
            POD=$(kubectl get pods -n $KSP_NAMESPACE | grep $2 | awk {'print $1'})

            if [ -z "$POD" ]; then
                echo "Error: Pod not found"
                exit 1
            fi

            kubectl describe pod $POD -n $KSP_NAMESPACE
        fi
        ;;
    watch)
        if [ -z "$2" ]; then
            echo "Error: Missing part of a pod name"
            show_help
            exit 1
        else
            watch -n 1 "kubectl get pods -n $KSP_NAMESPACE | grep $2"
        fi
        ;;
    exec)
        if [ -z "$2" ]; then
            echo "Error: Missing part of a pod name"
            show_help
            exit 1
        else
            POD=$(kubectl get pods -n $KSP_NAMESPACE | grep $2 | awk {'print $1'})

            if [ -z "$POD" ]; then
                echo "Error: Pod not found"
                exit 1
            fi

            kubectl exec -it $POD -n $KSP_NAMESPACE /bin/sh
        fi
        ;;
    pod)
        if [ -z "$2" ]; then
            echo "Error: Missing part of a pod name"
            show_help
            exit 1
        else
            POD=$(kubectl get pods -n $KSP_NAMESPACE | grep $2 | awk {'print $1'})

            if [ -z "$POD" ]; then
                echo "Error: Pod not found"
                exit 1
            fi

            echo $POD
        fi
        ;;
    pods)
        kubectl get pods -n $KSP_NAMESPACE
        ;;
    *)
        echo "unknown subcommand"
        ;;
esac
