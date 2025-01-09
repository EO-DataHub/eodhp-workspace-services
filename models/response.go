package models

// Response represents a generic API response structure.
type Response struct {
	Success      int         `json:"success"`
	ErrorCode    string      `json:"error_code,omitempty"`
	ErrorDetails string      `json:"error_details,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}
