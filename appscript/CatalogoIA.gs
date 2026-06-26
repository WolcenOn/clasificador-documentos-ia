const CATALOGO_IA_CONFIG = {
  BACKEND_URL: 'https://TU-SERVICIO.up.railway.app',
  APP_TOKEN: 'CAMBIA_ESTE_TOKEN'
};

function onOpen() {
  SpreadsheetApp.getUi()
    .createMenu('Catálogo IA')
    .addItem('Probar backend', 'probarBackendCatalogoIA')
    .addItem('Clasificar fila seleccionada', 'clasificarFilaSeleccionadaConIA')
    .addToUi();
}

function probarBackendCatalogoIA() {
  const response = UrlFetchApp.fetch(CATALOGO_IA_CONFIG.BACKEND_URL + '/health', {
    method: 'get',
    muteHttpExceptions: true
  });

  SpreadsheetApp.getUi().alert(
    'Respuesta backend: ' + response.getResponseCode() + '\n' + response.getContentText()
  );
}

function clasificarFilaSeleccionadaConIA() {
  const ss = SpreadsheetApp.getActiveSpreadsheet();
  const sheet = ss.getSheetByName('Catalogo_generado');
  const row = sheet.getActiveCell().getRow();

  if (row <= 1) {
    SpreadsheetApp.getUi().alert('Selecciona una fila de documento, no la cabecera.');
    return;
  }

  const headers = sheet.getRange(1, 1, 1, sheet.getLastColumn()).getValues()[0];
  const values = sheet.getRange(row, 1, 1, sheet.getLastColumn()).getValues()[0];
  const record = rowToObject_(headers, values);

  const payload = {
    fileName: record.Nombre || record.nombre || record.FileName || '',
    mimeType: record.MimeType || record.mimeType || '',
    driveFileId: record.ID || record.id || '',
    path: record.Ruta_ZIP || record.Ruta || record.path || '',
    allowedOptions: obtenerOpcionesPermitidas_(),
    currentTags: obtenerEtiquetasActuales_(record)
  };

  const result = llamarBackendClasificacion_(payload);
  aplicarPrimeraOpcionIA_(sheet, headers, row, result);

  SpreadsheetApp.getUi().alert('Clasificación aplicada con confianza: ' + result.confianza_global);
}

function llamarBackendClasificacion_(payload) {
  const response = UrlFetchApp.fetch(CATALOGO_IA_CONFIG.BACKEND_URL + '/api/classify', {
    method: 'post',
    contentType: 'application/json',
    headers: {
      Authorization: 'Bearer ' + CATALOGO_IA_CONFIG.APP_TOKEN
    },
    payload: JSON.stringify(payload),
    muteHttpExceptions: true
  });

  const status = response.getResponseCode();
  const text = response.getContentText();

  if (status < 200 || status >= 300) {
    throw new Error('Error clasificando con IA: ' + status + ' - ' + text);
  }

  return JSON.parse(text);
}

function obtenerOpcionesPermitidas_() {
  const ss = SpreadsheetApp.getActiveSpreadsheet();
  const sheet = ss.getSheetByName('Config_Opciones');

  if (!sheet) {
    return {};
  }

  const values = sheet.getDataRange().getValues();
  const opciones = {};

  for (let i = 1; i < values.length; i++) {
    const campo = values[i][0];
    const valor = values[i][1];

    if (!campo || !valor) continue;

    if (!opciones[campo]) {
      opciones[campo] = [];
    }

    opciones[campo].push(valor);
  }

  return opciones;
}

function obtenerEtiquetasActuales_(record) {
  const campos = [
    'ambito',
    'prueba',
    'trastorno_area',
    'programa',
    'edad_objetivo',
    'nivel',
    'tipo_documento',
    'idioma',
    'estado',
    'permisos'
  ];

  const etiquetas = {};
  campos.forEach(function(campo) {
    if (record[campo]) {
      etiquetas[campo] = record[campo];
    }
  });

  return etiquetas;
}

function aplicarPrimeraOpcionIA_(sheet, headers, row, result) {
  if (!result.opciones || !result.opciones.length) {
    throw new Error('El backend no devolvió opciones de etiquetado.');
  }

  const opcion = result.opciones[0];
  const etiquetas = opcion.etiquetas || {};

  Object.keys(etiquetas).forEach(function(campo) {
    const index = headers.indexOf(campo);
    if (index !== -1) {
      sheet.getRange(row, index + 1).setValue(etiquetas[campo]);
    }
  });

  setIfColumnExists_(sheet, headers, row, 'Nombre_normalizado', result.nombre_normalizado || '');
  setIfColumnExists_(sheet, headers, row, 'Hash_contenido', result.hash_referencia || '');
  setIfColumnExists_(sheet, headers, row, 'Fuente_etiquetado', result.fuente || 'reglas');
  setIfColumnExists_(sheet, headers, row, 'Confianza', confianzaTexto_(result.confianza_global));
  setIfColumnExists_(sheet, headers, row, 'palabras_clave', (opcion.palabras_clave || []).join('; '));
  setIfColumnExists_(sheet, headers, row, 'Etiquetas_aplicadas_JSON', JSON.stringify(etiquetas));
  setIfColumnExists_(sheet, headers, row, 'Opciones_IA_JSON', JSON.stringify(result.opciones));
}

function setIfColumnExists_(sheet, headers, row, columnName, value) {
  const index = headers.indexOf(columnName);
  if (index !== -1) {
    sheet.getRange(row, index + 1).setValue(value);
  }
}

function confianzaTexto_(value) {
  if (value >= 0.85) return 'Alta';
  if (value >= 0.65) return 'Media';
  return 'Baja';
}

function rowToObject_(headers, values) {
  const obj = {};
  headers.forEach(function(header, index) {
    if (header) {
      obj[header] = values[index];
    }
  });
  return obj;
}
