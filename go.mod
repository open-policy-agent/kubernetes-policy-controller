module github.com/open-policy-agent/gatekeeper

go 1.16

require (
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/asaskevich/govalidator v0.0.0-20200907205600-7a23bdc65eef // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0
	github.com/go-openapi/spec v0.20.3 // indirect
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.1.2
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/open-policy-agent/cert-controller v0.2.0
	github.com/open-policy-agent/frameworks/constraint v0.0.0-20210701194838-1dbe2618668d
	github.com/open-policy-agent/opa v0.29.4
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/cobra v1.1.3
	go.opencensus.io v0.22.4
	go.uber.org/zap v1.17.0
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.2
	k8s.io/apiextensions-apiserver v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/klog/v2 v2.8.0
	sigs.k8s.io/controller-runtime v0.9.2
	sigs.k8s.io/yaml v1.2.0
)
