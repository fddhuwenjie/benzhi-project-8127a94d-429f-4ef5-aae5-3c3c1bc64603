package webui

import (
	"net/http"

	"oralhistory/internal/application"
)

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	content, err := assets.ReadFile("static/index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "asset_error", "页面资源不可用")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) HandleListCases(w http.ResponseWriter, r *http.Request) {
	result, err := s.app.ListCases()
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cases": result})
}

func (s *Server) HandleCreateCase(w http.ResponseWriter, r *http.Request) {
	var cmd application.CreateCaseCommand
	if !decodeJSON(w, r, &cmd) {
		return
	}
	result, err := s.app.CreateCase(cmd)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, application.CommandResult{Case: result})
}

func (s *Server) HandleGetCase(w http.ResponseWriter, r *http.Request, caseID string) {
	result, err := s.app.GetWorkbench(caseID)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleFreeze(w http.ResponseWriter, r *http.Request, caseID string) {
	var meta application.CommandMeta
	if !decodeJSON(w, r, &meta) {
		return
	}
	meta.CaseID = caseID
	result, err := s.app.FreezeBaseline(meta)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleSaveSegment(w http.ResponseWriter, r *http.Request, caseID string) {
	var cmd application.AddSegmentCommand
	if !decodeJSON(w, r, &cmd) {
		return
	}
	cmd.CaseID = caseID
	result, err := s.app.AddSegment(cmd)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleSaveConstraint(w http.ResponseWriter, r *http.Request, caseID string) {
	var cmd application.AddConstraintCommand
	if !decodeJSON(w, r, &cmd) {
		return
	}
	cmd.CaseID = caseID
	result, err := s.app.AddConstraint(cmd)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleRegisterBatch(w http.ResponseWriter, r *http.Request, caseID string) {
	var cmd application.BatchRegistrationCommand
	if !decodeJSON(w, r, &cmd) {
		return
	}
	cmd.CaseID = caseID
	result, err := s.app.RegisterBatchContext(r.Context(), cmd)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleCheckConflicts(w http.ResponseWriter, r *http.Request, caseID string) {
	var meta application.CommandMeta
	if !decodeJSON(w, r, &meta) {
		return
	}
	meta.CaseID = caseID
	result, err := s.app.CheckConflicts(meta)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleRemediate(w http.ResponseWriter, r *http.Request, caseID string) {
	var cmd application.RemediateCommand
	if !decodeJSON(w, r, &cmd) {
		return
	}
	cmd.CaseID = caseID
	if cmd.Preview {
		result, err := s.app.PreviewRemediation(cmd)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	result, err := s.app.Remediate(cmd)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleGenerateCandidate(w http.ResponseWriter, r *http.Request, caseID string) {
	var meta application.CommandMeta
	if !decodeJSON(w, r, &meta) {
		return
	}
	meta.CaseID = caseID
	result, err := s.app.GenerateCandidate(meta)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleSubmitReview(w http.ResponseWriter, r *http.Request, caseID string) {
	var meta application.CommandMeta
	if !decodeJSON(w, r, &meta) {
		return
	}
	meta.CaseID = caseID
	result, err := s.app.SubmitReview(meta)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleReview(w http.ResponseWriter, r *http.Request, caseID string) {
	var cmd application.ReviewCommand
	if !decodeJSON(w, r, &cmd) {
		return
	}
	cmd.CaseID = caseID
	result, err := s.app.DecideReview(cmd)
	writeCommandResult(w, result, err)
}

func (s *Server) HandleVerify(w http.ResponseWriter, r *http.Request, caseID string) {
	result, err := s.app.Verify(caseID)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleManifestQuery(w http.ResponseWriter, r *http.Request, caseID string) {
	var query application.ManifestQuery
	if !decodeJSON(w, r, &query) {
		return
	}
	result, err := s.app.QueryManifest(caseID, query)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeCommandResult(w http.ResponseWriter, result any, err error) {
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"case": result})
}
