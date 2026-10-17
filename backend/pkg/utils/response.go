package utils

import (
	"encoding/json"
	"net/http"
)

type successEnvelope struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
}

type errorEnvelope struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		LogError("failed to write response", "err", err)
	}
}

// SuccessResponse writes {"status": "success", "data": ...}.
func SuccessResponse(w http.ResponseWriter, status int, data interface{}) {
	writeJSON(w, status, successEnvelope{Status: "success", Data: data})
}

// ErrorResponse writes {"status": "error", "message": "...", "details": ...}.
func ErrorResponse(w http.ResponseWriter, status int, message string, details interface{}) {
	writeJSON(w, status, errorEnvelope{Status: "error", Message: message, Details: details})
}
