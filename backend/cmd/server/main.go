package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/WolcenOn/clasificador-documentos-ia/backend/internal/classifier"
	"github.com/WolcenOn/clasificador-documentos-ia/backend/internal/gemini"
)

type errorResponse struct {
	Error string `json:"error"`
}

type confirmRequest struct {
	DriveFileID  string         `json:"driveFileId"`
	FileName     string         `json:"fileName"`
	MIMEType     string         `json:"mimeType"`
	Path         string         `json:"path"`
	Tags         map[string]any `json:"tags"`
	Source       string         `json:"source"`
	ConfirmedAt  string         `json:"confirmedAt,omitempty"`
}

type confirmResponse struct {
	Status      string         `json:"status"`
	Source      string         `json:"source"`
	DriveFileID string         `json:"driveFileId,omitempty"`
	FileName    string         `json:"fileName,omitempty"`
	Tags        map[string]any `json:"tags"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/debug/gemini", handleGeminiDebug)
	mux.HandleFunc("GET /api/debug/gemini-test", handleGeminiTest)
	mux.HandleFunc("POST /api/classify", handleClassify)
	mux.HandleFunc("POST /api/confirm", handleConfirm)

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

func handleGeminiDebug(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	model := geminiModel()

	writeJSON(w, http.StatusOK, map[string]any{
		"status":              "ok",
		"gemini_mode":         geminiMode(),
		"gemini_model":        model,
		"gemini_api_key_set":  apiKey != "",
		"gemini_api_key_chars": len(apiKey),
		"app_token_set":       strings.TrimSpace(os.Getenv("APP_TOKEN")) != "",
	})
}

func handleGeminiTest(w http.ResponseWriter, r *http.Request) {
	req := classifier.Request{
		FileName:    "prueba disfemia registro.pdf",
		MIMEType:    "application/pdf",
		DriveFileID: "debug_gemini_test",
		Path:        "RECURSOS / EVALUACION / DISFEMIA",
		AllowedOptions: map[string][]string{
			"tematica":          {"Lengua", "Psicología", "Matemáticas", "Otro"},
			"edad_objetivo":     {"3-5", "6-8", "9-12", "13-15", "Adultos"},
			"nivel":             {"Básico", "Intermedio", "Avanzado"},
			"campo_aplicacion":  {"Escolar", "Universitario", "Otro"},
			"tipo_documento":    {"PDF", "Imagen", "Google Docs", "Otro"},
			"idioma":            {"es", "en", "Otro"},
			"estado":            {"Borrador", "Final"},
			"permisos":          {"Propio", "Compartido (ver)", "Compartido (editar)"},
		},
		CurrentTags: map[string]any{},
	}

	rulesResult := classifier.Classify(req)
	client := gemini.New(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL"))
	if !client.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "error",
			"error":  "GEMINI_API_KEY no configurada",
		})
		return
	}

	geminiResult, err := client.Classify(r.Context(), req, rulesResult)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "error",
			"gemini_mode":  geminiMode(),
			"gemini_model": geminiModel(),
			"error":        err.Error(),
			"fallback":     rulesResult,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"gemini_mode":  geminiMode(),
		"gemini_model": geminiModel(),
		"result":       geminiResult,
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

	rulesResult := classifier.Classify(req)

	mode := geminiMode()
	if !shouldUseGemini(mode, rulesResult) {
		writeJSON(w, http.StatusOK, rulesResult)
		return
	}

	client := gemini.New(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL"))
	if !client.Enabled() {
		log.Printf("Gemini solicitado con GEMINI_MODE=%s, pero falta GEMINI_API_KEY; se devuelven reglas", mode)
		writeJSON(w, http.StatusOK, rulesResult)
		return
	}

	geminiResult, err := client.Classify(r.Context(), req, rulesResult)
	if err != nil {
		log.Printf("error Gemini; fallback reglas: %v", err)
		writeJSON(w, http.StatusOK, rulesResult)
		return
	}

	writeJSON(w, http.StatusOK, geminiResult)
}

func handleConfirm(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "JSON inválido"})
		return
	}

	if strings.TrimSpace(req.FileName) == "" && strings.TrimSpace(req.DriveFileID) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "fileName o driveFileId es obligatorio"})
		return
	}

	if len(req.Tags) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "tags es obligatorio"})
		return
	}

	if strings.TrimSpace(req.Source) == "" {
		req.Source = "usuario_validado"
	}
	if strings.TrimSpace(req.ConfirmedAt) == "" {
		req.ConfirmedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Siguiente fase: persistir esto en PostgreSQL.
	// Por ahora confirmamos que el endpoint recibe y valida correctamente.
	log.Printf("clasificación confirmada: file=%s id=%s source=%s tags=%v", req.FileName, req.DriveFileID, req.Source, req.Tags)

	writeJSON(w, http.StatusOK, confirmResponse{
		Status:      "ok",
		Source:      req.Source,
		DriveFileID: req.DriveFileID,
		FileName:    req.FileName,
		Tags:        req.Tags,
	})
}

func geminiMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GEMINI_MODE")))
	if mode == "" {
		return "off"
	}
	return mode
}

func geminiModel() string {
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		return "gemini-1.5-flash"
	}
	return model
}

func shouldUseGemini(mode string, rulesResult classifier.Response) bool {
	switch mode {
	case "always":
		return true
	case "low_confidence":
		return rulesResult.ConfianzaGlobal < 0.80
	case "missing_only":
		// Cuando añadamos PostgreSQL, aquí se usará: no encontrado en base de datos → Gemini.
		return true
	case "off":
		return false
	default:
		return false
	}
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
