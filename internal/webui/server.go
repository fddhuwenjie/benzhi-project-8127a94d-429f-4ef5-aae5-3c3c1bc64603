package webui

import (
	"io/fs"
	"net/http"
	"strings"

	"oralhistory/internal/application"
)

type Server struct {
	app    *application.Service
	static http.Handler
}

func NewServer(app *application.Service) *Server {
	staticFS, _ := fs.Sub(assets, "static")
	return &Server{app: app, static: http.FileServer(http.FS(staticFS))}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.route)
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		s.HandleIndex(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/static/") {
		http.StripPrefix("/static/", s.static).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/healthz" {
		s.HandleHealth(w, r)
		return
	}
	if r.URL.Path == "/api/cases" {
		if r.Method == http.MethodGet {
			s.HandleListCases(w, r)
		} else if r.Method == http.MethodPost {
			s.HandleCreateCase(w, r)
		} else {
			methodNotAllowed(w)
		}
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/cases/") {
		s.routeCase(w, r)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "路由不存在")
}

func (s *Server) routeCase(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/api/cases/")
	parts := strings.Split(strings.Trim(remainder, "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		s.HandleGetCase(w, r, parts[0])
		return
	}
	if len(parts) != 2 || r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	caseID, action := parts[0], parts[1]
	switch action {
	case "freeze":
		s.HandleFreeze(w, r, caseID)
	case "segments":
		s.HandleSaveSegment(w, r, caseID)
	case "constraints":
		s.HandleSaveConstraint(w, r, caseID)
	case "registration-batch":
		s.HandleRegisterBatch(w, r, caseID)
	case "check-conflicts":
		s.HandleCheckConflicts(w, r, caseID)
	case "remediate":
		s.HandleRemediate(w, r, caseID)
	case "candidate":
		s.HandleGenerateCandidate(w, r, caseID)
	case "submit-review":
		s.HandleSubmitReview(w, r, caseID)
	case "review":
		s.HandleReview(w, r, caseID)
	case "verify":
		s.HandleVerify(w, r, caseID)
	case "manifest-query":
		s.HandleManifestQuery(w, r, caseID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "案件操作不存在")
	}
}
