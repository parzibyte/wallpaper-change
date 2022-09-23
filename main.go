package main

import (
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron"
	"github.com/rs/xid"
)

const (
	NombreEjecutableWallpaperChange = "cambiarescritorio.exe"
	GistControlador                 = ""
	NombreBaseDeDatos               = "data.db"
)

type CambioImagen struct {
	Id               int64
	UrlImagen, Fecha string
}

/*
	Recibe el nombre de una imagen para poner como fondo. La imagen debe estar
	en el mismo directorio que este ejecutable
*/
func cambiarWallpaper(nombreImagen string) ([]byte, error) {
	ubicacionActual, _ := os.Getwd()
	ubicacionImagenCompleta := path.Join(ubicacionActual, nombreImagen)
	return exec.Command(NombreEjecutableWallpaperChange, ubicacionImagenCompleta).Output()
}

func descargarImagenYPonerlaComoFondo(rutaImagen string) error {
	imagen, err := descargarArchivoDeInternet(rutaImagen)
	if err != nil {
		return err
	}
	_, err = cambiarWallpaper(imagen)
	if err != nil {
		return err
	}
	err = os.Remove(imagen)
	if err != nil {
		return err
	}
	return registrarCambioDeImagen(rutaImagen)
}

func revisarGistYCambiarImagenSiEsNecesario() error {
	ultimoCambio, err := obtenerUltimoCambioDeImagen()
	err, rutaImagen, fecha := obtenerDetallesGist()
	if err != nil {
		return err
	}
	if ultimoCambio.Fecha < fecha || ultimoCambio.UrlImagen != rutaImagen {
		return descargarImagenYPonerlaComoFondo(rutaImagen)
	}
	return nil
}

func main() {
	err := crearTablas()
	if err != nil {
		log.Printf("Error creando tablas: %v", err)
		return
	}
	c := cron.New()
	defer c.Stop()
	// Agregarle funciones...

	// Ejecutar cada segundo toda la vida
	err = c.AddFunc("0 */1 * * *", func() {
		log.Printf("Soy cron")
		revisarGistYCambiarImagenSiEsNecesario()
	})
	if err != nil {
		log.Printf("Error iniciando cron: %v", err)
		return
	}

	// Comenzar
	c.Start()

	// Lo siguiente es únicamente para pausar el programa y no tiene nada
	// que ver con cron o el ejemplo, recuerda que
	// el programa se detiene con Ctrl + C
	select {}
}

func fechaYHoraActual() string {
	t := time.Now()
	return fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
}

func obtenerDetallesGist() (error, string, string) {
	clienteHttp := &http.Client{}
	peticion, err := http.NewRequest("GET", GistControlador, nil)
	if err != nil {
		return err, "", ""
	}
	respuesta, err := clienteHttp.Do(peticion)
	if err != nil {
		return err, "", ""
	}
	defer respuesta.Body.Close()
	cuerpoRespuesta, err := ioutil.ReadAll(respuesta.Body)
	if err != nil {
		return err, "", ""
	}
	respuestaString := string(cuerpoRespuesta)
	if respuesta.StatusCode != http.StatusOK {
		return fmt.Errorf("status code no fue OK, fue %v", respuesta.StatusCode), "", ""
	}
	respuestaArreglo := strings.Split(respuestaString, ",")
	if len(respuestaArreglo) != 2 {
		return fmt.Errorf("se esperaban 2 valores separados por coma (,), pero se encontraron: %d", len(respuestaArreglo)), "", ""
	}
	rutaImagen, fecha := respuestaArreglo[0], respuestaArreglo[1]
	return nil, rutaImagen, fecha
}

func extensionImagenSegunContentType(contentType string) string {
	if contentType == "image/jpeg" || contentType == "image/jpg" {
		return "jpg"
	} else if contentType == "image/png" {
		return "png"
	}
	return ""
}

func descargarArchivoDeInternet(url string) (string, error) {
	respuesta, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer respuesta.Body.Close()
	nombreArchivoSalida := fmt.Sprintf("%s.%s", xid.New().String(), extensionImagenSegunContentType(respuesta.Header.Get("Content-Type")))
	archivoSalida, err := os.Create(nombreArchivoSalida)
	if err != nil {
		return "", err
	}
	defer archivoSalida.Close()
	_, err = io.Copy(archivoSalida, respuesta.Body)
	return nombreArchivoSalida, err
}

func obtenerBaseDeDatos() (*sql.DB, error) {
	return sql.Open("sqlite3", NombreBaseDeDatos)

}

func crearTablas() error {
	db, err := obtenerBaseDeDatos()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cambios_imagen(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url_imagen TEXT NOT NULL,
		fecha TEXT NOT NULL
	); `)
	return err
}

func obtenerUltimoCambioDeImagen() (CambioImagen, error) {
	var cambio CambioImagen
	db, err := obtenerBaseDeDatos()
	if err != nil {
		return cambio, err
	}
	defer db.Close()

	fila := db.QueryRow("SELECT url_imagen, fecha FROM cambios_imagen ORDER BY id DESC LIMIT 1")
	err = fila.Scan(&cambio.UrlImagen, &cambio.Fecha)
	if err == sql.ErrNoRows {
		return cambio, nil
	}
	return cambio, err
}

func registrarCambioDeImagen(urlImagen string) error {
	db, err := obtenerBaseDeDatos()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("INSERT INTO cambios_imagen(url_imagen, fecha) VALUES (?, ?);", urlImagen, fechaYHoraActual())
	return err
}
