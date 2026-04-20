package model

import "time"

const MaxRetries = 3

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type Job struct {
	ID string `json:"id"`

	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	FileKey  string `json:"file_key"`

	Status JobStatus `json:"status"`
	Result string    `json:"result,omitempty"`
	Error  string    `json:"error,omitempty"`

	RetryCount int `json:"retry_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateJobRequest struct {
	FileName string
	FileSize int64
	FileData []byte
}

type JobResponse struct {
	ID         string    `json:"id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	Status     JobStatus `json:"status"`
	RetryCount int       `json:"retry_count"`
	Result     string    `json:"result,omitempty"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (j *Job) ToResponse() JobResponse {
	return JobResponse{
		ID:         j.ID,
		FileName:   j.FileName,
		FileSize:   j.FileSize,
		Status:     j.Status,
		RetryCount: j.RetryCount,
		Result:     j.Result,
		Error:      j.Error,
		CreatedAt:  j.CreatedAt,
		UpdatedAt:  j.UpdatedAt,
	}
}
