package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	storage "jobQueue/cloudinary"
	"jobQueue/model"
	"jobQueue/queue"
	"jobQueue/repository"
	"log"
	"strings"
	"time"

	pb "jobQueue/proto"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
	"google.golang.org/grpc"
)

type JobService struct {
	repo       *repository.JobRepository
	queue      *queue.Queue
	cloud      *storage.CloudinaryClient
	grpcClient pb.NotificationServiceClient
}

func NewJobService(repo *repository.JobRepository, q *queue.Queue, cloud *storage.CloudinaryClient) *JobService {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Println("gRPC connection error:", err)
	}

	client := pb.NewNotificationServiceClient(conn)

	return &JobService{
		repo:       repo,
		queue:      q,
		cloud:      cloud,
		grpcClient: client,
	}
}

func (s *JobService) sendEvent(
	stream pb.NotificationService_StreamEventsClient,
	job *model.Job,
	stage string,
	jobtype string,
	message string,

) {

	err := stream.Send(&pb.JobEvent{
		JobId:      job.ID,
		Stage:      stage,
		RetryCount: int32(job.RetryCount),
		Message:    message,
		FileName:   job.FileName,
		JobType:    jobtype,
	})

	if err != nil {
		log.Println("failed to send event:", err)
	}
}

func (s *JobService) CreateJob(req model.CreateJobRequest) (*model.JobResponse, error) {
	jobID := uuid.New().String()
	now := time.Now()

	fileURL, err := s.cloud.Upload(req.FileData, jobID+"_"+req.FileName)
	if err != nil {
		return nil, err
	}

	job := &model.Job{
		ID:         jobID,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		FileKey:    fileURL,
		Status:     model.StatusPending,
		RetryCount: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Save(job); err != nil {
		return nil, fmt.Errorf("failed to save job: %w", err)
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
	stream, err := s.grpcClient.StreamEvents(context.Background())
	if err != nil {
		log.Println("stream error:", err)
		return err
	}
	s.sendEvent(stream, job, "processing", "INFO", "processing started")

	s.repo.UpdateStatus(job.ID, model.StatusProcessing, "", "", job.RetryCount)
	time.Sleep(2 * time.Second)

	fileData, err := s.cloud.Download(job.FileKey)
	if err != nil {
		s.sendEvent(stream, job, "retry", "INFO", "download failed")
		return s.handleFailure(job, fmt.Sprintf("cannot download file from Cloudinary: %v", err), stream)
	}

	var scanner *bufio.Scanner

	if strings.HasSuffix(strings.ToLower(job.FileName), ".pdf") {
		pdfReader, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
		if err != nil {
			s.sendEvent(stream, job, "retry", "INFO", "pdf parse failed")
			return s.handleFailure(job, fmt.Sprintf("cannot parse pdf: %v", err), stream)
		}
		b, err := pdfReader.GetPlainText()
		if err != nil {
			s.sendEvent(stream, job, "retry", "INFO", "pdf text extract failed")
			return s.handleFailure(job, fmt.Sprintf("cannot extract text from pdf: %v", err), stream)
		}
		scanner = bufio.NewScanner(b)
	} else {
		scanner = bufio.NewScanner(bytes.NewReader(fileData))
	}

	var lineCount, wordCount, charCount int
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		wordCount += len(strings.Fields(line))
		charCount += len(line) + 1
	}

	if err := scanner.Err(); err != nil {
		s.sendEvent(stream, job, "retry", "INFO", "scanner error")
		return s.handleFailure(job, fmt.Sprintf("error reading file: %v", err), stream)
	}

	result := fmt.Sprintf(
		"Lines: %d | Words: %d | Characters: %d | Size: %d bytes",
		lineCount, wordCount, charCount, job.FileSize,
	)
	s.repo.UpdateStatus(job.ID, model.StatusCompleted, result, "", job.RetryCount)
	s.sendEvent(stream, job, "completed", "INFO", result)
	stream.CloseAndRecv()
	fmt.Printf("[Service] Job %s completed: %s\n", job.ID, result)
	return nil
}

func (s *JobService) handleFailure(job *model.Job, errMsg string, stream pb.NotificationService_StreamEventsClient) error {
	if job.RetryCount < model.MaxRetries {
		job.RetryCount++

		waitTime := time.Duration(job.RetryCount) * time.Second
		fmt.Printf("[Service] Job %s failed, retry %d/%d in %v\n",
			job.ID, job.RetryCount, model.MaxRetries, waitTime)
		time.Sleep(waitTime)

		retryMsg := fmt.Sprintf("retry %d/%d: %s", job.RetryCount, model.MaxRetries, errMsg)
		s.sendEvent(stream, job, "retry", "INFO", retryMsg)

		s.repo.UpdateStatus(job.ID, model.StatusPending, "", retryMsg, job.RetryCount)

		s.queue.Publish(job)

		return nil
	}

	fmt.Printf("[Service] Job %s PERMANENTLY FAILED after %d retries\n",
		job.ID, model.MaxRetries)
	s.sendEvent(stream, job, "failed", "WARNING", errMsg)
	s.repo.UpdateStatus(job.ID, model.StatusFailed, "", errMsg, job.RetryCount)
	stream.CloseAndRecv()
	return fmt.Errorf("job failed after %d retries: %s", model.MaxRetries, errMsg)
}
