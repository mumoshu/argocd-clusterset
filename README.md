# argocd-clusterset

`argocd-clusterset` is a command-line tool and Kubernetes controller to sync EKS clusters into ArgoCD [cluster secrets]().

Consider this as a tool to add EKS cluster auto-discovery to ArgoCD.

Intended to be used with [ArgoCD's ApplicationSet controller](https://github.com/argoproj-labs/applicationset), as you have no other "official"ish way to automatically deploy to auto-discovered clusters.

It's also recommended to use ArgoCD 1.8+, as ArgoCD's scalability has considerably increased starting that version, to help support managing a lot of clusters and application that might be the case when you use this and ApplicationSets. Please read [their blog post](https://blog.argoproj.io/please-welcome-argo-cd-v1-8-rc-5799850cb2b6?source=collection_home---4------0-----------------------) introducing ArgoCD 1.8-rc.1 for more information on the scalability improvement.

## Usage

`argocd-clusterset` is pretty experimental, so there's no tagged releases yet.

You need to clone this repository, and follow the below steps to deploy this as a K8s controller:

```
$ NAME=$YOUR_DOCKER_USER/argocd-clusterset make docker-buildx

$ (cd config/default; kustomize edit set image controller=$YOUR_DOCKER_USER/argocd-clusterset:latest)

# 
# With kustomize:
#

$ kustomize build config/default | kubectl apply -f - --dry-run
$ kustomize build config/default | kubectl apply -f -

#
# With helm:
#

$ helm upgrade --install --name clusterset-controller charts/clusterset-controller

$ cat <<EOF | kubectl apply -f -
apiVersion: clusterset.mumo.co/v1alpha1
kind: ClusterSet
metadata:
  name: myclusterset1
spec:
  selector:
    roleARN: "arn:aws:iam::123456789012:role/read-eks-cluster-role" # optional
    eksTags:
      foo: "bar"
  template:
    metadata:
      labels:
        env: "prod"
      config:
        awsAuthConfig:
          roleARN: "arn:aws:iam::123456789012:role/argocd-auth-role" # optional
EOF
```

Or to use it as a command-line tool, run:

```shell script
$ make build

$ ./argocd-clusterset sync \
  --namespace ns-for-cluster-secrets \
  --eks-tags environment=production --eks-tags owner=yourteam
```
