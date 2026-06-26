# Clasificador de documentos con IA

Clasificador de documentos y etiquetador automático para facilitar la búsqueda y clasificación de documentos pedagógicos de Audición y Lenguaje.

## Objetivo

Facilitar la búsqueda y catalogación de documentos almacenados en Google Drive según temática, tipo de documento, ámbito, edad objetivo, nivel, idioma y palabras clave.

## Arquitectura inicial

- **Google Drive**: almacenamiento de documentos.
- **Google Sheets**: base de datos visible del catálogo.
- **Apps Script**: integración con Drive y Sheets.
- **Backend Go**: API intermedia segura.
- **Gemini API**: clasificación inteligente de documentos en una fase posterior.
- **Railway**: despliegue del backend.
- **GitHub**: repositorio del código.

## Estructura

```text
backend/
appscript/
docs/
```

## Flujo general

1. Apps Script detecta o registra archivos de Google Drive.
2. Se normaliza el nombre del archivo.
3. Se buscan etiquetas existentes en Google Sheets.
4. Si no hay coincidencias fiables, Apps Script llama al backend.
5. El backend sugiere etiquetas por reglas.
6. Más adelante, el backend podrá usar Gemini para leer el documento y sugerir etiquetas.
7. Las etiquetas se guardan en Google Sheets.
8. El usuario puede revisar, corregir o aceptar las sugerencias.

## Estado actual

Fase inicial: backend mínimo en Go con endpoints de salud y clasificación por reglas.

## Endpoints iniciales

### `GET /health`

Comprueba si el backend está activo.

### `POST /api/classify`

Clasifica un documento usando nombre, ruta, MIME type y opciones permitidas.

Ejemplo de payload:

```json
{
  "fileName": "tabla de registro disfemia 1.PNG",
  "mimeType": "image/png",
  "driveFileId": "abc123",
  "path": "RECURSOS Y MATERIALES A.L / EVALUACION TRASTORNOS / DISFEMIA",
  "allowedOptions": {
    "ambito": ["Evaluación", "Intervención", "Materiales"],
    "trastorno_area": ["Disfemia", "TEL", "Pragmática"],
    "tipo_documento": ["PDF", "Imagen", "Documento", "Presentación"]
  }
}
```

## Variables de entorno

Ver `.env.example`.
