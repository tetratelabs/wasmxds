// Copyright Istio Authors
// Copyright 2020 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Some of the code here is extracted from
// https://github.com/istio/proxy/blob/85a0d22426f71369e6db75558adc2c7ae50bda05/tools/extensionserver/main/main.go

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	discoveryservice "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	extensionservice "github.com/envoyproxy/go-control-plane/envoy/service/extension/v3"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	wasmxdsv1alpha1 "github.com/tetratelabs/wasmxds/api/v1alpha1"
	"github.com/tetratelabs/wasmxds/controllers"
	"github.com/tetratelabs/wasmxds/imageprovider"
	"github.com/tetratelabs/wasmxds/imageprovider/httpprovider"
	"github.com/tetratelabs/wasmxds/imageprovider/localfs"
	"github.com/tetratelabs/wasmxds/imageprovider/ociregistory"
	"github.com/tetratelabs/wasmxds/imageprovider/s3provider"
	"github.com/tetratelabs/wasmxds/wasmxds"
)

var (
	scheme                                               = runtime.NewScheme()
	setupLog                                             = ctrl.Log.WithName("setup")
	watchNamespace                                       string
	enableAmazonECR, enableAmazonS3, enableAmazonS3Local bool
	allowInsecureHttps                                   bool
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(wasmxdsv1alpha1.AddToScheme(scheme))
	flag.StringVar(&watchNamespace, "n", "", "namespace for watching. The controller watches all namespaces by default")
	flag.BoolVar(&enableAmazonECR, "ecr", false, "Enable Amazon ECR provider. Disabled by default")
	flag.BoolVar(&enableAmazonS3, "s3", false, "Enable Amazon S3 provider. Disabled by default")

	// flags only for e2e
	flag.BoolVar(&enableAmazonS3Local, "s3-local", false, "For e2e only")
	flag.BoolVar(&allowInsecureHttps, "insecure-https", false, "For e2e only")
}

const (
	grpcMaxConcurrentStreams = 100000
	serverBindAddress        = ":8610"
)

func main() {
	flag.Parse()
	ctrl.SetLogger(zap.New())

	setupLog.Info("given flags",
		"-n", watchNamespace,
		"-ecr", enableAmazonECR,
		"-s3", enableAmazonS3,
	)

	grpcServer := grpc.NewServer(grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	lis, err := net.Listen("tcp", serverBindAddress)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: make image providers configurable
	providers := []imageprovider.WasmImageProvider{
		httpprovider.NewHttpProvider(),
		httpprovider.NewHttpsProvider(allowInsecureHttps),
		ociregistory.NewWebAssemblyHub("", ""),
		ociregistory.NewLocalRegistry("", "", "5000"),
		localfs.LocalFilesystem{},
	}

	if enableAmazonECR || enableAmazonS3 || enableAmazonS3Local {
		sess, err := session.NewSession()
		if err != nil {
			log.Fatal(err)
		}

		if enableAmazonECR {
			awsProviders, err := ociregistory.NewAmazonECR(sess)
			if err != nil {
				log.Fatal(err)
			}
			for _, p := range awsProviders {
				providers = append(providers, p)
			}
			setupLog.Info("Amazon ECR providers configured")
		}

		if enableAmazonS3 || enableAmazonS3Local {
			if enableAmazonS3Local {
				sess, err = session.NewSession(aws.NewConfig().
					WithRegion("us-west-1").
					WithCredentials(credentials.NewStaticCredentials("dummy", "dummy", "")).
					WithS3ForcePathStyle(true).
					WithEndpoint("http://localhost:4566"))
				if err != nil {
					log.Fatal(err)
				}
			}

			s3Provider, err := s3provider.NewAmazonS3(sess)
			if err != nil {
				log.Fatal(err)
			}
			providers = append(providers, s3Provider)
			setupLog.Info("Amazon s3 provider configured")
		}
	}

	server, err := wasmxds.NewServer(context.Background(), providers...)
	if err != nil {
		log.Fatalf("failed to create wasmxds server: %v", err)
	}
	discoveryservice.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	extensionservice.RegisterExtensionConfigDiscoveryServiceServer(grpcServer, server)
	runController(server)

	go func() {
		setupLog.Info("starting grpc server")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	defer func() {
		setupLog.Info("stopping grpc server")
		grpcServer.GracefulStop()
	}()

	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR2)
	<-gracefulStop
}

func runController(handler wasmxds.EventHandler) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: ":0", // disabled
		Namespace:          watchNamespace,
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	c := &controllers.WasmExtensionReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("WasmExtension"),
		Scheme: mgr.GetScheme(),
	}

	// pass handler to k8s controller to relay the CRUD event to xDS server
	c.SetEventHandler(handler)

	if err = c.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WasmExtension")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	go func() {
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()
}
