package tasks

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"projupdater/utils"
	"sort"
	"strings"
)

func GenerateRsyncFiles() error {
	// 检查 files.proj 是否存在
	if _, err := os.Stat("files.proj"); os.IsNotExist(err) {
		return fmt.Errorf("当前目录下不存在 files.proj 文件")
	}

	// 准备临时文件
	tempFile, err := ioutil.TempFile("", "rsync.files.*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// 使用和原始bash脚本相同的逻辑来过滤文件
	// 读取files.proj文件
	filesProjContent, err := ioutil.ReadFile("files.proj")
	if err != nil {
		return err
	}

	// 获取当前Git分支
	currentBranch, err := utils.GetCurrentBranch()
	if err != nil {
		fmt.Printf("警告：获取Git分支失败：%v，使用'main'作为默认分支\n", err)
		currentBranch = "main"
	}

	// 构建分支对应的diff文件路径
	branchDiffFile := fmt.Sprintf("./%s.gdio.diff", currentBranch)

	// 处理文件内容，移除引号并过滤
	lines := strings.Split(string(filesProjContent), "\n")
	var filteredLines []string

	for _, line := range lines {
		// 移除引号
		line = strings.Trim(line, "\"")
		if line == "" {
			continue
		}

		// 过滤掉不需要的文件
		if strings.HasSuffix(line, ".log") ||
			strings.HasSuffix(line, ".bak") ||
			strings.HasSuffix(line, ".tmp") ||
			strings.HasSuffix(line, ".swp") ||
			strings.Contains(line, ".git") ||
			strings.Contains(line, ".svn") ||
			line == branchDiffFile {
			continue
		}

		filteredLines = append(filteredLines, line)
	}

	// 确保包含.gitconfig文件
	hasGitConfig := false
	for _, line := range filteredLines {
		if line == "./.gitconfig" {
			hasGitConfig = true
			break
		}
	}
	if !hasGitConfig {
		filteredLines = append(filteredLines, "./.gitconfig")
	}

	// 确保包含update_proj和update_proj.log文件
	hasUpdateProj := false
	hasUpdateProjLog := false
	for _, line := range filteredLines {
		if line == "./update_proj" {
			hasUpdateProj = true
		}
		if line == "./update_proj.log" {
			hasUpdateProjLog = true
		}
	}
	if !hasUpdateProj {
		filteredLines = append(filteredLines, "./update_proj")
	}
	if !hasUpdateProjLog {
		filteredLines = append(filteredLines, "./update_proj.log")
	}

	// 排序
	sort.Strings(filteredLines)

	// 写入临时文件，每行一个文件路径，不带引号
	for _, line := range filteredLines {
		fmt.Fprintln(tempFile, line)
	}
	tempFile.Close()

	// 读取临时文件内容
	content, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		return err
	}

	// 写入 rsync.files
	err = ioutil.WriteFile("rsync.files", content, 0644)
	if err != nil {
		return err
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
