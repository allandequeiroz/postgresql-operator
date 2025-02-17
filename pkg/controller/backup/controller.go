package backup

import (
	"github.com/dev4devs-com/postgresql-operator/pkg/apis/postgresql-operator/v1alpha1"
	"github.com/dev4devs-com/postgresql-operator/pkg/service"
	"github.com/dev4devs-com/postgresql-operator/pkg/utils"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/api/batch/v1beta1"
)

var log = logf.Log.WithName("controller_backup")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Backup Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBackup{client: mgr.GetClient(), scheme: mgr.GetScheme(), config: mgr.GetConfig()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("backup-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Backup
	err = c.Watch(&source.Kind{Type: &v1alpha1.Backup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch CronJob resource controlled and created by it
	if err := service.Watch(c, &v1beta1.CronJob{}, true, &v1alpha1.Backup{}); err != nil {
		return err
	}

	// Watch Secret resource controlled and created by it
	if err := service.Watch(c, &v1.Secret{}, true, &v1alpha1.Backup{}); err != nil {
		return err
	}

	// Watch Service resource managed by the Postgresql
	if err := service.Watch(c, &v1.Service{}, false, &v1alpha1.Postgresql{}); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileBackup implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileBackup{}

// ReconcileBackup reconciles a Backup object
type ReconcileBackup struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	config    *rest.Config
	scheme    *runtime.Scheme
	dbPod     *v1.Pod
	dbService *v1.Service
}

// Reconcile reads that state of the cluster for a Backup object and makes changes based on the state read
// and what is in the Backup.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBackup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Backup ...")

	bkp, err := service.FetchBackupCR(request.Name, request.Namespace, r.client)
	if err != nil {
		reqLogger.Error(err, "Failed to get the Backup Custom Resource. Check if the Backup CR is applied in the cluster")
		return reconcile.Result{}, err
	}

	// Add const values for mandatory specs
	utils.AddBackupMandatorySpecs(bkp)

	// Create mandatory objects for the Backup
	if err := r.createResources(bkp, request); err != nil {
		reqLogger.Error(err, "Failed to create and update the secondary resource required for the Backup CR")
		return reconcile.Result{}, err
	}

	// Update the CR status for the primary resource
	if err := r.createUpdateCRStatus(request); err != nil {
		reqLogger.Error(err, "Failed to create and update the status in the Backup CR")
		return reconcile.Result{}, err
	}

	// stop reconciliation
	return reconcile.Result{}, nil
}

//createUpdateCRStatus will create and update the status in the CR applied in the cluster
func (r *ReconcileBackup) createUpdateCRStatus(request reconcile.Request) error {
	if err := r.updatePodDatabaseFoundStatus(request); err != nil {
		return err
	}

	if err := r.updateServiceDbServiceFoundStatus(request); err != nil {
		return err
	}

	if err := r.updateCronJobStatus(request); err != nil {
		return err
	}

	if err := r.updateDBSecretStatus(request); err != nil {
		return err
	}

	if err := r.updateAWSSecretStatus(request); err != nil {
		return err
	}

	if err := r.updateEncSecretStatus(request); err != nil {
		return err
	}

	if err := r.updateBackupStatus(request); err != nil {
		return err
	}
	return nil
}

//createResources will create and update the secondary resource which are required in order to make works successfully the primary resource(CR)
func (r *ReconcileBackup) createResources(bkp *v1alpha1.Backup, request reconcile.Request) error {
	// Check if the database instance was created
	db, err := service.FetchPostgreSQL(bkp.Spec.PostgresqlCRName, request.Namespace, r.client)
	if err != nil {
		return err
	}

	// Set in the ReconcileBackup the Pod database created by PostgreSQL
	// NOTE: This data is required in order to create the secrets which will access the database container to do the backup
	if err := r.setDatabasePod(bkp, db); err != nil {
		return err
	}

	// Set in the ReconcileBackup the Service database created by PostgreSQL
	// NOTE: This data is required in order to create the secrets which will access the database container to do the backup
	if err := r.setDatabaseService(bkp, db); err != nil {
		return err
	}

	// Checks if the secret with the database is created, if not create one
	if err := r.createDatabaseSecret(bkp, db); err != nil {
		return err
	}

	// Check if the secret with the aws data is created, if not create one
	// NOTE: The user can config in the CR to use a pre-existing one by informing the name
	if err := r.createAwsSecret(bkp); err != nil {
		return err
	}

	// Check if the encryptionKey was configured
	if utils.IsEncryptionKeyOptionConfig(bkp) {
		// // Check if the encryptionKey is created, if not create one
		// NOTE: The user can config in the CR to use a pre-existing one by informing the name
		if err := r.createEncryptionKey(bkp); err != nil {
			return err
		}
	}

	// Check if the cronJob is created, if not create one
	if err := r.createCronJob(bkp); err != nil {
		return err
	}
	return nil
}
