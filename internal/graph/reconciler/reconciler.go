package reconciler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Reconciler coordinates reconciliation handlers to detect resources
// that have been deleted from Kubernetes but whose DELETE events were missed.
type Reconciler struct {
	config        Config
	graphClient   graph.Client
	dynamicClient dynamic.Interface
	handlers      []ReconcileHandler
	logger        *logging.Logger

	// Lifecycle
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.RWMutex
}

// New creates a new Reconciler.
func New(config Config, graphClient graph.Client, restConfig *rest.Config) (*Reconciler, error) {
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	r := &Reconciler{
		config:        config,
		graphClient:   graphClient,
		dynamicClient: dynamicClient,
		handlers:      []ReconcileHandler{},
		logger:        logging.GetLogger("graph.reconciler"),
		stopCh:        make(chan struct{}),
	}

	// Register default handlers
	r.RegisterHandler(NewPodTerminationReconciler(dynamicClient))

	return r, nil
}

// RegisterHandler adds a reconciliation handler.
func (r *Reconciler) RegisterHandler(handler ReconcileHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, handler)
	r.logger.Info("Registered reconciliation handler: %s", handler.Name())
}

// Name implements lifecycle.Component.
func (r *Reconciler) Name() string {
	return "graph.reconciler"
}

// Start implements lifecycle.Component.
func (r *Reconciler) Start(ctx context.Context) error {
	if !r.config.Enabled {
		r.logger.Info("Reconciler is disabled")
		return nil
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = true
	r.stopCh = make(chan struct{})
	r.mu.Unlock()

	r.logger.Info("Starting reconciler with interval %v, batch size %d", r.config.Interval, r.config.BatchSize)

	r.wg.Add(1)
	go r.runLoop(ctx)

	return nil
}

// Stop implements lifecycle.Component.
func (r *Reconciler) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	close(r.stopCh)
	r.mu.Unlock()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("Reconciler stopped")
		return nil
	case <-ctx.Done():
		r.logger.Warn("Reconciler shutdown timeout")
		return ctx.Err()
	}
}

// runLoop is the main reconciliation loop.
func (r *Reconciler) runLoop(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()

	// Run immediately on start
	r.runReconciliation(ctx)

	for {
		select {
		case <-ticker.C:
			r.runReconciliation(ctx)
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// runReconciliation executes all handlers.
func (r *Reconciler) runReconciliation(ctx context.Context) {
	startTime := time.Now()
	r.logger.Info("Starting reconciliation cycle")

	r.mu.RLock()
	handlers := r.handlers
	r.mu.RUnlock()

	totalDeleted := 0
	totalChecked := 0

	for _, handler := range handlers {
		// Fetch resources from graph that need reconciliation
		resources, err := r.fetchResourcesForHandler(ctx, handler)
		if err != nil {
			r.logger.Error("Failed to fetch resources for %s: %v", handler.Name(), err)
			continue
		}

		if len(resources) == 0 {
			r.logger.Debug("No resources to reconcile for %s", handler.Name())
			continue
		}

		r.logger.Info("Reconciling %d %s resources", len(resources), handler.ResourceKind())

		// Run handler
		input := ReconcileInput{
			Resources: resources,
			BatchSize: r.config.BatchSize,
		}

		output, err := handler.Reconcile(ctx, input)
		if err != nil {
			r.logger.Error("Handler %s failed: %v", handler.Name(), err)
			continue
		}

		// Mark deleted resources in graph
		for _, uid := range output.ResourcesDeleted {
			if err := r.markResourceDeleted(ctx, uid); err != nil {
				r.logger.Error("Failed to mark resource %s as deleted: %v", uid, err)
			} else {
				totalDeleted++
			}
		}

		totalChecked += output.ResourcesChecked

		// Log any non-fatal errors
		for _, e := range output.Errors {
			r.logger.Warn("Handler %s encountered error: %v", handler.Name(), e)
		}

		r.logger.Info("Handler %s: checked=%d, deleted=%d, stillExist=%d",
			handler.Name(), output.ResourcesChecked, len(output.ResourcesDeleted), len(output.ResourcesStillExist))
	}

	duration := time.Since(startTime)
	r.logger.Info("Reconciliation cycle complete: checked=%d, deleted=%d, duration=%v",
		totalChecked, totalDeleted, duration)
}

// fetchResourcesForHandler queries graph for resources needing reconciliation.
func (r *Reconciler) fetchResourcesForHandler(ctx context.Context, handler ReconcileHandler) ([]GraphResource, error) {
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE r.kind = $kind
			  AND (r.deleted = false OR r.deleted IS NULL)
			RETURN r.uid as uid, r.kind as kind, r.apiGroup as apiGroup,
			       r.namespace as namespace, r.name as name
			LIMIT $limit
		`,
		Parameters: map[string]interface{}{
			"kind":  handler.ResourceKind(),
			"limit": r.config.BatchSize,
		},
	}

	result, err := r.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	resources := make([]GraphResource, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 5 {
			continue
		}

		resource := GraphResource{}
		if uid, ok := row[0].(string); ok {
			resource.UID = uid
		}
		if kind, ok := row[1].(string); ok {
			resource.Kind = kind
		}
		if apiGroup, ok := row[2].(string); ok {
			resource.APIGroup = apiGroup
		}
		if namespace, ok := row[3].(string); ok {
			resource.Namespace = namespace
		}
		if name, ok := row[4].(string); ok {
			resource.Name = name
		}

		if resource.UID != "" {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// markResourceDeleted updates the graph to mark a resource as deleted.
func (r *Reconciler) markResourceDeleted(ctx context.Context, uid string) error {
	now := time.Now().UnixNano()

	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $uid})
			SET r.deleted = true,
			    r.deletedAt = $deletedAt,
			    r.lastSeen = $deletedAt
		`,
		Parameters: map[string]interface{}{
			"uid":       uid,
			"deletedAt": now,
		},
	}

	_, err := r.graphClient.ExecuteQuery(ctx, query)
	return err
}
