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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
)

type Config struct {
	DryRun   bool
	NS       string
	RoleARN  string
	Name     string
	Endpoint string
	CAData   string
	Labels   map[string]string
	AwsAuthConfigRoleARN string
}

type ClusterSetConfig struct {
	DryRun               bool
	NS                   string
	RoleARN              string
	EKSTags              map[string]string
	Labels               map[string]string
	AWSAuthConfigRoleARN string
}

func Create(config Config) error {
	ns := config.NS
	roleARN := config.RoleARN
	name := config.Name
	endpoint := config.Endpoint
	caData := config.CAData
	dryRun := config.DryRun
	labels := config.Labels
	awsAuthConfigRoleARN := config.AwsAuthConfigRoleARN

	clientset, err := newClientset()
	if err != nil {
		return xerrors.Errorf("creating clientset: %w", err)
	}

	kubeclient := clientset.CoreV1().Secrets(ns)

	var object *corev1.Secret

	if endpoint == "" || caData == "" {
		var err error

		object, err = newClusterSecretFromName(ns, roleARN, name, labels, awsAuthConfigRoleARN)

		if err != nil {
			panic(err)
		}
	} else {
		object = newClusterSecretFromValues(ns, name, labels, endpoint, caData, awsAuthConfigRoleARN)
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
	_, err = kubeclient.Create(context.TODO(), object, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("Cluster secert %q created successfully\n", name)

	return nil
}

func CreateMissing(config ClusterSetConfig) error {
	ns := config.NS
	dryRun := config.DryRun

	clientset, err := newClientset()
	if err != nil {
		return xerrors.Errorf("creating clientset: %w", err)
	}

	kubeclient := clientset.CoreV1().Secrets(ns)

	objects, err := clusterSecretsFromClusters(ns, config.RoleARN, config.EKSTags, config.Labels, config.AWSAuthConfigRoleARN)
	if err != nil {
		return err
	}

	for _, object := range objects {
		// Manage resource
		if !dryRun {
			_, err := kubeclient.Create(context.TODO(), object, metav1.CreateOptions{})
			if err != nil {
				if errors.IsAlreadyExists(err) {
					fmt.Printf("Cluster secert %q has no change\n", object.Name)
				} else {
					return err
				}
			} else {
				fmt.Printf("Cluster secert %q created successfully\n", object.Name)
			}
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

	clientset, err := newClientset()
	if err != nil {
		return xerrors.Errorf("creating clientset: %w", err)
	}

	kubeclient := clientset.CoreV1().Secrets(ns)

	if dryRun {
		fmt.Fprintf(os.Stdout, "Cluster secrer %q deleted successfully (dry run)\n", name)

		return nil
	}

	// Manage resource
	err = kubeclient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("Cluster secert %q deleted successfully\n", name)

	return nil
}

func DeleteMissing(config ClusterSetConfig) error {
	ns := config.NS
	dryRun := config.DryRun

	clientset, err := newClientset()
	if err != nil {
		return xerrors.Errorf("creating clientset: %w", err)
	}

	kubeclient := clientset.CoreV1().Secrets(ns)

	labelSelectors := []string{
		fmt.Sprintf("%s=%s", SecretLabelKeyArgoCDType, SecretLabelValueArgoCDCluster),
	}

	for k, v := range config.Labels {
		labelSelectors = append(labelSelectors, fmt.Sprintf("%s=%s", k, v))
	}

	result, err := kubeclient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join(labelSelectors, ","),
	})
	if err != nil {
		return xerrors.Errorf("listing cluster secrets: %w", err)
	}

	objects, err := clusterSecretsFromClusters(ns, config.RoleARN, config.EKSTags, config.Labels, config.AWSAuthConfigRoleARN)
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

func clusterSecretsFromClusters(ns, roleARN string, tags, labels map[string]string, awsAuthConfigRoleARN string) ([]*corev1.Secret, error) {
	sess := awsclicompat.NewSession("", "", roleARN)

	eksClient := eks.New(sess)

	var secrets []*corev1.Secret

	process := func(nextToken *string) (*string, error) {
		log.Printf("Calling EKS ListClusters...")

		result, err := eksClient.ListClusters(&eks.ListClustersInput{
			NextToken: nextToken,
		})

		if err != nil {
			return nil, xerrors.Errorf("listing clusters: %w", err)
		}

		log.Printf("Found %d clusters.", len(result.Clusters))

		for _, clusterName := range result.Clusters {
			log.Printf("Checking cluster %s...", *clusterName)

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
				sec := newClusterSecretFromCluster(ns, *clusterName, labels, result, awsAuthConfigRoleARN)

				secrets = append(secrets, sec)
			} else {
				log.Printf("Cluster %s with tags %v did not match selector %v", *clusterName, result.Cluster.Tags, tags)
			}
		}

		return result.NextToken, nil
	}

	log.Printf("Computing desired cluster secrets from EKS clusters...")

	nextToken, err := process(nil)
	if err != nil {
		return nil, xerrors.Errorf("processing first set of EKS clusters: %w", err)
	}

	for nextToken = nil; nextToken != nil; {
		var err error

		nextToken, err = process(nextToken)

		if err != nil {
			return nil, err
		}
	}

	return secrets, nil
}

func newClusterSecretFromName(ns, roleARN, name string, labels map[string]string, awsAuthConfigRoleARN string) (*corev1.Secret, error) {
	sess := awsclicompat.NewSession("", "", roleARN)

	eksClient := eks.New(sess)

	result, err := eksClient.DescribeCluster(&eks.DescribeClusterInput{Name: aws.String(name)})
	if err != nil {
		return nil, err
	}

	return newClusterSecretFromCluster(ns, name, labels, result, awsAuthConfigRoleARN), nil
}

func newClusterSecretFromCluster(ns, name string, labels map[string]string, result *eks.DescribeClusterOutput, awsAuthConfigRoleARN string) *corev1.Secret {
	return newClusterSecretFromValues(ns, name, labels, *result.Cluster.Endpoint, *result.Cluster.CertificateAuthority.Data, awsAuthConfigRoleARN)
}

const (
	SecretLabelKeyArgoCDType      = "argocd.argoproj.io/secret-type"
	SecretLabelValueArgoCDCluster = "cluster"
)

func newClusterSecretFromValues(ns, name string, labels map[string]string, server, base64CA, awsAuthConfigRoleARN string) *corev1.Secret {
	lbls := map[string]string{
		SecretLabelKeyArgoCDType: SecretLabelValueArgoCDCluster,
	}

	for k, v := range labels {
		lbls[k] = v
	}

	// Create resource object
	object := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    lbls,
		},
		StringData: map[string]string{
			"name":   name,
			"server": server,
			"config": fmt.Sprintf(`{
      "awsAuthConfig": {
        "clusterName": "%s",
        "roleARN": "%s"
      },
      "tlsClientConfig": {
        "insecure": false,
        "caData": "%s"
      }
    }
`, name, awsAuthConfigRoleARN, base64CA),
		},
	}

	return object
}

func newClientset() (*kubernetes.Clientset, error) {
	var kubeconfig string
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	var config *rest.Config

	if info, _ := os.Stat(kubeconfig); info == nil {
		var err error

		log.Printf("Using in-cluster Kubernetes API client")

		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, xerrors.Errorf("GetNodeSClient: %w", err)
		}
	} else {
		var err error

		log.Printf("Using kubeconfig-based Kubernetes API client")

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, xerrors.Errorf("GetNodesClient: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, xerrors.Errorf("new for config: %w", err)
	}

	return clientset, nil
}
