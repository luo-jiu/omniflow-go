package repository

import domainlibrary "omniflow-go/internal/domain/library"
import pgmodel "omniflow-go/internal/repository/postgres/model"

func toDomainLibraryModel(row *pgmodel.Library) domainlibrary.Library {
	if row == nil {
		return domainlibrary.Library{}
	}

	return domainlibrary.Library{
		ID:      uint64(row.ID),
		UserID:  uint64(derefInt64(row.UserID)),
		Name:    row.Name,
		Starred: row.Starred,
	}
}

func nullableInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	v := value
	return &v
}

func derefInt64(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
