package http

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// maxUploadBody bounds the whole multipart request.
const maxUploadBody = domain.MaxDocumentBytes + 1<<20

// uploadDocument streams the multipart body into memory. It deliberately
// avoids ParseMultipartForm, which spills large files to temp files on disk:
// documents must never be written anywhere.
func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
	mr, err := r.MultipartReader()
	if err != nil {
		s.writeError(w, r, domain.Invalid("expected a multipart/form-data upload"))
		return
	}
	cmd := app.UploadCommand{OwnerID: mustUser(r).ID}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			s.writeError(w, r, uploadErr(err))
			return
		}
		if err := readPart(part, &cmd); err != nil {
			s.writeError(w, r, uploadErr(err))
			return
		}
	}
	if cmd.Data == nil {
		s.writeError(w, r, domain.Invalid("a PDF file is required"))
		return
	}
	res, err := s.cfg.Generation.UploadDocument(r.Context(), cmd)
	cmd.Data = nil // drop our reference; the queue owns the only copy now
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": res.JobID.String(), "chapterId": res.ChapterID.String(), "topicId": res.TopicID.String()})
}

func readPart(part *multipart.Part, cmd *app.UploadCommand) error {
	defer part.Close()
	if part.FormName() == "file" {
		data, err := io.ReadAll(io.LimitReader(part, domain.MaxDocumentBytes+1))
		if err != nil {
			return fmt.Errorf("read file part: %w", err)
		}
		cmd.Filename, cmd.Data = part.FileName(), data
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(part, 4096))
	if err != nil {
		return fmt.Errorf("read form field: %w", err)
	}
	v := strings.TrimSpace(string(b))
	switch part.FormName() {
	case "topicId":
		cmd.TopicID = domain.ID(v)
	case "newTopicName":
		cmd.NewTopicName = v
	case "title":
		cmd.Title = v
	case "count":
		n, err := strconv.Atoi(v)
		if err != nil {
			return domain.Invalid("count must be a number")
		}
		cmd.Count = n
	}
	return nil
}

func uploadErr(err error) error {
	var mbe *http.MaxBytesError
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &mbe):
		return domain.Invalid("document must be at most 15 MB")
	case errors.As(err, &ve):
		return err
	default:
		return domain.Invalid("could not read the upload")
	}
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	j, err := s.cfg.Generation.GetJob(r.Context(), mustUser(r).ID, idParam(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toJobDTO(j))
}
