package text

import (
	"path/filepath"
	"strings"
	"time"
)

// MetadataExtractor handles extraction of metadata from text files and content
type MetadataExtractor struct {
	languageDetector *LanguageDetector
	textExtractor    *TextExtractor
}

// NewMetadataExtractor creates a new metadata extractor
func NewMetadataExtractor() *MetadataExtractor {
	return &MetadataExtractor{
		languageDetector: NewLanguageDetector(),
		textExtractor:    NewTextExtractor(),
	}
}

// ExtractFromFile extracts comprehensive metadata from a file and its content
func (me *MetadataExtractor) ExtractFromFile(filePath, content string) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})

	// File-based metadata
	me.addFileMetadata(metadata, filePath)

	// Content-based metadata
	me.addContentMetadata(metadata, content)

	// Language detection metadata
	me.addLanguageMetadata(metadata, filePath, content)

	// Type classification metadata
	me.addTypeMetadata(metadata, filePath)

	return metadata, nil
}

// addFileMetadata adds file-system level metadata
func (me *MetadataExtractor) addFileMetadata(metadata map[string]interface{}, filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	metadata["file_extension"] = ext
	metadata["file_name"] = filepath.Base(filePath)
	metadata["file_dir"] = filepath.Dir(filePath)
	metadata["file_stem"] = strings.TrimSuffix(filepath.Base(filePath), ext)

	// Content type
	metadata["content_type"] = me.textExtractor.GetContentType(filePath)
	metadata["is_text_file"] = me.textExtractor.IsTextFile(filePath)

	// Processing timestamp
	metadata["processed_at"] = time.Now().UTC().Format(time.RFC3339)
}

// addContentMetadata adds content-based statistical metadata
func (me *MetadataExtractor) addContentMetadata(metadata map[string]interface{}, content string) {
	metadata["content_length"] = len(content)
	metadata["content_length_bytes"] = len([]byte(content))

	// Line and word counts
	lineCount := strings.Count(content, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		lineCount++ // Add 1 if content doesn't end with newline
	}
	metadata["line_count"] = lineCount
	metadata["word_count"] = me.textExtractor.EstimateWordCount(content)

	// Character statistics
	metadata["char_count"] = len([]rune(content))
	metadata["whitespace_count"] = me.countWhitespace(content)

	// Content characteristics
	metadata["has_content"] = len(strings.TrimSpace(content)) > 0
	metadata["is_empty"] = len(content) == 0
	metadata["is_binary"] = !me.isTextContent(content)
}

// addLanguageMetadata adds language detection metadata
func (me *MetadataExtractor) addLanguageMetadata(metadata map[string]interface{}, filePath, content string) {
	// Detect language
	language, confidence := me.languageDetector.DetectLanguage(filePath, content)
	if language != "unknown" {
		metadata["language"] = language
		metadata["language_confidence"] = confidence
		metadata["language_category"] = me.languageDetector.GetLanguageCategory(language)
	}

	// File type classifications
	metadata["is_code_file"] = me.languageDetector.IsCodeFile(filePath)
	metadata["is_script_file"] = me.languageDetector.IsScriptFile(filePath)
	metadata["is_config_file"] = me.languageDetector.IsConfigFile(filePath)
}

// addTypeMetadata adds file type classification metadata
func (me *MetadataExtractor) addTypeMetadata(metadata map[string]interface{}, filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Determine file category
	category := me.getFileCategory(ext)
	metadata["file_category"] = category

	// Add category-specific metadata
	switch category {
	case "documentation":
		metadata["is_documentation"] = true
		metadata["documentation_type"] = me.getDocumentationType(ext)
	case "configuration":
		metadata["is_configuration"] = true
		metadata["config_type"] = me.getConfigurationType(ext)
	case "source_code":
		metadata["is_source_code"] = true
		metadata["code_type"] = me.getCodeType(ext)
	case "data":
		metadata["is_data_file"] = true
		metadata["data_type"] = me.getDataType(ext)
	case "log":
		metadata["is_log_file"] = true
	}
}

// Helper methods

func (me *MetadataExtractor) countWhitespace(content string) int {
	count := 0
	for _, char := range content {
		if char == ' ' || char == '\t' || char == '\n' || char == '\r' {
			count++
		}
	}
	return count
}

