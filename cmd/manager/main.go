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
	
	// Load operator configuration from ConfigMap
	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		operatorNamespace = "tgp-system" // Default namespace
	}
	
	operatorConfig, err := config.LoadConfig(context.Background(), mgr.GetClient(), "tgp-operator-config", operatorNamespace)
	if err != nil {
		setupLog.Error(err, "failed to load operator configuration, using defaults")
		operatorConfig = config.DefaultConfig()
	} else {
		setupLog.Info("loaded operator configuration from ConfigMap", "namespace", operatorNamespace)
		// Debug: Log loaded configuration details
		setupLog.Info("configuration loaded", 
			"vultr.enabled", operatorConfig.Providers.Vultr.Enabled,
			"gcp.enabled", operatorConfig.Providers.GCP.Enabled,
			"vultr.secret", operatorConfig.Providers.Vultr.CredentialsRef.Name,
		)
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
