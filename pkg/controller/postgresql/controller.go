package postgresql

import (
	"github.com/dev4devs-com/postgresql-operator/pkg/apis/postgresql-operator/v1alpha1"
	"github.com/dev4devs-com/postgresql-operator/pkg/service"
	"github.com/dev4devs-com/postgresql-operator/pkg/utils"
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_postgresql")

// Add creates a new PostgreSQL Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePostgresql{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("postgresql-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Postgresql
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Postgresql{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	/** Watch for changes to secondary resource and create the owner PostgreSQL **/

	// Watch Deployment resource controlled and created by it
	if err := service.Watch(c, &v1.Deployment{}, true, &v1alpha1.Postgresql{}); err != nil {
		return err
	}

	// Watch PersistenceVolumeClaim resource controlled and created by it
	if err := service.Watch(c, &corev1.PersistentVolumeClaim{}, true, &v1alpha1.Postgresql{}); err != nil {
		return err
	}

	// Watch Service resource controlled and created by it
	if err := service.Watch(c, &corev1.Service{}, true, &v1alpha1.Postgresql{}); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePostgresql implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePostgresql{}

// ReconcilePostgresql reconciles a PostgreSQL object
type ReconcilePostgresql struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Postgresql object and makes changes based on the state read
// and what is in the Postgresql.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgresql) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Postgresql ...")

	db, err := service.FetchPostgreSQL(request.Name, request.Namespace, r.client)
	if err != nil {
		reqLogger.Error(err, "Failed to get the Postgresql Custom Resource. Check if the Backup CR is applied in the cluster")
		return reconcile.Result{}, err
	}

	// Add const values for mandatory specs
	utils.AddPostgresqlMandatorySpecs(db)

	if err := r.createResources(db); err != nil {
		reqLogger.Error(err, "Failed to create the secondary resource required for the PostgreSQL CR")
		return reconcile.Result{}, err
	}

	if err := r.manageResources(db); err != nil {
		reqLogger.Error(err, "Failed to manage resource required for the PostgreSQL CR")
		return reconcile.Result{}, err
	}

	if err := r.createUpdateCRStatus(request); err != nil {
		reqLogger.Error(err, "Failed to create and update the status in the PostgreSQL CR")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

//createUpdateCRStatus will create and update the status in the CR applied in the cluster
func (r *ReconcilePostgresql) createUpdateCRStatus(request reconcile.Request) error {

	if err := r.updateDeploymentStatus(request); err != nil {
		return err
	}

	if err := r.updateServiceStatus(request); err != nil {
		return err
	}

	if err := r.updatePvcStatus(request); err != nil {
		return err
	}

	if err := r.updateDBStatus(request); err != nil {
		return err
	}
	return nil
}

//createResources will create the secondary resource which are required in order to make works successfully the primary resource(CR)
func (r *ReconcilePostgresql) createResources(db *v1alpha1.Postgresql) error {
	// Check if deployment for the app exist, if not create one
	if err := r.createDeployment(db); err != nil {
		return err
	}

	// Check if service for the app exist, if not create one
	if err := r.createService(db); err != nil {
		return err
	}

	// Check if PersistentVolumeClaim for the app exist, if not create one
	if err := r.createPvc(db); err != nil {
		return err
	}

	return nil
}
