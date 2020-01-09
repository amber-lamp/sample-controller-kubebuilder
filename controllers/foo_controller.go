/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"github.com/prometheus/common/log"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	samplecontrollerv1alpha1 "github.com/amber-lamp/sample-controller-kubebuilder/api/v1alpha1"
)

// FooReconciler reconciles a Foo object
type FooReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=foos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=foos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployment,verbs=get;list;watch;create;update;delete
// +kubenuilder:rbac:groups="",reources=events,verbs=create;patch

func (r *FooReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("foo", req.NamespacedName)

	// your logic here
	var foo samplecontrollerv1alpha1.Foo
	log.Info("fetching Foo Resource")
	if err := r.Get(ctx, req.NamespacedName, &foo); err != nil {
		log.Error(err, "unable to fetch Foo")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.cleanupOwnedResources(ctx, log, &foo); err != nil {
		log.Error(err, "failed to clean up old Deployment resources for this Foo")
	}


	return ctrl.Result{}, nil
}

var (
	deploymentOwnerKey = ".metadata.controller"
	apiGVStr = samplecontrollerv1alpha1.GroupVersion.String()
)

func (r *FooReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.Deployment{}, deploymentOwnerKey, func(rawObj runtime.Object) []string {
		deployment := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deployment)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != "Foo" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&samplecontrollerv1alpha1.Foo{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func (r *FooReconciler) cleanupOwnedResources(ctx context.Context, log logr.Logger, foo *samplecontrollerv1alpha1.Foo) error {
	log.Info("finding existing Deployments for Foo resource")

	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments,
		client.InNamespace(foo.Namespace),
		client.MatchingFields(map[string]string{deploymentOwnerKey: foo.Name})); err != nil{
		return err
	}

	for _, deployment := range deployments.Items {
		if deployment.Name == foo.Spec.DeploymentName {
			continue
		}
		if err := r.Delete(ctx, &deployment); err != nil {
			log.Error(err, "Failed to Delete")
			return err
		}

		log.Info("Delete Deployment resource: " + deployment.Name)
		r.Recorder.Eventf(foo, corev1.EventTypeNormal, "Deleted",
			"Deleted deployment %q", deployment.Name)
	}
	return nil
}