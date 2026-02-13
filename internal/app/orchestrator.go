package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

type TaskShard struct {
	ID     string
	Index  int
	Total  int
	Subtask string
	Prompt string
}

type TaskResult struct {
	ID     string
	Index  int
	Output string
	Err    error
}

type Orchestrator struct {
	Client      *MinimaxClient
	MaxParallel int
}

func NewOrchestrator(client *MinimaxClient, maxParallel int) *Orchestrator {
	if maxParallel <= 0 {
		maxParallel = 50
	}
	if maxParallel > 900 {
		maxParallel = 900
	}
	return &Orchestrator{Client: client, MaxParallel: maxParallel}
}

func (o *Orchestrator) Run(ctx context.Context, shards []TaskShard) ([]TaskResult, error) {
	if o.Client == nil {
		return nil, errors.New("client is required")
	}
	if len(shards) == 0 {
		return nil, nil
	}
	workerLimit := o.MaxParallel
	if workerLimit > len(shards) {
		workerLimit = len(shards)
	}
	jobs := make(chan TaskShard)
	results := make(chan TaskResult)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for shard := range jobs {
			output, err := o.Client.Complete(ctx, shard.Prompt)
			results <- TaskResult{
				ID:     shard.ID,
				Index:  shard.Index,
				Output: output,
				Err:    err,
			}
		}
	}

	wg.Add(workerLimit)
	for i := 0; i < workerLimit; i++ {
		go worker()
	}

	go func() {
		for _, shard := range shards {
			jobs <- shard
		}
		close(jobs)
	}()

	out := make([]TaskResult, 0, len(shards))
	for i := 0; i < len(shards); i++ {
		result := <-results
		out = append(out, result)
	}

	wg.Wait()
	close(results)

	return out, nil
}

func SynthesizeResults(results []TaskResult) string {
	if len(results) == 0 {
		return ""
	}
	ordered := make([]TaskResult, len(results))
	copy(ordered, results)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Index == ordered[j].Index {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Index < ordered[j].Index
	})

	builder := ""
	for _, result := range ordered {
		if result.Err != nil {
			builder += fmt.Sprintf("[Shard %s Error] %v\n", result.ID, result.Err)
			continue
		}
		builder += fmt.Sprintf("[Shard %s]\n%s\n", result.ID, result.Output)
	}
	return builder
}
