package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
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
	client *http.Client
}

// NewURLChecker initializes a URLChecker with a timeout
func NewURLChecker(timeout time.Duration) *URLChecker {
	return &URLChecker{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// checkURL checks a single URL and optionally searches for a keyword
func (c *URLChecker) checkURL(url, keyword string) URLResult {
	result := URLResult{URL: url}
	start := time.Now()

	resp, err := c.client.Get(url)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.ResponseTime = time.Since(start)

	if keyword != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			result.Error = fmt.Sprintf("Error reading body: %v", err)
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

func main() {
	checker := NewURLChecker(10 * time.Second)

	// Root command
	var rootCmd = &cobra.Command{
		Use:   "urlcheck",
		Short: "A CLI tool to check URL status and content",
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
				if err := writeCSV(results, outputFile); err != nil {
					fmt.Println("Error writing CSV:", err)
					return
				}
				fmt.Println("Results saved to", outputFile)
			}
		},
	}
	listCmd.Flags().StringVarP(&keyword, "keyword", "k", "", "Keyword to search for in pages")
	listCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output CSV file for results")

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
