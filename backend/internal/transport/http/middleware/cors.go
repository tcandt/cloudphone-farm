package middleware

import "github.com/go-chi/cors"

func GetCorsOptions(allowedOrigins []string) cors.Options {
	return cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID", "X-Agent-Fingerprint", "X-Agent-ID", "X-Agent-Timestamp", "X-Agent-Nonce", "X-Agent-Signature"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}
}
