package usecase

type TagUseCase struct {
	searchType string
}

func NewTagUseCase() *TagUseCase {
	return &TagUseCase{searchType: "PostgreSQL"}
}

func (u *TagUseCase) SearchType() string {
	if u == nil || u.searchType == "" {
		return "PostgreSQL"
	}
	return u.searchType
}
