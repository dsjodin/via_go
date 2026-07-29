package model

type APIError struct {
	ErrorStatus  int    `json:"error_status"`
	ErrorMessage string `json:"error_message"`
}
