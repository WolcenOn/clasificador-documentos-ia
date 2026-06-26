# Integración con el Apps Script existente

Este documento explica cómo conectar el catalogador actual con el backend desplegado en Railway sin reescribir todo el script.

El script actual ya tiene:

- `Config_Campos`
- `Config_Opciones`
- sidebar dinámico
- multiselección con `appProperties`
- funciones `getManagedProps`, `updateManagedProps`, `readConfig`, `getFileSafe`

Por eso la integración se hace añadiendo el archivo `appscript/RailwayIA.gs` y tocando solo el HTML del sidebar.

---

## 1. Añadir archivo `RailwayIA.gs`

Copia el contenido de:

```text
appscript/RailwayIA.gs
```

en un nuevo archivo dentro del proyecto de Apps Script.

---

## 2. Configurar URL y token de Railway

En Apps Script, ejecuta una vez esta función desde el editor:

```javascript
function setupRailwayIA() {
  configurarRailwayIA(
    'https://TU-SERVICIO.up.railway.app',
    'EL_MISMO_APP_TOKEN_DE_RAILWAY'
  );
}
```

Después puedes borrar `setupRailwayIA` o dejarla comentada.

La URL y el token quedan guardados en `ScriptProperties`, no visibles en el código principal.

---

## 3. Probar conexión

Desde el editor de Apps Script ejecuta:

```javascript
function testRailwayIA() {
  Logger.log(probarRailwayIA());
}
```

La respuesta esperada debe incluir código `200` y el JSON de `/health`.

---

## 4. Añadir botón al sidebar principal

En `buildSidebarHtml()`, dentro de la pestaña `Etiquetar`, localiza esta zona:

```html
<button id="cargar">Cargar actuales</button>
<button id="addToQueue" title="Añadir este archivo a la cola">Añadir a cola</button>
```

Añade este botón justo después:

```html
<button id="sugerirIA" title="Sugerir etiquetas con Railway IA">Sugerir IA</button>
```

Debe quedar así:

```html
<button id="cargar">Cargar actuales</button>
<button id="addToQueue" title="Añadir este archivo a la cola">Añadir a cola</button>
<button id="sugerirIA" title="Sugerir etiquetas con Railway IA">Sugerir IA</button>
```

---

## 5. Añadir listener del botón

En el mismo `buildSidebarHtml()`, dentro del bloque `<script>`, añade este código después del listener de `addToQueue` o cerca de los eventos de la pestaña `Etiquetar`:

```javascript
document.getElementById("sugerirIA").addEventListener("click", function(){
  var url = document.getElementById("url").value.trim();
  var msg = document.getElementById("msg");

  if (!url) {
    msg.textContent = "Pega una URL o ID primero.";
    return;
  }

  msg.textContent = "Pidiendo sugerencia a Railway IA...";

  google.script.run
    .withSuccessHandler(function(res){
      var values = res.values || {};

      CFG.fields.forEach(function(f){
        if (values[f.campo] !== undefined) {
          setFieldValue(f, "", values[f.campo]);
          var del = document.getElementById("del_" + f.campo);
          if (del) del.checked = false;
        }
      });

      var confidence = res.result && res.result.confianza_global !== undefined
        ? res.result.confianza_global
        : "";

      msg.textContent = "Sugerencia cargada. Confianza: " + confidence + ". Revisa y pulsa Aplicar etiquetas.";
    })
    .withFailureHandler(function(err){
      msg.textContent = "Error IA: " + err.message;
    })
    .prepararPrimeraSugerenciaRailwayIA(url);
});
```

Este flujo NO aplica automáticamente las etiquetas. Solo rellena el formulario para que puedas revisar y luego pulsar `Aplicar etiquetas`.

---

## 6. Botón alternativo: aplicar automáticamente

Si quieres una opción más rápida, puedes crear otro botón:

```html
<button id="aplicarIA" title="Aplicar directamente la primera sugerencia IA">Aplicar IA</button>
```

Y este listener:

```javascript
document.getElementById("aplicarIA").addEventListener("click", function(){
  var url = document.getElementById("url").value.trim();
  var msg = document.getElementById("msg");

  if (!url) {
    msg.textContent = "Pega una URL o ID primero.";
    return;
  }

  msg.textContent = "Aplicando primera sugerencia IA...";

  google.script.run
    .withSuccessHandler(function(res){
      msg.textContent = "Etiquetas IA aplicadas a: " + res.name;
    })
    .withFailureHandler(function(err){
      msg.textContent = "Error IA: " + err.message;
    })
    .aplicarPrimeraSugerenciaRailwayIA(url);
});
```

Para uso real se recomienda empezar con `Sugerir IA`, revisar y aplicar manualmente.

---

## 7. Campos que debe devolver el backend

El backend debe devolver nombres de campo que existan en `Config_Campos`, por ejemplo:

```json
{
  "opciones": [
    {
      "etiquetas": {
        "tematica": "Lengua",
        "edad_objetivo": "6-8",
        "nivel": "Intermedio",
        "tipo_documento": "PDF",
        "idioma": "es"
      }
    }
  ]
}
```

Si el backend devuelve un campo que no existe en `Config_Campos`, el Apps Script lo ignorará.

---

## 8. Siguiente mejora

El siguiente paso será hacer que el backend acepte también contenido del documento o texto extraído para que Gemini pueda clasificar por contenido, no solo por nombre, ruta y MIME type.
