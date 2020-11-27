package run

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/mumoshu/argocd-clusterset/pkg/awsclicompat"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
)

type Config struct {
	DryRun   bool
	NS       string
	Name     string
	Endpoint string
	CAData   string
}

type ClusterSetConfig struct {
	DryRun  bool
	NS      string
	EKSTags map[string]string
}

func Create(config Config) error {
	ns := config.NS
	name := config.Name
	endpoint := config.Endpoint
	caData := config.CAData
	dryRun := config.DryRun

	clientset := newClientset()

	kubeclient := clientset.CoreV1().Secrets(ns)

	var object *corev1.Secret

	if endpoint == "" || caData == "" {
		var err error

		object, err = newClusterSecretFromName(ns, name)

		if err != nil {
			panic(err)
		}
	} else {
		object = newClusterSecretFromValues(ns, name, endpoint, caData)
	}

	if dryRun {
		text, err := yaml.Marshal(object)
		if err != nil {
			panic(err)
		}

		fmt.Fprintf(os.Stdout, "%s\n", text)

		return nil
	}

	// Manage resource
	_, err := kubeclient.Create(context.TODO(), object, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("Cluster secert %q created successfully\n", name)

	return nil
}

func CreateMissing(config ClusterSetConfig) error {
	ns := config.NS
	dryRun := config.DryRun

	clientset := newClientset()

	kubeclient := clientset.CoreV1().Secrets(ns)

	objects, err := clusterSecretsFromClusters(ns, config.EKSTags)
	if err != nil {
		return err
	}

	for _, object := range objects {
		// Manage resource
		if !dryRun {
			_, err := kubeclient.Create(context.TODO(), object, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}

			fmt.Printf("Cluster secert %q created successfully\n", object.Name)
		} else {
			fmt.Printf("Cluster secert %q created successfully (Dry Run)\n", object.Name)
		}
	}

	return nil
}

func Delete(config Config) error {
	ns := config.NS
	name := config.Name
	dryRun := config.DryRun

	clientset := newClientset()

	kubeclient := clientset.CoreV1().Secrets(ns)

	if dryRun {
		fmt.Fprintf(os.Stdout, "Cluster secrer %q deleted successfully (dry run)\n", name)

		return nil
	}

	// Manage resource
	err := kubeclient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("Cluster secert %q deleted successfully\n", name)

	return nil
}

func DeleteMissing(config ClusterSetConfig) error {
	ns := config.NS
	dryRun := config.DryRun

	clientset := newClientset()

	kubeclient := clientset.CoreV1().Secrets(ns)

	result, err := kubeclient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", SecretLabelKeyArgoCDType, SecretLabelValueArgoCDCluster),
	})
	if err != nil {
		return xerrors.Errorf("listing cluster secrets: %w", err)
	}

	objects, err := clusterSecretsFromClusters(ns, config.EKSTags)
	if err != nil {
		return err
	}

	desiredClusters := map[string]struct{}{}

	for _, obj := range objects {
		desiredClusters[obj.Name] = struct{}{}
	}

	for _, item := range result.Items {
		name := item.Name

		if _, desired := desiredClusters[name]; !desired {
			if dryRun {
				fmt.Printf("Cluster secert %q deleted successfully (Dry Run)\n", name)
			} else {
				// Manage resource
				err := kubeclient.Delete(context.TODO(), name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}

				fmt.Printf("Cluster secert %q deleted successfully\n", name)
			}
		}
	}

	return nil
}

func Sync(config ClusterSetConfig) error {
	if err := CreateMissing(config); err != nil {
		return xerrors.Errorf("creating missing cluster secrets: %w", err)
	}

	if err := DeleteMissing(config); err != nil {
		return xerrors.Errorf("deleting redundant cluster secrets: %w", err)
	}

	return nil
}

func clusterSecretsFromClusters(ns string, tags map[string]string) ([]*corev1.Secret, error) {
	sess := awsclicompat.NewSession("", "")

	eksClient := eks.New(sess)

	var secrets []*corev1.Secret

	var nextToken *string

	for nextToken = nil; nextToken != nil; {
		result, err := eksClient.ListClusters(&eks.ListClustersInput{
			NextToken: nextToken,
		})

		if err != nil {
			return nil, xerrors.Errorf("listing clusters: %w", err)
		}

		for _, clusterName := range result.Clusters {
			result, err := eksClient.DescribeCluster(&eks.DescribeClusterInput{Name: aws.String(*clusterName)})
			if err != nil {
				return nil, xerrors.Errorf("creating cluster secret: %w", err)
			}

			all := true
			for k, v := range tags {
				value := result.Cluster.Tags[k]

				all = all && value != nil && *value == v
			}

			if all {
				sec := newClusterSecretFromCluster(ns, *clusterName, result)

				secrets = append(secrets, sec)
			} else {
				log.Printf("Cluster %s with tags %v did not match selector %v", *clusterName, result.Cluster.Tags, tags)
			}
		}

		nextToken = result.NextToken
	}

	return secrets, nil
}

func newClusterSecretFromName(ns, name string) (*corev1.Secret, error) {
	sess := awsclicompat.NewSession("", "")

	eksClient := eks.New(sess)

	result, err := eksClient.DescribeCluster(&eks.DescribeClusterInput{Name: aws.String(name)})
	if err != nil {
		return nil, err
	}

	return newClusterSecretFromCluster(ns, name, result), nil
}

func newClusterSecretFromCluster(ns, name string, result *eks.DescribeClusterOutput) *corev1.Secret {
	return newClusterSecretFromValues(ns, name, *result.Cluster.Endpoint, *result.Cluster.CertificateAuthority.Data)
}

const (
	SecretLabelKeyArgoCDType      = "argocd.argoproj.io/secret-type"
	SecretLabelValueArgoCDCluster = "cluster"
)

func newClusterSecretFromValues(ns, name, server, base64CA string) *corev1.Secret {
	// Create resource object
	object := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				SecretLabelKeyArgoCDType: SecretLabelValueArgoCDCluster,
			},
		},
		StringData: map[string]string{
			"name":   name,
			"server": server,
			"config": fmt.Sprintf(`{
      "awsAuthConfig": {
        "clusterName": "%s"
      },
      "tlsClientConfig": {
        "insecure": false,
        "caData": "%s"
      }
    }
`, name, base64CA),
		},
	}

	return object
}

func newClientset() *kubernetes.Clientset {
	var kubeconfig string
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}
