package handlers

type ErrorDetail struct {
	Code  string `json:"code" example:"invalid_input"`
	Error string `json:"error" example:"invalid request body"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
