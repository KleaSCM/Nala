/**
 * Document ingestion and chunking pipeline.
 * ドキュメント取り込みとチャンク分割パイプラインね。
 *
 * Supports PDF, TXT, MD, CSV, JSON, HTML, DOCX extraction.
 * PDF、TXT、MD、CSV、JSON、HTML、DOCXの抽出をサポートしてるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Chunk struct {
	Index      int    `json:"index"`
	Content    string `json:"content"`
	TokenCount int    `json:"token_count"`
}

type DocumentProcessor struct{}

func NewDocumentProcessor() *DocumentProcessor {
	return &DocumentProcessor{}
}

func (dp *DocumentProcessor) Process(ctx context.Context, filePath string) (*ProcessedDocument, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	mime := detectMimeType(ext, data)
	contentHash := fmt.Sprintf("%x", sha256.Sum256(data))

	var content string
	switch mime {
	case "application/pdf":
		content = extractPDF(data)
	case "text/html", "text/xml":
		content = stripHTML(string(data))
	case "text/csv":
		content = extractCSV(data)
	case "application/json":
		content = extractJSON(data)
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		content = extractDOCX(data)
	default:
		content = string(data)
	}

	if isBinary(data) && content == "" {
		return &ProcessedDocument{
			Filename:    filepath.Base(filePath),
			Content:     "",
			ContentHash: contentHash,
			MimeType:    mime,
			IsBinary:    true,
		}, nil
	}

	return &ProcessedDocument{
		Filename:    filepath.Base(filePath),
		Content:     content,
		ContentHash: contentHash,
		MimeType:    mime,
		Size:        len(data),
	}, nil
}

type ProcessedDocument struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
	MimeType    string `json:"mime_type"`
	Size        int    `json:"size"`
	IsBinary    bool   `json:"is_binary"`
}

func detectMimeType(ext string, data []byte) string {
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".html", ".htm":
		return "text/html"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	default:
		if len(data) > 0 {
			contentType := httpDetectContentType(data[:min(len(data), 512)])
			return contentType
		}
		return "application/octet-stream"
	}
}

func httpDetectContentType(data []byte) string {
	if len(data) > 0 && data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	if len(data) > 0 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if len(data) > 4 && string(data[:4]) == "%PDF" {
		return "application/pdf"
	}
	if len(data) > 0 && data[0] == '<' {
		return "text/html"
	}
	return "application/octet-stream"
}

func isBinary(data []byte) bool {
	if len(data) > 512 {
		data = data[:512]
	}
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// ── Chunking ──────────────────────────────────────────────────────────

type Chunker struct {
	Strategy  string
	Size      int
	Overlap   int
}

func NewChunker(strategy string, size, overlap int) *Chunker {
	if size <= 0 {
		size = 1000
	}
	if overlap < 0 || overlap >= size {
		overlap = 200
	}
	return &Chunker{
		Strategy: strategy,
		Size:     size,
		Overlap:  overlap,
	}
}

func (c *Chunker) Chunk(ctx context.Context, content string) ([]Chunk, error) {
	if c.ContentTooLarge(content) {
		return nil, fmt.Errorf("content too large (max 10MB)")
	}
	switch c.Strategy {
	case "fixed":
		return c.fixedSize(content)
	case "semantic":
		return c.semantic(content)
	default:
		return c.recursive(content)
	}
}

func (c *Chunker) ContentTooLarge(content string) bool {
	return len(content) > 10*1024*1024
}

func (c *Chunker) fixedSize(content string) ([]Chunk, error) {
	if content == "" {
		return nil, nil
	}
	runes := []rune(content)
	var chunks []Chunk
	start := 0
	index := 0
	for start < len(runes) {
		end := start + c.Size
		if end > len(runes) {
			end = len(runes)
		}
		text := string(runes[start:end])
		chunks = append(chunks, Chunk{
			Index:      index,
			Content:    text,
			TokenCount: approxTokenCount(text),
		})
		index++
		start += c.Size - c.Overlap
		if start >= len(runes) {
			break
		}
	}
	return chunks, nil
}

func (c *Chunker) recursive(content string) ([]Chunk, error) {
	if content == "" {
		return nil, nil
	}

	// Split by paragraph first, then sentence, then word
	paragraphs := splitParagraphs(content)
	var chunks []Chunk
	var current strings.Builder
	index := 0

	for _, p := range paragraphs {
		if current.Len()+len(p) <= c.Size {
			current.WriteString(p)
			current.WriteString("\n\n")
		} else {
			if current.Len() > 0 {
				chunks = append(chunks, Chunk{
					Index:      index,
					Content:    strings.TrimSpace(current.String()),
					TokenCount: approxTokenCount(current.String()),
				})
				index++
				current.Reset()
				// Add overlap from end of previous
				if len(chunks) > 0 && c.Overlap > 0 {
					prev := chunks[len(chunks)-1].Content
					runes := []rune(prev)
					if len(runes) > c.Overlap {
						overlap := string(runes[len(runes)-c.Overlap:])
						current.WriteString(overlap)
						current.WriteString("\n\n")
					}
				}
			}
			current.WriteString(p)
			current.WriteString("\n\n")
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, Chunk{
			Index:      index,
			Content:    strings.TrimSpace(current.String()),
			TokenCount: approxTokenCount(current.String()),
		})
	}

	return chunks, nil
}

func (c *Chunker) semantic(content string) ([]Chunk, error) {
	// Semantic chunking groups by topic boundaries (headers, section breaks)
	if content == "" {
		return nil, nil
	}

	// Split on markdown headers or numbered sections
	sectionRE := regexp.MustCompile(`(?m)^(#{1,6}\s+|[\d]+\.[\s]+|[\*\-]\s+.*?[\n]{2,})`)
	sections := sectionRE.Split(content, -1)

	var chunks []Chunk
	var current strings.Builder
	index := 0

	for _, s := range sections {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if current.Len()+len(s) <= c.Size {
			if current.Len() > 0 {
				current.WriteString("\n\n")
			}
			current.WriteString(s)
		} else {
			if current.Len() > 0 {
				chunks = append(chunks, Chunk{
					Index:      index,
					Content:    strings.TrimSpace(current.String()),
					TokenCount: approxTokenCount(current.String()),
				})
				index++
			}
			current.Reset()
			current.WriteString(s)
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, Chunk{
			Index:      index,
			Content:    strings.TrimSpace(current.String()),
			TokenCount: approxTokenCount(current.String()),
		})
	}

	return chunks, nil
}

func splitParagraphs(text string) []string {
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func approxTokenCount(text string) int {
	return int(float64(len([]rune(text))) * 0.25)
}

// ── Extractors ────────────────────────────────────────────────────────

func extractPDF(data []byte) string {
	// Basic text extraction from PDF by finding text between stream markers
	// For proper extraction, use ledongthuc/pdf in production
	text := string(data)
	text = regexp.MustCompile(`(?s)stream\s.*?\sendstream`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\(([^)]*)\)`).ReplaceAllString(text, "$1 ")
	text = regexp.MustCompile(`[^\x20-\x7E\n]`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func stripHTML(html string) string {
	text := regexp.MustCompile(`<style[^>]*>.*?</style>`).ReplaceAllString(html, "")
	text = regexp.MustCompile(`<script[^>]*>.*?</script>`).ReplaceAllString(html, "")
	text = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`&[a-zA-Z]+;`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractCSV(data []byte) string {
	reader := csv.NewReader(strings.NewReader(string(data)))
	var b strings.Builder
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		b.WriteString(strings.Join(record, " | "))
		b.WriteString("\n")
	}
	return b.String()
}

func extractJSON(data []byte) string {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	pretty, _ := json.MarshalIndent(v, "", "  ")
	return string(pretty)
}

func extractDOCX(data []byte) string {
	// DOCX is a ZIP containing XML. For MVP, basic extraction
	content := string(data)
	// Try to find text between <w:t> tags
	re := regexp.MustCompile(`<w:t[^>]*>([^<]+)</w:t>`)
	matches := re.FindAllStringSubmatch(content, -1)
	var b strings.Builder
	for _, m := range matches {
		b.WriteString(m[1])
		b.WriteString(" ")
	}
	if b.Len() > 0 {
		return strings.TrimSpace(b.String())
	}
	return "[Binary DOCX - text extraction requires unioffice library]"
}
