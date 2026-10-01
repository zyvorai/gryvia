// Command gpu-reset-agent runs one GPU reset agent for the node named by NODE_NAME (or --node).
// Default is dry-run; --execute actually runs nvidia-smi --gpu-reset. See pkg/agent.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zyvorai/gryvia/operators/gpu-operator/pkg/agent"
)

func main() {
	node := flag.String("node", os.Getenv("NODE_NAME"), "node to act on (default $NODE_NAME)")
	execute := flag.Bool("execute", false, "really run nvidia-smi --gpu-reset (default: dry-run)")
	smi := flag.String("nvidia-smi", "nvidia-smi", "nvidia-smi binary to run with --execute")
	every := flag.Duration("poll", 15*time.Second, "how often to look at the node")
	flag.Parse()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		fail(err)
	}
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		fail(err)
	}
	a := &agent.Agent{Client: c, Node: *node, Execute: *execute, Runner: agent.SMIRunner{Path: *smi}}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	mode := "dry-run"
	if *execute {
		mode = "EXECUTE"
	}
	fmt.Printf("gpu-reset-agent node=%s mode=%s poll=%s\n", *node, mode, *every)
	err = a.Run(ctx, *every,
		func(state string) { fmt.Printf("handled a reset request: %s\n", state) },
		func(err error) { fmt.Fprintf(os.Stderr, "reconcile: %v\n", err) })
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gpu-reset-agent:", err)
	os.Exit(1)
}
