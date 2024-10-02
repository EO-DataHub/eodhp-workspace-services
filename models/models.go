package models

type Workspace struct {
	Name               string
	Namespace          string
	ServiceAccountName string
	AWSRoleName        string
}

type AWSEFSAccessPoint struct {
	Name        string
	FSID        string
	RootDir     string
	UID         int
	GID         int
	Permissions string
}

type AWSS3Bucket struct {
	BucketName      string
	BucketPath      string
	AccessPointName string
	EnvVar          string
}

type PersistentVolume struct {
	PVName          string
	StorageClass    string
	Size            string
	Driver          string
	AccessPointName string
}

type PersistentVolumeClaim struct {
	PVCName      string
	StorageClass string
	Size         string
	PVName       string
}

type WorkspaceRequest struct {
	Name                   string                  `json:"name"`
	Namespace              string                  `json:"namespace"`
	ServiceAccountName     string                  `json:"serviceAccountName"`
	AWSRoleName            string                  `json:"awsRoleName"`
	EFSAccessPoint         []AWSEFSAccessPoint     `json:"efsAccessPoint"`
	S3Buckets              []AWSS3Bucket           `json:"s3Buckets"`
	PersistentVolumes      []PersistentVolume      `json:"persistentVolume"`
	PersistentVolumeClaims []PersistentVolumeClaim `json:"persistentVolumeClaim"`
}
