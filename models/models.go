package models

import "github.com/google/uuid"

type ReqAWSEFSAccessPoint struct {
	Name string
}

type ReqAWSS3Bucket struct {
	BucketName string
}

type ReqStores struct {
	Object []ReqAWSS3Bucket
	Block  []ReqAWSEFSAccessPoint
}

type ReqMessagePayload struct {
	Status        string    `json:"status"`
	CorrelationId string    `json:"correlationId"`
	Name          string    `json:"name"`
	Account       uuid.UUID `json:"account"`
	AccountOwner  string    `json:"accountOwner"`
	MemberGroup   string    `json:"memberGroup"`
	Timestamp     int64     `json:"timestamp"`
	Stores        ReqStores `json:"stores"`
}

type AckPayload struct {
	MessagePayload ReqMessagePayload `json:"messagePayload"`
	AWS            AckAWSStatus      `json:"aws"`
}

type AckAWSStatus struct {
	Role AckAWSRoleStatus `json:"role,omitempty"`
	EFS  AckEFSStatus     `json:"efs,omitempty"`
	S3   AckS3Status      `json:"s3,omitempty"`
}

type AckAWSRoleStatus struct {
	Name string `json:"name,omitempty"`
	ARN  string `json:"arn,omitempty"`
}

type AckEFSStatus struct {
	AccessPoints []AckEFSAccessStatus `json:"accessPoints,omitempty"`
}

type AckEFSAccessStatus struct {
	Name          string `json:"name,omitempty"`
	AccessPointID string `json:"accessPointID,omitempty"`
	FSID          string `json:"fsID,omitempty"`
}

type AckS3Status struct {
	Buckets []AckS3BucketStatus `json:"buckets,omitempty"`
}

type AckS3BucketStatus struct {
	Name           string `json:"name,omitempty"`
	AccessPointARN string `json:"accessPointARN,omitempty"`
	RolePolicy     string `json:"rolePolicy,omitempty"`
	Path           string `json:"path,omitempty"`
	EnvVar         string `json:"envVar,omitempty"`
}
