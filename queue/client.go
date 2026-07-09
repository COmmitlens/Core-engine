package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

// Client wraps asynq.Client for enqueuing tasks
type Client struct {
	client *asynq.Client
}

// NewClient creates a new queue client connected to Redis
func NewClient(redisAddr, redisPassword string) *Client {
	c := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr, Password: redisPassword})
	return &Client{client: c}
}

// Close closes the underlying asynq client
func (c *Client) Close() error {
	return c.client.Close()
}

// EnqueueFetchAndStoreRepos enqueues a task to fetch and store repos for an installation
func (c *Client) EnqueueFetchAndStoreRepos(payload FetchAndStoreReposPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal FetchAndStoreRepos payload: %w", err)
	}

	task := asynq.NewTask(TypeFetchAndStoreRepos, data,
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
		asynq.Queue("default"),
	)

	info, err := c.client.EnqueueContext(context.Background(), task)
	if err != nil {
		return fmt.Errorf("failed to enqueue FetchAndStore http://localhost:3000Repos: %w", err)
	}
	log.Printf("[queue] Enqueued task %s: id=%s queue=%s", TypeFetchAndStoreRepos, info.ID, info.Queue)
	return nil
}

// EnqueueEmbedCommitFile enqueues an embedding task for a commit file (v1)
func (c *Client) EnqueueEmbedCommitFile(payload EmbedCommitFilePayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal EmbedCommitFile payload: %w", err)
	}

	task := asynq.NewTask(TypeEmbedCommitFile, data,
		asynq.MaxRetry(5),
		asynq.Timeout(2*time.Minute),
		asynq.Queue("embeddings"),
	)

	info, err := c.client.EnqueueContext(context.Background(), task)
	if err != nil {
		return fmt.Errorf("failed to enqueue EmbedCommitFile: %w", err)
	}
	log.Printf("[queue] Enqueued task %s: id=%s file=%s", TypeEmbedCommitFile, info.ID, payload.Filename)
	return nil
}

// EnqueueEmbedCommitFileV2 enqueues an embedding task for a commit file (v2/push flow)
func (c *Client) EnqueueEmbedCommitFileV2(payload EmbedCommitFileV2Payload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal EmbedCommitFileV2 payload: %w", err)
	}

	task := asynq.NewTask(TypeEmbedCommitFileV2, data,
		asynq.MaxRetry(5),
		asynq.Timeout(2*time.Minute),
		asynq.Queue("embeddings"),
	)

	info, err := c.client.EnqueueContext(context.Background(), task)
	if err != nil {
		return fmt.Errorf("failed to enqueue EmbedCommitFileV2: %w", err)
	}
	log.Printf("[queue] Enqueued task %s: id=%s file=%s", TypeEmbedCommitFileV2, info.ID, payload.Filename)
	return nil
}

// EnqueueHandlePushEvent enqueues a GitHub push webhook for background processing
func (c *Client) EnqueueHandlePushEvent(rawJSON []byte) error {
	payload := HandlePushEventPayload{RawJSON: rawJSON}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal HandlePushEvent payload: %w", err)
	}

	task := asynq.NewTask(TypeHandlePushEvent, data,
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
		asynq.Queue("webhooks"),
	)

	info, err := c.client.EnqueueContext(context.Background(), task)
	if err != nil {
		return fmt.Errorf("failed to enqueue HandlePushEvent: %w", err)
	}
	log.Printf("[queue] Enqueued task %s: id=%s", TypeHandlePushEvent, info.ID)
	return nil
}

// EnqueueHandleInstallationRepositoriesEvent enqueues a GitHub installation_repositories
// webhook for background processing (repos added/removed from an installation).
func (c *Client) EnqueueHandleInstallationRepositoriesEvent(rawJSON []byte) error {
	payload := HandleInstallationRepositoriesPayload{RawJSON: rawJSON}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal HandleInstallationRepositoriesEvent payload: %w", err)
	}

	task := asynq.NewTask(TypeHandleInstallationRepositoriesEvent, data,
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
		asynq.Queue("webhooks"),
	)

	info, err := c.client.EnqueueContext(context.Background(), task)
	if err != nil {
		return fmt.Errorf("failed to enqueue HandleInstallationRepositoriesEvent: %w", err)
	}
	log.Printf("[queue] Enqueued task %s: id=%s", TypeHandleInstallationRepositoriesEvent, info.ID)
	return nil
}
