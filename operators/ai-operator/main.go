package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
	"github.com/zyvorai/gryvia/operators/ai-operator/controllers"
	"github.com/zyvorai/gryvia/operators/ai-operator/pkg/timemachine"
	jobwebhook "github.com/zyvorai/gryvia/operators/ai-operator/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gryviav1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enableWebhooks bool
	var webhookCertDir string
	var fabricAware bool
	var fabricMaxPenalty float64

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager.")
	flag.BoolVar(&enableWebhooks, "enable-webhooks", false,
		"Serve the GryviaAIJob validating admission webhook (needs TLS certs in --webhook-cert-dir).")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"Directory holding tls.crt and tls.key for the webhook server.")

	flag.BoolVar(&fabricAware, "fabric-aware-scheduling", false,
		"Rank nodes with the fresh per-node fabric health published by the collector (GryviaNodeFabric). Per-job override: annotation gryvia.io/fabric-aware=true|false. Off by default.")
	flag.Float64Var(&fabricMaxPenalty, "fabric-max-penalty", 25,
		"Most points fabric health may subtract from a node score (0 or above 25 means 25).")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgrOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ai-operator.gryvia.io",
	}
	if enableWebhooks {
		mgrOpts.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: webhookCertDir,
		})
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.GryviaAIJobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("GryviaAIJob"),

		FabricAware:      fabricAware,
		FabricMaxPenalty: fabricMaxPenalty,
		Recorder:         mgr.GetEventRecorderFor("gryviaaijob-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GryviaAIJob")
		os.Exit(1)
	}

	// Create kubernetes clientset for pod log access
	clientset, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes clientset")
		os.Exit(1)
	}

	if err = (&controllers.GryviaLiveExperimentReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Log:       ctrl.Log.WithName("controllers").WithName("GryviaLiveExperiment"),
		Clientset: clientset,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GryviaLiveExperiment")
		os.Exit(1)
	}

	if err = (&controllers.GryviaCheckpointGuardReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("GryviaCheckpointGuard"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GryviaCheckpointGuard")
		os.Exit(1)
	}

	if err = (&controllers.GryviaTrainingProfilerReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Log:      ctrl.Log.WithName("controllers").WithName("GryviaTrainingProfiler"),
		Recorder: mgr.GetEventRecorderFor("gryviatrainingprofiler-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GryviaTrainingProfiler")
		os.Exit(1)
	}

	if err = (&controllers.GryviaModelLineageReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GryviaModelLineage")
		os.Exit(1)
	}

	if err = (&controllers.GryviaTrainingTimeMachineReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Log:         ctrl.Log.WithName("controllers").WithName("GryviaTrainingTimeMachine"),
		ForkHandler: timemachine.NewForkHandler(mgr.GetClient()),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GryviaTrainingTimeMachine")
		os.Exit(1)
	}

	if enableWebhooks {
		mgr.GetWebhookServer().Register(jobwebhook.ValidatePath,
			&admission.Webhook{Handler: jobwebhook.NewGryviaAIJobValidator(mgr.GetClient())})
		setupLog.Info("registered validating webhook", "path", jobwebhook.ValidatePath, "certDir", webhookCertDir)
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
