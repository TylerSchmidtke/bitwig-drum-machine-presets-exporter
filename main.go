package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	bitwigSamplePathMac    = "/Library/Application Support/Bitwig/Bitwig Studio/installed-packages/1.0/samples/"
	bitwigPresetGlobStrMac = "/Library/Application Support/Bitwig/Bitwig Studio/installed-packages/1.0/presets/Bitwig/Nektar's Acoustic Drums/*.bwpreset"
)

func main() {
	log := logrus.New()

	// TODO: Add support for other platforms
	if runtime.GOOS != "darwin" {
		log.Fatalf("sorry, the tool only supports MacOS today...")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to find home directory for getting path to Bitwig samples: %s", err.Error())
	}

	globPath := filepath.Join(homeDir, bitwigPresetGlobStrMac)
	presetFiles, err := filepath.Glob(globPath)
	if err != nil {
		log.Fatalf("failed to glob files: %s", err.Error())
	}

	_, err = os.Stat("export")
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("creating export directory")
			if err := os.Mkdir("export", 0700); err != nil {
				log.Fatalf("failed to create export directory: %s", err.Error())
			}
		} else {
			log.Fatalf("failed to stat export directory: %s", err.Error())
		}
	}

	samplePath := filepath.Join(homeDir, bitwigSamplePathMac)

	for _, presetPath := range presetFiles {
		samples, err := scanFileForSamples(presetPath)
		if err != nil {
			log.Fatalf("failed to process path %s: %s", presetPath, err.Error())
		}

		presetName := getPresetName(presetPath)
		fmt.Printf("samples for preset '%s' (path: %s): \n", presetPath, presetName)
		processPreset(presetName, samplePath, samples)
	}
}

// processPreset takes the samples for a preset and adds them to velocity specific directories
// along with an 'all' directory which contains all samples
func processPreset(name, baseSamplePath string, samples []string) error {
	for _, s := range samples {
		samplePath := filepath.Join(baseSamplePath, s)
		sampleType := getSampleType(s)
		sampleVelocity := getSampleVelocity(s)
		if err := processSample(name, samplePath, sampleType, sampleVelocity); err != nil {
			logrus.Errorf("failed to process sample '%s' for preset '%s': %s", s, name, err.Error())
		}
	}

	return nil
}

// processSample handles a specific sample
func processSample(name, samplePath, typ, vel string) error {
	logrus.Infof("copying sample for preset '%s': %s", name, samplePath)
	// Rimshots only have two samples - going to add them their own set and the 'all' set
	if strings.Contains(samplePath, "Rim") {
		dest := filepath.Join("export", name, "rim")
		if err := copySampleFile(samplePath, getDestFilePath(dest, samplePath)); err != nil {
			return err
		}
	} else {
		dest := filepath.Join("export", name, vel)
		if err := copySampleFile(samplePath, getDestFilePath(dest, samplePath)); err != nil {
			return err
		}
	}

	// add to the all directory as well
	dest := filepath.Join("export", name, "all")
	if err := copySampleFile(samplePath, getDestFilePath(dest, samplePath)); err != nil {
		return err
	}

	return nil
}

func getDestFilePath(dir, samplePath string) string {
	return filepath.Join(dir, filepath.Base(samplePath))
}

// copySampleFile copies the specified sample file from the src to the dest
func copySampleFile(src, dest string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Create all parent directories if needed
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, source)

	return err
}

func scanFileForSamples(path string) ([]string, error) {
	samplePaths := make([]string, 0)
	wavInFolderRE, err := regexp.Compile(`Bitwig[a-zA-Z0-9\s\/:\-']+\.wav`)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		text := scanner.Text()
		match := wavInFolderRE.FindAllString(text, -1)

		if match != nil {
			samplePaths = append(samplePaths, cleanBwPresetSampleText(match[len(match)-1]))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return samplePaths, nil
}

func cleanBwPresetSampleText(s string) string {
	s = strings.Replace(s, ":7/samples", "", 1)
	return s
}

func getSampleType(s string) string {
	s = strings.TrimPrefix(s, "Bitwig/Nektar's Acoustic Drums/samples/")
	s = strings.Split(s, "/")[0]

	return s
}

// getPresetName gets the name of the preset from the file path
func getPresetName(s string) string {
	re, _ := regexp.Compile(`([\w -]+).bwpreset`)
	match := re.FindStringSubmatch(s)

	// didn't find what we wanted, try to hack something together
	if match == nil || len(match) < 2 {
		return strings.TrimSuffix(filepath.Base(s), ".bwpreset")
	}

	return match[1]
}

// getSampleVelocity corresponds with the number/letter value at the end of the wav
func getSampleVelocity(s string) string {
	re, _ := regexp.Compile(`([a-zA-Z0-9]{1,2}).wav`)
	match := re.FindStringSubmatch(s)

	if match == nil || len(match) < 2 {
		s = filepath.Base(s)
		s = strings.TrimSuffix(s, ".wav")
		split := strings.Split(s, " ")
		s = split[len(split)-1]

		return s
	}

	return match[1]
}
