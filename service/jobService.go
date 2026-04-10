package service

import (
	"bufio"
	"bytes"
	"fmt"
	"jobQueue/model"
	"jobQueue/queue"
	"jobQueue/repository"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

const (
	// --- VISUALIZATION SETTINGS ---
	// Set to 0 for maximum production speed.
	// Adds a fake delay so the "processing" state becomes slow enough to be polled by the API and displayed
	ProcessingVisualizationDelay = 2 * time.Second
)

type JobService struct {
	repo  *repository.JobRepository
	queue *queue.Queue
}

func NewJobService(repo *repository.JobRepository, b *queue.Queue) *JobService {
	return &JobService{repo: repo, queue: b}
}

func (s *JobService) CreateJob(req model.CreateJobRequest) (*model.JobResponse, error) {
	jobID := uuid.New().String()
	now := time.Now()

	job := &model.Job{
		ID:         jobID,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		Status:     model.StatusPending,
		RetryCount: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Save(job); err != nil {
		return nil, fmt.Errorf("failed to save job: %w", err)
	}

	if err := s.repo.SaveFileData(jobID, req.FileData); err != nil {
		return nil, fmt.Errorf("failed to save file data: %w", err)
	}

	s.queue.Publish(job)

	resp := job.ToResponse()
	return &resp, nil
}

func (s *JobService) GetJobByID(id string) (*model.JobResponse, error) {
	job, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	resp := job.ToResponse()
	return &resp, nil
}

func (s *JobService) GetAllJobs() ([]model.JobResponse, error) {
	jobs, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	responses := make([]model.JobResponse, 0, len(jobs))
	for _, job := range jobs {
		responses = append(responses, job.ToResponse())
	}
	return responses, nil
}

func (s *JobService) ProcessFile(job *model.Job) error {
	s.repo.UpdateStatus(job.ID, model.StatusProcessing, "", "", job.RetryCount)
	if ProcessingVisualizationDelay > 0 {
		time.Sleep(ProcessingVisualizationDelay)
	}

	fileData, err := s.repo.FindFileData(job.ID)
	if err != nil {
		return s.handleFailure(job, fmt.Sprintf("cannot read file from MongoDB: %v", err))
	}

	var scanner *bufio.Scanner

	if strings.HasSuffix(strings.ToLower(job.FileName), ".pdf") {
		pdfReader, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
		if err != nil {
			return s.handleFailure(job, fmt.Sprintf("cannot parse pdf: %v", err))
		}
		b, err := pdfReader.GetPlainText()
		if err != nil {
			return s.handleFailure(job, fmt.Sprintf("cannot extract text from pdf: %v", err))
		}
		scanner = bufio.NewScanner(b)
	} else {
		reader := bytes.NewReader(fileData)
		scanner = bufio.NewScanner(reader)
	}

	var lineCount, wordCount, charCount int
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		wordCount += len(strings.Fields(line))
		charCount += len(line) + 1
	}

	if err := scanner.Err(); err != nil {
		return s.handleFailure(job, fmt.Sprintf("error reading file: %v", err))
	}

	result := fmt.Sprintf(
		"Lines: %d | Words: %d | Characters: %d | Size: %d bytes",
		lineCount, wordCount, charCount, job.FileSize,
	)
	s.repo.UpdateStatus(job.ID, model.StatusCompleted, result, "", job.RetryCount)
	fmt.Printf("[Service] Job %s completed: %s\n", job.ID, result)
	return nil
}

func (s *JobService) handleFailure(job *model.Job, errMsg string) error {
	if job.RetryCount < model.MaxRetries {
		job.RetryCount++

		waitTime := time.Duration(job.RetryCount) * time.Second
		fmt.Printf("[Service] Job %s failed, retry %d/%d in %v\n",
			job.ID, job.RetryCount, model.MaxRetries, waitTime)
		time.Sleep(waitTime)

		retryMsg := fmt.Sprintf("retry %d/%d: %s", job.RetryCount, model.MaxRetries, errMsg)
		s.repo.UpdateStatus(job.ID, model.StatusPending, "", retryMsg, job.RetryCount)

		s.queue.Publish(job)

		return nil
	}

	fmt.Printf("[Service] Job %s PERMANENTLY FAILED after %d retries\n",
		job.ID, model.MaxRetries)
	s.repo.UpdateStatus(job.ID, model.StatusFailed, "", errMsg, job.RetryCount)
	return fmt.Errorf("job failed after %d retries: %s", model.MaxRetries, errMsg)
}
