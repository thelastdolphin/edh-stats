package dto

type CreateDeckRequest struct {
	Name  string `json:"name" validate:"required, min=1, max=50"`
	Owner string `json:"owner" validate:"required, min=1, max=50"`
	Type  string `json:"type" validate:"required, oneof=aggro combo control midrange"`
}
