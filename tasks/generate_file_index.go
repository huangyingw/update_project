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
	"sync"
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
	tempFile.Close() // 关闭文件，让find命令可以直接写入

	// 准备错误日志文件
	errFile, err := ioutil.TempFile("", "files.proj.*.errors")
	if err != nil {
		return err
	}
	defer os.Remove(errFile.Name())
	errFile.Close() // 关闭文件，让find命令可以直接写入

	// 读取配置文件 - 并行读取提高性能
	var (
		pruneSuffixes []string
		pruneFiles    []string
		includeFiles  []string
		wg            sync.WaitGroup
		mu            sync.Mutex
		readError     error
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		ps, err := readConfig("prunefix.conf")
		if err != nil {
			mu.Lock()
			readError = err
			mu.Unlock()
			return
		}
		mu.Lock()
		pruneSuffixes = ps
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		pf, err := readConfig("prunefile.conf")
		if err != nil {
			mu.Lock()
			readError = err
			mu.Unlock()
			return
		}
		mu.Lock()
		pruneFiles = pf
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		inf, err := readConfig("includefile.conf")
		if err != nil {
			mu.Lock()
			readError = err
			mu.Unlock()
			return
		}
		mu.Lock()
		includeFiles = inf
		mu.Unlock()
	}()

	wg.Wait()

	if readError != nil {
		return readError
	}

	// 并行执行两条find命令
	var findWg sync.WaitGroup
	findErrChan := make(chan error, 2)

	findWg.Add(2)
	go func() {
		defer findWg.Done()
		// 优化find命令：增加-print0和使用xargs执行grep以提高性能
		cmd1 := buildOptimizedFindCommand(pruneSuffixes, false, tempFile.Name(), errFile.Name())
		fmt.Println("执行命令1:", cmd1)
		findCmd1 := exec.Command("sh", "-c", cmd1)
		if err := findCmd1.Run(); err != nil {
			findErrChan <- fmt.Errorf("find命令1执行出错：%v", err)
		}
	}()

	go func() {
		defer findWg.Done()
		cmd2 := buildOptimizedFindCommand(includeFiles, true, tempFile.Name(), errFile.Name())
		fmt.Println("执行命令2:", cmd2)
		findCmd2 := exec.Command("sh", "-c", cmd2)
		if err := findCmd2.Run(); err != nil {
			findErrChan <- fmt.Errorf("find命令2执行出错：%v", err)
		}
	}()

	findWg.Wait()
	close(findErrChan)

	// 检查find命令是否有错误，但继续执行而不中断
	for err := range findErrChan {
		fmt.Printf("警告：%v\n", err)
	}

	// 重新读取临时文件中的内容
	fileContent, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		return err
	}
	files := strings.Split(string(fileContent), "\n")

	// 过滤pruneFiles中的文件
	files = filterFiles(files, pruneFiles)

	// 过滤掉update_proj和update_proj.log文件
	var filteredFiles []string
	for _, file := range files {
		if file != "./update_proj" && file != "./update_proj.log" {
			filteredFiles = append(filteredFiles, file)
		}
	}
	files = filteredFiles

	files = uniqueStrings(files)
	sort.Strings(files)

	// 创建新的临时文件来写入最终结果
	finalTempFile, err := ioutil.TempFile("", "files.proj.*.final")
	if err != nil {
		return err
	}
	defer os.Remove(finalTempFile.Name())

	// 写入最终文件
	for _, file := range files {
		if file != "" {
			fmt.Fprintf(finalTempFile, "\"%s\"\n", file)
		}
	}
	finalTempFile.Close()

	// 替换原有的 files.proj
	err = os.Rename(finalTempFile.Name(), filesProjPath)
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

// 构建优化的find命令字符串，使用-print0和xargs提高性能
func buildOptimizedFindCommand(patterns []string, isInclude bool, outputFile, errorFile string) string {
	var cmd strings.Builder
	cmd.WriteString("find . -type f ")

	// 构建路径参数
	if len(patterns) > 0 {
		cmd.WriteString("\\( ")
		for i, pattern := range patterns {
			if i > 0 {
				cmd.WriteString("-o ")
			}

			cmd.WriteString(fmt.Sprintf("-path '%s' ", pattern))
		}
		cmd.WriteString("\\) ")

		if isInclude {
			// 包含模式: 匹配指定路径的文件，大小小于9000k
			cmd.WriteString("-size -9000k ")
		} else {
			// 排除模式: 排除指定路径，然后匹配其他大小大于0的文件
			cmd.WriteString("-a -prune -o -size +0 -type f ")
		}
	} else {
		// 如果没有模式，则默认查找所有文件
		if isInclude {
			cmd.WriteString("-size -9000k ")
		} else {
			cmd.WriteString("-size +0 ")
		}
	}

	// 使用-print0和xargs配合grep提高性能
	cmd.WriteString("-print0 | xargs -0 -I{} -P8 grep -Il \"\" {} 2>>")
	cmd.WriteString(errorFile)

	// 输出重定向
	if isInclude {
		cmd.WriteString(fmt.Sprintf(" >>%s || true", outputFile))
	} else {
		cmd.WriteString(fmt.Sprintf(" >%s || true", outputFile))
	}

	return cmd.String()
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
