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

package constraint

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/open-policy-agent/frameworks/constraint/pkg/apis/templates/v1alpha1"
	opa "github.com/open-policy-agent/frameworks/constraint/pkg/client"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const project = "gatekeeper.sh"

type Adder struct {
	Opa opa.Client
}

// Add creates a new Constraint Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (a *Adder) Add(mgr manager.Manager, kind string) error {
	r := newReconciler(mgr, kind, a.Opa)
	return add(mgr, r, kind)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kind string, opa opa.Client) reconcile.Reconciler {
	return &ReconcileConstraint{Client: mgr.GetClient(), scheme: mgr.GetScheme(), opa: opa, Kind: kind}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler, kind string) error {
	// Create a new controller
	c, err := controller.New(fmt.Sprintf("%s-controller", strings.ToLower(kind)), mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to the provided constraint
	instance := unstructured.Unstructured{}
	gv := v1alpha1.SchemeGroupVersion
	group := fmt.Sprintf("constraints.%s", project)
	version := gv.Version
	instance.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	err = c.Watch(&source.Kind{Type: &instance}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileConstraint{}

// ReconcileConstraint reconciles an arbitrary constraint object described by Kind
type ReconcileConstraint struct {
	client.Client
	scheme *runtime.Scheme
	opa    opa.Client
	Kind   string
}

// Reconcile reads that state of the cluster for a constraint object and makes changes based on the state read
// and what is in the constraint.Spec
// +kubebuilder:rbac:groups=constraints.gatekeeper.sh,resources=*,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileConstraint) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &unstructured.Unstructured{}
	gv := v1alpha1.SchemeGroupVersion
	group := fmt.Sprintf("constraints.%s", project)
	version := gv.Version
	instance.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: r.Kind})
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	finalizerName := "constraint.finalizers.gatekeeper.sh"
	if instance.GetDeletionTimestamp().IsZero() {
		if !containsString(finalizerName, instance.GetFinalizers()) {
			instance.SetFinalizers(append(instance.GetFinalizers(), finalizerName))
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{Requeue: true}, nil
			}
		}
		glog.Infof("instance to be added: %#v", instance)
		unstructured.RemoveNestedField(instance.Object, "status", "errors")

		if _, err := r.opa.AddConstraint(context.Background(), instance); err != nil {
			return reconcile.Result{}, err
		}
		unstructured.SetNestedField(instance.Object, true, "status", "enforced")
		if err := r.Update(context.Background(), instance); err != nil {
			return reconcile.Result{Requeue: true}, nil
		}
	} else {
		// Handle deletion
		if containsString(finalizerName, instance.GetFinalizers()) {
			if _, err := r.opa.RemoveConstraint(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
			instance.SetFinalizers(removeString(finalizerName, instance.GetFinalizers()))
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{Requeue: true}, nil
			}
		}
	}

	return reconcile.Result{}, nil
}

func containsString(s string, items []string) bool {
	for _, item := range items {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(s string, items []string) []string {
	var rval []string
	for _, item := range items {
		if item != s {
			rval = append(rval, item)
		}
	}
	return rval
}
