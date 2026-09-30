package main

import (
	"flag"
	"fmt"
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
	var kueueIntegration bool
	var kueueStrictAdmission bool
	var kueueDefaultQueue string
	var admissionGate bool
	var admissionDefaultHours float64
	var ml mlOptions
	var mergeFabricSignals bool

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

	flag.BoolVar(&kueueIntegration, "kueue-integration", false,
		"Create the batch Job of a job with a queue suspended and labelled kueue.x-k8s.io/queue-name so Kueue admits all its pods together, and report Kueue's admission state as phase Queued. Needs Kueue installed. Off by default: nothing changes without it.")
	flag.BoolVar(&kueueStrictAdmission, "kueue-strict-admission", false, "Require Kueue admission for tenant batch jobs even if the default LocalQueue is missing. Requires --kueue-integration.")
	flag.StringVar(&kueueDefaultQueue, "kueue-default-queue", "gryvia",
		"With --kueue-integration: LocalQueue used by jobs in tenant-* namespaces that name no queue (only if that LocalQueue exists).")
	flag.BoolVar(&admissionGate, "admission-gate", false,
		"Before creating a job's workload, check the quotas and hard budgets covering its namespace (spend from usage records plus a forecast for the job) and reject it instead of creating it. Fails open on lookup errors. Off by default.")
	flag.Float64Var(&admissionDefaultHours, "admission-default-hours", 1,
		"Hours a job without spec.timeout is assumed to run for the admission gate's cost forecast.")
	ml.bind(flag.CommandLine)
	flag.BoolVar(&mergeFabricSignals, "merge-fabric-signals", false,
		"Fold the per-node entries collectors write into GryviaFabricSignal status.nodes[] into the top-level status (max for degradation metrics, sample-weighted means for ratios, stale entries ignored). Off by default.")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	if kueueStrictAdmission && !kueueIntegration {
		fmt.Fprintln(os.Stderr, "--kueue-strict-admission requires --kueue-integration")
		os.Exit(1)
	}

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

		FabricAware:          fabricAware,
		FabricMaxPenalty:     fabricMaxPenalty,
		KueueIntegration:     kueueIntegration,
		KueueStrictAdmission: kueueStrictAdmission,
		KueueDefaultQueue:    kueueDefaultQueue,
		Recorder:             mgr.GetEventRecorderFor("gryviaaijob-controller"),

		AdmissionGate:         admissionGate,
		AdmissionDefaultHours: admissionDefaultHours,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GryviaAIJob")
		os.Exit(1)
	}

	if mergeFabricSignals {
		if err = (&controllers.GryviaFabricSignalReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "GryviaFabricSignal")
			os.Exit(1)
		}
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

	// The ML controllers: their flags and defaults are in ml_controllers.go, docs/ml-controllers.md explains them.
	if ml.enabled {
		if ml.autoServeGPUCount < 0 {
			setupLog.Error(nil, "--autoserve-default-gpu-count must not be negative")
			os.Exit(1)
		}
		if err = (&controllers.GryviaWorkspaceReconciler{
			Client:       mgr.GetClient(),
			Scheme:       mgr.GetScheme(),
			Log:          ctrl.Log.WithName("controllers").WithName("GryviaWorkspace"),
			JupyterImage: ml.workspaceJupyterImage,
			CodeImage:    ml.workspaceCodeImage,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "GryviaWorkspace")
			os.Exit(1)
		}

		if err = (&controllers.GryviaInferenceServiceReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			Log:    ctrl.Log.WithName("controllers").WithName("GryviaInferenceService"),
			Images: map[gryviav1.InferenceBackend]string{
				gryviav1.BackendVLLM:        ml.inferenceImageVLLM,
				gryviav1.BackendTriton:      ml.inferenceImageTriton,
				gryviav1.BackendTensorRTLLM: ml.inferenceImageTensorRT,
				gryviav1.BackendTorchServe:  ml.inferenceImageTorchServe,
			},
			HealthPath:         ml.inferenceHealthPath,
			CanaryStartupGrace: ml.canaryStartupGrace,
			GatewayRouting:     ml.inferenceGatewayRouting,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "GryviaInferenceService")
			os.Exit(1)
		}

		if err = (&controllers.GryviaModelRegistryReconciler{
			Client:            mgr.GetClient(),
			Scheme:            mgr.GetScheme(),
			Log:               ctrl.Log.WithName("controllers").WithName("GryviaModelRegistry"),
			AutoServeGPUCount: int32(ml.autoServeGPUCount),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "GryviaModelRegistry")
			os.Exit(1)
		}

		if err = (&controllers.GryviaWorkflowReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			Log:              ctrl.Log.WithName("controllers").WithName("GryviaWorkflow"),
			MaxParallelSteps: ml.workflowMaxParallelSteps,
			AllowWebhooks:    ml.workflowAllowWebhooks,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "GryviaWorkflow")
			os.Exit(1)
		}

		if err = (&controllers.GryviaAutoTunerReconciler{
			Client:            mgr.GetClient(),
			Scheme:            mgr.GetScheme(),
			Log:               ctrl.Log.WithName("controllers").WithName("GryviaAutoTuner"),
			MaxTrialsCap:      int32(ml.tunerMaxTrials),
			MaxParallelismCap: int32(ml.tunerMaxParallelism),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "GryviaAutoTuner")
			os.Exit(1)
		}
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
