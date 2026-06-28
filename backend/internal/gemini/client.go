package gemini

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/WolcenOn/clasificador-documentos-ia/backend/internal/classifier"
)

const defaultModel = "gemini-1.5-flash"

type Client struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type ClassificationResult struct {
	Opciones []geminiOption `json:"opciones"`
}

type geminiOption struct {
	Nombre        string         `json:"nombre"`
	Confianza     float64        `json:"confianza"`
	Etiquetas     map[string]any `json:"etiquetas"`
	PalabrasClave []string       `json:"palabras_clave"`
	Justificacion string         `json:"justificacion"`
}

type generateContentRequest struct {
	Contents         []content          `json:"contents"`
	GenerationConfig generationConfig   `json:"generationConfig"`
}

type generationConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	ResponseMIMEType string  `json:"responseMimeType,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func New(apiKey, model string) *Client {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}
	return &Client{
		APIKey: strings.TrimSpace(apiKey),
		Model:  model,
		HTTPClient: &http.Client{
			Timeout: 35 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

func (c *Client) Classify(ctx context.Context, req classifier.Request, rules classifier.Response) (classifier.Response, error) {
	if !c.Enabled() {
		return classifier.Response{}, errors.New("GEMINI_API_KEY no configurada")
	}

	promptBytes, _ := json.MarshalIndent(buildPromptData(req, rules), "", "  ")
	prompt := buildPrompt(string(promptBytes))

	body := generateContentRequest{
		Contents: []content{
			{
				Role: "user",
				Parts: []part{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: generationConfig{
			Temperature:      0.2,
			ResponseMIMEType: "application/json",
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return classifier.Response{}, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.Model, c.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return classifier.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return classifier.Response{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return classifier.Response{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classifier.Response{}, fmt.Errorf("Gemini HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var genResp generateContentResponse
	if err := json.Unmarshal(respBody, &genResp); err != nil {
		return classifier.Response{}, fmt.Errorf("respuesta Gemini no parseable: %w", err)
	}
	if genResp.Error != nil {
		return classifier.Response{}, fmt.Errorf("Gemini error %s: %s", genResp.Error.Status, genResp.Error.Message)
	}
	if len(genResp.Candidates) == 0 || len(genResp.Candidates[0].Content.Parts) == 0 {
		return classifier.Response{}, errors.New("Gemini no devolvió texto")
	}

	text := strings.TrimSpace(genResp.Candidates[0].Content.Parts[0].Text)
	text = stripJSONFence(text)

	var parsed ClassificationResult
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return classifier.Response{}, fmt.Errorf("JSON Gemini inválido: %w; body=%s", err, text)
	}
	if len(parsed.Opciones) == 0 {
		return classifier.Response{}, errors.New("Gemini no devolvió opciones")
	}

	options := make([]classifier.Option, 0, len(parsed.Opciones))
	for i, opt := range parsed.Opciones {
		tags := normalizeTags(opt.Etiquetas, req.AllowedOptions)
		if len(tags) == 0 {
			continue
		}
		keywords := uniqueNonEmpty(opt.PalabrasClave)
		if tags["palabras_clave"] != "" {
			keywords = uniqueNonEmpty(append(keywords, splitKeywords(tags["palabras_clave"])...))
		}
		confidence := clamp(opt.Confianza)
		if confidence == 0 {
			confidence = 0.65
		}
		name := strings.TrimSpace(opt.Nombre)
		if name == "" {
			name = "Clasificación sugerida por Gemini"
		}
		options = append(options, classifier.Option{
			IDOpcion:      makeOptionID(req, tags, i),
			Nombre:        name,
			Confianza:     confidence,
			Etiquetas:     tags,
			PalabrasClave: keywords,
			Justificacion: strings.TrimSpace(opt.Justificacion),
		})
	}

	if len(options) == 0 {
		return classifier.Response{}, errors.New("Gemini devolvió etiquetas fuera de las opciones permitidas")
	}

	globalConfidence := options[0].Confianza
	for _, opt := range options {
		if opt.Confianza > globalConfidence {
			globalConfidence = opt.Confianza
		}
	}

	return classifier.Response{
		DriveFileID:        req.DriveFileID,
		NombreNormalizado: rules.NombreNormalizado,
		HashReferencia:    rules.HashReferencia,
		ConfianzaGlobal:   globalConfidence,
		Fuente:            "gemini",
		Opciones:          options,
	}, nil
}

func buildPromptData(req classifier.Request, rules classifier.Response) map[string]any {
	return map[string]any{
		"documento": map[string]any{
			"fileName":       req.FileName,
			"mimeType":       req.MIMEType,
			"driveFileId":    req.DriveFileID,
			"path":           req.Path,
			"sizeBytes":      req.SizeBytes,
			"md5Checksum":    req.MD5Checksum,
			"modifiedTime":   req.ModifiedTime,
			"contentHash":    req.ContentHash,
			"contentText":    truncate(req.ContentText, 12000),
		},
		"opciones_permitidas": req.AllowedOptions,
		"etiquetas_actuales":  req.CurrentTags,
		"sugerencia_reglas":   rules,
	}
}

func buildPrompt(dataJSON string) string {
	return `Eres un asistente experto en catalogar documentos educativos, terapéuticos y profesionales de Google Drive.

