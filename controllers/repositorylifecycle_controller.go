/*
MIT License

Copyright (c) 2021 M.-Leander Reimer

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package controllers

import (
	"context"
	"errors"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	ecrv1beta1 "github.com/lreimer/aws-ecr-operator/api/v1beta1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

const ecrLifecycleFinalizer = "lifecycle.ecr.aws.cloud.qaware.de/finalizer"

// RepositoryLifecycleReconciler reconciles a RepositoryLifecycle object
type RepositoryLifecycleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositorylifecycles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositorylifecycles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositorylifecycles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *RepositoryLifecycleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllog.FromContext(ctx).WithValues("repositoryLifecycle", req.NamespacedName)

	client, awserr := CreateEcrClient()
	if awserr != nil {
		logger.Error(awserr, "Unable to create ECR client.")
		return ctrl.Result{}, awserr
	}

	// lookup the RepositoryLifecycle instance for this reconcile request
	repositoryLifecycle := &ecrv1beta1.RepositoryLifecycle{}
	geterr := r.Get(ctx, req.NamespacedName, repositoryLifecycle)
	if geterr != nil {
		if k8serrors.IsNotFound(geterr) {
			// check for already deleted, might occur due to timing and duplicate reconcile
			logger.Info("RepositoryLifecycle already deleted. Skipping.")
			return ctrl.Result{}, nil
		}

		logger.Error(geterr, "Failed to get RepositoryLifecycle.")
		return ctrl.Result{}, geterr
	}

	// Check if the RepositoryLifecycle instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isRepositoryLifecycleMarkedToBeDeleted := repositoryLifecycle.GetDeletionTimestamp() != nil
	if isRepositoryLifecycleMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(repositoryLifecycle, ecrLifecycleFinalizer) {
			// Run finalization logic for repositoryLifecycle. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeRepositoryLifecycle(logger, client, repositoryLifecycle); err != nil {
				return ctrl.Result{}, err
			}

			// Remove ecrLifecycleFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(repositoryLifecycle, ecrLifecycleFinalizer)
			err := r.Update(ctx, repositoryLifecycle)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// check if the referenced repository exists in the same namespace
	objectKey := k8stypes.NamespacedName{Name: repositoryLifecycle.Spec.RepositoryName, Namespace: req.Namespace}
	repository := &ecrv1beta1.Repository{}
	geterr = r.Get(ctx, objectKey, repository)
	if geterr != nil {
		// wait and requeue until repository can be found
		logger.Error(geterr, "Unable to get referenced Repository object.", "objectKey", objectKey)
		return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, geterr
	}

	// reconcile and create the lifecycle policy
	setout, seterr := client.PutLifecyclePolicy(context.TODO(), &ecr.PutLifecyclePolicyInput{
		RepositoryName:      aws.String(repositoryLifecycle.Spec.RepositoryName),
		LifecyclePolicyText: aws.String(repositoryLifecycle.Spec.LifecyclePolicyText),
	})
	if seterr != nil {
		logger.Error(seterr, "Could not set ECR LifecyclePolicy.")
		return ctrl.Result{}, seterr
	}

	logger.Info("Successfully set ECR LifecyclePolicy.", "RepositoryName", setout.RepositoryName, "LifecyclePolicyText", setout.LifecyclePolicyText)

	// add finalizer for this CR
	if !controllerutil.ContainsFinalizer(repositoryLifecycle, ecrLifecycleFinalizer) {
		logger.Info("Update Finalizer and OwnerReference for RepositoryLifecycle.")
		controllerutil.AddFinalizer(repositoryLifecycle, ecrLifecycleFinalizer)
		controllerutil.SetOwnerReference(repository, repositoryLifecycle, r.Scheme)
		upderr := r.Update(ctx, repositoryLifecycle)
		if upderr != nil {
			logger.Error(upderr, "Unable to update RepositoryLifecycle with Finalizer and OwnerReference")
			return ctrl.Result{}, upderr
		}
	}

	return ctrl.Result{}, nil
}

func (r *RepositoryLifecycleReconciler) finalizeRepositoryLifecycle(logger logr.Logger, client *ecr.Client, rl *ecrv1beta1.RepositoryLifecycle) error {
	_, delerr := client.DeleteLifecyclePolicy(context.TODO(), &ecr.DeleteLifecyclePolicyInput{
		RepositoryName: aws.String(rl.Spec.RepositoryName),
	})
	if delerr != nil {
		var rnfe *types.RepositoryNotFoundException
		if errors.As(delerr, &rnfe) {
			// check for already deleted, might occur due to timing and duplicate reconcile
			logger.Info("Repository already deleted. Skipping RepositoryLifecycle delete.")
			return nil
		} else {
			logger.Error(delerr, "Failed to delete RepositoryLifecycle.", "repositoryLifecycleName", rl.Name)
			return delerr
		}
	}

	logger.Info("Successfully finalized and deleted RepositoryLifecycle.")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryLifecycleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ecrv1beta1.RepositoryLifecycle{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
