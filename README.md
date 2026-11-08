# dym-bridge

Aplicación de escritorio multiplataforma (Windows, macOS y Linux) en Go diseñada para simplificar el acceso y la gestión de sitios **WordPress gestionados en Dymmer** bajo entornos de alta seguridad.

---

## Características

* **🔒 Túnel SSH y Proxy HTTP Inverso Inteligente**:
  * Diseñado para WordPress donde el panel de administración (`/wp-admin/`) solo escucha en `localhost` y está bloqueado para el tráfico público.
  * Inyecta automáticamente la cabecera `Host: dominio.com` y reescribe las redirecciones canónicas `Location` de WordPress para que no fallen al acceder por `http://127.0.0.1:8080/wp-admin/`.
  * Botón de **un solo clic** para abrir el panel de administración en el navegador predeterminado del sistema (respetando contraseñas guardadas y extensiones).
* **📁 Explorador de Archivos SFTP (Estilo Windows Explorer)**:
  * Navegación compacta con rutas abreviadas, doble clic en carpetas y acceso directo a rutas mediante el botón de puntos suspensivos.
  * Subida de archivos, descarga, creación de nuevas carpetas y eliminación con confirmación.
  * **Soporte Drag & Drop**: Arrastre archivos o carpetas desde el explorador del sistema operativo hacia la ventana para iniciar la subida con barra de progreso.
  * Selección mediante checkboxes y progreso de transferencias.
* **🟢 Bandeja del Sistema (Systray)**:
  * Al cerrar la ventana principal (`X`), la aplicación continúa ejecutándose en segundo plano manteniendo el túnel activo.
  * Menú en el icono de la bandeja para abrir el explorador, abrir WordPress Admin, reconectar el túnel o salir.
* **⚙️ Configuración Sencilla (`dym-bridge.ini`)**:
  * Basta con colocar un archivo `dym-bridge.ini` en la misma carpeta del ejecutable (`.exe` en Windows o binario en macOS/Linux).
  * Admite claves privadas relativas (ej. `key_file = id_ed25519`) ubicadas en la misma carpeta.

---

## Conexión con Dymmer

Con una cuenta de Dymmer, pulsa **Sign in with Dymmer** (o `dym-bridge login`): la aplicación genera su propia clave,
publica solo la clave pública y recibe los recursos a los que tienes acceso. Ver [docs/signin.md](docs/signin.md).

## Configuración (`dym-bridge.ini`)

Cree un archivo llamado `dym-bridge.ini` junto al ejecutable:

```ini
[ssh]
# Servidor y puerto SSH
host = ssh.ejemplo.com
port = 22
user = cliente_wp

# Autenticación por clave privada (recomendada) o contraseña
key_file = id_ed25519
# passphrase = mi_contraseña_si_aplica
# password = SecretoAlternativo

[wordpress]
# Dominio canónico de WordPress (para cabecera HTTP Host)
host_header = cliente.com
# Puerto remoto en localhost donde escucha WordPress (por defecto 80)
remote_port = 80
# Puerto local donde se expone el proxy inverso (por defecto 8080)
local_port = 8080
# Ruta del panel de administración
admin_path = /wp-admin/

[sftp]
# Directorio remoto inicial al abrir el explorador
remote_path = /var/www/html/wp-content
```

---

## Compilación y Empaquetado

Requiere la versión de Go indicada en `go.mod`. La aplicación principal usa GoGPU y compila con `CGO_ENABLED=0`, sin WebView. El empaquetado DMG requiere macOS y Python 3.

```bash
# Compilar binario nativo para la máquina actual
make build

# Ejecutar tests automatizados
make test

# Generar bundle universal para macOS (.app arm64 + amd64)
make bundle

# Preparar las herramientas de empaquetado (una vez)
make dmg-deps

# Crear instalador DMG universal para macOS
make dmg

# Compilar ejecutable para Windows (dym-bridge.exe)
make windows
```

---

## Estructura del Proyecto

* `cmd/dym-bridge/main.go`: Punto de entrada, lock de instancia única y carga de configuración.
* `pkg/config/`: Lectura y validación de `dym-bridge.ini`, resolución de rutas relativas.
* `pkg/tunnel/`: Cliente SSH con keepalive y proxy HTTP inverso con reescritura de cabeceras.
* `pkg/sftp/`: Operaciones SFTP y tracking de progreso en transferencias.
* `pkg/gpuui/`: Interfaz nativa GoGPU, login, explorador dual, Sites y transferencias.
* `pkg/desktop/`: Integración de escritorio y callback macOS sin CGo.
* `pkg/ui/`: Fuentes históricas Fyne, excluidas de la compilación normal mediante el tag `fyne`.
* `pkg/lock/`: Mutex/lock para impedir instancias duplicadas concurrentes.
* `pkg/version/`: Información de versión y build.

## Interfaz principal GoGPU

`cmd/dym-bridge` usa ahora la interfaz GoGPU. Se conservan el login OAuth/PKCE con Dymmer, publicación de la clave pública SSH, perfiles cifrados, CLI, INI, SFTP y túneles. Sites se rellena con las interfaces aprovisionadas; los enlaces manuales quedan en Advanced. Los INI WordPress conservan su acceso al panel.

El DMG en `bin/dym-bridge.dmg` incluye fondo de marca y disposición Finder para arrastrar la app a Applications. El mismo empaquetado se usa en CI, con firma y notarización cuando están configuradas. Los binarios y el DMG locales no se publican automáticamente.

Detalles y limitaciones comprobadas: [docs/gogpu-preview.md](docs/gogpu-preview.md).
