module github.com/mumoshu/argocd-clusterset

go 1.13

require (
	github.com/aws/aws-sdk-go v1.35.29
	github.com/go-logr/logr v0.2.1
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.4
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920 // indirect
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/yaml v1.2.0
)

replace (
github.com/go-logr/zapr v0.1.0 => github.com/go-logr/zapr v0.2.0
)