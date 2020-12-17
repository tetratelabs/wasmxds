package s3provider

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmazonS3_ProviderKey(t *testing.T) {
	assert.Equal(t, "s3", (&AmazonS3{}).ProviderKey())
}

func TestAmazonS3_Fetch(t *testing.T) {
	t.Run("invalid uri", func(t *testing.T) {
		_, err := (&AmazonS3{}).Fetch(nil, "a.wasm")
		assert.Error(t, err)
		t.Log(err)
	})
	t.Run("ok", func(t *testing.T) {

		sess, err := session.NewSession(aws.NewConfig().
			WithRegion("us-west-1").
			WithCredentials(credentials.NewStaticCredentials("dummy", "dummy", "")).
			WithS3ForcePathStyle(true).
			WithEndpoint("http://localhost:4566"))
		require.NoError(t, err)

		bucket := "fawefa-name"
		key := "to/the/wasm/binary.wasm"
		exp := []byte{1, 2, 3}
		{
			client := s3.New(sess)
			_, _ = client.CreateBucket(&s3.CreateBucketInput{
				Bucket: aws.String(bucket),
			})
		}
		{
			client := s3manager.NewUploader(sess)
			in := bytes.NewReader(exp)
			_, err = client.Upload(&s3manager.UploadInput{
				Body:   in,
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			require.NoError(t, err)
		}

		as, _ := NewAmazonS3(sess)
		actual, err := as.Fetch(nil, filepath.Join(bucket, key))
		require.NoError(t, err)
		assert.Equal(t, exp, actual)
	})
}
