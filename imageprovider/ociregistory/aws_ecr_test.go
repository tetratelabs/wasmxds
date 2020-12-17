// +build ci_aws_test
// this test only runs in Github Actions since here we use actual ECR registries

package ociregistory

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/mathetake/gasm/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAmazonECR_Pull(t *testing.T) {
	sess, err := session.NewSession()
	if err != nil {
		t.Fatalf("failed to start AWS session: %v", err)
	}

	// retrieve account id
	stsClient := sts.New(sess, aws.NewConfig())
	accountInfo, err := stsClient.GetCallerIdentity(nil)
	if err != nil {
		t.Fatalf("get-caller-identity failed: %v", err)
	}

	providers, err := NewAmazonECR(sess)
	if err != nil {
		t.Fatal(err)
	}

	regionToProvider := make(map[string]*AmazonECR, len(providers))
	for _, p := range providers {
		regionToProvider[p.region] = p
	}
	for _, c := range []struct {
		region, uri string
	}{
		{region: "us-west-1", uri: "%s.dkr.ecr.us-west-1.amazonaws.com/wasmxds-ci-test:hello"},
		{region: "ap-northeast-1", uri: "%s.dkr.ecr.ap-northeast-1.amazonaws.com/wasmxds-ci-test:hello"},
		{region: "eu-central-1", uri: "%s.dkr.ecr.eu-central-1.amazonaws.com/wasmxds-ci-test:hello"},
	} {
		provider := regionToProvider[c.region]
		image, err := provider.Fetch(context.Background(), fmt.Sprintf(c.uri, *accountInfo.Account))
		require.NoError(t, err)
		require.Greater(t, len(image), 0)
		module, err := wasm.DecodeModule(bytes.NewReader(image))
		require.NoError(t, err)
		var proxyWasmExported bool
		for _, exp := range module.SecExports {
			t.Log(exp.Name)
			if strings.Contains(exp.Name, "proxy_") {
				proxyWasmExported = true
			}
		}
		assert.True(t, proxyWasmExported, "region: %s", c.region)
	}
}
