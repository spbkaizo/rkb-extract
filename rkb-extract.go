package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type TrackInfo struct {
	TrackNumber string
	Title       string
	Performer   string
	Filename    string
}

func main() {

	if len(os.Args) != 4 {
		fmt.Println("Usage: <program> <original_cue_path> <flac_file_path> <album_name>")
		os.Exit(1)
	}

	originalCuePath := os.Args[1]
	flacFilePath := os.Args[2]
	albumName := os.Args[3]

	correctedCuePath := originalCuePath + "-fixed.cue"
	if err := fixCueSheet(originalCuePath, correctedCuePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fix cue sheet: %v\n", err)
		return
	}
	// Assuming shnsplit is available in your system's PATH
	if err := splitFlacFile(correctedCuePath, flacFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to split FLAC file: %v\n", err)
		return
	}
	log.Printf("Split done")

	tracks, err := parseCueSheet(correctedCuePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cue sheet: %v\n", err)
		return
	}
	log.Printf("Tracks Parsed")

	// Assuming FLAC files are in the current directory and named as track number-title by shnsplit
	for _, track := range tracks {
		// Ensure filename matches the shnsplit output format: "track number-title.flac"
		// The title used in the filename should be sanitized to match the filesystem and shnsplit conventions
		//sanitizedTitle := sanitizeTitle(track.Title)
		filename := fmt.Sprintf("%02s-%s.flac", track.TrackNumber, track.Title)
		if err := applyMetadata(filename, track, albumName); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to apply metadata for %s: %v\n", filename, err)
		}

	}
	log.Printf("Complete")
}

// Helper function to sanitize title for filenames
func sanitizeTitle(title string) string {
	// Example sanitization, adjust according to your needs and `shnsplit` behavior
	re := regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	sanitized := re.ReplaceAllString(title, "")
	//sanitized = strings.ReplaceAll(sanitized, " ", "_") // Replace spaces with underscores
	return sanitized
}

// copyCueSheet just copies the original cue sheet to a new file without modification.
func copyCueSheet(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}

// splitFlacFile uses shnsplit to split the FLAC file according to the cue sheet.
func splitFlacFile(cuePath, flacPath string) error {
	cmd := exec.Command("shnsplit", "-O", "always", "-f", cuePath, "-o", "flac", "-t", "%n-%t", flacPath)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func fixCueSheet(inputPath, outputPath string) error {
	// Open the input cue file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
		return err
	}
	defer inputFile.Close()

	// Create the output cue file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		return err
	}
	defer outputFile.Close()

	scanner := bufio.NewScanner(inputFile)
	writer := bufio.NewWriter(outputFile)

	// Regular expression to match INDEX 01 lines with timestamps
	indexRegex := regexp.MustCompile(`INDEX 01 (\d+):(\d+):(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Process and modify INDEX 01 lines to convert hours into minutes
		modifiedLine := indexRegex.ReplaceAllStringFunc(line, func(match string) string {
			parts := indexRegex.FindStringSubmatch(match)
			if len(parts) == 4 {
				// Convert hours and minutes to total minutes
				hours, _ := strconv.Atoi(parts[1])
				minutes, _ := strconv.Atoi(parts[2])
				seconds := parts[3]

				totalMinutes := hours*60 + minutes

				// Return the modified timestamp with hours converted to minutes and added .000 for milliseconds
				return fmt.Sprintf("INDEX 01 %02d:%s.000", totalMinutes, seconds)
			}
			return match
		})

		// Write the modified line to the output file
		_, err := writer.WriteString(modifiedLine + "\n")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to output file: %v\n", err)
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading from input file: %v\n", err)
		return err
	}

	if err := writer.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Error flushing output to file: %v\n", err)
		return err
	}

	fmt.Println("Cue sheet modification completed successfully.")
	return err
}

// parseCueSheet parses the cue sheet and returns a slice of TrackInfo.
func parseCueSheet(cuePath string) ([]TrackInfo, error) {
	file, err := os.Open(cuePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tracks []TrackInfo
	scanner := bufio.NewScanner(file)

	// Use to capture if we are currently within a TRACK block
	//withinTrack := false

	for scanner.Scan() {
		line := scanner.Text()

		/*
			if strings.HasPrefix(line, "\tTRACK") {
				log.Printf("TRACK found")
				withinTrack = true                     // Now within a track block
				trackNumber := strings.Fields(line)[1] // Assuming TRACK number format is consistent
				tracks = append(tracks, TrackInfo{TrackNumber: trackNumber})
			} else if strings.HasPrefix(line, "TITLE") && withinTrack {
				// Only process TITLE if within a track block
				title := strings.Trim(line[len("TITLE")+1:], "\"")
				tracks[len(tracks)-1].Title = title
			} else if strings.HasPrefix(line, "PERFORMER") && withinTrack {
				// Only process PERFORMER if within a track block
				performer := strings.Trim(line[len("PERFORMER")+1:], "\"")
				tracks[len(tracks)-1].Performer = performer
			} else if strings.HasPrefix(line, "FILE") {
				withinTrack = false // Exited the track-specific block upon encountering a new FILE line
			}
		*/
		// Parse track information
		if strings.HasPrefix(line, "\tTRACK") {
			trackNumber := strings.Fields(line)[1] // Assuming TRACK number format is consistent
			scanner.Scan()                         // Next line should be the title
			titleLine := scanner.Text()
			title := extractMetadata("TITLE", titleLine)

			scanner.Scan() // Next line should be the performer
			performerLine := scanner.Text()
			performer := extractMetadata("PERFORMER", performerLine)
			tracks = append(tracks, TrackInfo{
				TrackNumber: trackNumber,
				Title:       title,
				Performer:   performer,
			})

		}
	}
	log.Printf("tracks: %v", tracks)
	return tracks, scanner.Err()
}

func extractMetadata(field, line string) string {
	re := regexp.MustCompile(fmt.Sprintf(`%s "(.+)"`, field))
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func parseLine(line string) string {
	re := regexp.MustCompile(`\"(.+?)\"`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func applyMetadata(filepath string, track TrackInfo, albumName string) error {
	//log.Printf("Working on %v, %v, %v", filepath, track, albumName)
	cmd := exec.Command("metaflac",
		"--remove-all-tags",
		"--set-tag=TITLE="+track.Title,
		"--set-tag=ARTIST="+track.Performer,
		"--set-tag=ALBUM="+albumName,
		filepath)

	// Capture and display any errors from metaflac for diagnostics
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply metadata, error: %v, output: %s", err, output)
	}

	return nil
}
