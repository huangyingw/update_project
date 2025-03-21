package utils

import (
	"io"
	"os"
	"strings"
)

func CopyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return &os.PathError{Op: "copy", Path: src, Err: err}
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// CopyFileAndReplace 复制文件并替换内容
func CopyFileAndReplace(src, dst, oldStr, newStr string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// 替换内容
	output := strings.ReplaceAll(string(input), oldStr, newStr)

	// 写入到目标文件
	return os.WriteFile(dst, []byte(output), 0644)
}

// ReplaceInFile 替换文件中的内容
func ReplaceInFile(filename, oldStr, newStr string) error {
	input, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	output := strings.ReplaceAll(string(input), oldStr, newStr)
	return os.WriteFile(filename, []byte(output), 0644)
}
