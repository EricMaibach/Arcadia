package text

import (
	"path/filepath"
	"strings"
)

// LanguageDetector handles language detection for text content
type LanguageDetector struct{}

// NewLanguageDetector creates a new language detector
func NewLanguageDetector() *LanguageDetector {
	return &LanguageDetector{}
}

// DetectFromExtension detects programming language based on file extension
func (ld *LanguageDetector) DetectFromExtension(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	return getLanguageByExtension(ext)
}

// DetectFromContent performs basic language detection from content
func (ld *LanguageDetector) DetectFromContent(content string) (string, float64, error) {
	// Very basic language detection
	// In a real implementation, you'd use a proper language detection library

	// Count English indicators
	englishIndicators := []string{"the", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with"}
	englishCount := 0
	words := strings.Fields(strings.ToLower(content))

	for _, word := range words {
		for _, indicator := range englishIndicators {
			if word == indicator {
				englishCount++
				break
			}
		}
	}

	if len(words) == 0 {
		return "unknown", 0.0, nil
	}

	confidence := float64(englishCount) / float64(len(words))
	if confidence > 0.1 {
		return "en", confidence, nil
	}

	return "unknown", 0.0, nil
}

// DetectLanguage combines extension and content-based detection
func (ld *LanguageDetector) DetectLanguage(filePath, content string) (string, float64) {
	// First try extension-based detection (for programming languages)
	if progLang := ld.DetectFromExtension(filePath); progLang != "" {
		return progLang, 0.8 // Moderate confidence based on extension
	}

	// Fall back to content-based detection (for natural languages)
	if natLang, confidence, err := ld.DetectFromContent(content); err == nil && natLang != "unknown" {
		return natLang, confidence
	}

	return "unknown", 0.0
}

// IsCodeFile determines if a file contains code based on its extension
func (ld *LanguageDetector) IsCodeFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	codeExtensions := map[string]bool{
		".py":   true,
		".go":   true,
		".js":   true,
		".ts":   true,
		".jsx":  true,
		".tsx":  true,
		".java": true,
		".c":    true,
		".cpp":  true,
		".h":    true,
		".hpp":  true,
		".cs":   true,
		".php":  true,
		".rb":   true,
		".pl":   true,
		".r":    true,
		".swift": true,
		".kt":   true,
		".scala": true,
		".clj":  true,
		".hs":   true,
		".ml":   true,
		".fs":   true,
		".dart": true,
		".lua":  true,
		".perl": true,
		".tcl":  true,
		".m":    true,
		".mm":   true,
		".vb":   true,
		".pas":  true,
		".f":    true,
		".f90":  true,
		".ada":  true,
		".cob":  true,
		".cobol": true,
	}

	return codeExtensions[ext]
}

// IsScriptFile determines if a file is a script based on its extension
func (ld *LanguageDetector) IsScriptFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	scriptExtensions := map[string]bool{
		".sh":   true,
		".bash": true,
		".zsh":  true,
		".fish": true,
		".csh":  true,
		".tcsh": true,
		".ksh":  true,
		".ps1":  true,
		".psm1": true,
		".psd1": true,
		".bat":  true,
		".cmd":  true,
	}

	return scriptExtensions[ext]
}

// IsConfigFile determines if a file is a configuration file based on its extension
func (ld *LanguageDetector) IsConfigFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	configExtensions := map[string]bool{
		".ini":        true,
		".conf":       true,
		".config":     true,
		".properties": true,
		".toml":       true,
		".yaml":       true,
		".yml":        true,
		".json":       true,
		".xml":        true,
	}

	return configExtensions[ext]
}

// GetLanguageCategory returns the category of the detected language
func (ld *LanguageDetector) GetLanguageCategory(language string) string {
	programmingLanguages := map[string]bool{
		"python": true, "go": true, "javascript": true, "typescript": true,
		"java": true, "c": true, "cpp": true, "csharp": true,
		"php": true, "ruby": true, "perl": true, "r": true,
		"swift": true, "kotlin": true, "scala": true, "clojure": true,
		"haskell": true, "ocaml": true, "fsharp": true, "dart": true,
		"lua": true, "tcl": true, "objectivec": true, "visualbasic": true,
		"pascal": true, "fortran": true, "ada": true, "cobol": true,
	}

	scriptLanguages := map[string]bool{
		"bash": true, "zsh": true, "fish": true, "csh": true,
		"powershell": true, "batch": true,
	}

	markupLanguages := map[string]bool{
		"html": true, "xml": true, "markdown": true,
		"tex": true, "latex": true, "rst": true,
		"asciidoc": true, "org": true, "wiki": true,
	}

	styleLanguages := map[string]bool{
		"css": true, "scss": true, "sass": true, "less": true,
	}

	dataLanguages := map[string]bool{
		"json": true, "yaml": true, "toml": true, "csv": true,
		"sql": true, "xml": true,
	}

	if programmingLanguages[language] {
		return "programming"
	}
	if scriptLanguages[language] {
		return "script"
	}
	if markupLanguages[language] {
		return "markup"
	}
	if styleLanguages[language] {
		return "style"
	}
	if dataLanguages[language] {
		return "data"
	}

	// Natural languages
	if len(language) == 2 { // ISO 639-1 codes
		return "natural"
	}

	return "unknown"
}

// Helper function extracted from analyzer.go
func getLanguageByExtension(ext string) string {
	languages := map[string]string{
		".py":   "python",
		".go":   "go",
		".js":   "javascript",
		".ts":   "typescript",
		".jsx":  "javascript",
		".tsx":  "typescript",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".cxx":  "cpp",
		".cc":   "cpp",
		".h":    "c",
		".hpp":  "cpp",
		".hxx":  "cpp",
		".cs":   "csharp",
		".sql":  "sql",
		".sh":   "bash",
		".bash": "bash",
		".zsh":  "zsh",
		".fish": "fish",
		".csh":  "csh",
		".tcsh": "csh",
		".ksh":  "bash",
		".ps1":  "powershell",
		".psm1": "powershell",
		".psd1": "powershell",
		".bat":  "batch",
		".cmd":  "batch",
		".rb":   "ruby",
		".php":  "php",
		".pl":   "perl",
		".pm":   "perl",
		".r":    "r",
		".tex":  "latex",
		".ltx":  "latex",
		".swift": "swift",
		".kt":   "kotlin",
		".scala": "scala",
		".clj":  "clojure",
		".hs":   "haskell",
		".ml":   "ocaml",
		".fs":   "fsharp",
		".dart": "dart",
		".lua":  "lua",
		".tcl":  "tcl",
		".m":    "objectivec",
		".mm":   "objectivec",
		".vb":   "visualbasic",
		".pas":  "pascal",
		".f":    "fortran",
		".f90":  "fortran",
		".ada":  "ada",
		".cob":  "cobol",
		".cobol": "cobol",
		".html": "html",
		".htm":  "html",
		".css":  "css",
		".scss": "scss",
		".sass": "sass",
		".less": "less",
		".md":   "markdown",
		".xml":  "xml",
		".json": "json",
		".yaml": "yaml",
		".yml":  "yaml",
		".toml": "toml",
		".csv":  "csv",
		".rst":  "rst",
		".asciidoc": "asciidoc",
		".adoc": "asciidoc",
		".org":  "org",
		".wiki": "wiki",
	}

	return languages[ext]
}