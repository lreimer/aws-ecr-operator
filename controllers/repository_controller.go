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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ecrv1beta1 "github.com/lreimer/aws-ecr-operator/api/v1beta1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
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
	logger := ctrl.Log.WithName("controllers").WithName("Repository").WithValues("repository", req.NamespacedName)

	client, awserr := createEcrClient()
	if awserr != nil {
		logger.Error(awserr, "Unable to create ECR client.")
		return ctrl.Result{}, awserr
	}

	// lookup the Repository instance for this reconcile request
	repository := &ecrv1beta1.Repository{}
	k8serr := r.Get(ctx, req.NamespacedName, repository)
	if k8serr != nil {
		if errors.IsNotFound(k8serr) {
			// delete the associated AWS ECR repository
			output, err := client.DeleteRepository(context.TODO(), &ecr.DeleteRepositoryInput{
				RepositoryName: aws.String(repository.Name),
				Force:          true,
			})
			if err != nil {
				logger.Error(err, "Could not delete ECR repository.")
				return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, err
			}

			logger.Info("Successfully deleted ECR repository.", "repositoryUri", output.Repository.RepositoryUri)
			return ctrl.Result{}, nil
		}

		logger.Error(k8serr, "Failed to get Repository.")
		return ctrl.Result{}, k8serr
	}

	// get a list of all matching AWS ECR repositories
	repolist, repoerr := client.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repository.Name},
		MaxResults:      aws.Int32(1),
	})
	if repoerr != nil {
		logger.Error(repoerr, "Could not retrieve list of ECR repository.")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, repoerr
	}

	if len(repolist.Repositories) == 0 {
		// reconcile and create AWS ECR repository
		input := &ecr.CreateRepositoryInput{
			RepositoryName:     aws.String(repository.Name),
			ImageTagMutability: repository.Spec.ImageTagMutability,
		}

		output, err := client.CreateRepository(context.TODO(), input)
		if err != nil {
			logger.Error(err, "Could not create ECR repository.")
			return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, err
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
	} else {
		// reconcile and update AWS ECR repository
		output, err := client.PutImageTagMutability(context.TODO(), &ecr.PutImageTagMutabilityInput{
			RepositoryName:     aws.String(repository.Name),
			ImageTagMutability: repository.Spec.ImageTagMutability,
		})

		if err != nil {
			logger.Error(err, "Could not updated imageTagMutability for ECR repository.")
			return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(5) * time.Second}, err
		}

		logger.Info("Updated imageTagMutability for ECR repository.", "RepositoryName", output.RepositoryName,
			"ImageTagMutability", output.ImageTagMutability)
	}

	return ctrl.Result{}, nil
}

func createEcrClient() (*ecr.Client, error) {
	// load the default AWS config from ENV or shared files
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := ecr.NewFromConfig(cfg)
	return client, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ecrv1beta1.Repository{}).
		Complete(r)
}
