package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func generateHTML(urls []string) {
	file, err := os.Create("images.html")
	if err != nil {
		fmt.Println("Error creating HTML file:", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	writer.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<title>Generated Images</title>\n</head>\n<body>\n")
	writer.WriteString("<h1>Generated Images</h1>\n")

	for i, url := range urls {
		writer.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"Image %d\" style=\"width:300px; height:auto; margin:10px;\"/>\n", url, i+1))
	}

	writer.WriteString("</body>\n</html>\n")
	writer.Flush()

	fmt.Println("HTML file created: images.html")
}

func main() {
	file, err := os.Open("image_urls.txt")
	if err != nil {
		fmt.Println("Error opening URL file:", err)
		return
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Extract URL from the line
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			urls = append(urls, parts[1])
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading URL file:", err)
		return
	}

	// Generate HTML file with the URLs
	generateHTML(urls)
}
