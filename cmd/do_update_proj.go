package cmd

import (
	"fmt"
	"projupdater/tasks"
	"sync"
)

func DoUpdateProj() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tasks.GenerateFileIndex(); err != nil {
			errChan <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tasks.GenerateRsyncFiles(); err != nil {
			errChan <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tasks.GitRemoteUpdate(); err != nil {
			errChan <- err
		}
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	fmt.Println("开始更新cscope索引...")
	if err := tasks.RunCscope(); err != nil {
		return fmt.Errorf("更新cscope索引失败: %w", err)
	}

	return nil
}
