package http

import (
	"net/http"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// scopeRequest names an attempt scope: exactly one of chapterId and topicId is set.
type scopeRequest struct {
	ChapterID string `json:"chapterId"`
	TopicID   string `json:"topicId"`
}

func (r scopeRequest) scope() (domain.AttemptScope, error) {
	switch {
	case r.ChapterID != "" && r.TopicID == "":
		return domain.ChapterScope(domain.ID(r.ChapterID)), nil
	case r.TopicID != "" && r.ChapterID == "":
		return domain.TopicScope(domain.ID(r.TopicID)), nil
	default:
		return domain.AttemptScope{}, domain.Invalid("exactly one of chapterId and topicId is required")
	}
}

type submitQuizRequest struct {
	scopeRequest
	QuestionIDs []string `json:"questionIds"`
	Answers     []struct {
		QuestionID string `json:"questionId"`
		Selected   []int  `json:"selected"`
	} `json:"answers"`
}

type drawQuizRequest struct {
	scopeRequest
	Count int `json:"count"`
}

// drawnQuizDTO never carries correct answers: the quiz is still being taken.
type drawnQuizDTO struct {
	scopeDTO
	Questions []questionDTO `json:"questions"`
}

func (s *Server) drawQuiz(w http.ResponseWriter, r *http.Request) {
	var req drawQuizRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	scope, err := req.scope()
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	q, err := s.cfg.Practice.DrawQuiz(r.Context(), mustUser(r).ID, scope, req.Count)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := drawnQuizDTO{scopeDTO: toScopeDTO(q.Scope, q.ScopeName), Questions: make([]questionDTO, len(q.Questions))}
	for i, dq := range q.Questions {
		out.Questions[i] = toScopedQuestionDTO(dq, false)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) submitQuiz(w http.ResponseWriter, r *http.Request) {
	var req submitQuizRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	scope, err := req.scope()
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	ids := make([]domain.ID, len(req.QuestionIDs))
	for i, id := range req.QuestionIDs {
		ids[i] = domain.ID(id)
	}
	responses := make(map[domain.ID][]int, len(req.Answers))
	for _, a := range req.Answers {
		responses[domain.ID(a.QuestionID)] = a.Selected
	}
	v, err := s.cfg.Practice.SubmitQuiz(r.Context(), mustUser(r).ID, scope, ids, responses)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAttemptDTO(v))
}

func (s *Server) getAttempt(w http.ResponseWriter, r *http.Request) {
	v, err := s.cfg.Practice.GetAttempt(r.Context(), mustUser(r).ID, idParam(r))
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
			attemptSummaryDTO: toAttemptSummary(a.Attempt),
			ChapterTitle:      a.ChapterTitle,
			TopicID:           a.TopicID.String(),
			TopicName:         a.TopicName,
			ScopeDeleted:      a.ScopeDeleted,
			ChapterCount:      a.ChapterCount,
		}
	}
	writeJSON(w, http.StatusOK, out)
}
