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

package ociregistory

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sts"
)

func init() {
	for _, r := range endpoints.AwsPartition().Regions() {
		targetRegions = append(targetRegions, r.ID())
	}
}

type AmazonECR struct {
	*imagePuller
	region string
}

var targetRegions []string

const amazonECRHostSubdomainTemplate = "%s.dkr.ecr.%s.amazonaws.com"

// NewAmazonECR returns multiple image providers for each given AWS region
func NewAmazonECR(sess *session.Session) ([]*AmazonECR, error) {
	// retrieve account id
	stsClient := sts.New(sess, aws.NewConfig().
		WithMaxRetries(3))
	accountInfo, err := stsClient.GetCallerIdentity(nil)
	if err != nil {
		return nil, fmt.Errorf("get-caller-identity failed: %w", err)
	}

	if accountInfo.Account == nil { // just in case
		return nil, errors.New("account id not found in get-caller-identity response")
	}

	// TODO: support cross account
	accountID := *accountInfo.Account
	ret := make([]*AmazonECR, 0, len(targetRegions))
	for _, region := range targetRegions {
		host := fmt.Sprintf(amazonECRHostSubdomainTemplate, accountID, region)
		ecrClient := ecr.New(sess, aws.NewConfig().
			WithRegion(region).
			WithMaxRetries(3))

		credentialProvider := func() (username, password string, err error) {
			res, err := ecrClient.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
			if err != nil {
				err = fmt.Errorf("GetAuthorizationToken failed: %w", err)
				return
			}

			if len(res.AuthorizationData) < 1 || res.AuthorizationData[0].AuthorizationToken == nil { // just in case
				err = fmt.Errorf("authorization data not found in GetAuthorizationToken: %w", err)
				return
			}

			raw, err := base64.StdEncoding.DecodeString(*res.AuthorizationData[0].AuthorizationToken)
			if err != nil {
				err = fmt.Errorf("error decoding credential: %w", err)
				return
			}

			up := strings.Split(string(raw), ":")
			if len(up) != 2 {
				// just in case
				err = fmt.Errorf("acquired authorization data in invalid format")
				return
			}

			username = up[0]
			password = up[1]
			return
		}

		ret = append(ret, &AmazonECR{
			imagePuller: newImagePuller(host, credentialProvider),
			region:      region,
		})
	}
	return ret, nil
}
