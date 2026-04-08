package repository

import (
	"strings"

	domainuser "omniflow-go/internal/domain/user"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

func toDomainUserModel(row *pgmodel.User) domainuser.User {
	if row == nil {
		return domainuser.User{}
	}

	nickname := strings.TrimSpace(derefString(row.Nickname))
	if nickname == "" {
		nickname = row.Username
	}

	status := domainuser.StatusPending
	switch row.Status {
	case int16(userStatusActive):
		status = domainuser.StatusActive
	case int16(userStatusDisabled):
		status = domainuser.StatusDisabled
	}

	return domainuser.User{
		ID:       uint64(row.ID),
		Username: row.Username,
		Nickname: nickname,
		Phone:    derefString(row.Phone),
		Email:    derefString(row.Email),
		Ext:      row.Ext,
		Status:   status,
	}
}

func nullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func derefString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return strings.TrimSpace(*ptr)
}
