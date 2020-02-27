package main

import (
	"context"
	"fmt"
	"log"
	"runtime"

	"github.com/plexsystems/sandbox-operator/apis"
	"github.com/plexsystems/sandbox-operator/controller"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	kubemetrics "github.com/operator-framework/operator-sdk/pkg/kube-metrics"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/operator-framework/operator-sdk/pkg/restmapper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var (
	metricsHost               = "0.0.0.0"
	metricsPort         int32 = 8383
	operatorMetricsPort int32 = 8686
	version                   = "0.3.0"
)

func main() {
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Fatalf("watch namespace: %v", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("get config: %v", err)
	}

	err = leader.Become(context.TODO(), "sandbox-operator-lock")
	if err != nil {
		log.Fatalf("leader promotion: %v", err)
	}

	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          namespace,
		MapperProvider:     restmapper.NewDynamicRESTMapper,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		log.Fatalf("new manager: %v", err)
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatalf("add crd scheme: %v", err)
	}

	if err := controller.Add(mgr); err != nil {
		log.Fatalf("add sandbox controller: %v", err)
	}

	service, err := serveMetrics(cfg)
	if err != nil {
		log.Fatalf("serve metrics: %v", err)
	}

	services := []*v1.Service{service}
	_, err = metrics.CreateServiceMonitors(cfg, namespace, services)
	if err != nil {
		if err == metrics.ErrServiceMonitorNotPresent {
			log.Println("prometheus-operator not found. skipping service monitor creation")
		} else {
			log.Fatalf("create service monitors: %v", err)
		}
	}

	log.Println("Starting operator...")
	log.Println(fmt.Sprintf("Version: %s", version))
	log.Println(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Println(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("starting operator: %s", err)
	}
}

func serveMetrics(cfg *rest.Config) (*v1.Service, error) {
	customResourceKinds, err := k8sutil.GetGVKsFromAddToScheme(apis.AddToScheme)
	if err != nil {
		return nil, fmt.Errorf("get schemes: %w", err)
	}

	err = kubemetrics.GenerateAndServeCRMetrics(cfg, []string{""}, customResourceKinds, metricsHost, operatorMetricsPort)
	if err != nil {
		return nil, fmt.Errorf("serve metrics: %w", err)
	}

	servicePorts := []v1.ServicePort{
		{Port: metricsPort, Name: metrics.OperatorPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: metricsPort}},
		{Port: operatorMetricsPort, Name: metrics.CRPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: operatorMetricsPort}},
	}

	service, err := metrics.CreateMetricsService(context.TODO(), cfg, servicePorts)
	if err != nil {
		return nil, fmt.Errorf("create metrics service: %w", err)
	}

	return service, nil
}
