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

package sync

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/open-policy-agent/gatekeeper/pkg/logging"
	"github.com/open-policy-agent/gatekeeper/pkg/metrics"
	"github.com/open-policy-agent/gatekeeper/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller").WithValues("metaKind", "Sync")

type Adder struct {
	Opa          OpaDataClient
	Events       <-chan event.GenericEvent
	MetricsCache *MetricsCache
}

// Add creates a new Sync Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (a *Adder) Add(mgr manager.Manager) error {
	reporter, err := NewStatsReporter()
	if err != nil {
		log.Error(err, "Sync metrics reporter could not start")
		return err
	}

	r, err := newReconciler(mgr, a.Opa, *reporter, a.MetricsCache)
	if err != nil {
		return err
	}
	return add(mgr, r, a.Events)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(
	mgr manager.Manager,
	opa OpaDataClient,
	reporter Reporter,
	metricsCache *MetricsCache) (reconcile.Reconciler, error) {

	return &ReconcileSync{
		reader:       mgr.GetCache(),
		scheme:       mgr.GetScheme(),
		opa:          opa,
		log:          log,
		reporter:     reporter,
		metricsCache: metricsCache,
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler, events <-chan event.GenericEvent) error {
	// Create a new controller
	c, err := controller.New("sync-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to the provided resource
	return c.Watch(
		&source.Channel{
			Source:         events,
			DestBufferSize: 1024,
		},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: util.EventPacker{}},
	)
}

var _ reconcile.Reconciler = &ReconcileSync{}

type MetricsCache struct {
	mux        sync.RWMutex
	Cache      map[string]tags
	KnownKinds map[string]bool
}

type tags struct {
	kind   string
	status metrics.Status
}

// ReconcileSync reconciles an arbitrary object described by Kind
type ReconcileSync struct {
	reader client.Reader

	scheme       *runtime.Scheme
	opa          OpaDataClient
	log          logr.Logger
	reporter     Reporter
	metricsCache *MetricsCache
}

// +kubebuilder:rbac:groups=constraints.gatekeeper.sh,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for an object and makes changes based on the state read
// and what is in the constraint.Spec
func (r *ReconcileSync) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	timeStart := time.Now()
	gvk, unpackedRequest, err := util.UnpackRequest(request)
	if err != nil {
		// Unrecoverable, do not retry.
		// TODO(OREN) add metric
		log.Error(err, "unpacking request", "request", request)
		return reconcile.Result{}, nil
	}

	reportMetrics := false
	defer func() {
		if reportMetrics {
			if err := r.reporter.reportSyncDuration(time.Since(timeStart)); err != nil {
				log.Error(err, "failed to report sync duration")
			}

			r.metricsCache.ReportSync(&r.reporter)

			if err := r.reporter.reportLastSync(); err != nil {
				log.Error(err, "failed to report last sync timestamp")
			}
		}
	}()

	instance := &unstructured.Unstructured{}
	instance.SetGroupVersionKind(gvk)

	err = r.reader.Get(context.TODO(), unpackedRequest.NamespacedName, instance)
	syncKey := strings.Join([]string{instance.GetNamespace(), instance.GetName()}, "/")
	if err != nil {
		if errors.IsNotFound(err) {
			// This is a deletion; remove the data
			instance.SetNamespace(unpackedRequest.Namespace)
			instance.SetName(unpackedRequest.Name)
			if _, err := r.opa.RemoveData(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
			r.metricsCache.DeleteObject(syncKey)
			reportMetrics = true
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		if _, err := r.opa.RemoveData(context.Background(), instance); err != nil {
			return reconcile.Result{}, err
		}
		r.metricsCache.DeleteObject(syncKey)
		reportMetrics = true
		return reconcile.Result{}, nil
	}

	r.log.V(logging.DebugLevel).Info("data will be added", "data", instance)
	if _, err := r.opa.AddData(context.Background(), instance); err != nil {
		r.metricsCache.addObject(syncKey, tags{
			kind:   instance.GetKind(),
			status: metrics.ErrorStatus,
		})
		reportMetrics = true

		return reconcile.Result{}, err
	}

	r.metricsCache.addObject(syncKey, tags{
		kind:   instance.GetKind(),
		status: metrics.ActiveStatus,
	})

	r.metricsCache.addKind(instance.GetKind())

	reportMetrics = true

	return reconcile.Result{}, nil
}

func NewMetricsCache() *MetricsCache {
	return &MetricsCache{
		Cache:      make(map[string]tags),
		KnownKinds: make(map[string]bool),
	}
}

// need to know encountered kinds to reset metrics for that kind
// this is a known memory leak
func (c *MetricsCache) addKind(key string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.KnownKinds[key] = true
}

func (c *MetricsCache) addObject(key string, t tags) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.Cache[key] = tags{
		kind:   t.kind,
		status: t.status,
	}
}

func (c *MetricsCache) DeleteObject(key string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.Cache, key)
}

func (c *MetricsCache) ReportSync(reporter *Reporter) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	totals := make(map[tags]int)
	for _, v := range c.Cache {
		totals[v]++
	}

	for kind := range c.KnownKinds {
		for _, status := range metrics.AllStatuses {
			if err := reporter.reportSync(
				tags{
					kind:   kind,
					status: status,
				},
				int64(totals[tags{
					kind:   kind,
					status: status,
				}])); err != nil {
				log.Error(err, "failed to report sync")
			}
		}
	}
}
