package inboundAdapters

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	wbhk "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/berops/claudie/services/frontend/domain/usecases"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
)

var (
	healthcheckPort = ":8081"
	healthcheckPath = "healthz"
)

type manifestController struct {
	mgr                        manager.Manager
	ctx                        context.Context
	healthcheckClient          *http.Client
	validationWebhookPort      int
	validationWebhookCertDir   string
	validationWebhookPath      string
	validationWebhookNamespace string
}

// NewManifestController creates a new instance of an controller-runtime that will validate the secret with input-manifest
// It takes a context.Context as a parameter, and retunrs a *manifestController instance
func NewManifestController(ctx context.Context) (*manifestController, error) {
	// lookup environment variables
	portString, err := lookupEnv("WEBHOOK_TLS_PORT")
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}
	namespace, err := lookupEnv("NAMESPACE")
	if err != nil {
		return nil, err
	}
	certDir, err := lookupEnv("WEBHOOK_CERT_DIR")
	if err != nil {
		return nil, err
	}
	webhookPath, err := lookupEnv("WEBHOOK_PATH")
	if err != nil {
		return nil, err
	}

	// create http.Client for healthech requests
	client := &http.Client{}

	// create ManifestController object
	var mc = manifestController{
		ctx:                        ctx,
		healthcheckClient:          client,
		validationWebhookPort:      port,
		validationWebhookNamespace: namespace,
		validationWebhookPath:      webhookPath,
		validationWebhookCertDir:   certDir,
	}

	// Setup the controler manager for input-manifest
	mc.mgr, err = ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Namespace:              mc.validationWebhookNamespace,
		HealthProbeBindAddress: healthcheckPort,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to set up manifest-controller: %w", err)
	}

	// Register a healthcheck endpoint
	if err := mc.mgr.AddHealthzCheck(healthcheckPath, healthz.Ping); err != nil {
		return nil, err
	}

	// Register the input-manifest validation endpoint
	hookServer := &wbhk.Server{
		Port:    mc.validationWebhookPort,
		CertDir: mc.validationWebhookCertDir,
	}
	hookServer.Register(mc.validationWebhookPath, admission.WithCustomValidator(&corev1.Secret{}, &usecases.SecretValidator{}))
	if err := mc.mgr.Add(hookServer); err != nil {
		return nil, err
	}

	return &mc, nil
}

// Start starts the registered manifest-controller.
// Returns an error if there is an error starting any controller.
func (mc *manifestController) Start() {
	crlog.SetLogger(zerologr.New(&log.Logger))
	logger := crlog.Log

	// Start manager with webhook
	if err := mc.mgr.Start(mc.ctx); err != nil {
		logger.Error(err, "unable to run manifest-controller")
	}
}

// PerformHealthCheck perform health check for manifest-controller
func (mc *manifestController) PerformHealthCheck() error {
	req, err := http.NewRequestWithContext(mc.ctx, "GET", fmt.Sprintf("http://127.0.0.1%s/%s", healthcheckPort, healthcheckPath), nil)
	if err != nil {
		return err
	}
	resp, err := mc.healthcheckClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// lookupEnv take a string representing environment variable as an argument, and returns its value
// If the environment variable is not defined, it will return an error
func lookupEnv(env string) (string, error) {
	value, exists := os.LookupEnv(env)
	if !exists {
		return "", fmt.Errorf("environment variable %s not found", env)
	}
	log.Debug().Msgf("Using %s %s", env, value)

	return value, nil
}
