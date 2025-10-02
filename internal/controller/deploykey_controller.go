/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/google/go-github/v74/github"
	authv1alpha1 "github.com/odit-services/ghops/api/v1alpha1"
	"github.com/odit-services/ghops/internal/services"
)

// DeployKeyReconciler reconciles a DeployKey object
type DeployKeyReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	logger     *zap.SugaredLogger
	ghclient   *github.Client
	sshservice services.SSHService
}

const (
	GitHubKnownHosts = `
github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
`
	MaxRetries          = 5
	DefaultRequeueDelay = 10 * time.Minute // Default requeue for pending/reconciling
	SuccessRequeueDelay = 6 * time.Hour    // Requeue for successful resources
	RateLimitBaseDelay  = 30 * time.Minute // Base delay for rate limit errors
	MaxRequeueDelay     = 1 * time.Hour    // Maximum requeue delay
)

func (r *DeployKeyReconciler) HandleError(deploykey *authv1alpha1.DeployKey, err error) (ctrl.Result, error) {
	r.logger.Errorw("Failed to reconcile deploykey", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
	if deploykey.Status.CurrentRetries >= MaxRetries {
		deploykey.Status = authv1alpha1.DeployKeyStatus{
			CrStatus: authv1alpha1.CrStatus{
				State:             authv1alpha1.StateFailed,
				LastAction:        deploykey.Status.LastAction,
				LastMessage:       fmt.Sprintf("Failed to reconcile deploykey after %d retries: %v", MaxRetries, err),
				LastReconcileTime: time.Now().Format(time.RFC3339),
				CurrentRetries:    0,
			},
			SecretRef: deploykey.Status.SecretRef,
			Created:   deploykey.Status.Created,
		}
	} else {
		deploykey.Status = authv1alpha1.DeployKeyStatus{
			CrStatus: authv1alpha1.CrStatus{
				State:             authv1alpha1.StatePending,
				LastAction:        deploykey.Status.LastAction,
				LastMessage:       fmt.Sprintf("Failed to reconcile deploykey: %v", err),
				LastReconcileTime: time.Now().Format(time.RFC3339),
				CurrentRetries:    deploykey.Status.CurrentRetries + 1,
			},
			SecretRef: deploykey.Status.SecretRef,
			Created:   deploykey.Status.Created,
		}
	}
	updateErr := r.Status().Update(context.Background(), deploykey)
	if updateErr != nil {
		r.logger.Errorw("Failed to update deploykey status", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", updateErr)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	if deploykey.Status.CurrentRetries >= MaxRetries {
		r.logger.Errorw("Max retries reached for deploykey", "name", deploykey.Name, "namespace", deploykey.Namespace)
		return ctrl.Result{}, fmt.Errorf("max retries reached for deploykey %s/%s: %v", deploykey.Namespace, deploykey.Name, err)
	}

	// Implement exponential backoff with jitter
	var requeueDelay time.Duration
	if strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "403") {
		// For rate limiting, use longer delays
		requeueDelay = time.Duration(RateLimitBaseDelay.Minutes()+float64(deploykey.Status.CurrentRetries*10)) * time.Minute
		r.logger.Warnw("Rate limit detected, using extended backoff", "name", deploykey.Name, "namespace", deploykey.Namespace, "delay", requeueDelay)
	} else {
		// Exponential backoff for other errors: 5m, 10m, 20m, 40m...
		requeueDelay = time.Duration(5*(1<<deploykey.Status.CurrentRetries)) * time.Minute
		if requeueDelay > MaxRequeueDelay {
			requeueDelay = MaxRequeueDelay // Cap at 1 hour
		}
	}

	r.logger.Infow("Requeue deploykey with backoff", "name", deploykey.Name, "namespace", deploykey.Namespace, "delay", requeueDelay, "retries", deploykey.Status.CurrentRetries)
	return ctrl.Result{RequeueAfter: requeueDelay}, err
}

// +kubebuilder:rbac:groups=auth.github.odit.services,resources=deploykeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=auth.github.odit.services,resources=deploykeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=auth.github.odit.services,resources=deploykeys/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *DeployKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	r.logger.Infow("Reconciling DeployKey", "name", req.Name, "namespace", req.Namespace)

	// Fetch the DeployKey instance
	deploykey := &authv1alpha1.DeployKey{}
	if err := r.Get(ctx, req.NamespacedName, deploykey); err != nil {
		r.logger.Errorw("Failed to get DeployKey", "name", req.Name, "namespace", req.Namespace, "error", err)
		return r.HandleError(deploykey, err)
	}

	if deploykey.Status.State == authv1alpha1.StateFailed {
		r.logger.Infow("DeployKey is in failed state, not requeuing", "name", deploykey.Name, "namespace", deploykey.Namespace)
		return ctrl.Result{}, nil
	}

	// Skip reconciliation for successful resources that were recently reconciled
	if deploykey.Status.State == authv1alpha1.StateSuccess && deploykey.Status.Created {
		if lastReconcile, err := time.Parse(time.RFC3339, deploykey.Status.LastReconcileTime); err == nil {
			if time.Since(lastReconcile) < 5*time.Hour {
				r.logger.Debugw("Skipping reconciliation for recently successful DeployKey", "name", deploykey.Name, "namespace", deploykey.Namespace, "lastReconcile", lastReconcile)
				return ctrl.Result{RequeueAfter: SuccessRequeueDelay}, nil
			}
		}
	}

	// Only update status if it's actually changing
	if deploykey.Status.State != authv1alpha1.StateReconciling {
		deploykey.Status.State = authv1alpha1.StateReconciling
		if err := r.Status().Update(ctx, deploykey); err != nil {
			r.logger.Errorw("Failed to update DeployKey status to Reconciling", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}
	}

	// Check if the DeployKey is marked for deletion
	if deploykey.DeletionTimestamp != nil {
		r.logger.Infow("DeployKey is marked for deletion", "name", deploykey.Name, "namespace", deploykey.Namespace)

		deploykey.Status.LastAction = authv1alpha1.ActionDelete
		if err := r.Status().Update(ctx, deploykey); err != nil {
			r.logger.Errorw("Failed to update DeployKey status to Reconciling", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		// Only attempt to delete from GitHub if we have a valid key ID
		if deploykey.Status.GitHubKeyID > 0 {
			resp, err := r.ghclient.Repositories.DeleteKey(ctx, deploykey.Spec.Owner, deploykey.Spec.Repository, deploykey.Status.GitHubKeyID)
		if err != nil {
				// Log rate limit information if available
				if resp != nil && resp.Header != nil {
					if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
						if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
							if resetTime, parseErr := strconv.ParseInt(reset, 10, 64); parseErr == nil {
								resetTimeFormatted := time.Unix(resetTime, 0).Format(time.RFC3339)
								r.logger.Warnw("GitHub API rate limit info during deletion", "remaining", remaining, "resetTime", resetTimeFormatted)
							}
						}
					}
				}

				// Log the error but don't fail the deletion if the key doesn't exist on GitHub
				if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "Not Found") {
			r.logger.Errorw("Failed to delete deploy key from GitHub", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
				}
				r.logger.Warnw("Deploy key not found on GitHub, continuing with cleanup", "name", deploykey.Name, "namespace", deploykey.Namespace)
			}
		} else {
			r.logger.Infow("No GitHub key ID found, skipping GitHub deletion", "name", deploykey.Name, "namespace", deploykey.Namespace)
		}

		if err := DeleteSecret(ctx, r.Client, deploykey.Namespace, fmt.Sprintf("%s-deploykey", deploykey.Name)); err != nil {
			r.logger.Errorw("Failed to delete secret for deploy key", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		controllerutil.RemoveFinalizer(deploykey, "auth.github.odit.services/deploykey")
		if err := r.Update(ctx, deploykey); err != nil {
			r.logger.Errorw("Failed to remove finalizer from deploy key", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		deploykey.Status.State = authv1alpha1.StateSuccess
		if err := r.Status().Update(ctx, deploykey); err != nil {
			r.logger.Errorw("Failed to update DeployKey status after deletion", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		r.logger.Infow("Deleted deploykey", "name", deploykey.Name, "namespace", deploykey.Namespace)
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(deploykey, "auth.github.odit.services/deploykey") {
		r.logger.Debugw("Adding finalizer to deploykey resource", "name", req.Name, "namespace", req.Namespace)
		controllerutil.AddFinalizer(deploykey, "auth.github.odit.services/deploykey")
		err := r.Update(ctx, deploykey)
		if err != nil {
			r.logger.Errorw("Failed to add finalizer to deploykey resource", "name", req.Name, "namespace", req.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}
		r.logger.Debugw("Finalizer added to deploykey resource", "name", req.Name, "namespace", req.Namespace)
	}

	if !deploykey.Status.Created {
		r.logger.Infow("Creating deploy key", "name", deploykey.Name, "namespace", deploykey.Namespace)
		deploykey.Status.LastAction = authv1alpha1.ActionCreate
		if err := r.Status().Update(ctx, deploykey); err != nil {
			r.logger.Errorw("Failed to update DeployKey status to Reconciling", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		var pubKey, privKey string
		var err error
		switch deploykey.Spec.KeyType {
		case authv1alpha1.ED25519:
			r.logger.Infow("Generating ED25519 key pair for deploy key", "name", deploykey.Name, "namespace", deploykey.Namespace)
			privKey, pubKey, err = r.sshservice.GenerateED25519KeyPair()
			if err != nil {
				r.logger.Errorw("Failed to generate ED25519 SSH key pair", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
				return r.HandleError(deploykey, err)
			}
		case authv1alpha1.RSA:
			r.logger.Infow("Generating RSA key pair for deploy key", "name", deploykey.Name, "namespace", deploykey.Namespace)
			privKey, pubKey, err = r.sshservice.GenerateRSAKeyPair()
			if err != nil {
				r.logger.Errorw("Failed to generate RSA SSH key pair", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
				return r.HandleError(deploykey, err)
			}
		default:
			err := fmt.Errorf("unsupported key type: %s", deploykey.Spec.KeyType)
			r.logger.Errorw("Invalid key type specified for deploy key", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-deploykey", deploykey.Name),
				Namespace: deploykey.Namespace,
			},
			StringData: map[string]string{
				"identity":     privKey,
				"identity.pub": pubKey,
				"known_hosts":  GitHubKnownHosts,
			},
		}
		if err := CreateSecret(ctx, r.Client, secret); err != nil {
			r.logger.Errorw("Failed to create secret for deploy key", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		deploykey.Status.SecretRef = secret.Name
		if err := r.Status().Update(ctx, deploykey); err != nil {
			r.logger.Errorw("Failed to update DeployKey status secretref", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		readOnly := deploykey.Spec.Permission == authv1alpha1.ReadOnly
		keyrequest := &github.Key{
			Key:      &pubKey,
			Title:    &deploykey.Spec.Title,
			ReadOnly: &readOnly,
		}

		keyresponse, resp, err := r.ghclient.Repositories.CreateKey(ctx, deploykey.Spec.Owner, deploykey.Spec.Repository, keyrequest)
		if err != nil {
			r.logger.Errorw("Failed to create deploy key on GitHub", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)

			// Log rate limit information if available
			if resp != nil && resp.Header != nil {
				if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
					if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
						if resetTime, parseErr := strconv.ParseInt(reset, 10, 64); parseErr == nil {
							resetTimeFormatted := time.Unix(resetTime, 0).Format(time.RFC3339)
							r.logger.Warnw("GitHub API rate limit info", "remaining", remaining, "resetTime", resetTimeFormatted)
						}
					}
				}
			}

			err = DeleteSecret(ctx, r.Client, deploykey.Namespace, secret.Name)
			if err != nil {
				r.logger.Errorw("Failed to delete secret after GitHub key creation failure", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			}

			return r.HandleError(deploykey, err)
		}

		deploykey.Status.Created = true
		deploykey.Status.State = authv1alpha1.StateSuccess
		deploykey.Status.LastMessage = "Deploy key created successfully"
		deploykey.Status.LastReconcileTime = time.Now().Format(time.RFC3339)
		deploykey.Status.CurrentRetries = 0
		deploykey.Status.GitHubKeyID = keyresponse.GetID()

		if err := r.Status().Update(ctx, deploykey); err != nil {
			r.logger.Errorw("Failed to update DeployKey status after creation", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
			return r.HandleError(deploykey, err)
		}

		r.logger.Infow("Deploy key created", "name", deploykey.Name, "namespace", deploykey.Namespace)

	} else {
		r.logger.Debugw("Deploy key already exists, validating...", "name", deploykey.Name, "namespace", deploykey.Namespace)

		// Only update status if it's not already successful
		if deploykey.Status.State != authv1alpha1.StateSuccess {
			deploykey.Status.LastAction = authv1alpha1.ActionUpdate
			deploykey.Status.State = authv1alpha1.StateSuccess
			deploykey.Status.LastMessage = "Deploy key validated successfully"
			deploykey.Status.LastReconcileTime = time.Now().Format(time.RFC3339)
			if err := r.Status().Update(ctx, deploykey); err != nil {
				r.logger.Errorw("Failed to update DeployKey status", "name", deploykey.Name, "namespace", deploykey.Namespace, "error", err)
				return r.HandleError(deploykey, err)
			}
		}
	}

	// Only requeue if the resource is in a pending or reconciling state
	// Successful resources don't need continuous reconciliation
	if deploykey.Status.State == authv1alpha1.StatePending || deploykey.Status.State == authv1alpha1.StateReconciling {
		return ctrl.Result{RequeueAfter: DefaultRequeueDelay}, nil
	}

	// For successful resources, only requeue if there are changes or much longer intervals
	return ctrl.Result{RequeueAfter: SuccessRequeueDelay}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeployKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}

	var zapLogLevel zapcore.Level
	err := zapLogLevel.UnmarshalText([]byte(strings.ToLower(logLevel)))
	if err != nil {
		zapLogLevel = zapcore.InfoLevel
	}

	zapConfig := zap.NewProductionConfig()
	zapConfig.Level = zap.NewAtomicLevelAt(zapLogLevel)
	zapLogger, _ := zapConfig.Build()
	defer func() {
		_ = zapLogger.Sync()
	}()
	r.logger = zapLogger.Sugar()

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}

	r.ghclient = github.NewClient(nil).WithAuthToken(ghToken)
	r.logger.Infow("DeployKeyReconciler initialized", "logLevel", logLevel)

	r.sshservice = &services.DefaultSSHService{
		RSAKeyLength: services.DefaultRSAKeyLength,
	}

	r.logger.Infow("Setting up DeployKeyReconciler with controller manager")

	// Create a more restrictive predicate to reduce unnecessary reconciliations
	statusChangePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Only reconcile if generation changed (spec updates) or if status indicates failure/pending
			if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
				return true
			}

			// Check if the resource is in a state that needs reconciliation
			newDK, ok := e.ObjectNew.(*authv1alpha1.DeployKey)
			if !ok {
				return false
			}

			// Only reconcile if not in success state or if it's a new resource
			return newDK.Status.State != authv1alpha1.StateSuccess || !newDK.Status.Created
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true // Always reconcile new resources
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true // Always handle deletions
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1alpha1.DeployKey{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.LabelChangedPredicate{},
			statusChangePredicate,
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1, // Limit to 1 concurrent reconciliation to reduce API pressure
		}).
		Named("deploykey").
		Complete(r)
}
