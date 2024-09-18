package tasks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func RunCscope() error {
	sourceFile := "cscopesourcefile.bak"
	tempCscopeOut := "cscope.out.bak"
	tempCscopeIn := "cscope.out.bak.in"
	tempCscopePo := "cscope.out.bak.po"

	defer func() {
		os.Remove(sourceFile + ".bak")
		os.Remove(tempCscopeOut)
		os.Remove(tempCscopeIn)
		os.Remove(tempCscopePo)
	}()

	// 复制 files.proj
	err := copyFile("files.proj", sourceFile)
	if err != nil {
		return err
	}

	// 替换特殊字符
	err = replaceInFile(sourceFile, `\\ `, ` `)
	if err != nil {
		return err
	}

	// 运行 cscope
	cmd := exec.Command("cscope", "-bq", "-i", sourceFile, "-f", tempCscopeOut)
	err = cmd.Run()
	if err != nil {
		return err
	}

	// 替换索引文件
	err = os.Rename(tempCscopeOut, "cscope.out")
	if err != nil {
		return err
	}
	err = os.Rename(tempCscopeIn, "cscope.in.out")
	if err != nil {
		return err
	}
	err = os.Rename(tempCscopePo, "cscope.po.out")
	if err != nil {
		return err
	}

	fmt.Println("成功更新 cscope 索引文件")
	return nil
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.WriteFile(dst, input, 0644)
	return err
}

func replaceInFile(filename, oldStr, newStr string) error {
	input, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	output := strings.ReplaceAll(string(input), oldStr, newStr)
	err = os.WriteFile(filename, []byte(output), 0644)
	return err
}
