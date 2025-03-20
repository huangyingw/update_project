package tasks

import (
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

	// 创建处理后的临时文件
	finalTempFile, err := ioutil.TempFile("", "files.proj.*.final")
	if err != nil {
		return err
	}
	defer os.Remove(finalTempFile.Name())
	
	// 读取临时文件，排序并去重
	fileBytes, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		return err
	}
	
	// 处理文件路径 - 与bash脚本保持一致
	fileLines := strings.Split(string(fileBytes), "\n")
	var processedLines []string
	
	for _, line := range fileLines {
		if line == "" {
			continue
		}
		// 去除开头的 ./
		line = strings.TrimPrefix(line, "./")
		// 添加 "./ 前缀和 " 后缀
		if !strings.HasPrefix(line, "\"./") {
			line = "\"./"+line+"\""
		}
		processedLines = append(processedLines, line)
	}
	
	// 对行进行排序并去重
	sort.Strings(processedLines)
	processedLines = uniqueStrings(processedLines)
	
	// 读取 prunefile.conf 并进行文件排除
	pruneFileMap := make(map[string]struct{})
	for _, file := range pruneFiles {
		pruneFileMap[file] = struct{}{}
	}
	
	// 将处理后的行写入临时文件
	for _, line := range processedLines {
		// 跳过空行和以 "../." 开头的条目
		if line == "" || strings.HasPrefix(line, "\"../.") {
			continue
		}
		
		// 确保格式一致，将 "." 替换为 "./"
		if line == "\"." {
			line = "\"./\""
		}
		
		// 检查是否在排除列表中
		_, excluded := pruneFileMap[line]
		if !excluded {
			fmt.Fprintln(finalTempFile, line)
		}
	}
	
	finalTempFile.Close()
	
	// 验证生成的文件不为空
	fi, err := os.Stat(finalTempFile.Name())
	if err != nil || fi.Size() == 0 {
		return fmt.Errorf("生成的文件为空")
	}
	
	// 检查文件是否有变化
	diffCmd := exec.Command("diff", "-q", finalTempFile.Name(), filesProjPath)
	if diffCmd.Run() == nil {
		fmt.Println("files.proj 没有变化，保留原文件")
		return nil
	}
	
	// 替换原有的 files.proj
	tempNewFile := filesProjPath + ".new"
	if err := utils.CopyFile(finalTempFile.Name(), tempNewFile); err != nil {
		return err
	}
	if err := os.Rename(tempNewFile, filesProjPath); err != nil {
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

// 构建优化的find命令字符串
func buildOptimizedFindCommand(patterns []string, isInclude bool, outputFile, errorFile string) string {
	var cmd strings.Builder
	
	if isInclude {
		// 包含模式
		cmd.WriteString("find . ")
		
		// 添加包含模式
		if len(patterns) > 0 {
			cmd.WriteString("\\( ")
			for i, pattern := range patterns {
				if i > 0 {
					cmd.WriteString("-o ")
				}
				// 统一使用-wholename参数，与bash脚本保持一致
				cmd.WriteString(fmt.Sprintf("-wholename '%s' ", pattern))
			}
			cmd.WriteString("\\) ")
			cmd.WriteString("-type f -size -9000k ")
		} else {
			cmd.WriteString("-type f -size -9000k ")
		}
	} else {
		// 排除模式 - 在Linux和macOS上使用统一的方式
		cmd.WriteString("find . ")
		
		// 如果有模式，构建排除条件
		if len(patterns) > 0 {
			cmd.WriteString("\\( ")
			
			// 对所有模式统一使用-wholename
			for i, pattern := range patterns {
				if i > 0 {
					cmd.WriteString("-o ")
				}
				cmd.WriteString(fmt.Sprintf("-wholename '%s' ", pattern))
			}
			
			// 使用-prune来排除这些模式，再使用-o来包含其他文件
			cmd.WriteString("\\) -a -prune -o ")
		}
		
		// 添加常规文件查找
		cmd.WriteString("-size +0 -type f ")
	}
	
	// 使用grep过滤文本文件 - 与bash脚本保持一致
	// 使用 + 而不是 \; 来提高性能
	cmd.WriteString("-exec grep -Il \"\" {} + 2>>")
	cmd.WriteString(errorFile)
	
	// 添加输出重定向
	if isInclude {
		cmd.WriteString(fmt.Sprintf(" >>%s || true", outputFile))
	} else {
		cmd.WriteString(fmt.Sprintf(" >%s || true", outputFile))
	}
	
	return cmd.String()
}

func readConfig(filename string) ([]string, error) {
	var lines []string
	
	// 先检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return lines, nil
	}
	
	// 读取文件内容
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	// 处理文件内容
	fileLines := strings.Split(string(content), "\n")
	for _, line := range fileLines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	
	return lines, nil
}

func uniqueStrings(input []string) []string {
	// 对于大量数据，使用map比循环检查更高效
	seen := make(map[string]struct{}, len(input))
	var result []string
	
	for _, str := range input {
		if str == "" {
			continue
		}
		if _, found := seen[str]; !found {
			seen[str] = struct{}{}
			result = append(result, str)
		}
	}
	return result
}

func appendUniqueLine(filePath, line string) error {
	// 先检查文件是否存在，如果不存在则创建
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 创建目录（如果需要）
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}
		// 创建文件并写入行
		return ioutil.WriteFile(filePath, []byte(line+"\n"), 0644)
	}
	
	// 读取整个文件
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	
	// 检查行是否已存在
	lines := strings.Split(string(content), "\n")
	for _, existingLine := range lines {
		if existingLine == line {
			return nil // 行已存在，不需要添加
		}
	}
	
	// 追加新行
	lines = append(lines, line)
	
	// 去除空行并排序
	var nonEmptyLines []string
	for _, l := range lines {
		if l != "" {
			nonEmptyLines = append(nonEmptyLines, l)
		}
	}
	sort.Strings(nonEmptyLines)
	
	// 写回文件
	return ioutil.WriteFile(filePath, []byte(strings.Join(nonEmptyLines, "\n")+"\n"), 0644)
}

