package vectordb

import (
	"strings"
	"unicode/utf8"
)

const (
	DefaultChunkSize    = 512  // 目标分块大小（字符数，中文约等于 token 数）
	DefaultChunkOverlap = 128  // 重叠字符数
	MinChunkSize        = 50   // 最小分块大小
)

// Chunk 一个文本分块
type Chunk struct {
	Text       string
	Index      int
	TokenCount int
}

// ChunkText 将文本递归分割为固定大小的分块
// 优先按段落 > 句子 > 字符边界分割，保留上下文重叠
func ChunkText(text string, chunkSize, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap <= 0 {
		overlap = DefaultChunkOverlap
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	runeLen := utf8.RuneCountInString(text)
	if runeLen <= chunkSize {
		return []Chunk{{Text: text, Index: 0, TokenCount: estimateTokens(text)}}
	}

	separators := []string{"\n\n", "\n", "。", "！", "？", ". ", "! ", "? ", "；", "; ", "，", ", "}
	segments := recursiveSplit(text, separators, chunkSize)

	var chunks []Chunk
	var currentChunk strings.Builder
	currentLen := 0
	idx := 0

	for _, seg := range segments {
		segLen := utf8.RuneCountInString(seg)

		if currentLen+segLen > chunkSize && currentLen > MinChunkSize {
			chunkText := strings.TrimSpace(currentChunk.String())
			if chunkText != "" {
				chunks = append(chunks, Chunk{
					Text:       chunkText,
					Index:      idx,
					TokenCount: estimateTokens(chunkText),
				})
				idx++
			}

			// 保留重叠部分
			overlapText := getOverlapText(currentChunk.String(), overlap)
			currentChunk.Reset()
			currentChunk.WriteString(overlapText)
			currentLen = utf8.RuneCountInString(overlapText)
		}

		currentChunk.WriteString(seg)
		currentLen += segLen
	}

	// 最后一块
	if currentLen > 0 {
		chunkText := strings.TrimSpace(currentChunk.String())
		if chunkText != "" {
			chunks = append(chunks, Chunk{
				Text:       chunkText,
				Index:      idx,
				TokenCount: estimateTokens(chunkText),
			})
		}
	}

	return chunks
}

// recursiveSplit 递归按分隔符分割文本
func recursiveSplit(text string, separators []string, chunkSize int) []string {
	if utf8.RuneCountInString(text) <= chunkSize {
		return []string{text}
	}

	// 找到第一个有效分隔符
	for i, sep := range separators {
		parts := strings.SplitAfter(text, sep)
		if len(parts) <= 1 {
			continue
		}

		var result []string
		for _, part := range parts {
			if utf8.RuneCountInString(part) <= chunkSize {
				result = append(result, part)
			} else if i+1 < len(separators) {
				result = append(result, recursiveSplit(part, separators[i+1:], chunkSize)...)
			} else {
				result = append(result, splitByRunes(part, chunkSize)...)
			}
		}
		return result
	}

	return splitByRunes(text, chunkSize)
}

// splitByRunes 按字符数强制分割
func splitByRunes(text string, size int) []string {
	runes := []rune(text)
	var parts []string
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[i:end]))
	}
	return parts
}

// getOverlapText 从文本末尾取指定字符数作为重叠内容
func getOverlapText(text string, overlapSize int) string {
	runes := []rune(text)
	if len(runes) <= overlapSize {
		return text
	}
	return string(runes[len(runes)-overlapSize:])
}

// estimateTokens 估算 token 数（中文约 1 字 = 1-2 token，英文约 4 字符 = 1 token）
func estimateTokens(text string) int {
	runes := []rune(text)
	chinese := 0
	ascii := 0
	for _, r := range runes {
		if r > 127 {
			chinese++
		} else {
			ascii++
		}
	}
	return chinese + ascii/4
}
