package queue

import "jobQueue/model"

type Queue struct {
	jobChannel chan *model.Job
}

func NewQueue() *Queue {
	return &Queue{
		jobChannel: make(chan *model.Job, 100),
	}
}

func (b *Queue) Publish(job *model.Job) {
	b.jobChannel <- job
}

func (b *Queue) Subscribe() <-chan *model.Job {
	return b.jobChannel
}
