package manager

import (
	"flag"
	"github.com/spf13/pflag"
	"time"

	clustersetv1alpha1 "github.com/mumoshu/argocd-clusterset/api/v1alpha1"
	"github.com/mumoshu/argocd-clusterset/pkg/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = clustersetv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

type Manager struct {
	MetricsAddr          string
	EnableLeaderElection bool
	SyncPeriod           time.Duration
}

func (m *Manager) AddFlags(fs flag.FlagSet) {
	fs.StringVar(&m.MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&m.EnableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.DurationVar(&m.SyncPeriod, "sync-period", 30*time.Second, "Determines the minimum frequency at which K8s resources managed by this controller are reconciled.")

	//	flag.Parse()
}

func (m *Manager) AddPFlags(fs *pflag.FlagSet) {
	fs.StringVar(&m.MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&m.EnableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.DurationVar(&m.SyncPeriod, "sync-period", 30*time.Second, "Determines the minimum frequency at which K8s resources managed by this controller are reconciled.")

	//	flag.Parse()
}

func (m *Manager) Run() error {
	var (
		err error
	)

	logger := zap.New(func(o *zap.Options) {
		o.Development = true
	})

	ctrl.SetLogger(logger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: m.MetricsAddr,
		LeaderElection:     m.EnableLeaderElection,
		Port:               9443,
		SyncPeriod:         &m.SyncPeriod,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	clusterSetReconciler := &controllers.ClusterSetReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterSet"),
		Scheme: mgr.GetScheme(),
	}

	if err = clusterSetReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterSet")
		return err
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}
