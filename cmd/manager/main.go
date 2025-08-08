package main

import (
	"context"
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
	"github.com/solanyn/tgp-operator/pkg/controllers"
	"github.com/solanyn/tgp-operator/pkg/pricing"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() { //nolint:gochecknoinits // Required for Kubernetes scheme registration
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(tgpv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 ctrl.Options{}.Metrics,
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "tgp-operator-leader-election",
		LeaderElectionNamespace: "",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	pricingCache := pricing.NewCache(time.Minute * 15)
	
	// Load operator configuration from ConfigMap with retry logic
	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		operatorNamespace = "tgp-system" // Default namespace
	}
	
	var operatorConfig *config.OperatorConfig
	var configErr error
	
	// Retry ConfigMap loading with exponential backoff
	for attempt := 0; attempt < 5; attempt++ {
		operatorConfig, configErr = config.LoadConfig(context.Background(), mgr.GetClient(), "tgp-operator-config", operatorNamespace)
		if configErr == nil {
			setupLog.Info("loaded operator configuration from ConfigMap", 
				"namespace", operatorNamespace,
				"attempt", attempt+1,
				"vultr.enabled", operatorConfig.Providers.Vultr.Enabled,
				"gcp.enabled", operatorConfig.Providers.GCP.Enabled,
				"vultr.secret", operatorConfig.Providers.Vultr.CredentialsRef.Name,
				"vultr.key", operatorConfig.Providers.Vultr.CredentialsRef.Key,
			)
			break
		}
		
		waitTime := time.Duration(1<<attempt) * time.Second // 1s, 2s, 4s, 8s, 16s
		setupLog.Info("ConfigMap loading failed, retrying", "attempt", attempt+1, "error", configErr.Error(), "retryIn", waitTime)
		time.Sleep(waitTime)
	}
	
	if configErr != nil {
		setupLog.Error(configErr, "failed to load operator configuration after retries, using defaults")
		operatorConfig = config.DefaultConfig()
	}

	// Setup GPUNodeClass controller
	if err = (&controllers.GPUNodeClassReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("GPUNodeClass"),
		Config: operatorConfig,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GPUNodeClass")
		os.Exit(1)
	}

	// Setup GPUNodePool controller
	if err = (&controllers.GPUNodePoolReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Log:          ctrl.Log.WithName("controllers").WithName("GPUNodePool"),
		Config:       operatorConfig,
		PricingCache: pricingCache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GPUNodePool")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
