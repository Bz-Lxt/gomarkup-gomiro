package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gomiro/internal/config"
	"gomiro/internal/store"
)

var allowedMIME = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type UploadAPI struct {
	DB  *store.DB
	Cfg config.Config
}

func (a UploadAPI) Post(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.Cfg.MaxUploadBytes+512)
	if err := r.ParseMultipartForm(a.Cfg.MaxUploadBytes); err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, APIError{Code: "too_large", Message: "file exceeds 5MB"})
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_field", Message: "file required"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, a.Cfg.MaxUploadBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_field", Message: "read failed"})
		return
	}
	if int64(len(data)) > a.Cfg.MaxUploadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, APIError{Code: "too_large", Message: "file exceeds 5MB"})
		return
	}
	mime := hdr.Header.Get("Content-Type")
	if _, ok := allowedMIME[mime]; !ok {
		mime = http.DetectContentType(data)
		if _, ok := allowedMIME[mime]; !ok {
			writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_type", Message: "mime not allowed"})
			return
		}
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if err := os.MkdirAll(a.Cfg.UploadDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "fs", Message: "upload dir"})
		return
	}
	dest := filepath.Join(a.Cfg.UploadDir, hash)
	if _, err := os.Stat(dest); err != nil {
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, APIError{Code: "fs", Message: "write failed"})
			return
		}
	}
	written := int64(len(data))
	_ = a.DB.InsertUpload(r.Context(), hash, mime, int(written))
	writeJSON(w, http.StatusCreated, map[string]any{
		"hash": hash,
		"mime": mime,
		"bytes": written,
		"url":  "/api/v1/files/" + hash,
	})
}

func (a UploadAPI) Get(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if len(hash) != 64 {
		writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_field", Message: "hash invalid"})
		return
	}
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_field", Message: "hash invalid"})
			return
		}
	}
	mime, ok, err := a.DB.GetUpload(r.Context(), hash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "db", Message: "lookup failed"})
		return
	}
	path := filepath.Join(a.Cfg.UploadDir, hash)
	if !ok {
		if _, err := os.Stat(path); err != nil {
			writeJSON(w, http.StatusNotFound, APIError{Code: "not_found", Message: "file not found"})
			return
		}
		mime = "application/octet-stream"
	}
	if !strings.HasPrefix(mime, "image/") {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}
