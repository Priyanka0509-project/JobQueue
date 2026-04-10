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
	
	ID string `bson:"_id" json:"id"`

	FileName string    `bson:"file_name" json:"file_name"`
	FileSize int64     `bson:"file_size" json:"file_size"`
	Status   JobStatus `bson:"status"    json:"status"`
	Result   string    `bson:"result"    json:"result,omitempty"`
	Error    string    `bson:"error"     json:"error,omitempty"`

	
	RetryCount int `bson:"retry_count" json:"retry_count"`

	

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
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