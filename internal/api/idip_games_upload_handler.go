package api

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"starcrystal/server/internal/service"
)

func (s *Server) handleIdipGamesH5Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	const maxBody = 60 << 20
	if err := r.ParseMultipartForm(maxBody); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid multipart form"})
		return
	}
	metaRaw := strings.TrimSpace(r.FormValue("meta"))
	if metaRaw == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "meta required"})
		return
	}
	meta, err := service.ParseH5UploadMetaJSON([]byte(metaRaw))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid meta json"})
		return
	}

	uploadMode := strings.TrimSpace(r.FormValue("uploadMode"))
	var result service.H5UploadResult
	if uploadMode == "folder" {
		entries, readErr := readH5FolderMultipart(r, maxBody)
		if readErr != nil {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: readErr.Error()})
			return
		}
		result, err = service.ProcessH5FolderUpload(entries, meta)
	} else {
		file, header, fileErr := r.FormFile("file")
		if fileErr != nil {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "file required"})
			return
		}
		defer file.Close()
		data, readErr := io.ReadAll(io.LimitReader(file, maxBody))
		if readErr != nil {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "read file failed"})
			return
		}
		result, err = service.ProcessH5Upload(data, header.Filename, meta)
	}
	if h5UploadError(w, s, err) {
		return
	}
	service.DefaultAuditRecorder.Record(r.Context(), idipUsername(r), "h5_upload", meta.GameID, map[string]any{
		"gameDirName":     result.GameDirName,
		"minigameVersion": result.MinigameVersion,
		"configVersion":   result.ConfigVersion,
		"packageBytes":    result.PackageBytes,
		"uploadMode":      uploadMode,
	})
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: result})
}

func readH5FolderMultipart(r *http.Request, maxBody int64) ([]service.H5FolderEntry, error) {
	if r.MultipartForm == nil {
		return nil, errors.New("folder files required")
	}
	headers := r.MultipartForm.File["files"]
	if len(headers) == 0 {
		return nil, errors.New("folder files required")
	}
	var total int64
	out := make([]service.H5FolderEntry, 0, len(headers))
	for _, header := range headers {
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(file, maxBody-total))
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		total += int64(len(data))
		if total > maxBody {
			return nil, service.ErrH5PackageTooLarge
		}
		rel := strings.TrimSpace(header.Filename)
		if rel == "" {
			return nil, errors.New("invalid folder file path")
		}
		out = append(out, service.H5FolderEntry{RelPath: rel, Data: data})
	}
	return out, nil
}

func h5UploadError(w http.ResponseWriter, s *Server, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrH5ZipNameMismatch),
		errors.Is(err, service.ErrH5ZipInvalidLayout),
		errors.Is(err, service.ErrH5TarNameMismatch),
		errors.Is(err, service.ErrH5TarInvalidLayout),
		errors.Is(err, service.ErrH5TarVersionMismatch),
		errors.Is(err, service.ErrH5UnsupportedPackage),
		errors.Is(err, service.ErrH5FolderInvalidLayout),
		errors.Is(err, service.ErrH5FolderEmpty),
		errors.Is(err, service.ErrH5MissingBootstrap),
		errors.Is(err, service.ErrH5GameDirConflict),
		errors.Is(err, service.ErrH5GameDirMismatch),
		errors.Is(err, service.ErrH5VersionNotIncreased),
		errors.Is(err, service.ErrH5PackageTooLarge):
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
	default:
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
	}
	return true
}

func (s *Server) handleIdipAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	gameID := strings.TrimSpace(r.URL.Query().Get("gameId"))
	total, list := service.ListAuditLogs(limit, offset, action, gameID)
	s.writeJSON(w, http.StatusOK, Response{
		Code: 0, Message: "ok",
		Data: map[string]any{
			"total": total,
			"list":  list,
		},
	})
}
