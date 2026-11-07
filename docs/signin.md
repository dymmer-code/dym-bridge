# Conexión con tu cuenta de Dymmer

Con tu cuenta de Dymmer, dym-bridge se configura solo: no hace falta copiar claves ni editar ficheros.

## Cómo funciona

1. Abre dym-bridge y pulsa **Sign in with Dymmer** (o ejecuta `dym-bridge login` en una terminal).
2. Elige una **clave maestra**. Protege la clave privada de este equipo; no se envía a ningún sitio.
3. Se abre el navegador en Dymmer. Inicia sesión y aprueba este equipo. El navegador te preguntará si abrir
   dym-bridge: acepta.
4. dym-bridge crea una clave propia de este equipo (Ed25519), envía **solo la clave pública** y recibe la lista de
   recursos a los que tienes acceso: servidor, puerto, usuario, ruta inicial y las claves del servidor.
5. Elige el recurso y abre el explorador SFTP.

La clave privada nunca sale del equipo y se guarda cifrada con tu clave maestra. El permiso que Dymmer entrega al
iniciar sesión es de un solo uso: se gasta en la llamada de configuración y no se guarda. Para actualizar tus
recursos, vuelve a iniciar sesión.

## Seguridad

* El servidor al que te conectas debe presentar una de las claves que Dymmer ha comunicado; si cambia, dym-bridge se
  niega a conectar y te pide iniciar sesión de nuevo.
* La respuesta del navegador solo se acepta si pertenece a este inicio de sesión (parámetro `state`) y se canjea con
  PKCE: otra aplicación que registre el mismo esquema no puede usar el código.
* Puedes ver y revocar tus equipos en *Mis accesos* de Dymmer.

## El esquema `dym-bridge://`

El navegador vuelve a la aplicación con `dym-bridge://callback`. Cada sistema debe registrarlo:

| Sistema | Cómo |
| --- | --- |
| macOS | Lo declara el `Info.plist` de la aplicación (`make bundle`). |
| Linux | Instala `packaging/linux/dym-bridge.desktop` y ejecuta `update-desktop-database`. |
| Windows | El instalador escribe las claves de `packaging/windows/dym-bridge-protocol.reg` (usuario actual). |

Si ya hay un dym-bridge esperando el inicio de sesión, el sistema lanza uno nuevo con la dirección como argumento y
este se la pasa al primero y termina.

## Configuración manual

El fichero `dym-bridge.ini` sigue funcionando para quien lo prefiera; ver el README.
