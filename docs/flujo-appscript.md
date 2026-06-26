# Flujo de integración con Apps Script

## Objetivo

Permitir que la hoja de cálculo del catálogo use el backend para sugerir etiquetas según el nombre, ruta y metadatos de cada archivo.

## Flujo inicial

1. El usuario selecciona una fila en `Catalogo_generado`.
2. Ejecuta el menú **Catálogo IA > Clasificar fila seleccionada**.
3. Apps Script lee los datos de la fila.
4. Apps Script lee las opciones permitidas desde `Config_Opciones`.
5. Apps Script llama al backend `/api/classify`.
6. El backend devuelve una opción de etiquetado inicial.
7. Apps Script aplica las etiquetas a la fila.

## Columnas recomendadas nuevas

Añadir a `Catalogo_generado` cuando sea posible:

```text
Nombre_normalizado
Hash_contenido
Fuente_etiquetado
Modelo_IA
Version_taxonomia
Opciones_IA_JSON
Revisado_por_usuario
Fecha_revision
```

## Notas

- `Hash_contenido` por ahora guarda un hash de referencia basado en ID, nombre y ruta.
- Más adelante se puede sustituir por hash real del contenido.
- `Fuente_etiquetado` inicialmente será `reglas`.
- Cuando integremos Gemini, podrá ser `gemini`, `gemini_confirmado_usuario` o similar.
