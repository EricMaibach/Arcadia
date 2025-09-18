package base

// ProcessorType defines the type of document processor
type ProcessorType string

const (
	// ProcessorTypeText handles plain text files
	ProcessorTypeText ProcessorType = "text"

	// ProcessorTypePDF handles PDF documents
	ProcessorTypePDF ProcessorType = "pdf"

	// ProcessorTypeMarkdown handles Markdown files
	ProcessorTypeMarkdown ProcessorType = "markdown"
)

// DocumentType represents metadata about a document type
type DocumentType struct {
	// Type is the processor type that should handle this document
	Type ProcessorType

	// MimeType is the MIME type of the document
	MimeType string

	// Extension is the file extension (without the dot)
	Extension string

	// Confidence is how confident we are about this document type (0.0 to 1.0)
	Confidence float64

	// Metadata contains additional information about the document type
	Metadata map[string]interface{}
}

// String returns the string representation of the ProcessorType
func (pt ProcessorType) String() string {
	return string(pt)
}

// IsValid checks if the ProcessorType is valid
func (pt ProcessorType) IsValid() bool {
	switch pt {
	case ProcessorTypeText, ProcessorTypePDF, ProcessorTypeMarkdown:
		return true
	default:
		return false
	}
}