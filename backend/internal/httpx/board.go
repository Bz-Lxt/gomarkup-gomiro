package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"gomiro/internal/config"
	"gomiro/internal/model"
	"gomiro/internal/store"
	"gomiro/internal/timeutil"
)

type BoardAPI struct {
	DB  *store.DB
	Cfg config.Config
}

type createBoardReq struct {
	Title    string `json:"title"`
	Passcode string `json:"passcode"`
}

type patchBoardReq struct {
	Title     string `json:"title"`
	Passcode  string `json:"passcode"`
	ClearPass bool   `json:"clearPass"`
	Thumbnail string `json:"thumbnail"`
}

type unlockReq struct {
	Passcode string `json:"passcode"`
}

func (a BoardAPI) List(w http.ResponseWriter, r *http.Request) {
	boards, err := a.DB.ListBoards(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "db", Message: "list failed"})
		return
	}
	items := make([]model.BoardListItem, 0, len(boards))
	for _, b := range boards {
		items = append(items, model.BoardListItem{
			ID: b.ID, Title: b.Title, HasPass: b.HasPass, Thumbnail: b.Thumbnail,
			CreatedAt: timeutil.Format(b.CreatedAt),
			UpdatedAt: timeutil.Format(b.UpdatedAt),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a BoardAPI) Create(w http.ResponseWriter, r *http.Request) {
	var req createBoardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_json", Message: "invalid body"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "未命名白板"
	}
	if utf8.RuneCountInString(title) > 80 {
		writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_field", Message: "title too long"})
		return
	}
	hash := ""
	if strings.TrimSpace(req.Passcode) != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Passcode), a.Cfg.BcryptCost)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, APIError{Code: "hash", Message: "passcode hash failed"})
			return
		}
		hash = string(h)
	}
	now := timeutil.Now()
	b := model.Board{
		ID: model.NewID("b"), Title: title, PassHash: hash,
		CreatedAt: now, UpdatedAt: now, HasPass: hash != "",
	}
	if err := a.DB.InsertBoard(r.Context(), b); err != nil {
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "db", Message: "create failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": b.ID, "title": b.Title, "hasPass": b.HasPass,
		"createdAt": timeutil.Format(b.CreatedAt),
		"updatedAt": timeutil.Format(b.UpdatedAt),
	})
}

func (a BoardAPI) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := a.DB.GetBoard(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, APIError{Code: "not_found", Message: "board not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "db", Message: "get failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": b.ID, "title": b.Title, "hasPass": b.HasPass, "thumbnail": b.Thumbnail,
		"createdAt": timeutil.Format(b.CreatedAt),
		"updatedAt": timeutil.Format(b.UpdatedAt),
	})
}

func (a BoardAPI) Patch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req patchBoardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_json", Message: "invalid body"})
		return
	}
	pass := "__keep__"
	if req.ClearPass {
		pass = ""
	} else if strings.TrimSpace(req.Passcode) != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Passcode), a.Cfg.BcryptCost)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, APIError{Code: "hash", Message: "passcode hash failed"})
			return
		}
		pass = string(h)
	}
	if err := a.DB.UpdateBoard(r.Context(), id, strings.TrimSpace(req.Title), pass, req.Thumbnail); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, APIError{Code: "not_found", Message: "board not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "db", Message: "update failed"})
		return
	}
	a.Get(w, r)
}

func (a BoardAPI) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.DB.DeleteBoard(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, APIError{Code: "not_found", Message: "board not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "db", Message: "delete failed"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a BoardAPI) Unlock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req unlockReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIError{Code: "bad_json", Message: "invalid body"})
		return
	}
	b, err := a.DB.GetBoard(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, APIError{Code: "not_found", Message: "board not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIError{Code: "db", Message: "get failed"})
		return
	}
	if b.PassHash != "" && bcrypt.CompareHashAndPassword([]byte(b.PassHash), []byte(req.Passcode)) != nil {
		writeJSON(w, http.StatusForbidden, APIError{Code: "forbidden", Message: "passcode rejected"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": b.ID})
}
