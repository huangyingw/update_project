package cmd

import (
	"projupdater/tasks"
	"sync"
)

func DoUpdateProj() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 4)

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
		if err := tasks.RunCscope(); err != nil {
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

	return nil
}
