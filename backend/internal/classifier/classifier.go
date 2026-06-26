package classifier

import (
	"crypto/sha256"
	"encoding/hex"
	"mime"
	"path/filepath"
	"sort"
	"strings"

	"github.com/WolcenOn/clasificador-documentos-ia/backend/internal/normalizer"
)

type Request struct {
	FileName       string              `json:"fileName"`
	MIMEType       string              `json:"mimeType"`
	DriveFileID    string              `json:"driveFileId"`
	Path           string              `json:"path"`
	AllowedOptions map[string][]string `json:"allowedOptions"`
	CurrentTags    map[string]string   `json:"currentTags,omitempty"`
}

type Response struct {
	DriveFileID        string   `json:"driveFileId,omitempty"`
	NombreNormalizado string   `json:"nombre_normalizado"`
	HashReferencia    string   `json:"hash_referencia"`
	ConfianzaGlobal   float64  `json:"confianza_global"`
	Fuente            string   `json:"fuente"`
	Opciones          []Option `json:"opciones"`
}

type Option struct {
	IDOpcion      string            `json:"id_opcion"`
	Nombre        string            `json:"nombre"`
	Confianza     float64           `json:"confianza"`
	Etiquetas     map[string]string `json:"etiquetas"`
	PalabrasClave []string          `json:"palabras_clave"`
	Justificacion string            `json:"justificacion"`
}

func Classify(req Request) Response {
	normalizedName := normalizer.Filename(req.FileName)
	searchText := strings.ToLower(strings.TrimSpace(normalizedName + " " + req.Path + " " + req.FileName))

	tags := map[string]string{}
	keywords := []string{}
	reasons := []string{}
	score := 0.35

	if normalizedName != "" {
		score += 0.15
		keywords = append(keywords, strings.Fields(normalizedName)...)
		reasons = append(reasons, "se ha podido usar el nombre normalizado del archivo")
	}

	inferDocumentType(req, tags)
	if tags["tipo_documento"] != "" {
		score += 0.10
		reasons = append(reasons, "el tipo de documento se ha inferido por MIME type o extensión")
	}

	applyKeywordRules(searchText, tags, &keywords, &reasons, &score)
	limitToAllowedOptions(tags, req.AllowedOptions)

	if len(tags) == 0 {
		tags["estado"] = firstAllowed(req.AllowedOptions, "estado", "Pendiente")
		reasons = append(reasons, "no se han encontrado reglas suficientes; conviene revisar con IA")
		score = 0.25
	}

	keywords = uniqueNonEmpty(keywords)
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	confidence := clamp(score)
	optionID := makeOptionID(req, tags, normalizedName)

	return Response{
		DriveFileID:        req.DriveFileID,
		NombreNormalizado: normalizedName,
		HashReferencia:    makeReferenceHash(req),
		ConfianzaGlobal:   confidence,
		Fuente:            "reglas",
		Opciones: []Option{
			{
				IDOpcion:      optionID,
				Nombre:        "Clasificación inicial por reglas",
				Confianza:     confidence,
				Etiquetas:     tags,
				PalabrasClave: keywords,
				Justificacion: strings.Join(reasons, "; "),
			},
		},
	}
}

func inferDocumentType(req Request, tags map[string]string) {
	mimeType := strings.ToLower(req.MIMEType)
	ext := strings.ToLower(filepath.Ext(req.FileName))

	if mimeType == "" && ext != "" {
		mimeType = mime.TypeByExtension(ext)
	}

	switch {
	case strings.Contains(mimeType, "pdf") || ext == ".pdf":
		tags["tipo_documento"] = "PDF"
	case strings.Contains(mimeType, "image") || ext == ".png" || ext == ".jpg" || ext == ".jpeg":
		tags["tipo_documento"] = "Imagen"
	case strings.Contains(mimeType, "presentation") || ext == ".ppt" || ext == ".pptx":
		tags["tipo_documento"] = "Presentación"
	case strings.Contains(mimeType, "spreadsheet") || ext == ".xls" || ext == ".xlsx":
		tags["tipo_documento"] = "Hoja de cálculo"
	case strings.Contains(mimeType, "document") || ext == ".doc" || ext == ".docx":
		tags["tipo_documento"] = "Documento"
	}
}

