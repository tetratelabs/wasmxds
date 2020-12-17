package s3provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type AmazonS3 struct {
	client *s3manager.Downloader
}

func NewAmazonS3(sess *session.Session) (*AmazonS3, error) {
	client := s3manager.NewDownloader(sess)
	return &AmazonS3{client: client}, nil
}

func (a *AmazonS3) Fetch(_ context.Context, uri string) ([]byte, error) {
	u := strings.SplitN(uri, "/", 2)
	if len(u) != 2 {
		return nil, fmt.Errorf("specified uri is malformed for "+
			"s3: uri must be in '<s3_bucket_name>/path/to/wasm/binary' but got %s", uri)
	}

	buf := aws.NewWriteAtBuffer(nil)
	_, err := a.client.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(u[0]),
		Key:    aws.String(u[1]),
	})
	if err != nil {
		return nil, fmt.Errorf("error downloading from s3: %v", err)
	}
	return buf.Bytes(), nil
}

func (*AmazonS3) ProviderKey() string {
	return "s3"
}
