package repository

import (
	"omniflow-go/internal/config"
	objectrepo "omniflow-go/internal/repository/object"
	"omniflow-go/internal/storage"
)

func NewObjectStorage(cfg *config.Config) (storage.ObjectStorage, func(), error) {
	return objectrepo.NewObjectStorage(cfg)
}
