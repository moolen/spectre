#!/bin/bash
# Script to generate random Kubernetes changes for testing watch server
# Creates deployments, modifies them randomly, and cleans up in a loop

set -e

NAMESPACE="${NAMESPACE:-default}"
DEPLOYMENT_NAME="test-deployment-$(date +%s)"
MIN_REPLICAS=1
MAX_REPLICAS=3

# Array of nginx image versions to rotate through
NGINX_VERSIONS=("1.19" "1.20" "1.21" "1.22" "1.23" "1.24" "1.25")

# Array of possible environment variable names and values
ENV_NAMES=("ENV" "DEBUG" "LOG_LEVEL" "CACHE_SIZE" "WORKER_COUNT")
ENV_VALUES=("production" "development" "true" "false" "info" "debug" "warn" "128m" "256m" "512m" "2" "4" "8")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ERROR:${NC} $1"
}

success() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] SUCCESS:${NC} $1"
}

warning() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] WARNING:${NC} $1"
}

# Generate random number in range
random_range() {
    local min=$1
    local max=$2
    echo $((RANDOM % (max - min + 1) + min))
}

# Get random element from array
random_element() {
    local arr=("$@")
    local idx=$((RANDOM % ${#arr[@]}))
    echo "${arr[$idx]}"
}

# Create deployment
create_deployment() {
    local replicas=$1
    local image_version=$2
    
    log "Creating deployment ${DEPLOYMENT_NAME} with ${replicas} replicas (nginx:${image_version})"
    
    cat <<EOF | kubectl apply -f - -n ${NAMESPACE}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${DEPLOYMENT_NAME}
  labels:
    app: test-app
    generator: change-generator
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: test-app
      deployment: ${DEPLOYMENT_NAME}
  template:
    metadata:
      labels:
        app: test-app
        deployment: ${DEPLOYMENT_NAME}
    spec:
      containers:
      - name: nginx
        image: nginx:${image_version}
        ports:
        - containerPort: 80
        env:
        - name: INITIAL_ENV
          value: "true"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
EOF
    
    if [ $? -eq 0 ]; then
        success "Deployment created"
    else
        error "Failed to create deployment"
        return 1
    fi
}

# Scale deployment
scale_deployment() {
    local new_replicas=$1
    
    log "Scaling deployment to ${new_replicas} replicas"
    kubectl scale deployment/${DEPLOYMENT_NAME} --replicas=${new_replicas} -n ${NAMESPACE}
    
    if [ $? -eq 0 ]; then
        success "Scaled to ${new_replicas} replicas"
    else
        error "Failed to scale deployment"
        return 1
    fi
}

# Update image
update_image() {
    local new_version=$1
    
    log "Updating image to nginx:${new_version}"
    kubectl set image deployment/${DEPLOYMENT_NAME} nginx=nginx:${new_version} -n ${NAMESPACE}
    
    if [ $? -eq 0 ]; then
        success "Image updated to nginx:${new_version}"
    else
        error "Failed to update image"
        return 1
    fi
}

# Add or update environment variable
update_env() {
    local env_name=$1
    local env_value=$2
    
    log "Setting environment variable ${env_name}=${env_value}"
    kubectl set env deployment/${DEPLOYMENT_NAME} ${env_name}=${env_value} -n ${NAMESPACE}
    
    if [ $? -eq 0 ]; then
        success "Environment variable updated"
    else
        error "Failed to update environment variable"
        return 1
    fi
}

# Remove environment variable
remove_env() {
    local env_name=$1
    
    log "Removing environment variable ${env_name}"
    kubectl set env deployment/${DEPLOYMENT_NAME} ${env_name}- -n ${NAMESPACE}
    
    if [ $? -eq 0 ]; then
        success "Environment variable removed"
    else
        warning "Failed to remove environment variable (may not exist)"
    fi
}

# Update resource limits
update_resources() {
    local cpu_request="${1:-100m}"
    local mem_request="${2:-128Mi}"
    local cpu_limit="${3:-200m}"
    local mem_limit="${4:-256Mi}"
    
    log "Updating resource limits (CPU: ${cpu_request}/${cpu_limit}, Memory: ${mem_request}/${mem_limit})"
    
    kubectl patch deployment/${DEPLOYMENT_NAME} -n ${NAMESPACE} --type='json' -p="[
        {\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/resources/requests/cpu\", \"value\": \"${cpu_request}\"},
        {\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/resources/requests/memory\", \"value\": \"${mem_request}\"},
        {\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/resources/limits/cpu\", \"value\": \"${cpu_limit}\"},
        {\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/resources/limits/memory\", \"value\": \"${mem_limit}\"}
    ]"
    
    if [ $? -eq 0 ]; then
        success "Resources updated"
    else
        error "Failed to update resources"
        return 1
    fi
}

# Wait for deployment to be ready
wait_for_ready() {
    local timeout=60
    
    log "Waiting for deployment to be ready (timeout: ${timeout}s)"
    kubectl wait --for=condition=available --timeout=${timeout}s deployment/${DEPLOYMENT_NAME} -n ${NAMESPACE} 2>/dev/null
    
    if [ $? -eq 0 ]; then
        success "Deployment is ready"
    else
        warning "Deployment not ready within timeout"
    fi
}

# Delete deployment
delete_deployment() {
    log "Deleting deployment ${DEPLOYMENT_NAME}"
    kubectl delete deployment/${DEPLOYMENT_NAME} -n ${NAMESPACE} --grace-period=5
    
    if [ $? -eq 0 ]; then
        success "Deployment deleted"
    else
        error "Failed to delete deployment"
        return 1
    fi
}

# Perform random operation
random_operation() {
    local operations=(
        "scale"
        "update_image"
        "add_env"
        "remove_env"
        "update_resources"
    )
    
    local op=$(random_element "${operations[@]}")
    
    case $op in
        scale)
            local replicas=$(random_range ${MIN_REPLICAS} ${MAX_REPLICAS})
            scale_deployment ${replicas}
            ;;
        update_image)
            local version=$(random_element "${NGINX_VERSIONS[@]}")
            update_image ${version}
            ;;
        add_env)
            local env_name=$(random_element "${ENV_NAMES[@]}")
            local env_value=$(random_element "${ENV_VALUES[@]}")
            update_env ${env_name} ${env_value}
            ;;
        remove_env)
            local env_name=$(random_element "${ENV_NAMES[@]}")
            remove_env ${env_name}
            ;;
        update_resources)
            local cpu_req=$(random_element "50m" "100m" "150m" "200m")
            local cpu_lim=$(random_element "100m" "200m" "300m" "400m")
            # Pick memory request and limit ensuring request <= limit
            local mem_configs=(
                "64Mi 128Mi"
                "64Mi 256Mi"
                "128Mi 256Mi"
                "128Mi 512Mi"
                "256Mi 512Mi"
                "256Mi 1Gi"
                "512Mi 1Gi"
            )
            local mem_config=$(random_element "${mem_configs[@]}")
            local mem_req=$(echo ${mem_config} | cut -d' ' -f1)
            local mem_lim=$(echo ${mem_config} | cut -d' ' -f2)
            update_resources ${cpu_req} ${mem_req} ${cpu_lim} ${mem_lim}
            ;;
    esac
    
    # Random wait between operations
    local wait_time=$(random_range 2 8)
    log "Waiting ${wait_time} seconds before next operation"
    sleep ${wait_time}
}

# Cleanup function
cleanup() {
    warning "Caught interrupt signal, cleaning up..."
    if kubectl get deployment/${DEPLOYMENT_NAME} -n ${NAMESPACE} &>/dev/null; then
        delete_deployment
    fi
    exit 0
}

# Set trap for cleanup
trap cleanup SIGINT SIGTERM

# Main loop
main() {
    log "Starting change generator (namespace: ${NAMESPACE})"
    log "Press Ctrl+C to stop"
    echo ""
    
    local iteration=1
    
    while true; do
        echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
        log "Iteration #${iteration}"
        echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
        
        # Generate new deployment name for this iteration
        DEPLOYMENT_NAME="test-deployment-$(date +%s)"
        
        # Create deployment with random initial values
        local initial_replicas=$(random_range ${MIN_REPLICAS} ${MAX_REPLICAS})
        local initial_version=$(random_element "${NGINX_VERSIONS[@]}")
        
        create_deployment ${initial_replicas} ${initial_version}
        
        # Wait for it to be ready
        wait_for_ready
        
        # Perform random number of operations (between 3 and 8)
        local num_operations=$(random_range 3 8)
        log "Will perform ${num_operations} random operations"
        
        for i in $(seq 1 ${num_operations}); do
            echo ""
            log "Operation ${i}/${num_operations}"
            random_operation
        done
        
        echo ""
        # Wait a bit before cleanup
        local cleanup_wait=$(random_range 5 10)
        log "Waiting ${cleanup_wait} seconds before cleanup"
        sleep ${cleanup_wait}
        
        # Delete deployment
        delete_deployment
        
        # Wait before next iteration
        local iteration_wait=$(random_range 10 20)
        echo ""
        log "Iteration #${iteration} complete. Waiting ${iteration_wait} seconds before next iteration"
        echo ""
        sleep ${iteration_wait}
        
        iteration=$((iteration + 1))
    done
}

# Run main function
main
