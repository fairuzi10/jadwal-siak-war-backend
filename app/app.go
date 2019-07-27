package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	siak "github.com/fairuzi10/jadwal-siak-war-backend"
	"github.com/gofrs/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	registerHandlers()
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func registerHandlers() {
	r := mux.NewRouter()
	r.Methods("POST").Path("/html-file").
		Handler(appHandler(uploadHTMLFileHandler))

	http.Handle("/", handlers.CombinedLoggingHandler(os.Stderr, r))
}

func uploadHTMLFile(r *http.Request) (url string, appErr *appError) {
	f, fh, err := r.FormFile("file")
	if err == http.ErrMissingFile {
		return "", &appError{
			Error:   err,
			Message: "`file` is required",
			Code:    http.StatusBadRequest,
		}
	}
	if err == http.ErrNotMultipart {
		return "", &appError{
			Error:   err,
			Message: "Use multipart/form-data",
			Code:    http.StatusBadRequest,
		}
	}
	if err != nil {
		return "", internalServerError(err)
	}
	contentType := fh.Header.Get("Content-Type")
	if contentType != "text/html" {
		return "", &appError{
			Error:   err,
			Message: "HTML file only",
			Code:    http.StatusBadRequest,
		}
	}

	// random filename, retaining existing extension.
	name := time.Now().Format(time.RFC3339) + "-" + uuid.Must(uuid.NewV4()).String() + path.Ext(fh.Filename)

	ctx := context.Background()
	w := siak.StorageBucket.Object(name).NewWriter(ctx)

	w.ContentType = contentType
	w.CacheControl = "public, max-age=86400" // 1 day

	if _, err := io.Copy(w, f); err != nil {
		return "", internalServerError(err)
	}
	if err := w.Close(); err != nil {
		return "", internalServerError(err)
	}

	const publicURL = "https://storage.googleapis.com/%s/%s"
	return fmt.Sprintf(publicURL, siak.StorageBucketName, name), nil
}

func uploadHTMLFileHandler(w http.ResponseWriter, r *http.Request) *appError {
	_, err := uploadHTMLFile(r)
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type appHandler func(http.ResponseWriter, *http.Request) *appError

type appError struct {
	Error   error
	Message string
	Code    int
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil { // e is *appError, not os.Error.
		log.Printf("Handler error: status code: %d, underlying err: %#v",
			e.Code, e.Error)

		http.Error(w, e.Message, e.Code)
	}
}

func internalServerError(err error) *appError {
	return &appError{
		Error:   err,
		Message: err.Error(),
		Code:    http.StatusInternalServerError,
	}
}
