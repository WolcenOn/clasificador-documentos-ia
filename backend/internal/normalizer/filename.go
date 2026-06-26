package normalizer

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var genericNames = map[string]bool{
	"archivo":           true,
	"documento":         true,
	"document":          true,
	"documents":         true,
	"file":              true,
	"scan":              true,
	"escaneo":           true,
	"imagen":            true,
	"image":             true,
	"img":               true,
	"download":          true,
	"descarga":          true,
	"untitled":          true,
	"sin titulo":        true,
	"nuevo documento":   true,
	"whatsapp document": true,
	"pdf":               true,
}

var datePattern = regexp.MustCompile(`\b\d{1,4}[\/\-.]\d{1,2}[\/\-.]\d{1,4}\b`)
var timePattern = regexp.MustCompile(`\b\d{1,2}[:.]\d{2}([:.]\d{2})?\b`)
var longNumberPattern = regexp.MustCompile(`\b\d{5,}\b`)
var spacesPattern = regexp.MustCompile(`\s+`)

func Filename(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.ToLower(name)
	name = removeAccents(name)

	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "(", " ")
	name = strings.ReplaceAll(name, ")", " ")
	name = strings.ReplaceAll(name, "[", " ")
	name = strings.ReplaceAll(name, "]", " ")

	name = datePattern.ReplaceAllString(name, " ")
	name = timePattern.ReplaceAllString(name, " ")
	name = longNumberPattern.ReplaceAllString(name, " ")
	name = spacesPattern.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	if genericNames[name] || len([]rune(name)) < 4 {
		return ""
	}

	return name
}

func removeAccents(input string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ä", "a", "â", "a",
		"é", "e", "è", "e", "ë", "e", "ê", "e",
		"í", "i", "ì", "i", "ï", "i", "î", "i",
		"ó", "o", "ò", "o", "ö", "o", "ô", "o",
		"ú", "u", "ù", "u", "ü", "u", "û", "u",
		"ñ", "n",
		"ç", "c",
	)

	cleaned := replacer.Replace(input)
	var b strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(' ')
	}

	return spacesPattern.ReplaceAllString(strings.TrimSpace(b.String()), " ")
}
