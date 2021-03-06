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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	ecrv1beta1 "github.com/lreimer/aws-ecr-operator/api/v1beta1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ecr.aws.cloud.qaware.de,resources=repositories/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Repository object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllog.FromContext(ctx).WithValues("repository", req.NamespacedName)

	client, awserr := CreateEcrClient()
	if awserr != nil {
		logger.Error(awserr, "Unable to create ECR client.")
		return ctrl.Result{}, awserr
	}

	// TODO: add Finalizer logic to delete Repository
	// lookup the Repository instance for this reconcile request
	repository := &ecrv1beta1.Repository{}
	k8serr := r.Get(ctx, req.NamespacedName, repository)
	if k8serr != nil {
		if k8serrors.IsNotFound(k8serr) {
			// delete the associated AWS ECR repository
			output, err := client.DeleteRepository(context.TODO(), &ecr.DeleteRepositoryInput{
				// need to use Name from request
				RepositoryName: aws.String(req.Name),
				Force:          true,
			})
			if err != nil {
				var rnfe *types.RepositoryNotFoundException
				if errors.As(err, &rnfe) {
					// check for already deleted, might occur due to timing and duplicate reconcile
					logger.Info("Repository already deleted. Skipping.")
					return ctrl.Result{}, nil
				} else {
					logger.Error(err, "Could not delete ECR repository.")
					return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, err
				}
			}

			logger.Info("Successfully deleted ECR repository.", "repositoryUri", output.Repository.RepositoryUri)
			return ctrl.Result{}, nil
		}

		logger.Error(k8serr, "Failed to get Repository.")
		return ctrl.Result{}, k8serr
	}

	// try to get the matching AWS ECR repository
	_, repoerr := client.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repository.Name},
	})
	if repoerr != nil {
		var rnfe *types.RepositoryNotFoundException
		if errors.As(repoerr, &rnfe) {
			// reconcile and create AWS ECR repository
			input := &ecr.CreateRepositoryInput{
				RepositoryName:             aws.String(repository.Name),
				ImageTagMutability:         createImageTagMutability(*repository),
				ImageScanningConfiguration: createImageScanningConfiguration(*repository),
				EncryptionConfiguration:    createEncryptionConfiguration(*repository),
				Tags:                       createTags(*repository),
			}

			output, err := client.CreateRepository(context.TODO(), input)
			if err != nil {
				// check for duplicate creation, might occur due to timing and duplicate reconcile
				logger.Info("Checking for RepositoryAlreadyExistsException due to CreateRepository error.", "error", err)

				var raee *types.RepositoryAlreadyExistsException
				if errors.As(err, &raee) {
					logger.Info("Repository already exists. Skipping.")
					return ctrl.Result{}, nil
				} else {
					logger.Error(err, "Could not create ECR repository.")
					return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, err
				}
			}

			logger.Info("Created ECR repository.", "RepositoryName", output.Repository.RepositoryName,
				"RepositoryUri", output.Repository.RepositoryUri)

			// we need to update the status
			repository.Status.RepositoryArn = *output.Repository.RepositoryArn
			repository.Status.RegistryId = *output.Repository.RegistryId
			repository.Status.RepositoryUri = *output.Repository.RepositoryUri

			err = r.Status().Update(ctx, repository)
			if err != nil {
				logger.Error(err, "Failed to update Repository status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		} else {
			logger.Error(repoerr, "Could not retrieve list of ECR repository.")
			return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, repoerr
		}
	}

	// reconcile and update AWS ECR repository ImageTagMutability
	mutout, muterr := client.PutImageTagMutability(context.TODO(), &ecr.PutImageTagMutabilityInput{
		RepositoryName:     aws.String(repository.Name),
		ImageTagMutability: createImageTagMutability(*repository),
	})
	if muterr != nil {
		logger.Error(muterr, "Could not update ImageTagMutability for ECR repository.")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, muterr
	}

	logger.Info("Updated ImageTagMutability for ECR repository.", "RepositoryName", mutout.RepositoryName,
		"ImageTagMutability", mutout.ImageTagMutability)

	// reconcile and update AWS ECR repository ImageScanningConfiguration
	scanout, scanerr := client.PutImageScanningConfiguration(context.TODO(), &ecr.PutImageScanningConfigurationInput{
		RepositoryName:             aws.String(repository.Name),
		ImageScanningConfiguration: createImageScanningConfiguration(*repository),
	})
	if scanerr != nil {
		logger.Error(muterr, "Could not update ImageScanningConfiguration for ECR repository.")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, scanerr
	}

	logger.Info("Updated ImageScanningConfiguration for ECR repository.", "RepositoryName", scanout.RepositoryName,
		"ImageScanningConfiguration", scanout.ImageScanningConfiguration)

	// reconcile and update AWS ECR repository tags
	_, tagerr := client.TagResource(context.TODO(), &ecr.TagResourceInput{
		ResourceArn: &repository.Status.RepositoryArn,
		Tags:        createTags(*repository),
	})
	if tagerr != nil {
		logger.Error(tagerr, "Could not update Tags for ECR repository.")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, tagerr
	}
	logger.Info("Updated Tags for ECR repository.", "ResourceArn", &repository.Status.RepositoryArn)

	// ATTENTION: update of AWS ECR repository EncryptionConfiguration not possible

	return ctrl.Result{}, nil
}

func createImageTagMutability(r ecrv1beta1.Repository) types.ImageTagMutability {
	value := string(r.Spec.ImageTagMutability)
	return types.ImageTagMutability(value)
}

func createImageScanningConfiguration(r ecrv1beta1.Repository) *types.ImageScanningConfiguration {
	c := r.Spec.ImageScanningConfiguration
	if c == nil {
		return nil
	}
	return &types.ImageScanningConfiguration{ScanOnPush: c.ScanOnPush}
}

func createEncryptionConfiguration(r ecrv1beta1.Repository) *types.EncryptionConfiguration {
	c := r.Spec.EncryptionConfiguration
	if c == nil {
		return nil
	}
	return &types.EncryptionConfiguration{EncryptionType: types.EncryptionType(c.EncryptionType), KmsKey: c.KmsKey}
}

func createTags(r ecrv1beta1.Repository) []types.Tag {
	tags := make([]types.Tag, 0)
	for k, v := range r.Labels {
		tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return tags
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ecrv1beta1.Repository{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
