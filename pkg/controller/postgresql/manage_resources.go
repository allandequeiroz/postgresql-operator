package postgresql

import (
	"context"
	"github.com/dev4devs-com/postgresql-operator/pkg/apis/postgresql-operator/v1alpha1"
	"github.com/dev4devs-com/postgresql-operator/pkg/service"
	"k8s.io/api/apps/v1"
)

// manageResources will ensure that the resource are with the expected values in the cluster
func (r *ReconcilePostgresql) manageResources(db *v1alpha1.Postgresql) error {
	// get the latest version of db deployment
	dep, err := service.FetchDeployment(db.Name, db.Namespace, r.client)
	if err != nil {
		return err
	}

	// Ensure the deployment size is the same as the spec
	r.ensureDepSize(db, dep)
	return nil
}

// ensureDepSize will ensure that the quanity of instances in the cluster for the PostgreSQL deployment is the same defined in the CR
func (r *ReconcilePostgresql) ensureDepSize(db *v1alpha1.Postgresql, dep *v1.Deployment) error {
	size := db.Spec.Size
	if *dep.Spec.Replicas != size {
		// Set the number of Replicas spec in the CR
		dep.Spec.Replicas = &size
		if err := r.client.Update(context.TODO(), dep); err != nil {
			return err
		}
	}
	return nil
}
