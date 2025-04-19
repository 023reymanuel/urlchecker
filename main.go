package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// URLResult represents the check result for a single URL
type URLResult struct {
	URL          string        `json:"url"`
	StatusCode   int           `json:"status_code"`
	ResponseTime time.Duration `json:"response_time"`
	KeywordFound bool          `json:"keyword_found"`
	Error        string        `json:"error"`
}

// URLChecker manages URL checking operations
type URLChecker struct {
	client  *http.Client
	retries int
}

// NewURLChecker initializes a URLChecker with a timeout and retry count
func NewURLChecker(timeout time.Duration, retries int) *URLChecker {
	return &URLChecker{
		client: &http.Client{
			Timeout: timeout,
		},
		retries: retries,
	}
}

// validateURL checks if a URL is well-formed
func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}
	return nil
}

// checkURL checks a single URL and optionally searches for a keyword
func (c *URLChecker) checkURL(rawURL, keyword string) URLResult {
	result := URLResult{URL: rawURL}

	// Validate URL
	if err := validateURL(rawURL); err != nil {
		result.Error = err.Error()
		return result
	}

	var resp *http.Response
	var err error
	start := time.Now()

	// Attempt request with retries
	for attempt := 0; attempt <= c.retries; attempt++ {
		resp, err = c.client.Get(rawURL)
		if err == nil {
			break
		}
		if attempt < c.retries {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}

	if err != nil {
		result.Error = fmt.Sprintf("failed after %d retries: %v", c.retries+1, err)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.ResponseTime = time.Since(start)

	if keyword != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			result.Error = fmt.Sprintf("error reading body: %v", err)
			return result
		}
		result.KeywordFound = strings.Contains(strings.ToLower(string(body)), strings.ToLower(keyword))
	}

	return result
}

// checkURLs checks multiple URLs concurrently
func (c *URLChecker) checkURLs(urls []string, keyword string) []URLResult {
	var wg sync.WaitGroup
	results := make([]URLResult, len(urls))
	resultChan := make(chan URLResult, len(urls))

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			resultChan <- c.checkURL(u, keyword)
		}(i, url)
	}

	wg.Wait()
	close(resultChan)

	for i := 0; i < len(urls); i++ {
		results[i] = <-resultChan
	}

	return results
}

// writeCSV writes results to a CSV file
func writeCSV(results []URLResult, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"URL", "StatusCode", "ResponseTime", "KeywordFound", "Error"}); err != nil {
		return err
	}

	// Write results
	for _, r := range results {
		if err := writer.Write([]string{
			r.URL,
			fmt.Sprintf("%d", r.StatusCode),
			r.ResponseTime.String(),
			fmt.Sprintf("%t", r.KeywordFound),
			r.Error,
		}); err != nil {
			return err
		}
	}

	return nil
}

// writeJSON writes results to a JSON file
func writeJSON(results []URLResult, outputFile string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputFile, data, 0644)
}

func main() {
	var timeoutSeconds int
	var retries int
	var outputFormat string

	// Root command
	var rootCmd = &cobra.Command{
		Use:   "urlcheck",
		Short: "A CLI tool to check URL status and content",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if timeoutSeconds <= 0 {
				fmt.Println("Error: timeout must be positive")
				os.Exit(1)
			}
			if retries < 0 {
				fmt.Println("Error: retries cannot be negative")
				os.Exit(1)
			}
			if outputFormat != "csv" && outputFormat != "json" {
				fmt.Println("Error: output format must be 'csv' or 'json'")
				os.Exit(1)
			}
		},
	}
	rootCmd.PersistentFlags().IntVar(&timeoutSeconds, "timeout", 10, "HTTP request timeout in seconds")
	rootCmd.PersistentFlags().IntVar(&retries, "retries", 0, "Number of retries for failed requests")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "csv", "Output format: csv or json")

	// Initialize checker after flags are parsed
	var checker *URLChecker
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		checker = NewURLChecker(time.Duration(timeoutSeconds)*time.Second, retries)
		return nil
	}

	// Check command
	var keyword string
	var checkCmd = &cobra.Command{
		Use:   "check [url]",
		Short: "Check a single URL",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			result := checker.checkURL(args[0], keyword)
			printResult(result)
		},
	}
	checkCmd.Flags().StringVarP(&keyword, "keyword", "k", "", "Keyword to search for in the page")

	// List command
	var outputFile string
	var listCmd = &cobra.Command{
		Use:   "list [file]",
		Short: "Check multiple URLs from a file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			urls, err := readURLs(args[0])
			if err != nil {
				fmt.Println("Error reading URLs:", err)
				return
			}
			results := checker.checkURLs(urls, keyword)
			for _, result := range results {
				printResult(result)
			}
			if outputFile != "" {
				var err error
				if outputFormat == "csv" {
					err = writeCSV(results, outputFile)
				} else {
					err = writeJSON(results, outputFile)
				}
				if err != nil {
					fmt.Println("Error writing output:", err)
					return
				}
				fmt.Println("Results saved to", outputFile)
			}
		},
	}
	listCmd.Flags().StringVarP(&keyword, "keyword", "k", "", "Keyword to search for in pages")
	listCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for results (CSV or JSON based on --format)")

	// Add commands to root
	rootCmd.AddCommand(checkCmd, listCmd)

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// readURLs reads URLs from a file, one per line
func readURLs(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var urls []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			urls = append(urls, line)
		}
	}
	return urls, nil
}

// printResult prints a single URL check result
func printResult(r URLResult) {
	if r.Error != "" {
		fmt.Printf("%s: Error - %s\n", r.URL, r.Error)
		return
	}
	fmt.Printf("%s: Status=%d, ResponseTime=%s", r.URL, r.StatusCode, r.ResponseTime)
	if r.KeywordFound {
		fmt.Print(", Keyword=Found")
	} else if r.StatusCode == 200 {
		fmt.Print(", Keyword=NotFound")
	}
	fmt.Println()
}
