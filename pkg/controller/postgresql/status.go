package postgresql

import (
	"context"
	"fmt"
	"github.com/dev4devs-com/postgresql-operator/pkg/apis/postgresql-operator/v1alpha1"
	"github.com/dev4devs-com/postgresql-operator/pkg/service"
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const statusOk = "OK"

//updateDBStatus returns error when status regards the all required resource could not be updated
func (r *ReconcilePostgresql) updateDBStatus(request reconcile.Request) error {
	db, err := service.FetchPostgreSQL(request.Name, request.Namespace, r.client)
	if err != nil {
		return err
	}

	statusMsgUpdate := statusOk
	// Check if all required resource were created and found
	if err := r.isAllCreated(db); err != nil {
		statusMsgUpdate = err.Error()
	}

	// Check if BackupStatus was changed, if yes update it
	if err := r.insertUpdateDatabaseStatus(db, statusMsgUpdate); err != nil {
		return err
	}
	return nil
}

// Check if DatabaseStatus was changed, if yes update it
func (r *ReconcilePostgresql) insertUpdateDatabaseStatus(db *v1alpha1.Postgresql, statusMsgUpdate string) error {
	if !reflect.DeepEqual(statusMsgUpdate, db.Status.DatabaseStatus) {
		db.Status.DatabaseStatus = statusOk
		if err := r.client.Status().Update(context.TODO(), db); err != nil {
			return err
		}
	}
	return nil
}

//updateDeploymentStatus returns error when status regards the deployment resource could not be updated
func (r *ReconcilePostgresql) updateDeploymentStatus(request reconcile.Request) error {
	db, err := service.FetchPostgreSQL(request.Name, request.Namespace, r.client)
	if err != nil {
		return err
	}

	dep, err := service.FetchDeployment(db.Name, db.Namespace, r.client)
	if err != nil {
		return err
	}

	// Check if Deployment Status was changed, if yes update it
	if err := r.insertUpdateDeploymentStatus(dep, db); err != nil {
		return err
	}

	return nil
}

// insertUpdateDeploymentStatus will check if Deployment status changed, if yes then and update it
func (r *ReconcilePostgresql) insertUpdateDeploymentStatus(deploymentStatus *v1.Deployment, db *v1alpha1.Postgresql) error {
	if !reflect.DeepEqual(deploymentStatus.Status, db.Status.DeploymentStatus) {
		db.Status.DeploymentStatus = deploymentStatus.Status
		if err := r.client.Status().Update(context.TODO(), db); err != nil {
			return err
		}
	}
	return nil
}

//updateServiceStatus returns error when status regards the service resource could not be updated
func (r *ReconcilePostgresql) updateServiceStatus(request reconcile.Request) error {
	db, err := service.FetchPostgreSQL(request.Name, request.Namespace, r.client)
	if err != nil {
		return err
	}

	ser, err := service.FetchService(db.Name, db.Namespace, r.client)
	if err != nil {
		return err
	}

	// Check if Service Status was changed, if yes update it
	if err := r.insertUpdateServiceStatus(ser, db); err != nil {
		return err
	}

	return nil
}

// insertUpdateDeploymentStatus will check if Service status changed, if yes then and update it
func (r *ReconcilePostgresql) insertUpdateServiceStatus(serviceStatus *corev1.Service, db *v1alpha1.Postgresql) error {
	if !reflect.DeepEqual(serviceStatus.Status, db.Status.ServiceStatus) {
		db.Status.ServiceStatus = serviceStatus.Status
		if err := r.client.Status().Update(context.TODO(), db); err != nil {
			return err
		}
	}
	return nil
}

// updatePvcStatus returns error when status regards the PersistentVolumeClaim resource could not be updated
func (r *ReconcilePostgresql) updatePvcStatus(request reconcile.Request) error {
	db, err := service.FetchPostgreSQL(request.Name, request.Namespace, r.client)
	if err != nil {
		return err
	}

	pvc, err := service.FetchPersistentVolumeClaim(db.Name, db.Namespace, r.client)
	if err != nil {
		return err
	}

	r.insertUpdatePvcStatus(pvc, db)
	return nil
}

// insertUpdatePvcStatus will check if Service status changed, if yes then and update it
func (r *ReconcilePostgresql) insertUpdatePvcStatus(pvc *corev1.PersistentVolumeClaim, db *v1alpha1.Postgresql) error {
	if !reflect.DeepEqual(pvc.Status, db.Status.PVCStatus) {
		db.Status.PVCStatus = pvc.Status
		if err := r.client.Status().Update(context.TODO(), db); err != nil {
			return err
		}
	}
	return nil
}

//validateBackup returns error when some requirement is missing
func (r *ReconcilePostgresql) isAllCreated(db *v1alpha1.Postgresql) error {

	// Check if the PersistentVolumeClaim was created
	_, err := service.FetchPersistentVolumeClaim(db.Name, db.Namespace, r.client)
	if err != nil {
		err = fmt.Errorf("Error: PersistentVolumeClaim is missing.")
	}

	// Check if the Deployment was created
	_, err = service.FetchDeployment(db.Name, db.Namespace, r.client)
	if err != nil {
		err = fmt.Errorf("Error: Deployment is missing.")
	}

	// Check if the Service was created
	_, err = service.FetchService(db.Name, db.Namespace, r.client)
	if err != nil {
		err = fmt.Errorf("Error: Service is missing.")
	}

	return nil
}
