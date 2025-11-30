#!/bin/bash

set -e

NAMESPACE="workloads"
SLEEP_INTERVAL=5

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

log_change() {
    echo -e "${GREEN}[CHANGE]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Array of busybox versions to randomly choose from
BUSYBOX_VERSIONS=("1.30" "1.31" "1.32" "1.33" "1.34" "1.35" "1.36")

# Function to get a random element from an array
get_random_element() {
    local array=("$@")
    local index=$((RANDOM % ${#array[@]}))
    echo "${array[$index]}"
}

# Function to change deployment image version
change_deployment_image() {
    local deployment=$1
    local version=$(get_random_element "${BUSYBOX_VERSIONS[@]}")

    log_change "Updating deployment '$deployment' busybox image to version $version"
    kubectl set image deployment/$deployment $deployment=busybox:$version \
        -n $NAMESPACE --record 2>/dev/null || \
        kubectl patch deployment $deployment -n $NAMESPACE \
            -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"$deployment\",\"image\":\"busybox:$version\"}]}}}}" \
            2>/dev/null || log_error "Failed to update image for deployment $deployment"
}

# Function to modify a configmap value
change_configmap_value() {
    local configmap=$1

    # Get the current configmap data
    local config_data=$(kubectl get configmap $configmap -n $NAMESPACE -o json 2>/dev/null)
    if [ $? -ne 0 ]; then
        log_error "Failed to get configmap $configmap"
        return 1
    fi

    # Get a random key from the configmap
    local keys=$(echo "$config_data" | jq -r '.data | keys[]' 2>/dev/null)
    if [ -z "$keys" ]; then
        log_error "ConfigMap $configmap has no data keys"
        return 1
    fi

    local key=$(echo "$keys" | shuf -n 1)
    local old_value=$(echo "$config_data" | jq -r ".data[\"$key\"]")

    # Generate a new value (append timestamp to show change)
    local new_value="${old_value}_modified_$(date +%s)"

    log_change "Updating configmap '$configmap' key '$key' from '$old_value' to '$new_value'"

    kubectl patch configmap $configmap -n $NAMESPACE --type merge \
        -p "{\"data\":{\"$key\":\"$new_value\"}}" 2>/dev/null || \
        log_error "Failed to update configmap $configmap"
}

# Function to change deployment env variable
change_deployment_env() {
    local deployment=$1

    # Array of common environment variables to randomly modify
    local env_vars=("LOG_LEVEL" "JWT_SECRET" "SESSION_STORE" "AUTH_PROVIDER")
    local env_var=$(get_random_element "${env_vars[@]}")

    # Generate new values based on the variable
    local new_value=""
    case $env_var in
        LOG_LEVEL)
            new_value=$(get_random_element "debug" "info" "warn" "error")
            ;;
        JWT_SECRET)
            new_value="secret_$(date +%s)"
            ;;
        SESSION_STORE)
            new_value=$(get_random_element "redis" "memcached" "in-memory")
            ;;
        AUTH_PROVIDER)
            new_value=$(get_random_element "oauth2" "saml" "ldap" "jwt")
            ;;
    esac

    log_change "Updating deployment '$deployment' env variable '$env_var' to '$new_value'"

    kubectl set env deployment/$deployment $env_var=$new_value \
        -n $NAMESPACE || \
        log_error "Failed to set env variable for deployment $deployment"
}

# Function to rollout restart a deployment
rollout_deployment() {
    local deployment=$1

    log_change "Triggering rollout restart for deployment '$deployment'"

    kubectl rollout restart deployment/$deployment -n $NAMESPACE 2>/dev/null || \
        log_error "Failed to rollout restart deployment $deployment"
}

# Function to get all deployments in the namespace
get_deployments() {
    kubectl get deployments -n $NAMESPACE -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n'
}

# Function to get all configmaps in the namespace
get_configmaps() {
    kubectl get configmaps -n $NAMESPACE -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | grep -v kube- | tr ' ' '\n'
}

# Main loop
log "Starting chaos changes loop for namespace '$NAMESPACE'"
log "Changes will occur every $SLEEP_INTERVAL seconds"
log ""

iteration=0
while true; do
    iteration=$((iteration + 1))
    log "--- Iteration $iteration ---"

    # Get all deployments and configmaps
    deployments=($(get_deployments))
    configmaps=($(get_configmaps))

    if [ ${#deployments[@]} -eq 0 ] && [ ${#configmaps[@]} -eq 0 ]; then
        log_error "No deployments or configmaps found in namespace $NAMESPACE"
        sleep $SLEEP_INTERVAL
        continue
    fi

    # Randomly choose between deployment or configmap
    all_resources=()
    for d in "${deployments[@]}"; do
        all_resources+=("deployment:$d")
    done
    for c in "${configmaps[@]}"; do
        all_resources+=("configmap:$c")
    done

    chosen_resource=$(get_random_element "${all_resources[@]}")
    resource_type="${chosen_resource%%:*}"
    resource_name="${chosen_resource#*:}"

    # Randomly choose an action
    action=$((RANDOM % 4))

    case $resource_type in
        deployment)
            case $action in
                0)
                    change_deployment_image "$resource_name"
                    ;;
                1)
                    change_deployment_env "$resource_name"
                    ;;
                2)
                    # Try to modify a configmap associated with this deployment
                    config_name="${resource_name}-config"
                    if kubectl get configmap $config_name -n $NAMESPACE &>/dev/null; then
                        change_configmap_value "$config_name"
                    else
                        change_deployment_image "$resource_name"
                    fi
                    ;;
                3)
                    rollout_deployment "$resource_name"
                    ;;
            esac
            ;;
        configmap)
            change_configmap_value "$resource_name"
            ;;
    esac

    log "Waiting ${SLEEP_INTERVAL}s before next change..."
    sleep $SLEEP_INTERVAL
done
