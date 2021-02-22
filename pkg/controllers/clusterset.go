/*
Copyright 2020 The argocd-clusterset authors.

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
	"fmt"
	"github.com/mumoshu/argocd-clusterset/pkg/run"
	"time"

	"github.com/go-logr/logr"
	//"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mumoshu/argocd-clusterset/api/v1alpha1"
)

const (
	containerName = "runner"
	finalizerName = "runner.clusterset.mumo.co"
)

// ClusterSetReconciler reconciles a ClusterSet object
type ClusterSetReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

// +kubebuilder:rbac:groups=clusterset.mumo.co,resources=clustersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterset.mumo.co,resources=clustersets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterset.mumo.co,resources=clustersets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *ClusterSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("clusterSet", req.NamespacedName)

	var clusterSet v1alpha1.ClusterSet
	if err := r.Get(ctx, req.NamespacedName, &clusterSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if clusterSet.ObjectMeta.DeletionTimestamp.IsZero() {
		finalizers, added := addFinalizer(clusterSet.ObjectMeta.Finalizers)

		if added {
			newRunner := clusterSet.DeepCopy()
			newRunner.ObjectMeta.Finalizers = finalizers

			if err := r.Update(ctx, newRunner); err != nil {
				log.Error(err, "Failed to update clusterSet")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}
	} else {
		finalizers, removed := removeFinalizer(clusterSet.ObjectMeta.Finalizers)

		if removed {
			// TODO do someo finalization if necessary

			newRunner := clusterSet.DeepCopy()
			newRunner.ObjectMeta.Finalizers = finalizers

			if err := r.Update(ctx, newRunner); err != nil {
				log.Error(err, "Failed to update clusterSet")
				return ctrl.Result{}, err
			}

			log.Info("Removed clusterSet")
		}

		return ctrl.Result{}, nil
	}

	config := run.ClusterSetConfig{
		DryRun:               false,
		NS:                   req.Namespace,
		RoleARN:              clusterSet.Spec.Selector.RoleARN,
		EKSTags:              clusterSet.Spec.Selector.EKSTags,
		Labels:               clusterSet.Spec.Template.Metadata.Labels,
		AWSAuthConfigRoleARN: clusterSet.Spec.Template.Metadata.Config.AWSAuthConfig.RoleARN,
	}

	if err := run.Sync(config); err != nil {
		log.Error(err, "Syncing clusters")

		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	r.Recorder.Event(&clusterSet, corev1.EventTypeNormal, "SyncFinished", fmt.Sprintf("Sync finished on '%s'", clusterSet.Name))

	return ctrl.Result{}, nil
}

func (r *ClusterSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("clusterset-controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterSet{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func addFinalizer(finalizers []string) ([]string, bool) {
	exists := false
	for _, name := range finalizers {
		if name == finalizerName {
			exists = true
		}
	}

	if exists {
		return finalizers, false
	}

	return append(finalizers, finalizerName), true
}

func removeFinalizer(finalizers []string) ([]string, bool) {
	removed := false
	result := []string{}

	for _, name := range finalizers {
		if name == finalizerName {
			removed = true
			continue
		}
		result = append(result, name)
	}

	return result, removed
}
