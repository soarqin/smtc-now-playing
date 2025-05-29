package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// Monitor represents a process monitor that can start a subprocess and monitor its output
type Monitor struct {
	exitGroup  ProcessExitGroup
	cmd        *exec.Cmd
	command    string
	args       []string
	outputChan chan string
	errorChan  chan error
	cancelChan chan struct{}
	isRunning  bool
	wg         sync.WaitGroup
	mutex      sync.RWMutex
}

// NewMonitor creates a new Monitor instance
func NewMonitor(g ProcessExitGroup, command string, args ...string) *Monitor {
	return &Monitor{
		exitGroup:  g,
		command:    command,
		args:       args,
		outputChan: make(chan string, 64),
		errorChan:  make(chan error, 8),
		cancelChan: make(chan struct{}),
		isRunning:  false,
	}
}

func (m *Monitor) internalStart() error {
	if m.isRunning {
		return fmt.Errorf("process is already running")
	}

	// Create command with context for cancellation
	m.cmd = exec.Command(m.command, m.args...)

	// Get stdout pipe
	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	if err := m.exitGroup.AddProcess(m.cmd.Process); err != nil {
		m.cmd.Process.Kill()
		return fmt.Errorf("failed to assign process to job object: %w", err)
	}

	m.isRunning = true

	// Start goroutine to read stdout
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case m.outputChan <- scanner.Text():
			case <-m.cancelChan:
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case m.errorChan <- fmt.Errorf("stdout scanner error: %w", err):
			}
		}
	}()

	return nil
}

func (m *Monitor) StartProcess() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	err := m.internalStart()
	if err != nil {
		return err
	}

	// Start goroutine to wait for process completion
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			err := m.cmd.Wait()
			m.mutex.Lock()
			m.isRunning = false
			m.mutex.Unlock()

			if err != nil && m.cancelChan != nil { // Only report error if not cancelled
				select {
				case m.errorChan <- fmt.Errorf("process exited with error: %w", err):
				}
				time.Sleep(1 * time.Second)
				m.mutex.Lock()
				defer m.mutex.Unlock()
				m.internalStart()
			} else {
				break
			}
		}
	}()

	return nil
}

// GetOutputChannel returns the channel that receives stdout lines
func (m *Monitor) GetOutputChannel() <-chan string {
	return m.outputChan
}

// GetErrorChannel returns the channel that receives errors
func (m *Monitor) GetErrorChannel() <-chan error {
	return m.errorChan
}

// IsRunning returns whether the process is currently running
func (m *Monitor) IsRunning() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isRunning
}

// Stop stops the monitoring process and kills the subprocess if running
func (m *Monitor) Stop() error {
	// Cancel context to stop all goroutines
	close(m.cancelChan)
	m.cancelChan = nil

	// Kill the process if it's still running
	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Wait for all goroutines to finish
	m.wg.Wait()

	// Close channels
	close(m.outputChan)
	close(m.errorChan)

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.isRunning = false
	return nil
}

// Join waits for the process to complete naturally
func (m *Monitor) Join() error {
	if !m.IsRunning() {
		return fmt.Errorf("no process is running")
	}

	// Wait for all goroutines to finish
	m.wg.Wait()
	return nil
}
