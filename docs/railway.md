# Despliegue del backend en Railway

## 1. Crear proyecto

1. Entrar en Railway.
2. Crear un proyecto nuevo.
3. Elegir **Deploy from GitHub repo**.
4. Seleccionar este repositorio.

## 2. Configurar directorio raíz

El backend está dentro de la carpeta:

```text
backend
```

En Railway, configurar el servicio para que use `backend` como directorio raíz si Railway no lo detecta automáticamente.

## 3. Variables de entorno

Añadir estas variables en el servicio:

```env
PORT=8080
APP_TOKEN=un_token_largo_y_privado
GEMINI_API_KEY=
DATABASE_URL=
```

Por ahora `GEMINI_API_KEY` y `DATABASE_URL` pueden quedar vacías porque el clasificador inicial funciona por reglas.

## 4. Probar salud del servicio

Cuando Railway genere una URL pública, probar:

```text
https://TU-SERVICIO.up.railway.app/health
```

La respuesta esperada es:

```json
{
  "service": "clasificador-documentos-ia",
  "status": "ok"
}
```

## 5. Probar clasificación

Ejemplo con curl:

```bash
curl -X POST "https://TU-SERVICIO.up.railway.app/api/classify" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TU_APP_TOKEN" \
  -d '{
    "fileName": "tabla de registro disfemia 1.PNG",
    "mimeType": "image/png",
    "driveFileId": "abc123",
    "path": "RECURSOS Y MATERIALES A.L / EVALUACION TRASTORNOS / DISFEMIA",
    "allowedOptions": {
      "ambito": ["Evaluación", "Intervención", "Materiales", "Programa"],
      "trastorno_area": ["Disfemia", "TEL", "Pragmática", "Fonología"],
      "tipo_documento": ["PDF", "Imagen", "Documento", "Presentación", "Hoja de cálculo"],
      "idioma": ["es", "ca", "en"]
    }
  }'
```
