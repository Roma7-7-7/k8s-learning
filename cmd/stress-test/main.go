//nolint:mnd,perfsprint,noctx,intrange,gosec,forbidigo,usestdlibvars,depguard // This is a stress test tool for an API that processes files.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	File            string
	MinProcessDelay int
	MaxProcessDelay int
	Concurrency     int
	QueryDelay      int
	Duration        int
	APIEndpoint     string
}

type JobResponse struct {
	ID               string                 `json:"id"`
	OriginalFilename string                 `json:"original_filename"`
	ProcessingType   string                 `json:"processing_type"`
	Parameters       map[string]interface{} `json:"parameters"`
	Status           string                 `json:"status"`
	DelayMS          int                    `json:"delay_ms"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}

type TestResult struct {
	TotalRequests   int
	SuccessRequests int
	FailedRequests  int
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	ErrorCounts     map[int]int
}

func main() {
	config := parseFlags()

	if err := validateConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting stress test with config: %+v", config)

	start := time.Now()
	result := runStressTest(config)
	actualDuration := time.Since(start)

	printResults(result, actualDuration)
}

func parseFlags() Config {
	var config Config

	flag.StringVar(&config.File, "file", "", "Path to the test file (required)")
	flag.IntVar(&config.MinProcessDelay, "min-process-delay", 0, "Minimum processing delay in milliseconds")
	flag.IntVar(&config.MaxProcessDelay, "max-process-delay", 30000, "Maximum processing delay in milliseconds")
	flag.IntVar(&config.Concurrency, "concurrency", 1, "Number of concurrent requests")
	flag.IntVar(&config.QueryDelay, "query-delay", 10, "Delay between requests in milliseconds")
	flag.IntVar(&config.Duration, "duration", 60, "Test duration in seconds")
	flag.StringVar(&config.APIEndpoint, "api-endpoint", "http://localhost:8080/api/v1/jobs", "API endpoint URL")

	flag.Parse()
	return config
}

func validateConfig(config Config) error {
	if config.File == "" {
		return fmt.Errorf("file parameter is required")
	}

	if _, err := os.Stat(config.File); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", config.File)
	}

	if config.MinProcessDelay < 0 {
		return fmt.Errorf("min-process-delay cannot be negative")
	}

	if config.MaxProcessDelay < 0 {
		return fmt.Errorf("max-process-delay cannot be negative")
	}

	if config.MinProcessDelay > config.MaxProcessDelay {
		return fmt.Errorf("min-process-delay cannot be greater than max-process-delay")
	}

	if config.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}

	if config.QueryDelay < 0 {
		return fmt.Errorf("query-delay cannot be negative")
	}

	if config.Duration < 1 {
		return fmt.Errorf("duration must be at least 1 second")
	}

	return nil
}

func runStressTest(config Config) TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Duration)*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	resultChan := make(chan requestResult, config.Concurrency*100)

	// Start workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go worker(ctx, &wg, config, resultChan)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return collectResults(resultChan)
}

type requestResult struct {
	Success    bool
	Latency    time.Duration
	StatusCode int
}

func worker(ctx context.Context, wg *sync.WaitGroup, config Config, resultChan chan<- requestResult) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			result := makeRequest(client, config)
			resultChan <- result

			if config.QueryDelay > 0 {
				time.Sleep(time.Duration(config.QueryDelay) * time.Millisecond)
			}
		}
	}
}

func makeRequest(client *http.Client, config Config) requestResult {
	start := time.Now()

	// Generate random delay within the specified range
	delayMS := config.MinProcessDelay
	if config.MaxProcessDelay > config.MinProcessDelay {
		delayMS = config.MinProcessDelay + rand.IntN(config.MaxProcessDelay-config.MinProcessDelay+1)
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file
	fileWriter, err := writer.CreateFormFile("file", filepath.Base(config.File))
	if err != nil {
		return requestResult{Success: false, Latency: time.Since(start), StatusCode: 0}
	}

	fileContent, err := os.ReadFile(config.File)
	if err != nil {
		return requestResult{Success: false, Latency: time.Since(start), StatusCode: 0}
	}

	if _, err := fileWriter.Write(fileContent); err != nil {
		return requestResult{Success: false, Latency: time.Since(start), StatusCode: 0}
	}

	// Add processing type (using wordcount as default)
	if err := writer.WriteField("processing_type", "wordcount"); err != nil {
		return requestResult{Success: false, Latency: time.Since(start), StatusCode: 0}
	}

	// Add delay_ms
	if err := writer.WriteField("delay_ms", fmt.Sprintf("%d", delayMS)); err != nil {
		return requestResult{Success: false, Latency: time.Since(start), StatusCode: 0}
	}

	if err := writer.Close(); err != nil {
		return requestResult{Success: false, Latency: time.Since(start), StatusCode: 0}
	}

	// Create and send request
	req, err := http.NewRequest("POST", config.APIEndpoint, &buf)
	if err != nil {
		return requestResult{Success: false, Latency: time.Since(start), StatusCode: 0}
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return requestResult{Success: false, Latency: latency, StatusCode: 0}
	}
	defer resp.Body.Close()

	// Read response body for debugging if needed
	_, _ = io.ReadAll(resp.Body)

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	return requestResult{Success: success, Latency: latency, StatusCode: resp.StatusCode}
}

func collectResults(resultChan <-chan requestResult) TestResult {
	var result TestResult
	result.ErrorCounts = make(map[int]int)

	var latencies []time.Duration

	for res := range resultChan {
		result.TotalRequests++

		if res.Success {
			result.SuccessRequests++
		} else {
			result.FailedRequests++
			result.ErrorCounts[res.StatusCode]++
		}

		latencies = append(latencies, res.Latency)
	}

	if len(latencies) > 0 {
		// Calculate latency statistics
		var totalLatency time.Duration
		result.MinLatency = latencies[0]
		result.MaxLatency = latencies[0]

		for _, latency := range latencies {
			totalLatency += latency
			if latency < result.MinLatency {
				result.MinLatency = latency
			}
			if latency > result.MaxLatency {
				result.MaxLatency = latency
			}
		}

		result.AverageLatency = totalLatency / time.Duration(len(latencies))
	}

	return result
}

func printResults(result TestResult, duration time.Duration) {
	fmt.Println("\n=== Stress Test Results ===")
	fmt.Printf("Total Requests: %d\n", result.TotalRequests)
	fmt.Printf("Successful Requests: %d (%.2f%%)\n",
		result.SuccessRequests,
		float64(result.SuccessRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("Failed Requests: %d (%.2f%%)\n",
		result.FailedRequests,
		float64(result.FailedRequests)/float64(result.TotalRequests)*100)

	if result.TotalRequests > 0 {
		fmt.Printf("Average Latency: %v\n", result.AverageLatency)
		fmt.Printf("Min Latency: %v\n", result.MinLatency)
		fmt.Printf("Max Latency: %v\n", result.MaxLatency)
		rps := float64(result.TotalRequests) / duration.Seconds()
		fmt.Printf("Requests/Second: %.2f\n", rps)
	}

	if len(result.ErrorCounts) > 0 {
		fmt.Println("\nError Breakdown:")
		for statusCode, count := range result.ErrorCounts {
			fmt.Printf("  HTTP %d: %d requests\n", statusCode, count)
		}
	}

	fmt.Println("=========================")
}
