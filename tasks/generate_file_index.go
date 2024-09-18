package tasks

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"projupdater/utils"
	"sort"
	"strings"
)

func GenerateFileIndex() error {
	targetDir, err := os.Getwd()
	if err != nil {
		return err
	}

	filesProjPath := filepath.Join(targetDir, "files.proj")
	if _, err := os.Stat(filesProjPath); os.IsNotExist(err) {
		return fmt.Errorf("当前目录下不存在 files.proj 文件")
	}

	// 备份原有的 files.proj
	backupPath := filesProjPath + ".bak"
	err = utils.CopyFile(filesProjPath, backupPath)
	if err != nil {
		return err
	}

	// 准备临时文件
	tempFile, err := ioutil.TempFile("", "files.proj.*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// 读取配置文件
	pruneSuffixes, err := readConfig("prunefix.conf")
	if err != nil {
		return err
	}
	pruneFiles, err := readConfig("prunefile.conf")
	if err != nil {
		return err
	}
	includeFiles, err := readConfig("includefile.conf")
	if err != nil {
		return err
	}

	// 构建 find 命令
	findArgs := []string{"."}
	for _, suf := range pruneSuffixes {
		findArgs = append(findArgs, "!", "-path", suf)
	}
	findArgs = append(findArgs, "-type", "f", "-size", "+0")
	findArgs = append(findArgs, "-print")

	cmd := exec.Command("find", findArgs...)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// 处理输出
	files := strings.Split(string(output), "\n")
	files = append(files, includeFiles...)
	files = filterFiles(files, pruneFiles)
	files = uniqueStrings(files)
	sort.Strings(files)

	// 写入临时文件
	for _, file := range files {
		if file != "" {
			fmt.Fprintf(tempFile, "\"%s\"\n", file)
		}
	}

	// 替换原有的 files.proj
	err = os.Rename(tempFile.Name(), filesProjPath)
	if err != nil {
		// 恢复备份
		utils.CopyFile(backupPath, filesProjPath)
		return err
	}

	// 更新 ~/all.proj
	allProjPath := filepath.Join(os.Getenv("HOME"), "all.proj")
	targetEntry := fmt.Sprintf("\"%s/files.proj\"", targetDir)
	err = appendUniqueLine(allProjPath, targetEntry)
	if err != nil {
		return err
	}

	fmt.Println("成功更新 files.proj")
	return nil
}

func readConfig(filename string) ([]string, error) {
	var lines []string
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return lines, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func filterFiles(files, pruneFiles []string) []string {
	pruneMap := make(map[string]struct{})
	for _, file := range pruneFiles {
		pruneMap[file] = struct{}{}
	}
	var result []string
	for _, file := range files {
		if _, found := pruneMap[file]; !found {
			result = append(result, file)
		}
	}
	return result
}

func uniqueStrings(input []string) []string {
	uniqueMap := make(map[string]struct{})
	var result []string
	for _, str := range input {
		if _, found := uniqueMap[str]; !found {
			uniqueMap[str] = struct{}{}
			result = append(result, str)
		}
	}
	return result
}

func appendUniqueLine(filePath, line string) error {
	// 检查是否已存在
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == line {
			return nil
		}
	}

	// 追加新行
	_, err = file.WriteString(line + "\n")
	return err
}
