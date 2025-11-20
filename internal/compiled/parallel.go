package compiled

import (
	"context"
	"sync"

	"github.com/roc-ops/gottp/internal/compiler"
)

// ParallelParser provides parallel parsing capabilities
type ParallelParser struct {
	compiled *compiler.CompiledTemplate
	workers  int
}

// NewParallelParser creates a new parallel parser
func NewParallelParser(compiled *compiler.CompiledTemplate, workers int) *ParallelParser {
	if workers <= 0 {
		workers = 1
	}
	return &ParallelParser{
		compiled: compiled,
		workers:  workers,
	}
}

// ParseParallel parses multiple inputs in parallel
func (p *ParallelParser) ParseParallel(inputs map[string]string, vars map[string]interface{}, options *ParseOptions) ([]interface{}, error) {
	// Create a channel for inputs
	inputChan := make(chan parseJob, len(inputs))
	resultChan := make(chan parseResult, len(inputs))
	
	// Send all inputs to channel
	for name, data := range inputs {
		inputChan <- parseJob{
			name: name,
			data: data,
		}
	}
	close(inputChan)
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.worker(inputChan, resultChan, vars, options)
		}()
	}
	
	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	results := make([]interface{}, 0, len(inputs))
	for result := range resultChan {
		if result.err != nil {
			return nil, result.err
		}
		results = append(results, result.data)
	}
	
	return results, nil
}

// parseJob represents a parsing job
type parseJob struct {
	name string
	data string
}

// parseResult represents a parsing result
type parseResult struct {
	data interface{}
	err  error
}

// worker processes parsing jobs
func (p *ParallelParser) worker(jobs <-chan parseJob, results chan<- parseResult, vars map[string]interface{}, options *ParseOptions) {
	runtime := NewRuntime(p.compiled)
	
	for job := range jobs {
		inputMap := map[string]string{
			job.name: job.data,
		}
		
		result, err := runtime.Parse(inputMap, vars, options)
		results <- parseResult{
			data: result,
			err:  err,
		}
	}
}

// ParseWithContext parses with context support for cancellation
func (p *ParallelParser) ParseWithContext(ctx context.Context, inputs map[string]string, vars map[string]interface{}, options *ParseOptions) ([]interface{}, error) {
	// Create a channel for inputs
	inputChan := make(chan parseJob, len(inputs))
	resultChan := make(chan parseResult, len(inputs))
	
	// Send all inputs to channel
	for name, data := range inputs {
		select {
		case inputChan <- parseJob{name: name, data: data}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	close(inputChan)
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.workerWithContext(ctx, inputChan, resultChan, vars, options)
		}()
	}
	
	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	results := make([]interface{}, 0, len(inputs))
	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				return results, nil
			}
			if result.err != nil {
				return nil, result.err
			}
			results = append(results, result.data)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// workerWithContext processes parsing jobs with context support
func (p *ParallelParser) workerWithContext(ctx context.Context, jobs <-chan parseJob, results chan<- parseResult, vars map[string]interface{}, options *ParseOptions) {
	runtime := NewRuntime(p.compiled)
	
	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}
			
			inputMap := map[string]string{
				job.name: job.data,
			}
			
			result, err := runtime.Parse(inputMap, vars, options)
			
			select {
			case results <- parseResult{data: result, err: err}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

