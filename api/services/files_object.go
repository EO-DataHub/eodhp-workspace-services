package services

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	awsclient "github.com/EO-DataHub/eodhp-workspace-services/internal/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// listObjectStoreItems lists files from the selected object store.
func (svc *FileService) listObjectStoreItems(r *http.Request, stores []ws_manager.ObjectStore) ([]FileItem, int, error) {
	if len(stores) == 0 {
		return nil, http.StatusBadRequest, errors.New("no object store configured")
	}
	store, err := selectObjectStore(stores)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	if store.Bucket == "" || store.Prefix == "" {
		return nil, http.StatusBadRequest, errors.New("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	prefix, err := safeS3Prefix(store.Prefix, "")
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	items, err := listS3Objects(r.Context(), s3Client, store, prefix, svc.responseTimeFormat())
	if err != nil {
		return nil, httpStatusFromError(err, http.StatusInternalServerError), err
	}

	// Status 0 means there is no error status code to propagate.
	return items, 0, nil
}

// uploadObjectStoreFiles uploads multipart files to the object store.
func (svc *FileService) uploadObjectStoreFiles(r *http.Request, store ws_manager.ObjectStore, files []*multipart.FileHeader) ([]FileItem, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return nil, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return nil, err
	}

	var items []FileItem
	for _, fh := range files {
		if fh == nil || fh.Filename == "" {
			continue
		}
		if err := validateFileName(fh.Filename); err != nil {
			return nil, err
		}

		src, err := fh.Open()
		if err != nil {
			return nil, err
		}

		key, err := safeS3Key(store.Prefix, fh.Filename)
		if err != nil {
			_ = src.Close()
			return nil, err
		}

		_, err = s3Client.PutObject(r.Context(), &s3.PutObjectInput{
			Bucket:      aws.String(store.Bucket),
			Key:         aws.String(key),
			Body:        src,
			ContentType: aws.String(fh.Header.Get("Content-Type")),
		})
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}

		items = append(items, FileItem{
			StoreType: storeTypeObject,
			FileName:  relativeS3Path(store.Prefix, key),
			Size:      fh.Size,
		})
	}

	return items, nil
}

// deleteObjectStoreFiles deletes object store files and reports per-file failures.
func (svc *FileService) deleteObjectStoreFiles(r *http.Request, store ws_manager.ObjectStore, paths []string) ([]string, []FileFail, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return nil, nil, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return nil, nil, err
	}

	var deleted []string
	var failed []FileFail
	for _, p := range paths {
		key, err := safeS3Key(store.Prefix, p)
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}

		_, err = s3Client.DeleteObject(r.Context(), &s3.DeleteObjectInput{
			Bucket: aws.String(store.Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		deleted = append(deleted, p)
	}

	return deleted, failed, nil
}

// getObjectStoreMetadata fetches metadata for a single object store file.
func (svc *FileService) getObjectStoreMetadata(r *http.Request, store ws_manager.ObjectStore, pathParam string) (FileItem, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return FileItem{}, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return FileItem{}, err
	}

	key, err := safeS3Key(store.Prefix, pathParam)
	if err != nil {
		return FileItem{}, err
	}

	resp, err := s3Client.HeadObject(r.Context(), &s3.HeadObjectInput{
		Bucket: aws.String(store.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return FileItem{}, err
	}

	item := FileItem{
		StoreType: storeTypeObject,
		FileName:  relativeS3Path(store.Prefix, key),
		Size:      aws.ToInt64(resp.ContentLength),
	}
	if resp.LastModified != nil {
		item.LastModified = resp.LastModified.UTC().Format(svc.responseTimeFormat())
	}
	if resp.ETag != nil {
		item.ETag = strings.Trim(*resp.ETag, "\"")
	}
	return item, nil
}

// newS3Client creates an S3 client using credentials resolved from the incoming request.
func (svc *FileService) newS3Client(r *http.Request) (*s3.Client, error) {
	creds, err := svc.getS3Credentials(r)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadDefaultConfig(r.Context(),
		config.WithRegion(svc.Config.AWS.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyId,
			creds.SecretAccessKey,
			creds.SessionToken,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure S3 client: %w", err)
	}
	return awsclient.NewS3ClientWithEndpoint(cfg, svc.Config.AWS.S3.Endpoint, svc.Config.AWS.S3.ForcePathStyle), nil
}

// getS3Credentials resolves either local static credentials or STS web identity credentials.
func (svc *FileService) getS3Credentials(r *http.Request) (awsclient.S3Credentials, error) {
	// Local/dev override: use static S3 keys when provided instead of STS.
	if svc.Config.AWS.S3.AccessKey != "" && svc.Config.AWS.S3.SecretKey != "" {
		return awsclient.S3Credentials{
			AccessKeyId:     svc.Config.AWS.S3.AccessKey,
			SecretAccessKey: svc.Config.AWS.S3.SecretKey,
			SessionToken:    "",
			Expiration:      "",
		}, nil
	}

	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return awsclient.S3Credentials{}, fmt.Errorf("authorization header missing")
	}

	roleARN := strings.TrimSpace(svc.Config.AWS.S3.RoleArn)
	if roleARN == "" {
		return awsclient.S3Credentials{}, fmt.Errorf("missing AWS role ARN for S3 credentials")
	}

	if svc.STS == nil {
		return awsclient.S3Credentials{}, fmt.Errorf("sts client not configured")
	}
	out, err := svc.STS.AssumeRoleWithWebIdentity(r.Context(), &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(roleARN),
		RoleSessionName:  aws.String("workspace-services"),
		WebIdentityToken: aws.String(token),
	})
	if err != nil {
		return awsclient.S3Credentials{}, err
	}
	if out.Credentials == nil {
		return awsclient.S3Credentials{}, fmt.Errorf("missing credentials from STS response")
	}
	resp := out

	if resp.Credentials.AccessKeyId == nil || resp.Credentials.SecretAccessKey == nil || resp.Credentials.SessionToken == nil || resp.Credentials.Expiration == nil {
		return awsclient.S3Credentials{}, fmt.Errorf("invalid credentials returned by STS")
	}

	return awsclient.S3Credentials{
		AccessKeyId:     *resp.Credentials.AccessKeyId,
		SecretAccessKey: *resp.Credentials.SecretAccessKey,
		SessionToken:    *resp.Credentials.SessionToken,
		Expiration:      resp.Credentials.Expiration.UTC().Format(svc.responseTimeFormat()),
	}, nil
}

// httpStatusFromError extracts an HTTP status from downstream errors, or returns fallback.
func httpStatusFromError(err error, fallback int) int {
	var statusCoder interface{ HTTPStatusCode() int }
	if errors.As(err, &statusCoder) {
		code := statusCoder.HTTPStatusCode()
		if code >= 400 && code <= 599 {
			return code
		}
	}
	return fallback
}