Tu tarea es proponer etiquetas estructuradas para el documento usando SOLO los campos y opciones permitidas.

Reglas obligatorias:
- Devuelve SOLO JSON válido. No añadas markdown ni explicaciones fuera del JSON.
- No inventes campos que no estén en opciones_permitidas, salvo palabras_clave si existe como campo de texto.
- Para campos con opciones permitidas, usa únicamente valores presentes en opciones_permitidas.
- Si dudas, omite el campo o usa una opción genérica como Otro solo si existe.
- palabras_clave debe ser texto corto separado por punto y coma dentro de etiquetas, y también array en palabras_clave.
- Devuelve entre 1 y 3 opciones.
- La confianza debe ser un número entre 0 y 1.

Formato exacto de salida:
{
  "opciones": [
    {
      "nombre": "Clasificación sugerida por Gemini",
      "confianza": 0.85,
      "etiquetas": {
        "tematica": "Lengua",
        "campo_aplicacion": "Escolar",
        "tipo_documento": "PDF",
        "idioma": "es",
        "estado": "Borrador",
        "palabras_clave": "lenguaje; actividad; vocabulario"
      },
      "palabras_clave": ["lenguaje", "actividad", "vocabulario"],
      "justificacion": "Motivo breve basado en nombre, ruta, metadatos o texto extraído."
    }
  ]
}

Datos de entrada:
` + dataJSON
}

func normalizeTags(raw map[string]any, allowed map[string][]string) map[string]string {
	out := map[string]string{}
	for field, value := range raw {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		parts := valueToParts(value)
		if len(parts) == 0 {
			continue
		}

		allowedValues, hasAllowed := allowed[field]
		if hasAllowed && len(allowedValues) > 0 {
			valid := []string{}
			for _, part := range parts {
				if canonical := canonicalOption(part, allowedValues); canonical != "" {
					valid = append(valid, canonical)
				}
			}
			valid = uniqueNonEmpty(valid)
			if len(valid) > 0 {
				out[field] = strings.Join(valid, "; ")
			}
			continue
		}

		// Campos sin lista de opciones, por ejemplo palabras_clave.
		clean := uniqueNonEmpty(parts)
		if len(clean) > 0 {
			out[field] = strings.Join(clean, "; ")
		}
	}
	return out
}

func valueToParts(value any) []string {
	switch v := value.(type) {
	case string:
		return splitKeywords(v)
	case []any:
		out := []string{}
		for _, item := range v {
			out = append(out, valueToParts(item)...)
		}
		return out
	case []string:
		return v
	default:
		if v == nil {
			return nil
		}
		return []string{fmt.Sprint(v)}
	}
}

func splitKeywords(text string) []string {
	text = strings.ReplaceAll(text, ",", ";")
	parts := strings.Split(text, ";")
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func canonicalOption(value string, options []string) string {
	value = strings.TrimSpace(value)
	for _, option := range options {
		option = strings.TrimSpace(option)
		if strings.EqualFold(value, option) {
			return option
		}
	}
	return ""
}

func makeOptionID(req classifier.Request, tags map[string]string, index int) string {
	pairs := []string{}
	for k, v := range tags {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	base := req.DriveFileID + "|" + req.FileName + "|" + req.ContentHash + "|" + strings.Join(pairs, "|") + fmt.Sprintf("|%d", index)
	sum := sha256.Sum256([]byte(base))
	return "gemini_" + hex.EncodeToString(sum[:])[:16]
}

func stripJSONFence(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}

func truncate(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max]
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
