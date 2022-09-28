package config

import (
	"net/url"
	"strings"
)

type S3 struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
}

func ParseS3URL(s3URL string) (S3, error) {
	u, err := url.Parse(s3URL)
	if err != nil {
		return S3{}, err
	}

	endpoint := u.Host
	accessKeyID := u.User.Username()
	secretAccessKey, _ := u.User.Password()
	bucketName := strings.TrimLeft(u.Path, "/")

	s3 := S3{
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		BucketName:      bucketName,
	}
	return s3, nil
}
