package utils

import (
	"bufio"
	"os"
	"sort"
	"strings"
)

// GetSortedFileContents 读取文件内容并返回排序后的行
func GetSortedFileContents(filename string) ([]string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")

	// 过滤空行
	var nonEmptyLines []string
	for _, line := range lines {
		if line != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	sort.Strings(nonEmptyLines)
	return nonEmptyLines, nil
}

// WriteLinesToFile 将字符串切片写入文件
func WriteLinesToFile(filename string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filename, []byte(content), 0644)
}

// ReadFileLines 读取文件行
func ReadFileLines(filename string) ([]string, error) {
	var lines []string

	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return lines, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return lines, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), "\"")
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines, scanner.Err()
}

// GetSortedUniqueLines 排序并去重字符串切片
func GetSortedUniqueLines(lines []string) []string {
	// 使用map去重
	uniqueMap := make(map[string]struct{})
	for _, line := range lines {
		uniqueMap[line] = struct{}{}
	}

	// 转换回切片
	var result []string
	for line := range uniqueMap {
		result = append(result, line)
	}

	// 排序
	sort.Strings(result)

	return result
}

// ReadFilesProj 读取并处理files.proj文件
func ReadFilesProj(filename string) ([]string, error) {
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
