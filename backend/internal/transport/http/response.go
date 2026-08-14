package http

import (
	"encoding/json"
	"net/http"
	"time"
)

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":      code,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
