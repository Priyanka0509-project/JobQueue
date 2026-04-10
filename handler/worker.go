package handler

import (
	"fmt"
	"jobQueue/queue"
	"jobQueue/service"
)

type JobHandler struct {
	queue   *queue.Queue
	service *service.JobService
}

func NewJobHandler(q *queue.Queue, svc *service.JobService) *JobHandler {
	return &JobHandler{queue: q, service: svc}
}

func (h *JobHandler) Start() {
	fmt.Println("Started — waiting for jobs...")
	for job := range h.queue.Subscribe() {
		fmt.Printf("Processing job: %s (retry: %d)\n",
			job.ID, job.RetryCount)
		if err := h.service.ProcessFile(job); err != nil {
			fmt.Printf("Job %s permanently failed: %v\n", job.ID, err)
		}
	}
}

func (h *JobHandler) StartPool(count int) {
	for i := 1; i <= count; i++ {
		workerNum := i
		go func() {
			fmt.Printf("[Handler-%d] Started\n", workerNum)
			for job := range h.queue.Subscribe() {
				fmt.Printf("[Handler-%d] Job %s (retry: %d)\n",
					workerNum, job.ID, job.RetryCount)
				h.service.ProcessFile(job)
			}
		}()
	}
	fmt.Printf("[Handler] Pool of %d workers started\n", count)
}