func applyKeywordRules(text string, tags map[string]string, keywords *[]string, reasons *[]string, score *float64) {
	keywordRules := []struct {
		Needles []string
		Field   string
		Value   string
		Words   []string
	}{
		{[]string{"evaluacion", "evaluación", "registro", "prueba", "test"}, "ambito", "Evaluación", []string{"evaluación", "registro"}},
		{[]string{"intervencion", "intervención", "actividad", "ejercicio", "material"}, "ambito", "Intervención", []string{"intervención", "actividad"}},
		{[]string{"programa"}, "ambito", "Programa", []string{"programa"}},
		{[]string{"disfemia", "tartamudez"}, "trastorno_area", "Disfemia", []string{"disfemia"}},
		{[]string{"disglosia"}, "trastorno_area", "Disglosia", []string{"disglosia"}},
		{[]string{"dislexia"}, "trastorno_area", "Dislexia", []string{"dislexia"}},
		{[]string{"tel", "trastorno especifico del lenguaje", "trastorno específico del lenguaje"}, "trastorno_area", "TEL", []string{"TEL"}},
		{[]string{"fonologia", "fonología", "articulacion", "articulación", "dislalia"}, "trastorno_area", "Fonología", []string{"fonología", "articulación"}},
		{[]string{"morfologia", "morfología", "sintaxis"}, "trastorno_area", "Morfología y sintaxis", []string{"morfología", "sintaxis"}},
		{[]string{"lexico", "léxico", "semantica", "semántica", "vocabulario"}, "trastorno_area", "Léxico-semántica", []string{"léxico", "vocabulario"}},
		{[]string{"pragmatica", "pragmática", "inferencias", "habilidades sociales"}, "trastorno_area", "Pragmática", []string{"pragmática", "inferencias"}},
		{[]string{"infantil", "3-5", "3 5"}, "edad_objetivo", "3-5", []string{"infantil"}},
		{[]string{"primaria", "6-8", "6 8"}, "edad_objetivo", "6-8", []string{"primaria"}},
		{[]string{"secundaria", "13-15", "13 15"}, "edad_objetivo", "13-15", []string{"secundaria"}},
	}

	for _, rule := range keywordRules {
		if containsAny(text, rule.Needles) {
			if tags[rule.Field] == "" {
				tags[rule.Field] = rule.Value
			} else if !strings.Contains(strings.ToLower(tags[rule.Field]), strings.ToLower(rule.Value)) {
				tags[rule.Field] += "; " + rule.Value
			}
			*keywords = append(*keywords, rule.Words...)
			*reasons = append(*reasons, "se han detectado palabras clave relacionadas con "+rule.Value)
			*score += 0.08
		}
	}

	if tags["idioma"] == "" {
		tags["idioma"] = "es"
	}
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func limitToAllowedOptions(tags map[string]string, allowed map[string][]string) {
	if len(allowed) == 0 {
		return
	}

	for field, value := range tags {
		options, ok := allowed[field]
		if !ok || len(options) == 0 {
			continue
		}

		parts := strings.Split(value, ";")
		valid := []string{}
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if optionExists(part, options) {
				valid = append(valid, canonicalOption(part, options))
			}
		}

		if len(valid) == 0 {
			delete(tags, field)
			continue
		}

		tags[field] = strings.Join(uniqueNonEmpty(valid), "; ")
	}
}

func optionExists(value string, options []string) bool {
	return canonicalOption(value, options) != ""
}

func canonicalOption(value string, options []string) string {
	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(option)) {
			return strings.TrimSpace(option)
		}
	}
	return ""
}

func firstAllowed(allowed map[string][]string, field, fallback string) string {
	if values := allowed[field]; len(values) > 0 {
		return values[0]
	}
	return fallback
}

func makeReferenceHash(req Request) string {
	base := req.DriveFileID + "|" + req.FileName + "|" + req.Path
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:])
}

func makeOptionID(req Request, tags map[string]string, normalizedName string) string {
	pairs := []string{}
	for k, v := range tags {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	base := makeReferenceHash(req) + "|" + normalizedName + "|" + strings.Join(pairs, "|")
	sum := sha256.Sum256([]byte(base))
	return "rules_" + hex.EncodeToString(sum[:])[:16]
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
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
		result = append(result, value)
	}
	return result
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 0.95 {
		return 0.95
	}
	return value
}
