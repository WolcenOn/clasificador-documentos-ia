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
	CurrentTags    map[string]any      `json:"currentTags,omitempty"`
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

	applyCatalogKeywordRules(searchText, tags, &keywords, &reasons, &score)
	applyDefaults(tags, req.AllowedOptions, &reasons)

	keywords = uniqueNonEmpty(keywords)
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	if len(keywords) > 0 {
		tags["palabras_clave"] = strings.Join(keywords, "; ")
	}

	limitToAllowedOptions(tags, req.AllowedOptions)

	if len(tags) == 0 {
		tags["estado"] = firstAllowed(req.AllowedOptions, "estado", "Borrador")
		reasons = append(reasons, "no se han encontrado reglas suficientes; conviene revisar manualmente o con IA")
		score = 0.25
	}

	confidence := clamp(score)
	optionID := makeOptionID(req, tags, normalizedName)

	return Response{
		DriveFileID:        req.DriveFileID,
		NombreNormalizado: normalizedName,
		HashReferencia:    makeReferenceHash(req),
		ConfianzaGlobal:   confidence,
		Fuente:            "reglas_catalogador",
		Opciones: []Option{
			{
				IDOpcion:      optionID,
				Nombre:        "Clasificación inicial para catalogador",
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
	case strings.Contains(mimeType, "image") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".gif":
		tags["tipo_documento"] = "Imagen"
	case strings.Contains(mimeType, "presentation") || strings.Contains(mimeType, "google-apps.presentation") || ext == ".ppt" || ext == ".pptx":
		tags["tipo_documento"] = "Google Slides"
	case strings.Contains(mimeType, "spreadsheet") || strings.Contains(mimeType, "google-apps.spreadsheet") || ext == ".xls" || ext == ".xlsx" || ext == ".csv":
		tags["tipo_documento"] = "Google Sheets"
	case strings.Contains(mimeType, "document") || strings.Contains(mimeType, "google-apps.document") || ext == ".doc" || ext == ".docx" || ext == ".odt":
		tags["tipo_documento"] = "Google Docs"
	case strings.Contains(mimeType, "video") || ext == ".mp4" || ext == ".mov" || ext == ".avi":
		tags["tipo_documento"] = "Vídeo"
	}
}

func applyCatalogKeywordRules(text string, tags map[string]string, keywords *[]string, reasons *[]string, score *float64) {
	keywordRules := []struct {
		Needles []string
		Field   string
		Value   string
		Words   []string
	}{
		// Temática principal compatible con Config_Campos.
		{[]string{"lengua", "lectura", "escritura", "comunicacion", "comunicación", "vocabulario", "semantica", "semántica", "morfologia", "morfología", "sintaxis", "pragmatica", "pragmática", "fonologia", "fonología", "articulacion", "articulación", "dislalia", "tel", "disfemia", "tartamudez", "disglosia"}, "tematica", "Lengua", []string{"lengua"}},
		{[]string{"psicologia", "psicología", "conducta", "emocion", "emoción", "atencion", "atención", "memoria", "funciones ejecutivas"}, "tematica", "Psicología", []string{"psicología"}},
		{[]string{"matematicas", "matemáticas", "calculo", "cálculo", "numeracion", "numeración"}, "tematica", "Matemáticas", []string{"matemáticas"}},
		{[]string{"ciencias", "naturales", "biologia", "biología", "fisica", "física", "quimica", "química"}, "tematica", "Ciencias", []string{"ciencias"}},
		{[]string{"historia", "geografia", "geografía", "sociales"}, "tematica", "Historia", []string{"historia"}},
		{[]string{"arte", "plastica", "plástica", "musica", "música", "dibujo"}, "tematica", "Arte", []string{"arte"}},
		{[]string{"tecnologia", "tecnología", "informatica", "informática", "digital", "tic"}, "tematica", "Tecnología", []string{"tecnología"}},
		{[]string{"salud", "sanitario", "medico", "médico", "logopedia", "terapia"}, "tematica", "Salud", []string{"salud"}},
		{[]string{"economia", "economía", "empresa", "finanzas"}, "tematica", "Economía", []string{"economía"}},

		// Campo de aplicación.
		{[]string{"escolar", "colegio", "aula", "alumno", "alumna", "primaria", "infantil", "secundaria", "educacion", "educación"}, "campo_aplicacion", "Escolar", []string{"escolar"}},
		{[]string{"universidad", "universitario"}, "campo_aplicacion", "Universitario", []string{"universitario"}},
		{[]string{"formacion profesional", "formación profesional", "fp"}, "campo_aplicacion", "Formación profesional", []string{"formación profesional"}},
		{[]string{"empresa", "laboral", "trabajo"}, "campo_aplicacion", "Empresa", []string{"empresa"}},
		{[]string{"investigacion", "investigación", "paper", "estudio"}, "campo_aplicacion", "Investigación", []string{"investigación"}},
		{[]string{"ocio", "juego", "ludico", "lúdico"}, "campo_aplicacion", "Ocio", []string{"ocio"}},

		// Edad objetivo.
		{[]string{"infantil", "3-5", "3 5", "tres cinco"}, "edad_objetivo", "3-5", []string{"infantil"}},
		{[]string{"1 primaria", "2 primaria", "primero primaria", "segundo primaria", "6-8", "6 8"}, "edad_objetivo", "6-8", []string{"6-8"}},
		{[]string{"3 primaria", "4 primaria", "5 primaria", "6 primaria", "9-12", "9 12"}, "edad_objetivo", "9-12", []string{"9-12"}},
		{[]string{"secundaria", "eso", "13-15", "13 15"}, "edad_objetivo", "13-15", []string{"secundaria"}},
		{[]string{"bachillerato", "16-18", "16 18"}, "edad_objetivo", "16-18", []string{"bachillerato"}},
		{[]string{"adultos", "adulto"}, "edad_objetivo", "Adultos", []string{"adultos"}},
		{[]string{"seniors", "mayores", "tercera edad"}, "edad_objetivo", "Seniors", []string{"seniors"}},

		// Nivel.
		{[]string{"basico", "básico", "inicial", "facil", "fácil"}, "nivel", "Básico", []string{"básico"}},
		{[]string{"intermedio", "medio"}, "nivel", "Intermedio", []string{"intermedio"}},
		{[]string{"avanzado", "dificil", "difícil", "experto"}, "nivel", "Avanzado", []string{"avanzado"}},

		// Estado y permisos si aparecen explícitamente.
		{[]string{"borrador", "draft"}, "estado", "Borrador", []string{"borrador"}},
		{[]string{"final", "definitivo"}, "estado", "Final", []string{"final"}},
		{[]string{"propio"}, "permisos", "Propio", []string{"propio"}},
		{[]string{"compartido ver", "solo lectura"}, "permisos", "Compartido (ver)", []string{"compartido"}},
		{[]string{"compartido editar", "editable"}, "permisos", "Compartido (editar)", []string{"editable"}},

		// Palabras clave específicas del ámbito de audición y lenguaje.
		{[]string{"evaluacion", "evaluación", "registro", "prueba", "test"}, "palabras_clave", "evaluación", []string{"evaluación", "registro"}},
		{[]string{"intervencion", "intervención", "actividad", "ejercicio", "material"}, "palabras_clave", "intervención", []string{"intervención", "actividad"}},
		{[]string{"programa"}, "palabras_clave", "programa", []string{"programa"}},
		{[]string{"disfemia", "tartamudez"}, "palabras_clave", "disfemia", []string{"disfemia"}},
		{[]string{"disglosia"}, "palabras_clave", "disglosia", []string{"disglosia"}},
		{[]string{"dislexia"}, "palabras_clave", "dislexia", []string{"dislexia"}},
		{[]string{"tel", "trastorno especifico del lenguaje", "trastorno específico del lenguaje"}, "palabras_clave", "TEL", []string{"TEL"}},
		{[]string{"fonologia", "fonología", "articulacion", "articulación", "dislalia"}, "palabras_clave", "fonología", []string{"fonología", "articulación"}},
		{[]string{"morfologia", "morfología", "sintaxis"}, "palabras_clave", "morfología y sintaxis", []string{"morfología", "sintaxis"}},
		{[]string{"lexico", "léxico", "semantica", "semántica", "vocabulario"}, "palabras_clave", "léxico-semántica", []string{"léxico", "vocabulario"}},
		{[]string{"pragmatica", "pragmática", "inferencias", "habilidades sociales"}, "palabras_clave", "pragmática", []string{"pragmática", "inferencias"}},
	}

	for _, rule := range keywordRules {
		if containsAny(text, rule.Needles) {
			appendTagValue(tags, rule.Field, rule.Value)
			*keywords = append(*keywords, rule.Words...)
			*reasons = append(*reasons, "se han detectado palabras clave relacionadas con "+rule.Value)
			*score += 0.06
		}
	}
}

func applyDefaults(tags map[string]string, allowed map[string][]string, reasons *[]string) {
	if tags["idioma"] == "" {
		tags["idioma"] = firstAllowed(allowed, "idioma", "es")
	}

	if tags["campo_aplicacion"] == "" && optionExists("Escolar", allowed["campo_aplicacion"]) {
		tags["campo_aplicacion"] = "Escolar"
		*reasons = append(*reasons, "se ha usado Escolar como campo de aplicación por defecto")
	}

	if tags["estado"] == "" && optionExists("Borrador", allowed["estado"]) {
		tags["estado"] = "Borrador"
	}
}

func appendTagValue(tags map[string]string, field, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if tags[field] == "" {
		tags[field] = value
		return
	}

	parts := strings.Split(tags[field], ";")
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}
	tags[field] += "; " + value
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
			// Campos de texto como palabras_clave no tienen lista de opciones.
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
		for _, value := range values {
			if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(fallback)) {
				return strings.TrimSpace(value)
			}
		}
		return strings.TrimSpace(values[0])
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
