package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Custom writer to track progress
type progressWriter struct {
	total      int64
	downloaded int64
	startTime  time.Time
	endTime    time.Time
}
type rateLimitedReader struct {
	reader      io.Reader
	bytesPerSec int64
}

var (
	outputPath  string
	url         string
	b           bool
	err         error
	bytesPerSec int64
)

func main() {
	// Define flags
	output := flag.String("O", "", "Output file name")
	pathFlag := flag.String("P", "", "Path to save the file")
	speedLimit := flag.String("rate-limit", "", "Download speed limit (e.g., 40M, 5K)")
	filedownload := flag.String("i", "", "Downloads all links in a txt file")
	background := flag.Bool("B", false, "Download will be in the background")
	// mirror := flag.String("m", "", "download the entire website being")
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run . [flags] URL")
		return
	}
	url = args[0]
	startTime := time.Now()

	/////////////////////////////////////////////////////// -O / -P
	// Set the output file name
	if *output != "" {
		if *pathFlag != "" {
			outputPath = filepath.Join(*pathFlag, *output) // path + name
		} else {
			outputPath = *output // name
		}
	} else {
		fileName := path.Base(url) // name
		if *pathFlag != "" {
			outputPath = filepath.Join(*pathFlag, fileName) // path + name
		} else {
			outputPath = fileName // name
		}
	}
	///////////////////////////////////////////////////////

	/////////////////////////////////////////////////////// -rate-limit
	// Parse the speed limit
	if *speedLimit != "" {
		bytesPerSec, err = parseSpeedLimit(*speedLimit)
		if err != nil {
			fmt.Printf("Invalid speed limit: %v\n", err)
			return
		}
	} else {
		bytesPerSec = 100000000000 // No limit
	}
	///////////////////////////////////////////////////////

	/////////////////////////////////////////////////////// -i
	if *filedownload != "" {
		Muldownloads(*filedownload, *pathFlag, bytesPerSec)
		return
	}
	///////////////////////////////////////////////////////

	/////////////////////////////////////////////////////// -b
	if *background {
		b = true
		fmt.Print(`Output will be written to "wget-log".`)
		downloadFile(url, outputPath, bytesPerSec)
		return
	}
	///////////////////////////////////////////////////////

	fmt.Printf("start at %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("downloading %s to %s\n", url, outputPath)
	err = downloadFile(url, outputPath, bytesPerSec)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	// Print end time
	endTime := time.Now()
	fmt.Printf("finished at %s\n", endTime.Format("2006-01-02 15:04:05"))
}

func parseSpeedLimit(speedLimit string) (int64, error) { // speedlimit 40m
	unit := speedLimit[len(speedLimit)-1:]  //  40m    m
	value := speedLimit[:len(speedLimit)-1] // 40m   40
	speed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	switch strings.ToLower(unit) {
	case "k":
		return int64(speed * 1000), nil
	case "m":
		return int64(speed * 1000000), nil
	default:
		return 0, err
	}
}

// Download the file
func Muldownloads(file string, pathFlag string, bytesPerSec int64) {
	open, err := os.Open(file)
	if err != nil {
		fmt.Println("error opening txt file", err)
		return
	}
	defer open.Close()
	text, err := ioutil.ReadAll(open)
	if err != nil {
		fmt.Println("error reading file", err)
		return
	}
	urls := strings.Split(string(text), "\n")
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		fileName := path.Base(url)
		outputPath := fileName
		if pathFlag != "" {
			outputPath = filepath.Join(pathFlag, fileName)
		}
		fmt.Printf("downloading %s to %s\n", url, outputPath)
		err = downloadFile(url, outputPath, bytesPerSec)
		if err != nil {
			fmt.Printf("error downloading %s: %v\n", url, err)
		}
	}
}

func downloadFile(url string, outputPath string, bytesPerSec int64) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	// Get the total size of the content
	totalSize, err := strconv.Atoi(response.Header.Get("Content-Length"))
	if err != nil {
		return err
	}
	// Create the file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	// Create a progress writer
	pw := &progressWriter{total: int64(totalSize), startTime: time.Now()}
	// Create a rate-limited reader
	rateLimited := &rateLimitedReader{reader: response.Body, bytesPerSec: bytesPerSec}
	// Write the body to file with progress and rate limiting
	_, err = io.Copy(outFile, io.TeeReader(rateLimited, pw))
	if err != nil {
		return err
	}
	fmt.Println() // Move to the next line after the progress bar
	return nil
}

///////////////////////////////////////////////////////

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloaded += int64(n)
	elapsed := time.Since(pw.startTime).Seconds()
	speed := float64(pw.downloaded) / 1000000 / elapsed
	percentage := float64(pw.downloaded) / float64(pw.total) * 100
	left := (float64(pw.total) - float64(pw.downloaded)) / speed / 1000000
	if b {
		f := float64((pw.total)) / float64(1000000)
		y := "start at " + pw.startTime.Format("2006-01-02 15:04:05") + "\n" + "sending request, awaiting response... status 200 OK" + "\n"
		y += "content size: " + strconv.Itoa(int(pw.total)) + "[~" + strconv.FormatFloat(f, 'f', 2, 64) + "MB]" + "\n"
		y += "saving file to: " + "./" + outputPath + "\n"
		y += "Downloaded [" + url + "]" + "\n"
		pw.endTime = time.Now()
		y += "finished at " + pw.endTime.Format("2006-01-02 15:04:05")
		file, _ := os.Create("wget-log")
		file.WriteString(y)
	} else {
		fmt.Printf("\r%10.0f KiB / %10.0f KiB [%.0f%%] %.0f MB/s %.0f MB %2.0f seconds left", float64(pw.downloaded)/1024, float64(pw.total)/1024, percentage, speed, float64(pw.total)/1000000, left)
	}
	return n, nil
}

func (r *rateLimitedReader) Read(p []byte) (n int, err error) {
	start := time.Now()
	n, err = r.reader.Read(p)
	if n > 0 {
		// n is 10 mb,  bytespersec is 2 mb
		// 10 / 2 = 5
		expectedDuration := time.Duration(n*int(time.Second)) / time.Duration(r.bytesPerSec) // = 5
		elapsed := time.Since(start)                                                         // 3 seconds passed
		sleepDuration := expectedDuration - elapsed                                          // 5 - 3
		if sleepDuration > 0 {                                                               // 2
			time.Sleep(sleepDuration) // sleep for 2 seconds
		}
	}
	return
}

// if *mirror != "" {
// 	err := downloadFile(url, "index.html", bytesPerSec)
// 	if err != nil {
// 		fmt.Println("err mirroring", err)
// 	}
// }
// Print start time
