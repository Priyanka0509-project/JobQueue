package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"jobQueue/model"
	"jobQueue/service"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type JobController struct {
	service *service.JobService
}

func NewJobController(svc *service.JobService, mux *http.ServeMux) *JobController {
	c := &JobController{service: svc}

	mux.HandleFunc("/job/{id}", c.GetJob)
	mux.HandleFunc("/jobs", c.ListJobs)
	mux.HandleFunc("/health", c.Health)
	return c
}

// Upload godoc
// @Summary Upload a file (.txt, .pdf, .csv)
// @Description Upload a file (.txt, .pdf, or .csv) for processing in the job queue
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File to upload"
// @Success 200 {object} model.JobResponse
// @Failure 400 {object} map[string]string
// @Router /upload [post]
func (c *JobController) Upload(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		writeError(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, "cannot read multipart form", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// w.WriteHeader(http.StatusOK)

	flusher, canFlush := w.(http.Flusher)

	sendProgress := func(percent int, message string) {
		fmt.Fprintf(w, `{"progress":%d,"message":"%s"}`+"\n", percent, message)
		if canFlush {
			flusher.Flush()
		}
	}

	sendProgress(0, "Starting upload...")

	part, err := mr.NextPart()
	if err != nil {
		writeError(w, "error reading file", http.StatusBadRequest)
		return
	}

	if part.FormName() != "file" {
		writeError(w, "file field missing", http.StatusBadRequest)
		return
	}

	filename := part.FileName()

	ext := strings.ToLower(filepath.Ext(filename))
	allowed := map[string]bool{
		".txt": true,
		".pdf": true,
		".csv": true,
	}

	if !allowed[ext] {
		writeError(w, fmt.Sprintf("unsupported file type: %s", ext), http.StatusBadRequest)
		return
	}

	totalSize := r.ContentLength
	if totalSize <= 0 {
		totalSize = 50 * 1024 * 1024 // fallback
	}

	var buf bytes.Buffer
	chunk := make([]byte, 32*1024)

	var bytesRead int64
	lastReportTime := time.Now()

	for {
		n, err := part.Read(chunk)

		if n > 0 {
			buf.Write(chunk[:n])
			bytesRead += int64(n)

			if totalSize > 0 {
				percent := int(bytesRead * 100 / totalSize)
				if percent > 100 {
					percent = 100
				}

				// send progress every 1 second
				if time.Since(lastReportTime) >= 3*time.Second {
					sendProgress(percent,
						fmt.Sprintf("Uploading... %d%%", percent))
					lastReportTime = time.Now()
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			sendProgress(0, fmt.Sprintf("upload failed: %v", err))
			return
		}
	}

	sendProgress(100, "Upload complete — creating job...")

	req := model.CreateJobRequest{
		FileName: filename,
		FileSize: int64(len(buf.Bytes())),
		FileData: buf.Bytes(),
	}

	fmt.Println("DEBUG FILE SIZE:", len(buf.Bytes()))

	response, err := c.service.CreateJob(req)
	if err != nil {
		sendProgress(0, fmt.Sprintf("failed to create job: %v", err))
		return
	}

	finalJSON, _ := json.Marshal(response)
	fmt.Fprintln(w, string(finalJSON))
	if canFlush {
		flusher.Flush()
	}

	lastStatus := response.Status

	for {
		select {
		case <-r.Context().Done():
			return

		case <-time.After(300 * time.Millisecond):

			updatedJob, err := c.service.GetJobByID(response.ID)
			if err != nil {
				return
			}

			if updatedJob.Status != lastStatus {
				lastStatus = updatedJob.Status

				statusJSON, _ := json.Marshal(updatedJob)
				fmt.Fprintln(w, string(statusJSON))

				if canFlush {
					flusher.Flush()
				}
			}

			if updatedJob.Status == model.StatusCompleted ||
				updatedJob.Status == model.StatusFailed {
				return
			}
		}
	}
}

// GetJob godoc
// @Summary Get a job by ID
// @Description Fetches the status and results of a text processing job
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} model.JobResponse
// @Failure 404 {object} map[string]string
// @Router /job/{id} [get]
func (c *JobController) GetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/job/")
	if id == "" {
		writeError(w, "job ID required in URL: /job/{id}", http.StatusBadRequest)
		return
	}
	response, err := c.service.GetJobByID(id)
	if err != nil {
		writeError(w, "job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, response, http.StatusOK)
}

// ListJobs godoc
// @Summary List all jobs
// @Description Fetches a list of all jobs currently in the system
// @Produce json
// @Success 200 {array} model.JobResponse
// @Failure 500 {object} map[string]string
// @Router /jobs [get]
func (c *JobController) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	responses, err := c.service.GetAllJobs()
	if err != nil {
		writeError(w, "failed to fetch jobs", http.StatusInternalServerError)
		return
	}
	writeJSON(w, responses, http.StatusOK)
}

// Health godoc
// @Summary Check server health
// @Description Returns an OK status if the server is running
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (c *JobController) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func writeJSON(w http.ResponseWriter, data any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	writeJSON(w, map[string]string{"error": msg}, code)
}
