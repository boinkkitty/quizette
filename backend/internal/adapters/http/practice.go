package http

import (
	"net/http"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

type startAttemptRequest struct {
	Count int `json:"count"`
}

type submitAttemptRequest struct {
	Answers []struct {
		QuestionID string `json:"questionId"`
		Selected   []int  `json:"selected"`
	} `json:"answers"`
}

func (s *Server) startAttempt(w http.ResponseWriter, r *http.Request) {
	var req startAttemptRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	v, err := s.cfg.Practice.StartChapterAttempt(r.Context(), mustUser(r).ID, idParam(r), req.Count)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAttemptDTO(v))
}

func (s *Server) submitAttempt(w http.ResponseWriter, r *http.Request) {
	var req submitAttemptRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	responses := make(map[domain.ID][]int, len(req.Answers))
	for _, a := range req.Answers {
		responses[domain.ID(a.QuestionID)] = a.Selected
	}
	v, err := s.cfg.Practice.SubmitChapterAttempt(r.Context(), mustUser(r).ID, idParam(r), responses)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAttemptDTO(v))
}

func (s *Server) getAttempt(w http.ResponseWriter, r *http.Request) {
	v, err := s.cfg.Practice.GetChapterAttempt(r.Context(), mustUser(r).ID, idParam(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAttemptDTO(v))
}

func (s *Server) listAttempts(w http.ResponseWriter, r *http.Request) {
	attempts, err := s.cfg.Practice.ListChapterAttempts(r.Context(), mustUser(r).ID, idParam(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := make([]attemptSummaryDTO, len(attempts))
	for i, a := range attempts {
		out[i] = toAttemptSummary(a)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listAllAttempts(w http.ResponseWriter, r *http.Request) {
	attempts, err := s.cfg.Practice.ListAttempts(r.Context(), mustUser(r).ID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := make([]attemptHistoryItemDTO, len(attempts))
	for i, a := range attempts {
		out[i] = attemptHistoryItemDTO{
			attemptSummaryDTO: toAttemptSummary(a.ChapterAttempt),
			ChapterTitle:      a.ChapterTitle,
			TopicID:           a.TopicID.String(),
			TopicName:         a.TopicName,
		}
	}
	writeJSON(w, http.StatusOK, out)
}
