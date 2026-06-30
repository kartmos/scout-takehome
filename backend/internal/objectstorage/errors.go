package objectstorage

import (
	"errors"
	"fmt"
)

// Op identifies the storage operation that failed.
type Op string

const (
	OpPresignUpload   Op = "PresignUpload"
	OpPresignDownload Op = "PresignDownload"
	OpOpenOriginal    Op = "OpenOriginal"
	OpCheckBucket     Op = "CheckBucket"
)

// Category classifies the nature of a storage failure.
type Category int

const (
	CategoryInvalidInput      Category = iota + 1
	CategoryNotFound                   // object does not exist in the bucket
	CategoryBucketUnavailable          // configured bucket is missing or inaccessible
	CategoryInternal                   // upstream or transport failure
)

func (c Category) String() string {
	switch c {
	case CategoryInvalidInput:
		return "invalid input"
	case CategoryNotFound:
		return "not found"
	case CategoryBucketUnavailable:
		return "bucket unavailable"
	case CategoryInternal:
		return "internal error"
	default:
		return "unknown"
	}
}

// StorageError is a typed error from the object storage adapter.
// Error() never reveals credentials, signed URLs, or response bodies.
type StorageError struct {
	Op      Op
	Cat     Category
	safeMsg string
	cause   error
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("objectstorage %s: %s", e.Op, e.safeMsg)
}

func (e *StorageError) Unwrap() error { return e.cause }

// IsNotFound reports whether err represents a missing object.
func IsNotFound(err error) bool {
	var se *StorageError
	return errors.As(err, &se) && se.Cat == CategoryNotFound
}

// IsBucketUnavailable reports whether err represents an inaccessible bucket.
func IsBucketUnavailable(err error) bool {
	var se *StorageError
	return errors.As(err, &se) && se.Cat == CategoryBucketUnavailable
}

func newStorageError(op Op, cat Category, safeMsg string, cause error) *StorageError {
	return &StorageError{Op: op, Cat: cat, safeMsg: safeMsg, cause: cause}
}
