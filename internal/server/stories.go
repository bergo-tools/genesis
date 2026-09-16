package server

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zp/genesis/internal/store"
)

type storyInput struct {
	Title       *string          `json:"title"`
	Avatar      *string          `json:"avatar"`
	Description *string          `json:"description"`
	Genre       *string          `json:"genre"`
	Model       *string          `json:"model"`
	Opening     *string          `json:"opening"`
	Persona     *store.Persona   `json:"persona"`
	Characters  []characterInput `json:"characters"`
	Settings    *settingsInput   `json:"settings"`
	State       map[string]any   `json:"state"`
	Scene       *store.Scene     `json:"scene"`
}

func applyStoryInput(st *store.Story, in *storyInput) {
	if st == nil || in == nil {
		return
	}
	if in.Title != nil {
		st.Title = strings.TrimSpace(*in.Title)
	}
	if in.Avatar != nil {
		st.Avatar = strings.TrimSpace(*in.Avatar)
	}
	if in.Description != nil {
		st.Description = strings.TrimSpace(*in.Description)
	}
	if in.Genre != nil {
		st.Genre = strings.TrimSpace(*in.Genre)
	}
	if in.Model != nil {
		st.Model = strings.TrimSpace(*in.Model)
	}
	if in.Opening != nil {
		st.Opening = strings.TrimSpace(*in.Opening)
	}
	if in.Persona != nil {
		st.Persona = *in.Persona
	}
	if in.Characters != nil {
		chars := make([]*store.Character, 0, len(in.Characters))
		for _, c := range in.Characters {
			if cc := toCharacter(c); cc != nil {
				chars = append(chars, cc)
			}
		}
		st.Characters = chars
	}
	if in.State != nil {
		st.State = in.State
	}
	if in.Scene != nil {
		st.Scene = *in.Scene
	}
	applySettings(&st.Settings, in.Settings)
}

func (s *Server) handleListStories(w http.ResponseWriter, _ *http.Request) {
	stories, err := s.stories.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(stories))
	for _, st := range stories {
		out = append(out, map[string]any{
			"id":             st.ID,
			"title":          st.Title,
			"avatar":         st.Avatar,
			"description":    st.Description,
			"genre":          st.Genre,
			"opening":        st.Opening,
			"builtin":        st.Builtin,
			"characterCount": len(st.Characters),
			"updatedAt":      st.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"stories": out})
}

func (s *Server) handleGetStory(w http.ResponseWriter, r *http.Request) {
	st, err := s.stories.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleCreateStory(w http.ResponseWriter, r *http.Request) {
	var body storyInput
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := &store.Story{Settings: defaultSettings(s.cfg.Get()), State: map[string]any{}}
	applyStoryInput(st, &body)
	if st.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	if err := s.stories.Create(st); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (s *Server) handlePatchStory(w http.ResponseWriter, r *http.Request) {
	st, err := s.stories.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	var body storyInput
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	applyStoryInput(st, &body)
	if err := s.stories.Save(st); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDeleteStory(w http.ResponseWriter, r *http.Request) {
	if err := s.stories.Delete(r.PathValue("id")); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleStoryAssetUpload(w http.ResponseWriter, r *http.Request) {
	s.handleAssetUploadTo(w, r, s.stories.SaveAsset, "/api/stories/")
}

func (s *Server) handleStoryAssetGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	serveAssetFile(w, name, func() (io.ReadCloser, error) { return s.stories.OpenAsset(id, name) })
}

// handleAssetUploadTo is shared by session and story image uploads.
func (s *Server) handleAssetUploadTo(w http.ResponseWriter, r *http.Request, save func(string, io.Reader, string) (string, error), prefix string) {
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxAssetBytes+(1<<20))
	if err := r.ParseMultipartForm(store.MaxAssetBytes + (1 << 20)); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("expected a multipart image upload"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("missing form field \"file\""))
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	name, err := save(id, file, ext)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"name": name,
		"url":  prefix + id + "/assets/" + name,
	})
}

// serveAssetFile streams an asset with the right content type.
func serveAssetFile(w http.ResponseWriter, name string, open func() (io.ReadCloser, error)) {
	f, err := open()
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", mimeForAsset(name))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}
