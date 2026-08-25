package dto

import (
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// S3DataClass is the S3 storage class for object data.
// @Description S3DataClass is the S3 storage class for object data.
type S3DataClass string

const (
	S3DataClassStandard           S3DataClass = "STANDARD"
	S3DataClassGlacier            S3DataClass = "GLACIER"
	S3DataClassStandardIa         S3DataClass = "STANDARD_IA"
	S3DataClassOnezoneIa          S3DataClass = "ONEZONE_IA"
	S3DataClassIntelligentTiering S3DataClass = "INTELLIGENT_TIERING"
	S3DataClassDeepArchive        S3DataClass = "DEEP_ARCHIVE"
	S3DataClassOutposts           S3DataClass = "OUTPOSTS"
	S3DataClassGlacierIr          S3DataClass = "GLACIER_IR"
	S3DataClassSnow               S3DataClass = "SNOW"
	S3DataClassExpressOnezone     S3DataClass = "EXPRESS_ONEZONE"
)

var validS3DataClasses = []S3DataClass{
	"",
	S3DataClassStandard,
	S3DataClassGlacier,
	S3DataClassStandardIa,
	S3DataClassOnezoneIa,
	S3DataClassIntelligentTiering,
	S3DataClassDeepArchive,
	S3DataClassOutposts,
	S3DataClassGlacierIr,
	S3DataClassSnow,
	S3DataClassExpressOnezone,
}

// Validate checks that the S3 data class is supported.
func (c S3DataClass) Validate() error {
	if slices.Contains(validS3DataClasses, c) {
		return nil
	}
	return errValidationInvalidValue("data", c, validS3DataClasses)
}

// S3MetadataClass is the S3 storage class for metadata.
// @Description S3MetadataClass is the S3 storage class for metadata.
type S3MetadataClass string

const (
	S3MetadataClassStandard           S3MetadataClass = "STANDARD"
	S3MetadataClassStandardIa         S3MetadataClass = "STANDARD_IA"
	S3MetadataClassIntelligentTiering S3MetadataClass = "INTELLIGENT_TIERING"
	S3MetadataClassExpressOnezone     S3MetadataClass = "EXPRESS_ONEZONE"
	S3MetadataClassOnezoneIa          S3MetadataClass = "ONEZONE_IA"
	S3MetadataClassOutposts           S3MetadataClass = "OUTPOSTS"
)

var validS3MetadataClasses = []S3MetadataClass{
	"",
	S3MetadataClassStandard,
	S3MetadataClassStandardIa,
	S3MetadataClassIntelligentTiering,
	S3MetadataClassExpressOnezone,
	S3MetadataClassOnezoneIa,
	S3MetadataClassOutposts,
}

// Validate checks that the S3 metadata class is supported.
func (c S3MetadataClass) Validate() error {
	if slices.Contains(validS3MetadataClasses, c) {
		return nil
	}
	return errValidationInvalidValue("metadata", c, validS3MetadataClasses)
}

// GcpDataClass is the GCP storage class for object data.
// @Description GcpDataClass is the GCP storage class for object data.
type GcpDataClass string

const (
	GcpDataClassStandard GcpDataClass = "STANDARD"
	GcpDataClassNearline GcpDataClass = "NEARLINE"
	GcpDataClassColdline GcpDataClass = "COLDLINE"
	GcpDataClassArchive  GcpDataClass = "ARCHIVE"
)

var validGcpDataClasses = []GcpDataClass{
	"",
	GcpDataClassStandard,
	GcpDataClassNearline,
	GcpDataClassColdline,
	GcpDataClassArchive,
}

// Validate checks that the GCP data class is supported.
func (c GcpDataClass) Validate() error {
	if slices.Contains(validGcpDataClasses, c) {
		return nil
	}
	return errValidationInvalidValue("data", c, validGcpDataClasses)
}

// AzureDataClass is the Azure storage tier for object data.
// @Description AzureDataClass is the Azure storage tier for object data.
type AzureDataClass string

const (
	AzureDataClassHot     AzureDataClass = "Hot"
	AzureDataClassCool    AzureDataClass = "Cool"
	AzureDataClassCold    AzureDataClass = "Cold"
	AzureDataClassArchive AzureDataClass = "Archive"
)

var validAzureDataClasses = []AzureDataClass{
	"",
	AzureDataClassHot,
	AzureDataClassCool,
	AzureDataClassCold,
	AzureDataClassArchive,
}

// Validate checks that the Azure data tier is supported.
func (c AzureDataClass) Validate() error {
	if slices.Contains(validAzureDataClasses, c) {
		return nil
	}
	return errValidationInvalidValue("data", c, validAzureDataClasses)
}

// AzureMetadataClass is the Azure storage tier for metadata.
// @Description AzureMetadataClass is the Azure storage tier for metadata.
type AzureMetadataClass string

const (
	AzureMetadataClassHot  AzureMetadataClass = "Hot"
	AzureMetadataClassCool AzureMetadataClass = "Cool"
	AzureMetadataClassCold AzureMetadataClass = "Cold"
)

var validAzureMetadataClasses = []AzureMetadataClass{
	"",
	AzureMetadataClassHot,
	AzureMetadataClassCool,
	AzureMetadataClassCold,
}

// Validate checks that the Azure metadata tier is supported.
func (c AzureMetadataClass) Validate() error {
	if slices.Contains(validAzureMetadataClasses, c) {
		return nil
	}
	return errValidationInvalidValue("metadata", c, validAzureMetadataClasses)
}

func s3DataClassToString(c S3DataClass) string {
	return string(c)
}

func s3MetadataClassToString(c S3MetadataClass) string {
	return string(c)
}

func newS3DataClassFromString(s string) S3DataClass {
	return S3DataClass(s)
}

func newS3MetadataClassFromString(s string) S3MetadataClass {
	return S3MetadataClass(s)
}

func gcpDataClassToString(c GcpDataClass) string {
	return string(c)
}

func newGcpDataClassFromString(s string) GcpDataClass {
	return GcpDataClass(s)
}

func azureDataClassToString(c AzureDataClass) string {
	return string(c)
}

func azureMetadataClassToString(c AzureMetadataClass) string {
	return string(c)
}

func newAzureDataClassFromString(s string) AzureDataClass {
	return AzureDataClass(s)
}

func newAzureMetadataClassFromString(s string) AzureMetadataClass {
	return AzureMetadataClass(s)
}

func storageClassFromS3(s *S3StorageClass) *model.StorageClass {
	if s == nil {
		return nil
	}
	return &model.StorageClass{
		DataClass:     s3DataClassToString(s.DataClass),
		MetadataClass: s3MetadataClassToString(s.MetadataClass),
	}
}

func storageClassFromGcp(s *GcpStorageClass) *model.StorageClass {
	if s == nil {
		return nil
	}
	return &model.StorageClass{
		DataClass:     gcpDataClassToString(s.DataClass),
		MetadataClass: "",
	}
}

func storageClassFromAzure(s *AzureStorageClass) *model.StorageClass {
	if s == nil {
		return nil
	}
	return &model.StorageClass{
		DataClass:     azureDataClassToString(s.DataClass),
		MetadataClass: azureMetadataClassToString(s.MetadataClass),
	}
}
