package v1alpha1

import (
	"context"
	"errors"
	"k8s.io/api/authentication/v1beta1"
	"strings"

	projectv1 "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	authapi "k8s.io/api/authorization/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MigTokenSpec defines the desired state of MigToken
type MigTokenSpec struct {
	SecretRef              *kapi.ObjectReference `json:"secretRef"`
	MigClusterRef          *kapi.ObjectReference `json:"migClusterRef"`
	MigrationControllerRef *kapi.ObjectReference `json:"migrationControllerRef,omitempty"`
}

// MigTokenStatus defines the observed state of MigToken
type MigTokenStatus struct {
	Conditions
	ObservedDigest string `json:"observedDigest,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigToken is the Schema for the migtokens API
// +k8s:openapi-gen=true
type MigToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigTokenSpec   `json:"spec,omitempty"`
	Status MigTokenStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigTokenList contains a list of MigToken
type MigTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigToken `json:"items"`
}

type Authorized map[string]bool

func init() {
	SchemeBuilder.Register(&MigToken{}, &MigTokenList{})
}

// Function to determine if a user can *verb* on *resource*
// If name is "" then it means all resources
// If group is "*" then it means all API Groups
// if namespace is "" then it means all cluster scoped resources
func (r *MigToken) CanI(client k8sclient.Client, verb, group, resource, namespace, name string) (bool, error) {
	sar := authapi.SelfSubjectAccessReview{
		Spec: authapi.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authapi.ResourceAttributes{
				Resource:  resource,
				Group:     group,
				Namespace: namespace,
				Verb:      verb,
				Name:      name,
			},
		},
	}

	tokenClient, err := r.GetClient(client)
	if err != nil {
		return false, err
	}

	err = tokenClient.Create(context.TODO(), &sar)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

// Check if the user has `use` verb on the associated MigrationController resource
func (r *MigToken) HasUsePermission(client k8sclient.Client) (bool, error) {
	migControllerName := "migration-controller"
	migControllerNamespace := "openshift-migration"
	if r.Spec.MigrationControllerRef != nil {
		migControllerName = r.Spec.MigrationControllerRef.Name
		migControllerNamespace = r.Spec.MigrationControllerRef.Namespace
	}
	allowed, err := r.CanI(client, "use", "migration.openshift.io", "migrationcontrollers", migControllerNamespace, migControllerName)
	return allowed, err
}

func (r *MigToken) HasReadPermission(client k8sclient.Client, namespaces []string) (Authorized, error) {
	authorized := Authorized{}
	for _, namespace := range namespaces {
		allowed, err := r.CanI(client, "get", "", "namespaces", namespace, namespace)
		if err != nil {
			return authorized, err
		}
		authorized[namespace] = allowed
	}

	return authorized, nil
}

func (r *MigToken) HasMigratePermission(client k8sclient.Client, namespaces []string) (Authorized, error) {
	resources := []string{
		"/pods",
		"apps/deployments",
		"apps.openshift.io/deploymentconfigs",
		"apps/daemonsets",
		"apps/replicasets",
		"apps/statefulsets",
		"/persistentvolumeclaims"}
	verbs := []string{"get", "create", "update", "delete"}

	authorized := Authorized{}
	for _, namespace := range namespaces {
		authorized[namespace] = true
	loop:
		for _, resource := range resources {
			for _, verb := range verbs {
				groupResource := strings.Split(resource, "/")
				group := groupResource[0]
				resource := groupResource[1]
				allowed, err := r.CanI(client, verb, group, resource, namespace, "")
				if err != nil {
					return nil, err
				}
				if !allowed {
					authorized[namespace] = false
					break loop
				}
			}
		}
	}

	return authorized, nil
}

func (r *MigToken) Authenticate(client k8sclient.Client) (bool, error) {
	cluster, err := GetCluster(client, r.Spec.MigClusterRef)
	if err != nil {
		return false, err
	}
	clusterClient, err := cluster.GetClient(client)
	if err != nil {
		return false, err
	}
	token, err := r.GetToken(client)
	if err != nil {
		return false, err
	}
	tokenReview := v1beta1.TokenReview{
		Spec: v1beta1.TokenReviewSpec{
			Token: token,
		},
	}
	err = clusterClient.Create(context.TODO(), &tokenReview)
	if err != nil {
		return false, err
	}
	return tokenReview.Status.Authenticated, nil
}

func (r *MigToken) GetToken(client k8sclient.Client) (string, error) {
	secret, err := GetSecret(client, r.Spec.SecretRef)
	if err != nil {
		return "", err
	}
	if secret == nil {
		return "", errors.New("identity secret not found")
	}
	if secret.Data["token"] == nil {
		return "", errors.New("identity secret doesn't contain token")
	}
	return string(secret.Data["token"]), nil
}

func (r *MigToken) GetClient(client k8sclient.Client) (k8sclient.Client, error) {
	cluster, err := GetCluster(client, r.Spec.MigClusterRef)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, errors.New("migcluster not found")
	}
	token, err := r.GetToken(client)
	if err != nil {
		return nil, err
	}
	restCfg, err := cluster.BuildRestConfigWithToken(token)
	if err != nil {
		return nil, err
	}

	return k8sclient.New(restCfg, k8sclient.Options{Scheme: scheme.Scheme})
}

func (r *MigToken) GetProjectClient(client k8sclient.Client) (*projectv1.ProjectV1Client, error) {
	cluster, err := GetCluster(client, r.Spec.MigClusterRef)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, errors.New("migcluster not found")
	}
	token, err := r.GetToken(client)
	if err != nil {
		return nil, err
	}
	restCfg, err := cluster.BuildRestConfigWithToken(token)
	if err != nil {
		return nil, err
	}
	return projectv1.NewForConfig(restCfg)
}