func (me *MetadataExtractor) isTextContent(content string) bool {
	// Simple heuristic: if more than 95% of characters are printable, consider it text
	if len(content) == 0 {
		return true
	}

	printableCount := 0
	for _, char := range content {
		if char >= 32 && char <= 126 || char == '\t' || char == '\n' || char == '\r' {
			printableCount++
		}
	}

	ratio := float64(printableCount) / float64(len(content))
	return ratio >= 0.95
}

func (me *MetadataExtractor) getFileCategory(ext string) string {
	documentationExts := map[string]bool{
		".md": true, ".txt": true, ".rst": true, ".asciidoc": true,
		".adoc": true, ".org": true, ".wiki": true, ".tex": true,
		".ltx": true, ".rtf": true, ".html": true, ".htm": true,
	}

	configExts := map[string]bool{
		".ini": true, ".conf": true, ".config": true, ".properties": true,
		".toml": true, ".yaml": true, ".yml": true, ".json": true,
		".xml": true, ".cfg": true, ".env": true,
	}

	sourceCodeExts := map[string]bool{
		".py": true, ".go": true, ".js": true, ".ts": true, ".jsx": true,
		".tsx": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
		".hpp": true, ".cs": true, ".php": true, ".rb": true, ".pl": true,
		".r": true, ".swift": true, ".kt": true, ".scala": true, ".clj": true,
		".hs": true, ".ml": true, ".fs": true, ".dart": true, ".lua": true,
	}

	dataExts := map[string]bool{
		".csv": true, ".tsv": true, ".sql": true, ".db": true,
		".sqlite": true, ".json": true, ".xml": true, ".yaml": true,
		".yml": true,
	}

	logExts := map[string]bool{
		".log": true, ".out": true, ".err": true, ".trace": true,
	}

	if documentationExts[ext] {
		return "documentation"
	}
	if configExts[ext] {
		return "configuration"
	}
	if sourceCodeExts[ext] {
		return "source_code"
	}
	if dataExts[ext] {
		return "data"
	}
	if logExts[ext] {
		return "log"
	}

	return "text"
}

func (me *MetadataExtractor) getDocumentationType(ext string) string {
	docTypes := map[string]string{
		".md": "markdown",
		".txt": "plain_text",
		".rst": "restructured_text",
		".asciidoc": "asciidoc",
		".adoc": "asciidoc",
		".org": "org_mode",
		".wiki": "wiki",
		".tex": "latex",
		".ltx": "latex",
		".html": "html",
		".htm": "html",
		".rtf": "rich_text",
	}

	if docType, exists := docTypes[ext]; exists {
		return docType
	}
	return "unknown"
}

func (me *MetadataExtractor) getConfigurationType(ext string) string {
	configTypes := map[string]string{
		".ini": "ini",
		".conf": "generic_config",
		".config": "generic_config",
		".properties": "properties",
		".toml": "toml",
		".yaml": "yaml",
		".yml": "yaml",
		".json": "json",
		".xml": "xml",
		".cfg": "generic_config",
		".env": "environment",
	}

	if configType, exists := configTypes[ext]; exists {
		return configType
	}
	return "unknown"
}

func (me *MetadataExtractor) getCodeType(ext string) string {
	return getLanguageByExtension(ext)
}

func (me *MetadataExtractor) getDataType(ext string) string {
	dataTypes := map[string]string{
		".csv": "csv",
		".tsv": "tsv",
		".sql": "sql",
		".db": "database",
		".sqlite": "sqlite",
		".json": "json",
		".xml": "xml",
		".yaml": "yaml",
		".yml": "yaml",
	}

	if dataType, exists := dataTypes[ext]; exists {
		return dataType
	}
	return "unknown"
}

// ExtractBasicMetadata extracts minimal metadata for quick processing
func (me *MetadataExtractor) ExtractBasicMetadata(filePath string, contentLength int) map[string]interface{} {
	metadata := make(map[string]interface{})

	ext := strings.ToLower(filepath.Ext(filePath))
	metadata["file_extension"] = ext
	metadata["file_name"] = filepath.Base(filePath)
	metadata["content_length"] = contentLength
	metadata["content_type"] = me.textExtractor.GetContentType(filePath)
	metadata["is_text_file"] = me.textExtractor.IsTextFile(filePath)
	metadata["processed_at"] = time.Now().UTC().Format(time.RFC3339)

	// Language from extension only (fast)
	if lang := me.languageDetector.DetectFromExtension(filePath); lang != "" {
		metadata["language"] = lang
		metadata["language_confidence"] = 0.8
	}

	return metadata
}