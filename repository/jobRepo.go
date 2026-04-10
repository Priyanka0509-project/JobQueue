package repository

import (
	"bytes"
	"context"
	"fmt"
	"jobQueue/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoCollection interface {
	InsertOne(ctx context.Context, document interface{}) error
	FindOneByID(ctx context.Context, id string) (*model.Job, error)
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	FindAll(ctx context.Context) ([]*model.Job, error)
	SaveFileData(ctx context.Context, id string, data []byte) error
	FindFileData(ctx context.Context, id string) ([]byte, error)
}

type realMongoCollection struct {
	client    *mongo.Client
	db        *mongo.Database
	jobsCol   *mongo.Collection
	bucket    *gridfs.Bucket
}

func newRealMongoCollection(uri string) (*realMongoCollection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := client.Database("jobqueue")
	
	bucket, err := gridfs.NewBucket(db)
	if err != nil {
		return nil, fmt.Errorf("could not create gridfs bucket: %w", err)
	}

	return &realMongoCollection{
		client:   client,
		db:       db,
		jobsCol:  db.Collection("jobs"),
		bucket:   bucket,
	}, nil
}

func (m *realMongoCollection) InsertOne(ctx context.Context, doc interface{}) error {
	_, err := m.jobsCol.InsertOne(ctx, doc)
	return err
}

func (m *realMongoCollection) FindOneByID(ctx context.Context, id string) (*model.Job, error) {
	var job model.Job
	err := m.jobsCol.FindOne(ctx, bson.M{"_id": id}).Decode(&job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (m *realMongoCollection) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	update := bson.M{"$set": fields}
	_, err := m.jobsCol.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (m *realMongoCollection) FindAll(ctx context.Context) ([]*model.Job, error) {
	cursor, err := m.jobsCol.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var jobs []*model.Job
	if err = cursor.All(ctx, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (m *realMongoCollection) SaveFileData(ctx context.Context, id string, data []byte) error {
	// First, try to delete any existing file with this name (if upserting)
	// gridfs doesn't instantly overwrite by name, we could accumulate duplicates
	// so for safety on retry, we drop existing ones with the same ID
	cursor, err := m.bucket.Find(bson.M{"filename": id})
	if err == nil {
		type gridfsFile struct {
			ID interface{} `bson:"_id"`
		}
		var files []gridfsFile
		if err = cursor.All(ctx, &files); err == nil {
			for _, f := range files {
				_ = m.bucket.Delete(f.ID)
			}
		}
	}

	uploadStream, err := m.bucket.OpenUploadStream(id)
	if err != nil {
		return fmt.Errorf("failed to open gridfs stream: %w", err)
	}
	defer uploadStream.Close()

	if _, err := uploadStream.Write(data); err != nil {
		return fmt.Errorf("failed to write to gridfs stream: %w", err)
	}
	return nil
}

func (m *realMongoCollection) FindFileData(ctx context.Context, id string) ([]byte, error) {
	var buf bytes.Buffer
	_, err := m.bucket.DownloadToStreamByName(id, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to download from gridfs stream: %w", err)
	}
	return buf.Bytes(), nil
}

type JobRepository struct {
	col mongoCollection
}

func NewJobRepository(uri string) (*JobRepository, error) {
	fmt.Printf("[Repo] Connecting to: %s\n", uri)

	col, err := newRealMongoCollection(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongo: %w", err)
	}

	fmt.Println("[Repo] Ready")
	return &JobRepository{col: col}, nil
}

func ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (r *JobRepository) Save(job *model.Job) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	if err := r.col.InsertOne(ctx, job); err != nil {
		return fmt.Errorf("save job failed: %w", err)
	}
	return nil
}

func (r *JobRepository) FindByID(id string) (*model.Job, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	job, err := r.col.FindOneByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *JobRepository) UpdateStatus(id string, status model.JobStatus, result, errMsg string, retryCount int) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	return r.col.UpdateFields(ctx, id, map[string]interface{}{
		"status":      status,
		"result":      result,
		"error":       errMsg,
		"retry_count": retryCount,
		"updated_at":  time.Now(),
	})
}

func (r *JobRepository) FindAll() ([]*model.Job, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	return r.col.FindAll(ctx)
}

func (r *JobRepository) SaveFileData(id string, data []byte) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	return r.col.SaveFileData(ctx, id, data)
}

func (r *JobRepository) FindFileData(id string) ([]byte, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	return r.col.FindFileData(ctx, id)
}
