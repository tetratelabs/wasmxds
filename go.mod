module github.com/tetratelabs/wasmxds

go 1.15

require (
	github.com/aws/aws-sdk-go v1.35.25
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869
	github.com/containerd/containerd v1.3.2
	github.com/deislabs/oras v0.8.1
	github.com/envoyproxy/go-control-plane v0.9.7
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.4.2
	github.com/mathetake/gasm v0.0.0-20200928142744-80e74517647c
	github.com/opencontainers/image-spec v1.0.1
	github.com/prometheus/common v0.9.1 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.6.1
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/sys v0.0.0-20200331124033-c3d80250170d // indirect
	google.golang.org/grpc v1.32.0
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
)

replace github.com/tetratelabs/wasmxds => ./
