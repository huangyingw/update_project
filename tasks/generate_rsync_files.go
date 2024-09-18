package tasks

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func GenerateRsyncFiles() error {
	// 读取 files.proj
	filesProj, err := readFilesProj("files.proj")
	if err != nil {
		return err
	}

	// 读取 prunefix.rsync 和 includefile.rsync
	pruneSuffixes, err := readConfig("prunefix.rsync")
	if err != nil {
		return err
	}
	includeFiles, err := readConfig("includefile.rsync")
	if err != nil {
		return err
	}

	// 过滤 files.proj
	files := filterBySuffix(filesProj, pruneSuffixes)
	files = append(files, includeFiles...)
	files = uniqueStrings(files)

	// 写入 rsync.files
	rsyncFiles, err := os.Create("rsync.files")
	if err != nil {
		return err
	}
	defer rsyncFiles.Close()

	for _, file := range files {
		fmt.Fprintln(rsyncFiles, file)
	}

	fmt.Println("成功生成 rsync.files")
	return nil
}

func readFilesProj(filename string) ([]string, error) {
	var files []string
	file, err := os.Open(filename)
	if err != nil {
		return files, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), "\"")
		line = strings.ReplaceAll(line, "\\ ", " ")
		if line != "" {
			files = append(files, line)
		}
	}
	return files, scanner.Err()
}

func filterBySuffix(files, suffixes []string) []string {
	var result []string
	for _, file := range files {
		match := false
		for _, suf := range suffixes {
			if strings.HasSuffix(file, suf) {
				match = true
				break
			}
		}
		if !match {
			result = append(result, file)
		}
	}
	return result
}
