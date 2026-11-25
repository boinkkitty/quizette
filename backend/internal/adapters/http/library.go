package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

func idParam(r *http.Request) domain.ID { return domain.ID(chi.URLParam(r, "id")) }

type nameRequest struct {
	Name string `json:"name"`
}

type titleRequest struct {
	Title string `json:"title"`
}

func (s *Server) listTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := s.cfg.Library.ListTopics(r.Context(), mustUser(r).ID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := make([]topicDTO, len(topics))
	for i, t := range topics {
		out[i] = topicDTO{
			ID: t.ID.String(), Name: t.Name, ChapterCount: t.ChapterCount, CreatedAt: t.CreatedAt,
			AttemptCount: t.AttemptCount, LastScorePct: t.LastScorePct, BestScorePct: t.BestScorePct,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createTopic(w http.ResponseWriter, r *http.Request) {
	var req nameRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	t, err := s.cfg.Library.CreateTopic(r.Context(), mustUser(r).ID, req.Name)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, topicDTO{ID: t.ID.String(), Name: t.Name, CreatedAt: t.CreatedAt})
}

func (s *Server) getTopic(w http.ResponseWriter, r *http.Request) {
	d, err := s.cfg.Library.GetTopic(r.Context(), mustUser(r).ID, idParam(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	chapters := make([]chapterSummaryDTO, len(d.Chapters))
	for i, c := range d.Chapters {
		chapters[i] = toChapterSummary(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"topic":    topicDTO{ID: d.Topic.ID.String(), Name: d.Topic.Name, ChapterCount: len(d.Chapters), CreatedAt: d.Topic.CreatedAt},
		"chapters": chapters,
	})
}

func (s *Server) renameTopic(w http.ResponseWriter, r *http.Request) {
	var req nameRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	t, err := s.cfg.Library.RenameTopic(r.Context(), mustUser(r).ID, idParam(r), req.Name)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, topicDTO{ID: t.ID.String(), Name: t.Name, CreatedAt: t.CreatedAt})
}

func (s *Server) deleteTopic(w http.ResponseWriter, r *http.Request) {
	if err := s.cfg.Library.DeleteTopic(r.Context(), mustUser(r).ID, idParam(r)); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listChapters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := app.ChapterFilter{TopicID: domain.ID(q.Get("topicId")), Status: domain.ChapterStatus(q.Get("status"))}
	chapters, err := s.cfg.Library.ListChapters(r.Context(), mustUser(r).ID, filter)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := make([]chapterSummaryDTO, len(chapters))
	for i, c := range chapters {
		out[i] = toChapterSummary(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getChapter(w http.ResponseWriter, r *http.Request) {
	d, err := s.cfg.Library.GetChapter(r.Context(), mustUser(r).ID, idParam(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toChapterDTO(d))
}

func (s *Server) renameChapter(w http.ResponseWriter, r *http.Request) {
	var req titleRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if _, err := s.cfg.Library.RenameChapter(r.Context(), mustUser(r).ID, idParam(r), req.Title); err != nil {
		s.writeError(w, r, err)
		return
	}
	s.getChapter(w, r)
}

func (s *Server) deleteChapter(w http.ResponseWriter, r *http.Request) {
	if err := s.cfg.Library.DeleteChapter(r.Context(), mustUser(r).ID, idParam(r)); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getQuestionSet(w http.ResponseWriter, r *http.Request) {
	set, err := s.cfg.Library.GetQuestionSet(r.Context(), mustUser(r).ID, idParam(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	qs := make([]questionDTO, len(set.Questions))
	for i, q := range set.Questions {
		qs[i] = toQuestionDTO(q, true)
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": set.ID.String(), "chapterId": set.ChapterID.String(), "questions": qs})
}
