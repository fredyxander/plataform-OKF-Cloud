package domain

type JobMessage struct {
	JobID   string `json:"jobId"`
	Attempt int    `json:"attempt"`
}