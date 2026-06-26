/** ===========================
 *  Integración Railway IA
 *  Complemento para el Catalogador Drive existente.
 *
 *  Requiere funciones ya existentes en el script principal:
 *  - getIdFromInput
 *  - readConfig
 *  - getFileSafe
 *  - updateManagedProps
 *  - saveLastProps
 *  - logRevision
 * =========================== */

const RAILWAY_IA_PROPS = {
  BACKEND_URL: 'CATALOGO_IA_BACKEND_URL',
  APP_TOKEN: 'CATALOGO_IA_APP_TOKEN'
};

/** Ejecutar una vez desde Apps Script para guardar URL y token sin dejarlos visibles en el código. */
function configurarRailwayIA(backendUrl, appToken) {
  backendUrl = String(backendUrl || '').trim().replace(/\/$/, '');
  appToken = String(appToken || '').trim();

  if (!backendUrl) throw new Error('backendUrl es obligatorio.');
  if (!appToken) throw new Error('appToken es obligatorio.');

  const props = PropertiesService.getScriptProperties();
  props.setProperty(RAILWAY_IA_PROPS.BACKEND_URL, backendUrl);
  props.setProperty(RAILWAY_IA_PROPS.APP_TOKEN, appToken);

  return 'Railway IA configurado correctamente.';
}

function obtenerConfigRailwayIA_() {
  const props = PropertiesService.getScriptProperties();
  const backendUrl = props.getProperty(RAILWAY_IA_PROPS.BACKEND_URL);
  const appToken = props.getProperty(RAILWAY_IA_PROPS.APP_TOKEN);

  if (!backendUrl) {
    throw new Error('Falta configurar CATALOGO_IA_BACKEND_URL. Ejecuta configurarRailwayIA(url, token).');
  }
  if (!appToken) {
    throw new Error('Falta configurar CATALOGO_IA_APP_TOKEN. Ejecuta configurarRailwayIA(url, token).');
  }

  return { backendUrl, appToken };
}

function probarRailwayIA() {
  const cfg = obtenerConfigRailwayIA_();
  const response = UrlFetchApp.fetch(cfg.backendUrl + '/health', {
    method: 'get',
    muteHttpExceptions: true
  });

  return {
    status: response.getResponseCode(),
    body: response.getContentText()
  };
}

/** Devuelve sugerencias del backend, pero NO las aplica al archivo. */
function sugerirEtiquetasRailwayIA(urlOrId) {
  const fileId = getIdFromInput(urlOrId);
  if (!fileId) throw new Error('URL/ID no válido.');

  const file = Drive.Files.get(fileId, {
    fields: 'id,name,mimeType,webViewLink,appProperties,parents'
  });

  if (!file || file.trashed) throw new Error('No se pudo acceder al archivo o está en papelera.');

  const payload = {
    fileName: file.name || '',
    mimeType: file.mimeType || '',
    driveFileId: file.id,
    path: construirRutaBasica_(file),
    allowedOptions: obtenerOpcionesPermitidasParaIA_(),
    currentTags: getManagedProps(fileId)
  };

  return llamarBackendRailwayIA_(payload);
}

/** Solicita sugerencias al backend y aplica automáticamente la primera opción. */
function aplicarPrimeraSugerenciaRailwayIA(urlOrId) {
  const fileId = getIdFromInput(urlOrId);
  if (!fileId) throw new Error('URL/ID no válido.');

  const result = sugerirEtiquetasRailwayIA(fileId);
  const option = obtenerPrimeraOpcionIA_(result);
  const toSet = convertirEtiquetasIAParaProps_(option.etiquetas || {});

  const updated = updateManagedProps(fileId, toSet, []);
  saveLastProps(toSet);

  try {
    logRevision(updated, {
      fuente_etiquetado: result.fuente || 'railway_reglas',
      confianza_global: String(result.confianza_global || ''),
      etiquetas: toSet,
      opcion: option
    }, []);
  } catch (e) {
    // No bloqueamos la aplicación si falla el log.
    console.log('No se pudo registrar Revision_Registro: ' + e.message);
  }

  return {
    id: updated.id,
    name: updated.name,
    link: updated.webViewLink,
    result: result,
    applied: toSet
  };
}

/** Función pensada para el sidebar: rellena los campos con la primera sugerencia sin guardar. */
function prepararPrimeraSugerenciaRailwayIA(urlOrId) {
  const result = sugerirEtiquetasRailwayIA(urlOrId);
  const option = obtenerPrimeraOpcionIA_(result);

  return {
    result: result,
    option: option,
    values: convertirEtiquetasIAParaProps_(option.etiquetas || {}),
    palabras_clave: option.palabras_clave || []
  };
}

function llamarBackendRailwayIA_(payload) {
  const cfg = obtenerConfigRailwayIA_();

  const response = UrlFetchApp.fetch(cfg.backendUrl + '/api/classify', {
    method: 'post',
    contentType: 'application/json',
    headers: {
      Authorization: 'Bearer ' + cfg.appToken
    },
    payload: JSON.stringify(payload),
    muteHttpExceptions: true
  });

  const status = response.getResponseCode();
  const text = response.getContentText();

  if (status < 200 || status >= 300) {
    throw new Error('Error Railway IA ' + status + ': ' + text);
  }

  return JSON.parse(text);
}

function obtenerOpcionesPermitidasParaIA_() {
  const cfg = readConfig();
  const out = {};

  (cfg.fields || []).forEach(function(field) {
    if (field.tipo === 'select') {
      out[field.campo] = cfg.options[field.campo] || [];
    }
  });

  return out;
}

function obtenerPrimeraOpcionIA_(result) {
  if (!result || !result.opciones || !result.opciones.length) {
    throw new Error('El backend no devolvió opciones de etiquetado.');
  }
  return result.opciones[0];
}

function convertirEtiquetasIAParaProps_(etiquetas) {
  const cfg = readConfig();
  const fieldsByName = {};
  (cfg.fields || []).forEach(function(field) {
    fieldsByName[field.campo] = field;
  });

  const out = {};

  Object.keys(etiquetas || {}).forEach(function(campo) {
    if (!fieldsByName[campo]) return;

    const field = fieldsByName[campo];
    const value = etiquetas[campo];

    if (field.tipo === 'select') {
      if (Array.isArray(value)) {
        out[campo] = value.map(String).map(function(v) { return v.trim(); }).filter(Boolean);
      } else {
        out[campo] = String(value || '')
          .split(';')
          .map(function(v) { return v.trim(); })
          .filter(Boolean);
      }
    } else {
      out[campo] = String(value || '').trim();
    }
  });

  return out;
}

function construirRutaBasica_(file) {
  const props = file.appProperties || {};
  if (props.Ruta_ZIP) return props.Ruta_ZIP;
  if (props.Ruta) return props.Ruta;
  if (props.path) return props.path;
  return '';
}
