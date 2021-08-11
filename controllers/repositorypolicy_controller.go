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

// RepositoryPolicyReconciler reconciles a RepositoryPolicy object
type RepositoryPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const ecrFinalizer = "ecr.aws.cloud.qaware.de/finalizer"

//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositorypolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositorypolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositorypolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RepositoryPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *RepositoryPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllog.FromContext(ctx).WithValues("repositoryPolicy", req.NamespacedName)

	client, awserr := CreateEcrClient()
	if awserr != nil {
		logger.Error(awserr, "Unable to create ECR client.")
		return ctrl.Result{}, awserr
	}

	// lookup the RepositoryPolicy instance for this reconcile request
	repositoryPolicy := &ecrv1beta1.RepositoryPolicy{}
	geterr := r.Get(ctx, req.NamespacedName, repositoryPolicy)
	if geterr != nil {
		if k8serrors.IsNotFound(geterr) {
			// check for already deleted, might occur due to timing and duplicate reconcile
			logger.Info("RepositoryPolicy already deleted. Skipping.")
			return ctrl.Result{}, nil
		}

		logger.Error(geterr, "Failed to get RepositoryPolicy.")
		return ctrl.Result{}, geterr
	}

	// Check if the RepositoryPolicy instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isRepositoryPolicyMarkedToBeDeleted := repositoryPolicy.GetDeletionTimestamp() != nil
	if isRepositoryPolicyMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(repositoryPolicy, ecrFinalizer) {
			// Run finalization logic for repositoryPolicy. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeRepositoryPolicy(logger, client, repositoryPolicy); err != nil {
				return ctrl.Result{}, err
			}

			// Remove memcachedFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(repositoryPolicy, ecrFinalizer)
			err := r.Update(ctx, repositoryPolicy)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// check if the referenced repository exists in the same namespace
	objectKey := k8stypes.NamespacedName{Name: repositoryPolicy.Spec.RepositoryName, Namespace: req.Namespace}
	repository := &ecrv1beta1.Repository{}
	geterr = r.Get(ctx, objectKey, repository)
	if geterr != nil {
		// wait and requeue until repository can be found
		logger.Error(geterr, "Unable to get referenced Repository object.", "objectKey", objectKey)
		return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, geterr
	}

	// reconcile and create the repository policy
	setout, seterr := client.SetRepositoryPolicy(context.TODO(), &ecr.SetRepositoryPolicyInput{
		RepositoryName: aws.String(repositoryPolicy.Spec.RepositoryName),
		PolicyText:     aws.String(repositoryPolicy.Spec.PolicyText),
		Force:          repositoryPolicy.Spec.Force,
	})
	if seterr != nil {
		logger.Error(seterr, "Could not set ECR RepositoryPolicy.")
		return ctrl.Result{}, seterr
	}

	logger.Info("Successfully set ECR RepositoryPolicy.", "RepositoryName", setout.RepositoryName, "PolicyText", setout.PolicyText)

	// add finalizer for this CR
	if !controllerutil.ContainsFinalizer(repositoryPolicy, ecrFinalizer) {
		logger.Info("Update Finalizer and OwnerReference for RepositoryPolicy.")
		controllerutil.AddFinalizer(repositoryPolicy, ecrFinalizer)
		controllerutil.SetOwnerReference(repository, repositoryPolicy, r.Scheme)
		upderr := r.Update(ctx, repositoryPolicy)
		if upderr != nil {
			logger.Error(upderr, "Unable to update RepositoryPolicy with Finalizer and OwnerReference")
			return ctrl.Result{}, upderr
		}
	}

	return ctrl.Result{}, nil
}

func (r *RepositoryPolicyReconciler) finalizeRepositoryPolicy(logger logr.Logger, client *ecr.Client, rp *ecrv1beta1.RepositoryPolicy) error {
	_, delerr := client.DeleteRepositoryPolicy(context.TODO(), &ecr.DeleteRepositoryPolicyInput{
		RepositoryName: aws.String(rp.Spec.RepositoryName),
	})
	if delerr != nil {
		var rnfe *types.RepositoryNotFoundException
		if errors.As(delerr, &rnfe) {
			// check for already deleted, might occur due to timing and duplicate reconcile
			logger.Info("Repository already deleted. Skipping RepositoryPolicy delete.")
			return nil
		} else {
			logger.Error(delerr, "Failed to delete RepositoryPolicy.", "repositoryPolicyName", rp.Name)
			return delerr
		}
	}

	logger.Info("Successfully finalized and deleted RepositoryPolicy.")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ecrv1beta1.RepositoryPolicy{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
