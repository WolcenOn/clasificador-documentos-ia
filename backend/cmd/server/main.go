package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/WolcenOn/clasificador-documentos-ia/backend/internal/classifier"
)

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /api/classify", handleClassify)

	wrapped := withCORS(withAuth(mux))

	log.Printf("backend escuchando en puerto %s", port)
	if err := http.ListenAndServe(":"+port, wrapped); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "clasificador-documentos-ia",
	})
}

func handleClassify(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req classifier.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "JSON inválido"})
		return
	}

	if strings.TrimSpace(req.FileName) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "fileName es obligatorio"})
		return
	}

	result := classifier.Classify(req)
	writeJSON(w, http.StatusOK, result)
}

func withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		expectedToken := os.Getenv("APP_TOKEN")
		if expectedToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		providedToken := strings.TrimPrefix(authHeader, "Bearer ")

		if providedToken != expectedToken {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "token no autorizado"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
