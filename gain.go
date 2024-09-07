package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type PredictionResponse struct {
	ID string `json:"id"`
}

type PollResponse struct {
	Status string   `json:"status"`
	Output []string `json:"output"`
	Error  string   `json:"error,omitempty"`
}

func getImageURL(pollID string, wg *sync.WaitGroup, results chan<- string) {
	defer wg.Done()

	for {
		time.Sleep(5 * time.Second)

		url := fmt.Sprintf("https://homepage.replicate.com/api/poll?id=%s", pollID)
		response, err := http.Get(url)
		if err != nil {
			fmt.Println("Error during polling:", err)
			results <- ""
			return
		}
		defer response.Body.Close()

		var pollData PollResponse
		content, _ := ioutil.ReadAll(response.Body)
		if err := json.Unmarshal(content, &pollData); err != nil {
			fmt.Println("Error decoding poll response:", err)
			results <- ""
			return
		}

		fmt.Println("Poll response:", pollData)

		switch pollData.Status {
		case "succeeded":
			if len(pollData.Output) > 0 {
				imageURL := pollData.Output[0]
				results <- imageURL
				return
			} else {
				fmt.Println("Output array is empty or not available")
				results <- ""
			}
		case "failed":
			fmt.Println("Prediction failed:", pollData.Error)
			results <- ""
			return
		default:
			fmt.Println("Prediction is still in progress...")
		}
	}
}

func getID(prompt string) string {
	const url = "https://homepage.replicate.com/api/prediction"
	requestBody := fmt.Sprintf(`{
    "model": "black-forest-labs/flux-schnell",
    "version": "f2ab8a5bfe79f02f0789a146cf5e73d2a4ff2684a98c2b303d1e1ff3814271db",
    "input": {
      "prompt": "%s"
    }
  }`, prompt)

	response, err := http.Post(url, "application/json", bytes.NewBufferString(requestBody))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return ""
	}
	defer response.Body.Close()

	var predictionData PredictionResponse
	content, _ := ioutil.ReadAll(response.Body)
	if err := json.Unmarshal(content, &predictionData); err != nil {
		fmt.Println("Error decoding initial response:", err)
		return ""
	}

	return predictionData.ID
}

func main() {
	fmt.Println("A simple tool to generate AI Images")

	// Ask the user how many prompts they want to enter
	fmt.Print("Enter the number of prompts: ")
	var numPrompts int
	fmt.Scanln(&numPrompts)

	// Create a reader to read full-line prompts from the user
	reader := bufio.NewReader(os.Stdin)

	// Collect the prompts from the user
	prompts := make([]string, numPrompts)
	for i := 0; i < numPrompts; i++ {
		fmt.Printf("Enter prompt %d: ", i+1)
		prompt, _ := reader.ReadString('\n') // Read the entire line including spaces
		prompt = strings.TrimSpace(prompt)   // Trim extra newlines and spaces
		prompts[i] = prompt
	}

	var wg sync.WaitGroup
	results := make(chan string, 3*numPrompts) // Channel to receive results

	// Send 3 requests for each prompt
	for _, prompt := range prompts {
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(prompt string) {
				pollID := getID(prompt)
				if pollID == "" {
					fmt.Println("No ID found in response")
					results <- ""
					return
				}
				getImageURL(pollID, &wg, results)
			}(prompt)
		}
	}

	// Close the results channel once all requests are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var urls []string
	for url := range results {
		if url != "" {
			urls = append(urls, url)
		}
	}

	// Print and write results to a file
	file, err := os.Create("image_urls.txt")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	fmt.Println("Results:")
	for i, url := range urls {
		fmt.Printf("Image URL %d: %s\n", i+1, url)
		_, err := file.WriteString(fmt.Sprintf("Image URL %d: %s\n", i+1, url))
		if err != nil {
			fmt.Println("Error writing to file:", err)
			return
		}
	}
}
