package repository

import (
	"context"
	"fmt"
	"jobQueue/model"
	"time"

	"github.com/gocql/gocql"
)

type JobRepository struct {
	session *gocql.Session
}

func NewJobRepository() (*JobRepository, error) {

	cluster := gocql.NewCluster("127.0.0.1")
	cluster.Keyspace = "jobqueue"
	cluster.Consistency = gocql.Quorum

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to connect cassandra: %w", err)
	}

	fmt.Println("[Repo] Cassandra connected")

	return &JobRepository{session: session}, nil
}

func ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (r *JobRepository) Save(job *model.Job) error {

	return r.session.Query(
		`INSERT INTO jobs 
		(job_id, file_name, file_size, file_key, status, result, error, retry_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.FileName,
		job.FileSize,
		job.FileKey,
		job.Status,
		job.Result,
		job.Error,
		job.RetryCount,
		job.CreatedAt,
		job.UpdatedAt,
	).Exec()
}

func (r *JobRepository) FindByID(id string) (*model.Job, error) {

	var job model.Job

	err := r.session.Query(
		`SELECT job_id, file_name, file_size, file_key, status, result, error, retry_count, created_at, updated_at
		FROM jobs WHERE job_id=?`,
		id,
	).Scan(
		&job.ID,
		&job.FileName,
		&job.FileSize,
		&job.FileKey,
		&job.Status,
		&job.Result,
		&job.Error,
		&job.RetryCount,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *JobRepository) UpdateStatus(id string, status model.JobStatus, result, errMsg string, retryCount int) error {

	return r.session.Query(
		`UPDATE jobs 
		SET status=?, result=?, error=?, retry_count=?, updated_at=? 
		WHERE job_id=?`,
		status,
		result,
		errMsg,
		retryCount,
		time.Now(),
		id,
	).Exec()
}

func (r *JobRepository) FindAll() ([]*model.Job, error) {

	iter := r.session.Query(
		`SELECT job_id, file_name, file_size, file_key, status, result, error, retry_count, created_at, updated_at FROM jobs`,
	).Iter()

	var jobs []*model.Job

	var job model.Job

	for iter.Scan(
		&job.ID,
		&job.FileName,
		&job.FileSize,
		&job.FileKey,
		&job.Status,
		&job.Result,
		&job.Error,
		&job.RetryCount,
		&job.CreatedAt,
		&job.UpdatedAt,
	) {
		j := job
		jobs = append(jobs, &j)
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return jobs, nil
}
