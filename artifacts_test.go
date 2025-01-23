package brunch

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArtifactsFrom(t *testing.T) {
	tests := []struct {
		name    string
		message *MessageData
		want    []Artifact
		wantErr bool
	}{
		{
			name:    "nil message",
			message: nil,
			want:    []Artifact{},
			wantErr: false,
		},
		{
			name: "empty message",
			message: &MessageData{
				Role:              "user",
				B64EncodedContent: base64.StdEncoding.EncodeToString([]byte("")),
			},
			want:    []Artifact{},
			wantErr: false,
		},
		{
			name: "single file artifact",
			message: &MessageData{
				Role:              "assistant",
				B64EncodedContent: base64.StdEncoding.EncodeToString([]byte("```go:test.go\npackage main\n\nfunc main() {}\n```")),
			},
			want: []Artifact{
				&FileArtifact{
					Id:       "12",
					Data:     "package main\n\nfunc main() {}\n",
					Name:     "test.go",
					FileType: stringPtr("go"),
				},
			},
			wantErr: false,
		},
		{
			name: "non-file artifact",
			message: &MessageData{
				Role:              "assistant",
				B64EncodedContent: base64.StdEncoding.EncodeToString([]byte("```\nsome non-file content\n```")),
			},
			want: []Artifact{
				&NonFileArtifact{
					Data: "some non-file content\n",
				},
			},
			wantErr: false,
		},
		{
			name: "multiple artifacts",
			message: &MessageData{
				Role:              "assistant",
				B64EncodedContent: base64.StdEncoding.EncodeToString([]byte("```go:test.go\npackage main\n```\n```\nnon-file content\n```")),
			},
			want: []Artifact{
				&FileArtifact{
					Id:       "12",
					Data:     "package main\n",
					Name:     "test.go",
					FileType: stringPtr("go"),
				},
				&NonFileArtifact{
					Data: "non-file content\n",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid base64",
			message: &MessageData{
				Role:              "assistant",
				B64EncodedContent: "invalid base64",
			},
			want:    []Artifact{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArtifactsFrom(tt.message)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, len(tt.want), len(got))

			for i := range got {
				switch v := got[i].(type) {
				case *FileArtifact:
					want := tt.want[i].(*FileArtifact)
					assert.Equal(t, want.Name, v.Name)
					assert.Equal(t, want.Data, v.Data)
					if want.FileType != nil {
						assert.Equal(t, *want.FileType, *v.FileType)
					} else {
						assert.Nil(t, v.FileType)
					}
				case *NonFileArtifact:
					want := tt.want[i].(*NonFileArtifact)
					assert.Equal(t, want.Data, v.Data)
				}
			}
		})
	}
}

func TestArtifactType(t *testing.T) {
	fileArtifact := &FileArtifact{}
	nonFileArtifact := &NonFileArtifact{}

	assert.Equal(t, ArtifactTypeFile, fileArtifact.Type())
	assert.Equal(t, ArtifactTypeNonFile, nonFileArtifact.Type())
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

func TestParseComplexMarkdown(t *testing.T) {
	// First test with standard language identifiers
	markdownContent := `I'll create examples of the Fibonacci sequence implemented in 5 different programming languages, each with a slightly different approach.

# Fibonacci Sequence Implementations

## Python - Using Recursion
The Python implementation uses classic recursion, though it's not the most efficient approach:

` + "```" + `python
def fibonacci_recursive(n):
    if n <= 1:
        return n
    return fibonacci_recursive(n-1) + fibonacci_recursive(n-2)

# Example usage
for i in range(10):
    print(f"F({i}) = {fibonacci_recursive(i)}")
` + "```" + `

## JavaScript - Using Generator Function
This JavaScript version uses a generator function to create an infinite Fibonacci sequence:

` + "```" + `javascript
function* fibonacciGenerator() {
    let prev = 0, curr = 1;
    yield prev;
    yield curr;
    
    while (true) {
        const next = prev + curr;
        yield next;
        prev = curr;
        curr = next;
    }
}

// Example usage
const fib = fibonacciGenerator();
for (let i = 0; i < 10; i++) {
    console.log(` + "`" + `F(${i}) = ${fib.next().value}` + "`" + `);
}
` + "```" + `

## Rust - Using Dynamic Programming
The Rust implementation uses an iterative approach with a vector to store previous values:

` + "```" + `rust
fn fibonacci_dynamic(n: usize) -> u64 {
    if n <= 1 {
        return n as u64;
    }
    
    let mut fib = vec![0; n + 1];
    fib[1] = 1;
    
    for i in 2..=n {
        fib[i] = fib[i-1] + fib[i-2];
    }
    
    fib[n]
}

fn main() {
    for i in 0..10 {
        println!("F({}) = {}", i, fibonacci_dynamic(i));
    }
}
` + "```" + `

## Go - Using Channels
This Go implementation uses channels to generate Fibonacci numbers concurrently:

` + "```" + `go
func fibonacciChannel(n int, ch chan int) {
    x, y := 0, 1
    for i := 0; i < n; i++ {
        ch <- x
        x, y = y, x+y
    }
    close(ch)
}

func main() {
    ch := make(chan int, 10)
    go fibonacciChannel(10, ch)
    
    i := 0
    for num := range ch {
        fmt.Printf("F(%d) = %d\n", i, num)
        i++
    }
}
` + "```" + `

## Ruby - Using Memoization
The Ruby version uses memoization through a hash to cache previously calculated values:

` + "```" + `ruby
def fibonacci_memoized(n, memo = {})
  return n if n <= 1
  memo[n] ||= fibonacci_memoized(n-1, memo) + fibonacci_memoized(n-2, memo)
end

# Example usage
10.times do |i|
  puts "F(#{i}) = #{fibonacci_memoized(i)}"
end
` + "```" + `

## Additional Examples with Language:Filename Format

` + "```" + `python:fib_recursive.py
def fibonacci(n):
    return n if n <= 1 else fibonacci(n-1) + fibonacci(n-2)
` + "```" + `

` + "```" + `javascript:fib.js
const fib = n => n <= 1 ? n : fib(n-1) + fib(n-2);
` + "```" + `

` + "```" + `go:fib.go
func fib(n int) int {
    if n <= 1 {
        return n
    }
    return fib(n-1) + fib(n-2)
}
` + "```" + `

` + "```" + `rust:fibonacci.rs
fn fib(n: u64) -> u64 {
    if n <= 1 { n } else { fib(n-1) + fib(n-2) }
}
` + "```" + `

` + "```" + `ruby:fibonacci.rb
def fib(n)
  n <= 1 ? n : fib(n-1) + fib(n-2)
end
` + "```" + ``

	msg := &MessageData{
		Role:              "assistant",
		B64EncodedContent: base64.StdEncoding.EncodeToString([]byte(markdownContent)),
	}

	artifacts, err := ParseArtifactsFrom(msg)
	assert.NoError(t, err)

	// We expect 10 code blocks and several non-file blocks in between
	totalArtifacts := len(artifacts)
	assert.Greater(t, totalArtifacts, 10, "Should find more than 10 blocks total (code blocks + descriptive blocks)")

	// Count the number of each type
	var fileArtifacts []*FileArtifact
	var nonFileArtifacts []*NonFileArtifact

	for _, artifact := range artifacts {
		switch a := artifact.(type) {
		case *FileArtifact:
			fileArtifacts = append(fileArtifacts, a)
		case *NonFileArtifact:
			nonFileArtifacts = append(nonFileArtifacts, a)
		}
	}

	assert.Equal(t, 10, len(fileArtifacts), "Should find exactly 10 code blocks")
	assert.Greater(t, len(nonFileArtifacts), 0, "Should find some non-file blocks")

	// First 5 artifacts should be language-only code blocks
	expectedTypes := []string{"python", "javascript", "rust", "go", "ruby"}
	codeBlockCount := 0
	for _, artifact := range artifacts {
		if fileArtifact, ok := artifact.(*FileArtifact); ok {
			if codeBlockCount < 5 {
				assert.Empty(t, fileArtifact.Name, "Name should be empty when no explicit name is given")
				assert.NotNil(t, fileArtifact.FileType, "FileType should not be nil")
				assert.Equal(t, expectedTypes[codeBlockCount], *fileArtifact.FileType, "FileType should match for artifact %d", codeBlockCount)
				assert.NotEmpty(t, fileArtifact.Data, "Data should not be empty for artifact %d", codeBlockCount)
				codeBlockCount++
			}
		}
	}

	// Next 5 artifacts should be code blocks with both language and filename
	expectedNames := []string{"fib_recursive.py", "fib.js", "fib.go", "fibonacci.rs", "fibonacci.rb"}
	expectedFileTypes := []string{"python", "javascript", "go", "rust", "ruby"}
	namedBlockCount := 0
	for _, artifact := range artifacts {
		if fileArtifact, ok := artifact.(*FileArtifact); ok {
			if len(fileArtifact.Name) > 0 {
				assert.Equal(t, expectedNames[namedBlockCount], fileArtifact.Name, "Name should match for named artifact %d", namedBlockCount)
				assert.NotNil(t, fileArtifact.FileType, "FileType should not be nil")
				assert.Equal(t, expectedFileTypes[namedBlockCount], *fileArtifact.FileType, "FileType should match for named artifact %d", namedBlockCount)
				assert.NotEmpty(t, fileArtifact.Data, "Data should not be empty for named artifact %d", namedBlockCount)
				namedBlockCount++
			}
		}
	}

	// Verify that non-file blocks contain descriptive text
	for _, artifact := range nonFileArtifacts {
		assert.NotEmpty(t, artifact.Data, "Non-file artifact should contain data")
		// The data should not contain code block markers
		assert.NotContains(t, artifact.Data, "```", "Non-file artifact should not contain code block markers")
	}
}
