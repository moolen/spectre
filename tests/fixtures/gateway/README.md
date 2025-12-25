# Gateway API Test Fixtures

This directory contains test fixtures for Gateway API extractors.

## Files

- `gateway-httproute.yaml`: Complete Gateway API setup with Gateway, HTTPRoute, and Services
- `cross-namespace.yaml`: Cross-namespace references example

## Usage

These fixtures can be used for:
1. Unit testing the extractors
2. Integration testing with a real Kubernetes cluster
3. E2E testing of the complete graph extraction flow

## Testing with kind

```bash
# Create a kind cluster with Gateway API CRDs
kind create cluster --name spectre-test

# Install Gateway API CRDs
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml

# Install a Gateway implementation (e.g., Istio, Envoy Gateway)
# Example with Envoy Gateway:
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v1.0.0 -n envoy-gateway-system --create-namespace

# Apply test fixtures
kubectl apply -f gateway-httproute.yaml

# Verify edges in graph
# (Requires Spectre to be running)
```

## Expected Relationships

After applying `gateway-httproute.yaml`, the following edges should be created:

1. **Gateway → GatewayClass** (REFERENCES_SPEC)
   - From: `default/Gateway/example-gateway`
   - To: `GatewayClass/example-gateway-class`
   - Field: `spec.gatewayClassName`

2. **HTTPRoute → Gateway** (REFERENCES_SPEC)
   - From: `default/HTTPRoute/example-route`
   - To: `default/Gateway/example-gateway`
   - Field: `spec.parentRefs[0]`

3. **HTTPRoute → Service (frontend)** (REFERENCES_SPEC)
   - From: `default/HTTPRoute/example-route`
   - To: `default/Service/frontend-service`
   - Field: `spec.rules[0].backendRefs[0]`

4. **HTTPRoute → Service (backend)** (REFERENCES_SPEC)
   - From: `default/HTTPRoute/example-route`
   - To: `default/Service/backend-service`
   - Field: `spec.rules[1].backendRefs[0]`

## Verification Queries

```cypher
// Find all Gateway API relationships
MATCH (source:ResourceIdentity)-[r:REFERENCES_SPEC]->(target:ResourceIdentity)
WHERE source.kind IN ['Gateway', 'HTTPRoute']
RETURN source.kind, source.name, type(r), target.kind, target.name

// Find the complete chain from GatewayClass to Service
MATCH path = (gc:ResourceIdentity {kind: 'GatewayClass'})<-[:REFERENCES_SPEC]-(gw:ResourceIdentity {kind: 'Gateway'})<-[:REFERENCES_SPEC]-(hr:ResourceIdentity {kind: 'HTTPRoute'})-[:REFERENCES_SPEC]->(svc:ResourceIdentity {kind: 'Service'})
RETURN gc.name, gw.name, hr.name, svc.name
```
