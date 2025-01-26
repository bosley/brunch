package brunch

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Artifact interface {
	Type() ArtifactType
	Write(dir string, name string) error
}

type ArtifactType int

const (
	ArtifactTypeFile ArtifactType = iota
	ArtifactTypeNonFile
)

type FileArtifact struct {
	Id       string
	Data     string
	Name     string
	FileType *string
}

type NonFileArtifact struct {
	Data string
}

func (a *FileArtifact) Type() ArtifactType {
	return ArtifactTypeFile
}

func (a *NonFileArtifact) Type() ArtifactType {
	return ArtifactTypeNonFile
}

func ParseArtifactsFrom(msg *MessageData) ([]Artifact, error) {
	if msg == nil {
		return []Artifact{}, nil
	}
	decodedContent, err := base64.StdEncoding.DecodeString(msg.B64EncodedContent)
	if err != nil {
		return []Artifact{}, err
	}
	p := &parser{
		role:    msg.Role,
		content: fmt.Sprintf("%s", decodedContent),
		idx:     0,
	}
	return p.parse()
}

type parser struct {
	role    string
	content string
	idx     int
}

func (p *parser) isNext(pos int, s byte) bool {
	if p.idx+pos >= len(p.content) {
		return false
	}
	if p.content[p.idx+pos] == s {
		return true
	}
	return false
}

func (p *parser) movIdxToEOL() bool {
	for p.idx < len(p.content) && p.content[p.idx] != '\n' {
		p.idx++
	}
	if p.idx < len(p.content) {
		p.idx++
	}
	return p.idx < len(p.content)
}

func (p *parser) parseUntilBlockIndicator() bool {
	for p.idx < len(p.content) {
		if p.content[p.idx] == '`' && p.isNext(1, '`') && p.isNext(2, '`') {
			break
		}
		p.idx++
	}
	return p.idx < len(p.content)
}

func (p *parser) parse() ([]Artifact, error) {
	result := []Artifact{}
	textStart := p.idx
	for p.idx < len(p.content) {
		if p.content[p.idx] == '`' && p.isNext(1, '`') && p.isNext(2, '`') {
			// If we have text before this code block, add it as a non-file artifact
			if textStart < p.idx {
				text := strings.TrimSpace(p.content[textStart:p.idx])
				if len(text) > 0 {
					result = append(result, &NonFileArtifact{
						Data: text,
					})
				}
			}
			p.idx += 3
			a, err := p.parseMarkdownBlock()
			if err != nil {
				return []Artifact{}, err
			}
			result = append(result, a)
			textStart = p.idx
		} else {
			p.idx++
		}
	}

	// Add any remaining text as a non-file artifact
	if textStart < p.idx {
		text := strings.TrimSpace(p.content[textStart:p.idx])
		if len(text) > 0 {
			result = append(result, &NonFileArtifact{
				Data: text,
			})
		}
	}

	return result, nil
}

func (p *parser) parseMarkdownBlock() (Artifact, error) {
	start := p.idx
	if !p.movIdxToEOL() {
		return nil, fmt.Errorf("no EOL found")
	}
	file_info := strings.TrimSpace(p.content[start : p.idx-1])
	if len(file_info) == 0 {
		return p.parseMarkdownNonFileBlock()
	}
	name := ""
	fileType := ""
	parts := strings.Split(file_info, ":")
	if len(parts) != 2 {
		fileType = file_info
	} else {
		fileType = parts[0]
		name = parts[1]
	}
	return p.parseMarkdownFileBlock(name, fileType)
}

func (p *parser) parseMarkdownFileBlock(name, fileType string) (Artifact, error) {
	start := p.idx
	if !p.parseUntilBlockIndicator() {
		return nil, fmt.Errorf("no block indicator found")
	}
	end := p.idx
	p.idx += 3
	return &FileArtifact{
		Id:       fmt.Sprintf("%d", start),
		Data:     p.content[start:end],
		Name:     name,
		FileType: &fileType,
	}, nil
}

func (p *parser) parseMarkdownNonFileBlock() (Artifact, error) {
	start := p.idx
	if !p.parseUntilBlockIndicator() {
		return nil, fmt.Errorf("no block indicator found")
	}
	end := p.idx
	p.idx += 3
	return &NonFileArtifact{
		Data: p.content[start:end],
	}, nil
}

func (a *FileArtifact) Write(dir string, name string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fileName := a.Name
	if fileName == "" {
		if name == "" {
			fileName = fmt.Sprintf("file_%s", a.Id)
		} else {
			fileName = name
		}
	}

	if a.FileType != nil && *a.FileType != "" {
		fileType := strings.TrimPrefix(*a.FileType, ".")
		if !strings.HasSuffix(fileName, "."+fileType) {
			fileName = fileName + "." + fileType
		}
	}

	fullPath := filepath.Join(dir, fileName)
	return os.WriteFile(fullPath, []byte(a.Data), 0644)
}

func (a *NonFileArtifact) Write(dir string, name string) error {
	if name == "" {
		return fmt.Errorf("name is required for writing artifacts")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if !strings.Contains(name, ".") {
		name = name + ".txt"
	}

	fullPath := filepath.Join(dir, name)
	return os.WriteFile(fullPath, []byte(a.Data), 0644)
}
