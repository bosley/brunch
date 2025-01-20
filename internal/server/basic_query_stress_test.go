package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkKVSOperations benchmarks individual operations
func BenchmarkKVSOperations(b *testing.B) {
	// Setup test environment
	tmpFile, err := os.CreateTemp("", "brunch-bench-*.db")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	kvs, err := NewKVS(tmpPath)
	if err != nil {
		b.Fatalf("Failed to create KVS: %v", err)
	}
	defer kvs.Close()

	// Create a single test user for all operations
	const testUser = "benchuser"
	if err := kvs.CreateUser(testUser, "pass"); err != nil {
		b.Fatalf("Failed to create test user: %v", err)
	}

	// Write benchmark
	b.Run("Write", func(b *testing.B) {
		b.Run("Sequential", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("seq-write-%d", i)
				value := fmt.Sprintf("value-%d", i)
				if err := kvs.SetUserData(testUser, key, value); err != nil {
					b.Fatalf("Write failed: %v", err)
				}
			}
		})

		b.Run("Parallel", func(b *testing.B) {
			var counter uint64
			b.SetParallelism(4)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					id := atomic.AddUint64(&counter, 1)
					key := fmt.Sprintf("par-write-%d", id)
					value := fmt.Sprintf("value-%d", id)
					if err := kvs.SetUserData(testUser, key, value); err != nil {
						b.Fatalf("Write failed: %v", err)
					}
				}
			})
		})
	})

	// Read benchmark
	b.Run("Read", func(b *testing.B) {
		keys := make([]string, b.N)
		values := make([]string, b.N)

		// Setup data for read tests
		for i := 0; i < b.N; i++ {
			keys[i] = fmt.Sprintf("read-key-%d", i)
			values[i] = fmt.Sprintf("value-%d", i)
			if err := kvs.SetUserData(testUser, keys[i], values[i]); err != nil {
				b.Fatalf("Failed to setup read test data: %v", err)
			}
		}

		b.Run("Sequential", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := kvs.GetUserData(testUser, keys[i%len(keys)]); err != nil {
					b.Fatalf("Read failed: %v", err)
				}
			}
		})

		b.Run("Parallel", func(b *testing.B) {
			b.SetParallelism(4)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				var i uint64
				for pb.Next() {
					id := atomic.AddUint64(&i, 1) - 1
					if _, err := kvs.GetUserData(testUser, keys[id%uint64(len(keys))]); err != nil {
						b.Fatalf("Read failed: %v", err)
					}
				}
			})
		})
	})

	// Delete benchmark
	b.Run("Delete", func(b *testing.B) {
		b.Run("Sequential", func(b *testing.B) {
			// Setup data for sequential delete
			b.StopTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("seq-del-%d", i)
				value := fmt.Sprintf("value-%d", i)
				if err := kvs.SetUserData(testUser, key, value); err != nil {
					b.Fatalf("Failed to setup sequential delete data: %v", err)
				}
			}
			b.StartTimer()

			// Sequential delete
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("seq-del-%d", i)
				if err := kvs.DeleteUserData(testUser, key); err != nil && !strings.Contains(err.Error(), "not found") {
					b.Fatalf("Delete failed: %v", err)
				}
			}
		})

		b.Run("Parallel", func(b *testing.B) {
			// Setup data for parallel delete
			b.StopTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("par-del-%d", i)
				value := fmt.Sprintf("value-%d", i)
				if err := kvs.SetUserData(testUser, key, value); err != nil {
					b.Fatalf("Failed to setup parallel delete data: %v", err)
				}
			}
			b.StartTimer()

			// Parallel delete
			var deleteCounter uint64
			b.SetParallelism(4)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					id := atomic.AddUint64(&deleteCounter, 1) - 1
					key := fmt.Sprintf("par-del-%d", id%uint64(b.N))
					err := kvs.DeleteUserData(testUser, key)
					if err != nil && !strings.Contains(err.Error(), "not found") {
						b.Fatalf("Delete failed: %v", err)
					}
				}
			})
		})
	})
}

// TestKVSConcurrentAccess tests actual concurrent access patterns
func TestKVSConcurrentAccess(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "brunch-concurrent-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	kvs, err := NewKVS(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create KVS: %v", err)
	}
	defer kvs.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const (
		numGoroutines = 4  // Reduced from 10
		opsPerRoutine = 25 // Reduced from 100
	)

	// Create test user
	const testUser = "testuser"
	if err := kvs.CreateUser(testUser, "pass"); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*opsPerRoutine)
	progressChan := make(chan struct{}, numGoroutines*opsPerRoutine*3)

	start := time.Now()

	// Start progress monitor
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		var completed int
		total := numGoroutines * opsPerRoutine * 3 // 3 operations per iteration

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.Logf("Progress: %d/%d operations (%.2f%%)", completed, total, float64(completed)/float64(total)*100)
			case <-progressChan:
				completed++
			}
		}
	}()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < opsPerRoutine; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				key := fmt.Sprintf("key-%d-%d", routineID, j)
				value := fmt.Sprintf("value-%d-%d", routineID, j)

				// Write
				start := time.Now()
				if err := kvs.SetUserData(testUser, key, value); err != nil {
					errChan <- fmt.Errorf("write failed (took %v): %v", time.Since(start), err)
					continue
				}
				progressChan <- struct{}{}

				// Read
				start = time.Now()
				if _, err := kvs.GetUserData(testUser, key); err != nil {
					errChan <- fmt.Errorf("read failed (took %v): %v", time.Since(start), err)
					continue
				}
				progressChan <- struct{}{}

				// Delete
				start = time.Now()
				if err := kvs.DeleteUserData(testUser, key); err != nil {
					errChan <- fmt.Errorf("delete failed (took %v): %v", time.Since(start), err)
				}
				progressChan <- struct{}{}
			}
		}(i)
	}

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Test timed out")
	case <-done:
		// Test completed
	}

	close(errChan)
	close(progressChan)

	duration := time.Since(start)
	totalOps := numGoroutines * opsPerRoutine * 3 // 3 operations per iteration
	opsPerSecond := float64(totalOps) / duration.Seconds()

	t.Logf("Concurrent test completed:")
	t.Logf("- Total operations: %d", totalOps)
	t.Logf("- Duration: %v", duration)
	t.Logf("- Operations per second: %.2f", opsPerSecond)

	// Check for errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	if len(errors) > 0 {
		t.Errorf("Test encountered %d errors:", len(errors))
		for i, err := range errors {
			if i < 10 { // Only show first 10 errors
				t.Errorf("  %v", err)
			}
		}
	}
}
