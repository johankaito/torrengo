package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// UserAgent is a customer browser user agent used in every HTTP connections
const UserAgent string = "Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.62 Safari/537.36"

// Fetch opens a url with a custom client created by user and returns the resulting html page.
// Cannot use the straight http.Get function because need to
// modify headers in order to set a fake user-agent.
func Fetch(url string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %v", err)
	}

	req.Header.Set("User-Agent", UserAgent)

	log.WithFields(log.Fields{
		"httpMethod":   req.Method,
		"url":          req.URL,
		"httpProtocol": req.Proto,
		"host":         req.Host,
		"headers":      req.Header,
	}).Debug("Successfully built HTTP request")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not launch request: %v", err)
	}

	log.WithFields(log.Fields{
		"httpStatus": resp.Status,
		"headers":    resp.Header,
	}).Debug("Successfully received HTTP response")

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

// FetchFromCloudflare fetches data from a Cloudflare protected webpage.
// It uses the cfscrape library in Python (https://github.com/Anorov/cloudflare-scrape).
// Python, Nodejs, and cfscrape should be installed for this to work.
func FetchFromCloudflare(url string, timeout time.Duration) (string, error) {
	// Build the Python script
	pythonScript := fmt.Sprintf(
		"import cfscrape as cs; s = cs.create_scraper(); print(s.get(\"%s\", timeout=%f).content)",
		url,
		timeout.Seconds(),
	)
	log.WithFields(log.Fields{
		"pythonScript": pythonScript,
	}).Debug("Built the Python script")

	// Launch Python script (-c meaning everything will be launched inline)
	cmd := exec.Command("python", "-c", pythonScript)

	// Get the standard and error outputs
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	if err != nil || errStr != "" {
		return "", fmt.Errorf("could not get HTTP response with Python: %v\n%v", errStr, outStr)
	}
	log.WithFields(log.Fields{
		"url": url,
	}).Debug("Successfully received HTTP response with Python")

	return outStr, nil
}

// DlFile downloads the torrent with a custom client created by user and returns the path of
// downloaded file.
func DlFile(fileURL string, client *http.Client) (string, error) {
	// Get torrent file name from url
	s := strings.Split(fileURL, "/")
	fileName := s[len(s)-1]

	// Create local torrent file
	out, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("could not create the torrent file named %s: %v", fileName, err)
	}
	defer out.Close()

	// Download torrent
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return "", fmt.Errorf("could not create request: %v", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not download the torrent file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return "", fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	// Save torrent to disk
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not save the torrent file to disk: %v", err)
	}

	// Get absolute file path of torrent
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", fmt.Errorf("could not retrieve current directory of saved filed: %v", err)
	}
	filePath := dir + "/" + fileName

	return filePath, nil
}
