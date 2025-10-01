# Documents Module

A comprehensive document management system for the Arcadia application that provides document processing, embedding generation, vector-based search, and metadata management capabilities.

## Table of Contents

- [Apache Tika Integration](#apache-tika-integration)
- [Quick Start](#quick-start)
- [Overview](#overview)
- [Architecture](#architecture)
- [Flow Paths](#flow-paths)
- [Component Documentation](#component-documentation)
- [API Documentation](#api-documentation)
- [Configuration Guide](#configuration-guide)
- [Performance Considerations](#performance-considerations)
- [Error Handling](#error-handling)
- [Development and Testing](#development-and-testing)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)
- [Glossary](#glossary)

## Apache Tika Integration

The Documents Module features comprehensive Apache Tika integration that provides enterprise-grade document processing capabilities for Office documents and serves as a universal fallback processor for unknown file formats.

### Key Features

#### Office Document Support (25+ Formats)
- **Microsoft Office**: .doc, .docx, .xls, .xlsx, .ppt, .pptx
- **OpenDocument**: .odt, .ods, .odp
- **Other Formats**: .rtf, .wpd, and more

#### Dual Processing Modes
1. **Office Mode** (Default): High-confidence processing for known Office formats
2. **Fallback Mode**: Universal processing for any file format with Tika capabilities

#### Enterprise Features
- **Circuit Breaker Protection**: Automatic failure handling and recovery
- **Multi-stage Detection**: Priority-based processor selection with confidence scoring
- **Secure Processing**: Path validation and restricted directory protection
- **Performance Optimization**: Connection pooling and intelligent retry mechanisms

#### Security & Reliability
- **Path Sanitization**: Prevents directory traversal attacks
- **File Size Limits**: Configurable limits prevent resource exhaustion
- **Timeout Controls**: Prevents hanging operations
- **Health Monitoring**: Continuous Tika server health checks

### Quick Tika Setup

```go
// Enable Tika with default Office-only mode
deps := Dependencies{
    DB:                database,
    EmbeddingProvider: embeddingProvider,
    Logger:           logger,
    Config:           DefaultDocumentsConfig(), // Tika auto-enabled if server available
}

// Process Office documents
result, err := module.ProcessFile(ctx, "/path/to/document.docx")
if err != nil {
    log.Printf("Processing failed: %v", err)
} else {
    fmt.Printf("Extracted %d characters from %s\n",
        len(result.Content), result.Document.FilePath)
}
```

### Configuration Examples

#### Basic Office Document Support
```yaml
tika:
  server_url: "http://localhost:9998"
  office_confidence: 0.9
  enable_fallback: false
```

#### Universal Fallback Mode
```yaml
tika:
  server_url: "http://localhost:9998"
  office_confidence: 0.9
  fallback_confidence: 0.3
  enable_fallback: true
  accept_all_formats: true
```

### Supported Document Types

| Category | Extensions | Confidence | Description |
|----------|------------|------------|-------------|
| Microsoft Office | .doc, .docx, .xls, .xlsx, .ppt, .pptx | 0.9 | Native Office format support |
| OpenDocument | .odt, .ods, .odp | 0.9 | OpenOffice/LibreOffice formats |
| Rich Text | .rtf | 0.9 | Rich Text Format documents |
| Fallback | Any format | 0.3 | Universal processing (when enabled) |

### Architecture Integration

The Tika processor integrates seamlessly with the Documents Module's multi-stage detection system:

```
File Input → MultiStageDetector → ProcessorRegistry → TikaProcessor
    ↓             ↓                     ↓               ↓
Detection    Confidence          Processor          Apache Tika
Priority     Scoring            Selection          Server
```

### Performance Characteristics

- **Small Office Docs** (< 1MB): ~100-200 docs/minute
- **Large Office Docs** (1-10MB): ~20-50 docs/minute
- **Complex Spreadsheets**: ~10-30 docs/minute
- **Fallback Processing**: ~50-100 files/minute

### Deployment Requirements

#### Docker Deployment (Recommended)
```yaml
version: '3.8'
services:
  tika:
    image: apache/tika:latest
    ports:
      - "9998:9998"
    environment:
      - TIKA_CONFIG_FILE=/opt/tika-config.xml
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 30s
      timeout: 10s
      retries: 3
```

#### Configuration
```go
config := &tika.TikaConfig{
    ServerURL:          "http://localhost:9998",
    Timeout:            30 * time.Second,
    MaxRetries:         3,
    MaxFileSize:        100 * 1024 * 1024, // 100MB
    AcceptAllFormats:   false, // Office-only mode
    OfficeConfidence:   0.9,
    FallbackConfidence: 0.3,
}
```

### Error Handling

The Tika integration provides robust error handling with automatic fallback:

```go
result, err := module.ProcessFile(ctx, filePath)
if err != nil {
    if strings.Contains(err.Error(), "tika server unavailable") {
        // Tika server is down - document will be skipped or
        // processed by alternative processor if available
        log.Printf("Tika server unavailable: %v", err)
    }
}
```

### Additional Documentation

For detailed configuration, deployment, and troubleshooting information, see:

- **[TIKA_INTEGRATION.md](TIKA_INTEGRATION.md)**: Comprehensive technical documentation
- **[TIKA_DEPLOYMENT.md](TIKA_DEPLOYMENT.md)**: Production deployment guide
- **[CONFIG_REFERENCE.md](CONFIG_REFERENCE.md)**: Complete configuration reference
- **[MIGRATION_TO_TIKA.md](MIGRATION_TO_TIKA.md)**: Migration guide for existing deployments
- **[PERFORMANCE_TUNING.md](PERFORMANCE_TUNING.md)**: Performance optimization guide

## Quick Start

Get up and running with the Documents Module in 5 minutes:

```go
// 1. Import the module
import "arcadia/modules/documents"

// 2. Setup dependencies
deps := documents.Dependencies{
    DB:                database,           // Your database instance
    EmbeddingProvider: embeddingProvider,  // Ollama or OpenAI provider
    Logger:           logger,             // Your logger
    Config:           documents.DefaultDocumentsConfig(),
}

// 3. Create and start the module
module, err := documents.NewDocumentsModule(deps)
if err != nil {
    log.Fatal(err)
}

if err := module.Start(ctx); err != nil {
    log.Fatal(err)
}

// 4. Process your first document
result, err := module.ProcessFile(ctx, "/path/to/document.txt")
if err != nil {
    log.Printf("Processing failed: %v", err)
} else {
    fmt.Printf("Processed: %s\n", result.Document.FilePath)
}

// 5. Search documents
results, err := module.SearchDocuments(ctx, "your search query", 5)
if err != nil {
    log.Printf("Search failed: %v", err)
} else {
    for _, result := range results {
        fmt.Printf("Found: %s (Score: %.3f)\n",
            result.Document.FilePath, result.BestScore)
    }
}
```

## Overview

The Documents Module is a core component of Arcadia that handles:

- **Document Processing**: Automatic file reading, text extraction, chunking, and embedding generation
- **Office Document Support**: Full support for Microsoft Office and OpenDocument formats via Apache Tika
- **Vector Search**: Semantic similarity search using embeddings
- **Storage Management**: Dual storage system with document metadata and vector embeddings
- **Content Analysis**: Optional content analysis and metadata extraction
- **Batch Operations**: Efficient processing of multiple documents
- **Universal Format Support**: Fallback processing for unknown file formats
- **File Watching**: Optional file system monitoring for automatic processing

## Architecture

The Documents Module follows a modular, layered architecture designed for scalability and maintainability:

```
┌─────────────────────────────────────────────────────────────────┐
│                        Public API Layer                         │
│  ┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐ │
│  │  DocumentsModule │  │   Facade API     │  │  External API   │ │
│  └─────────────────┘  └──────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────┐
│                         Core Processing                         │
│  ┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐ │
│  │Document Processor│  │  Search Engine   │  │Embedding Engine │ │
│  └─────────────────┘  └──────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐ │
│  │   Text Chunker  │  │Content Analyzer  │  │  Worker Pool    │ │
│  └─────────────────┘  └──────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────┐
│                         Storage Layer                           │
│  ┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐ │
│  │ Document Store  │  │   Vector Store   │  │  Cache Service  │ │
│  │   (SQL/Memory)  │  │ (Qdrant/Memory)  │  │   (Optional)    │ │
│  └─────────────────┘  └──────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────┐
│                      External Dependencies                      │
│  ┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐ │
│  │    Database     │  │Embedding Provider│  │   Logging &     │ │
│  │   (SQL/NoSQL)   │  │ (Ollama/OpenAI)  │  │   Metrics       │ │
│  └─────────────────┘  └──────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Flow Paths

### 1. File Processing Flow

The document processing pipeline transforms raw files into searchable content using specific classes and methods:

```
Entry Point: documentsModule.ProcessFile() → module.go:492

┌──────────────────────────────────────────────────────────────────────────────┐
│                         DocumentProcessor                                    │
│                    (core/processing.go:23)                                   │
├──────────────────────────────────────────────────────────────────────────────┤
│  ProcessFile(ctx, filePath) → *models.Document                              │
│                                                                              │
│  1. File Validation (core/processing.go:120)                                │
│     ├─ validateFilePath(path) → error                                       │
│     ├─ checkFileExists(path) → error                                        │
│     └─ enforceSecurityRestrictions(path) → error                            │
│                                                                              │
│  2. Content Extraction (core/processing.go:174)                             │
│     ├─ extractTextFromFile(path) → (string, error)                          │
│     ├─ detectMimeType(path) → string                                        │
│     └─ validateUTF8Content(content) → error                                 │
│                           │                                                  │
│                           ▼                                                  │
│  3. Text Chunking (SimpleTextChunker - core/chunking.go:13)                 │
│     ├─ ChunkText(content, config) → []models.TextChunk                      │
│     │  Data: content string → []TextChunk{                                  │
│     │    ID, DocumentID, Content, StartPos, EndPos, ChunkIndex}             │
│     └─ generateChunkID() → string                                           │
│                           │                                                  │
│                           ▼                                                  │
│  4. Document Storage (stores/document_store.go)                             │
│     ├─ SQLDocumentStore.StoreDocument(ctx, doc) → error                     │
│     │  OR MemoryDocumentStore.StoreDocument(ctx, doc) → error               │
│     │  Data: *models.Document{ID, FilePath, FileHash, Content,              │
│     │        Metadata, ChunkCount, CreatedAt, UpdatedAt}                    │
│     └─ calculateFileHash(content) → string                                  │
│                           │                                                  │
│                           ▼                                                  │
│  5. Embedding Generation (EmbeddingEngine - core/embedding.go:14)           │
│     ├─ GenerateEmbedding(ctx, chunk.Content) → ([]float32, error)           │
│     │  Via: OllamaEmbeddingProvider.GenerateEmbedding()                     │
│     │       (providers/ollama.go:115)                                       │
│     │  Data: string → []float32 (384-dimensional vector)                    │
│     └─ validateEmbedding(embedding) → error                                 │
│                           │                                                  │
│                           ▼                                                  │
│  6. Vector Storage (stores/qdrant.go OR stores/vector_store.go)             │
│     ├─ QdrantVectorStore.Store(ctx, vectorEntry) → error                    │
│     │  OR MemoryVectorStore.Store(ctx, vectorEntry) → error                 │
│     │  Data: *models.VectorEntry{ID, DocumentID, ChunkID,                   │
│     │        Vector, Dimension, Content, Metadata, CreatedAt}               │
│     └─ buildQdrantPayload(entry) → QDRantPayload                            │
└──────────────────────────────────────────────────────────────────────────────┘

Key Data Transformations:
├─ File Path (string) → File Content (string)
├─ File Content (string) → []models.TextChunk
├─ TextChunk.Content (string) → Embedding Vector ([]float32)
├─ Document Metadata → models.Document
└─ Embedding + Metadata → models.VectorEntry
```

**Detailed Steps:**

1. **File Validation**
   - Path sanitization (prevent directory traversal)
   - File existence and permission checks
   - Size limits enforcement
   - Security restrictions on system directories

2. **Content Extraction**
   - MIME type detection using multiple methods
   - UTF-8 validation for text content
   - Binary file rejection with heuristics
   - Content reading with error handling

3. **Text Chunking**
   - Configurable chunk size (default: 512 characters)
   - Overlap handling (default: 50 characters)
   - Boundary preservation (sentence/paragraph aware)
   - Chunk indexing and metadata

4. **Document Storage**
   - Document metadata creation
   - File hash calculation for deduplication
   - Database persistence (SQL or memory)
   - Relationship tracking

5. **Embedding Generation**
   - AI service integration (Ollama, OpenAI)
   - Batch processing for efficiency
   - Vector validation and normalization
   - Error handling and retries

6. **Vector Storage**
   - Vector database insertion (Qdrant or memory)
   - Similarity indexing for fast search
   - Metadata association
   - Cleanup on errors

### 2. Search Flow

The search process converts queries into relevant document results using specific classes and methods:

```
Entry Point: documentsModule.SearchDocuments() → module.go:562

┌──────────────────────────────────────────────────────────────────────────────┐
│                           SearchEngine                                      │
│                      (core/search.go:15)                                    │
├──────────────────────────────────────────────────────────────────────────────┤
│  SearchDocuments(ctx, query, topK) → []*models.DocumentSearchResult         │
│                                                                              │
│  1. Query Processing (core/search.go:50)                                    │
│     ├─ validateQuery(query) → error                                         │
│     ├─ normalizeQuery(query) → string                                       │
│     └─ extractParameters(topK) → int                                        │
│                           │                                                  │
│                           ▼                                                  │
│  2. Query Embedding (EmbeddingEngine - core/embedding.go:71)                │
│     ├─ embeddingEngine.GenerateEmbedding(ctx, query) → ([]float32, error)   │
│     │  Via: OllamaEmbeddingProvider.GenerateEmbedding()                     │
│     │       (providers/ollama.go:115)                                       │
│     │  Data: query string → queryVector []float32                           │
│     └─ validateEmbeddingDimension(vector) → error                           │
│                           │                                                  │
│                           ▼                                                  │
│  3. Vector Search (stores/qdrant.go OR stores/vector_store.go)              │
│     ├─ vectorStore.SearchSimilar(ctx, queryVector, topK*5) →                │
│     │  []*models.SearchResult                                               │
│     │  Data: queryVector []float32 → []SearchResult{                        │
│     │    Entry: *VectorEntry, Score: float32, Document: *Document}          │
│     └─ applySimilarityThreshold(results) → []*models.SearchResult           │
│                           │                                                  │
│                           ▼                                                  │
│  4. Result Grouping (core/search.go:76-120)                                 │
│     ├─ groupResultsByDocument(vectorResults) →                              │
│     │  map[string]*models.DocumentSearchResult                              │
│     │  Process: For each SearchResult:                                      │
│     │    ├─ Extract docID from result.Entry.DocumentID                      │
│     │    ├─ Group chunks by document ID                                     │
│     │    ├─ Track best score per document                                   │
│     │    └─ Create ChunkResult{Content, Score, ChunkIndex}                  │
│     └─ calculateBestScore(chunks) → float32                                 │
│                           │                                                  │
│                           ▼                                                  │
│  5. Document Retrieval (stores/document_store.go)                           │
│     ├─ documentStore.GetDocument(ctx, docID) → (*models.Document, error)    │
│     │  Data: docID string → *Document{ID, FilePath, FileHash,               │
│     │        Content, Metadata, ChunkCount, CreatedAt, UpdatedAt}           │
│     └─ enrichDocumentMetadata(doc) → *models.Document                       │
│                           │                                                  │
│                           ▼                                                  │
│  6. Response Formatting (core/search.go:121-150)                            │
│     ├─ sortResultsByScore(results) → []*models.DocumentSearchResult         │
│     ├─ limitResults(results, topK) → []*models.DocumentSearchResult         │
│     │  Data: map[docID]*DocumentSearchResult →                              │
│     │        []*DocumentSearchResult{Document, Chunks, BestScore}           │
│     └─ validateFinalResults(results) → error                                │
└──────────────────────────────────────────────────────────────────────────────┘

Enhanced Search Flow (SearchDocumentsEnhanced):
┌──────────────────────────────────────────────────────────────────────────────┐
│  7. Context Highlights (core/search.go:274)                                 │
│     ├─ generateContextHighlights(chunks, maxHighlights) → []string          │
│     ├─ extractRelevantContext(content, query) → string                      │
│     └─ highlightMatchingTerms(context, query) → string                      │
│                           │                                                  │
│                           ▼                                                  │
│  8. Content Assembly (core/search.go:206)                                   │
│     ├─ ReconstructContentFromChunks(vectors) → string                       │
│     ├─ truncateContent(content, maxSize) → string                           │
│     │  Data: []*VectorEntry → reconstructed content string                  │
│     └─ generateContentPreview(content) → string                             │
└──────────────────────────────────────────────────────────────────────────────┘

Key Data Transformations:
├─ Query (string) → Query Vector ([]float32)
├─ Vector Search → []*models.SearchResult
├─ SearchResults → map[docID]*DocumentSearchResult (grouped by document)
├─ Document IDs → []*models.Document (metadata retrieval)
└─ Final Results → []*models.DocumentSearchResult OR []*models.EnhancedDocumentSearchResult
```

**Detailed Steps:**

1. **Query Processing**
   - Text normalization and validation
   - Parameter extraction (topK, filters)
   - Search configuration application

2. **Query Embedding**
   - Same embedding model as documents
   - Vector generation and validation
   - Dimension consistency checks

3. **Vector Search**
   - Cosine similarity calculation
   - Top-K vector retrieval (expanded for grouping)
   - Score normalization and ranking

4. **Document Retrieval**
   - Document metadata fetching
   - Content reconstruction from chunks
   - Relationship resolution

5. **Result Aggregation**
   - Group vectors by document
   - Calculate best scores per document
   - Sort by relevance

6. **Response Formatting**
   - Context highlight generation
   - Content preview creation
   - Metadata enrichment

### 3. Storage Operations

The dual storage system ensures efficient data management using concrete implementations:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              Input Data                                      │
│                                                                              │
│  models.Document                        models.TextChunk                    │
│  ┌─────────────────────┐                ┌─────────────────────┐             │
│  │ ID: string          │                │ ID: string          │             │
│  │ FilePath: string    │                │ DocumentID: string  │             │
│  │ FileHash: string    │                │ Content: string     │             │
│  │ Content: string     │                │ StartPos: int       │             │
│  │ Metadata: map[..]   │                │ EndPos: int         │             │
│  │ ChunkCount: int     │                │ ChunkIndex: int     │             │
│  │ CreatedAt: time     │                │ CreatedAt: time     │             │
│  │ UpdatedAt: time     │                └─────────────────────┘             │
│  └─────────────────────┘                                                    │
└──────────────────────────────────────────────────────────────────────────────┘
           │                                            │
           ▼                                            ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                         Document Storage Layer                              │
│                      (stores/document_store.go)                             │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                    SQLDocumentStore                                    │ │
│  │                  (document_store.go:25)                                │ │
│  ├─────────────────────────────────────────────────────────────────────────┤ │
│  │  StoreDocument(ctx, doc) → error                                       │ │
│  │  GetDocument(ctx, docID) → (*Document, error)                          │ │
│  │  GetDocumentByPath(ctx, path) → (*Document, error)                     │ │
│  │  ListDocuments(ctx) → ([]*Document, error)                             │ │
│  │  DeleteDocument(ctx, docID) → error                                    │ │
│  │  UpdateDocumentMetadata(ctx, docID, metadata) → error                  │ │
│  │                                                                         │ │
│  │  SQL Schema: documents table                                           │ │
│  │  ├─ id (PRIMARY KEY)                                                    │ │
│  │  ├─ file_path (UNIQUE)                                                  │ │
│  │  ├─ file_hash                                                           │ │
│  │  ├─ content (TEXT)                                                      │ │
│  │  ├─ metadata (JSON)                                                     │ │
│  │  └─ timestamps                                                          │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                  OR                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                   MemoryDocumentStore                                  │ │
│  │                  (document_store.go:592)                               │ │
│  ├─────────────────────────────────────────────────────────────────────────┤ │
│  │  StoreDocument(ctx, doc) → error                                       │ │
│  │  GetDocument(ctx, docID) → (*Document, error)                          │ │
│  │  Same interface, in-memory map[string]*Document storage                │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                    Embedding Generation Layer                               │
│                     (core/embedding.go:14)                                  │
├──────────────────────────────────────────────────────────────────────────────┤
│  EmbeddingEngine                                                             │
│  ├─ GenerateEmbedding(ctx, text) → ([]float32, error)                       │
│  └─ Via providers: OllamaEmbeddingProvider OR OpenAIEmbeddingProvider       │
│                                                                              │
│  Transforms: TextChunk.Content → models.VectorEntry                         │
│  ┌─────────────────────┐                                                    │
│  │ ID: string          │                                                    │
│  │ DocumentID: string  │                                                    │
│  │ ChunkID: string     │                                                    │
│  │ Vector: []float32   │ ← Generated embedding (384 dimensions)             │
│  │ Dimension: int      │                                                    │
│  │ Content: string     │                                                    │
│  │ Metadata: string    │                                                    │
│  │ CreatedAt: time     │                                                    │
│  └─────────────────────┘                                                    │
└──────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                          Vector Storage Layer                               │
│                    (stores/qdrant.go, stores/vector_store.go)               │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                     QdrantVectorStore                                  │ │
│  │                     (stores/qdrant.go:16)                              │ │
│  ├─────────────────────────────────────────────────────────────────────────┤ │
│  │  Store(ctx, vectorEntry) → error                                       │ │
│  │  SearchSimilar(ctx, queryVector, topK) → ([]*SearchResult, error)      │ │
│  │  SearchWithFilter(ctx, vector, topK, filter) → ([]*SearchResult, ..)   │ │
│  │  GetVector(ctx, vectorID) → (*VectorEntry, error)                      │ │
│  │  DeleteVector(ctx, vectorID) → error                                   │ │
│  │                                                                         │ │
│  │  Uses: Qdrant Go Client                                                │ │
│  │  ├─ Collection-based storage                                            │ │
│  │  ├─ HNSW indexing for fast similarity search                           │ │
│  │  ├─ Cosine similarity metric                                            │ │
│  │  └─ QDRantPayload structure for metadata                               │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                  OR                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                    MemoryVectorStore                                   │ │
│  │                  (stores/vector_store.go:26)                           │ │
│  ├─────────────────────────────────────────────────────────────────────────┤ │
│  │  Store(ctx, vectorEntry) → error                                       │ │
│  │  SearchSimilar(ctx, queryVector, topK) → ([]*SearchResult, error)      │ │
│  │  Same interface, in-memory storage with linear similarity search       │ │
│  │  Uses: map[string]*VectorEntry + cosine similarity calculation         │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘

Storage Flow Coordination:
┌──────────────────────────────────────────────────────────────────────────────┐
│ 1. Document → DocumentStore.StoreDocument()                                 │
│ 2. TextChunk → EmbeddingEngine.GenerateEmbedding() → VectorEntry            │
│ 3. VectorEntry → VectorStore.Store()                                        │
│ 4. Search: Query → VectorStore.SearchSimilar() → DocumentStore.GetDocument()│
└──────────────────────────────────────────────────────────────────────────────┘
```

### 4. Key Data Structures and Interfaces

The following data structures and interfaces define the contracts between components:

```go
// Core Data Models (models/types.go)

type Document struct {
    ID         string                 `json:"id"`         // UUID
    FilePath   string                 `json:"file_path"`  // Absolute file path
    FileHash   string                 `json:"file_hash"`  // SHA-256 hash
    Content    string                 `json:"content"`    // Reconstructed content
    Metadata   map[string]interface{} `json:"metadata"`   // File metadata
    ChunkCount int                    `json:"chunk_count"`// Number of chunks
    CreatedAt  time.Time              `json:"created_at"`
    UpdatedAt  time.Time              `json:"updated_at"`
}

type TextChunk struct {
    ID         string    `json:"id"`          // UUID
    DocumentID string    `json:"document_id"` // Reference to parent document
    Content    string    `json:"content"`     // Chunk text content
    StartPos   int       `json:"start_pos"`   // Start position in original text
    EndPos     int       `json:"end_pos"`     // End position in original text
    ChunkIndex int       `json:"chunk_index"` // Index within document
    CreatedAt  time.Time `json:"created_at"`
}

type VectorEntry struct {
    ID         string    `json:"id"`          // UUID
    DocumentID string    `json:"document_id"` // Reference to parent document
    ChunkID    string    `json:"chunk_id"`    // Reference to source chunk
    Vector     []float32 `json:"vector"`      // Embedding vector (384 dims)
    Dimension  int       `json:"dimension"`   // Vector dimensionality
    Content    string    `json:"content"`     // Original chunk content
    Metadata   string    `json:"metadata"`    // JSON metadata
    CreatedAt  time.Time `json:"created_at"`
}

type SearchResult struct {
    Entry      *VectorEntry `json:"entry"`       // Found vector entry
    Score      float32      `json:"score"`       // Similarity score (0.0-1.0)
    Document   *Document    `json:"document"`    // Document metadata (optional)
    ChunkIndex int          `json:"chunk_index"` // Chunk position in document
}

type DocumentSearchResult struct {
    Document  *Document      `json:"document"`    // Document metadata
    Chunks    []*ChunkResult `json:"chunks"`      // Matching chunks
    BestScore float32        `json:"best_score"`  // Highest chunk score
}

type ChunkResult struct {
    Content    string  `json:"content"`     // Chunk content
    Score      float32 `json:"score"`       // Similarity score
    ChunkIndex int     `json:"chunk_index"` // Position in document
}
```

```go
// Core Processing Interfaces (interfaces/core.go)

type DocumentStoreInterface interface {
    StoreDocument(ctx context.Context, doc *models.Document) error
    GetDocument(ctx context.Context, docID string) (*models.Document, error)
    GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error)
    ListDocuments(ctx context.Context) ([]*models.Document, error)
    DeleteDocument(ctx context.Context, docID string) error
    UpdateDocumentMetadata(ctx context.Context, docID string, metadata map[string]interface{}) error

    // Batch operations
    StoreDocuments(ctx context.Context, docs []*models.Document) error
    DeleteDocuments(ctx context.Context, docIDs []string) error

    // Search and filtering
    SearchDocumentsByMetadata(ctx context.Context, filter map[string]interface{}) ([]*models.Document, error)

    // Health and management
    HealthCheck(ctx context.Context) error
    GetStats(ctx context.Context) (map[string]interface{}, error)
}

type VectorStoreInterface interface {
    Store(ctx context.Context, entry *models.VectorEntry) error
    GetVector(ctx context.Context, vectorID string) (*models.VectorEntry, error)
    SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]*models.SearchResult, error)
    SearchWithFilter(ctx context.Context, queryVector []float32, topK int, filter map[string]interface{}) ([]*models.SearchResult, error)
    DeleteVector(ctx context.Context, vectorID string) error

    // Batch operations
    StoreBatch(ctx context.Context, entries []*models.VectorEntry) error
    DeleteBatch(ctx context.Context, vectorIDs []string) error

    // Collection management
    CreateCollection(ctx context.Context, name string, dimension int) error
    DeleteCollection(ctx context.Context, name string) error

    // Health and management
    HealthCheck(ctx context.Context) error
    GetCollectionInfo(ctx context.Context) (map[string]interface{}, error)
}

type TextChunkerInterface interface {
    ChunkText(content string, config models.ChunkingConfig) []models.TextChunk
    ChunkDocument(doc *models.Document, config models.ChunkingConfig) ([]*models.TextChunk, error)
}

type EmbeddingProvider interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
    GetDimension() int
    GetModelName() string
    Initialize(ctx context.Context) error
    Close() error
    HealthCheck(ctx context.Context) error
}
```

```go
// Method Signatures for Key Operations

// DocumentProcessor (core/processing.go:23)
func (dp *DocumentProcessor) ProcessFile(ctx context.Context, filePath string) (*models.Document, error)
func (dp *DocumentProcessor) ProcessFiles(ctx context.Context, filePaths []string) ([]*models.ProcessResult, error)
func (dp *DocumentProcessor) validateFilePath(filePath string) error
func (dp *DocumentProcessor) extractTextFromFile(filePath string) (string, error)
func (dp *DocumentProcessor) detectMimeType(filePath string) string
func (dp *DocumentProcessor) calculateFileHash(content string) string

// SearchEngine (core/search.go:15)
func (se *SearchEngine) SearchDocuments(ctx context.Context, query string, topK int) ([]*models.DocumentSearchResult, error)
func (se *SearchEngine) SearchDocumentsEnhanced(ctx context.Context, query string, topK int, config models.SearchConfig) ([]*models.EnhancedDocumentSearchResult, error)
func (se *SearchEngine) GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error)
func (se *SearchEngine) ReconstructContentFromChunks(vectors []*models.VectorEntry) string
func (se *SearchEngine) generateContextHighlights(chunks []*models.ChunkResult, maxHighlights int) []string

// EmbeddingEngine (core/embedding.go:14)
func (ee *EmbeddingEngine) GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
func (ee *EmbeddingEngine) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
func (ee *EmbeddingEngine) CalculateSimilarity(vec1, vec2 []float32) float32
func (ee *EmbeddingEngine) ValidateEmbedding(embedding []float32) error

// SimpleTextChunker (core/chunking.go:13)
func (c *SimpleTextChunker) ChunkText(content string, config models.ChunkingConfig) []models.TextChunk
func (c *SimpleTextChunker) ChunkDocument(doc *models.Document, config models.ChunkingConfig) ([]*models.TextChunk, error)
func (c *SimpleTextChunker) generateChunkID() string

// QdrantVectorStore (stores/qdrant.go:16)
func (qvs *QdrantVectorStore) Store(ctx context.Context, entry *models.VectorEntry) error
func (qvs *QdrantVectorStore) SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]*models.SearchResult, error)
func (qvs *QdrantVectorStore) buildQdrantPayload(entry *models.VectorEntry) *QDRantPayload

// SQLDocumentStore (stores/document_store.go:25)
func (sds *SQLDocumentStore) StoreDocument(ctx context.Context, doc *models.Document) error
func (sds *SQLDocumentStore) GetDocument(ctx context.Context, docID string) (*models.Document, error)
func (sds *SQLDocumentStore) buildDocumentFromRow(row *sql.Row) (*models.Document, error)
```

**Storage Flow:**

1. **Document Store**: Maintains structured document metadata
   - **SQL Mode**: PostgreSQL/MySQL tables with ACID compliance
   - **Memory Mode**: In-memory hash maps for development/testing

2. **Vector Store**: Handles high-dimensional embeddings
   - **Qdrant Mode**: Specialized vector database with indexing
   - **Memory Mode**: Simple in-memory vector storage

3. **Data Consistency**: Ensures synchronization between stores
   - Transactional operations where supported
   - Cleanup procedures for failed operations
   - Integrity validation and repair tools

## Component Documentation

### Core Processing Engine

The core processing engine orchestrates the entire document workflow:

**DocumentProcessor** (`core/processing.go`)
- **Purpose**: Main processing coordinator
- **Key Features**:
  - File validation and security checks
  - Content extraction with MIME type detection
  - Text chunking with configurable strategies
  - Embedding generation coordination
  - Batch processing capabilities
  - Error handling and retry logic

**Configuration**:
```go
// Note: ProcessorConfig is internal. Use DocumentsConfig for module configuration
type DocumentsConfig struct {
    // Core configuration
    ChunkingConfig models.ChunkingConfig `json:"chunking"`
    SearchConfig   models.SearchConfig   `json:"search"`

    // Processing configuration
    MaxWorkers        int `json:"max_workers"`
    ProcessingTimeout int `json:"processing_timeout_seconds"`
    BatchSize         int `json:"batch_size"`
    RetryAttempts     int `json:"retry_attempts"`
    RetryDelay        int `json:"retry_delay_seconds"`

    // Storage configuration
    VectorStoreConfig   map[string]interface{} `json:"vector_store"`
    DocumentStoreConfig map[string]interface{} `json:"document_store"`

    // Cache configuration
    CacheEnabled bool `json:"cache_enabled"`
    CacheTTL     int  `json:"cache_ttl_seconds"`
    CacheMaxSize int  `json:"cache_max_size"`

    // File watching configuration
    FileWatcherEnabled bool     `json:"file_watcher_enabled"`
    WatchPaths         []string `json:"watch_paths"`
    IgnorePatterns     []string `json:"ignore_patterns"`

    // Metrics and monitoring
    MetricsEnabled      bool `json:"metrics_enabled"`
    HealthCheckInterval int  `json:"health_check_interval_seconds"`

    // Rate limiting
    RateLimitEnabled  bool `json:"rate_limit_enabled"`
    RateLimitRequests int  `json:"rate_limit_requests"`
    RateLimitDuration int  `json:"rate_limit_duration_seconds"`

    // Feature flags
    EnableAdvancedSearch  bool `json:"enable_advanced_search"`
    EnableContentAnalysis bool `json:"enable_content_analysis"`
    EnableEventBus        bool `json:"enable_event_bus"`
}
```

### Vector and Document Stores

**Document Store** (`stores/document_store.go`)
- **Purpose**: Structured document metadata storage
- **Implementations**:
  - **SQL Store**: Relational database with full ACID compliance
  - **Memory Store**: In-memory storage for development

**Vector Store** (`stores/vector_store.go`, `stores/qdrant.go`)
- **Purpose**: High-dimensional vector storage and similarity search
- **Implementations**:
  - **Qdrant Store**: Production-grade vector database
  - **Memory Store**: Simple in-memory vectors

**Storage Configuration**:
```go
// Vector Store Configuration
VectorStoreConfig: map[string]interface{}{
    "type":       "qdrant",        // "qdrant" or "memory"
    "dimension":  384,             // embedding dimension (match your provider)
    "metric":     "cosine",        // similarity metric
    "batch_size": 100,             // batch processing size
    "timeout":    30,              // operation timeout in seconds
}

// Document Store Configuration
DocumentStoreConfig: map[string]interface{}{
    "type":       "sql",           // "sql" or "memory"
    "table_name": "documents",     // SQL table name
    "batch_size": 100,             // batch processing size
    "timeout":    30,              // operation timeout in seconds
}
```

### Embedding Providers

**Embedding Engine** (`core/embedding.go`)
- **Purpose**: Manages embedding generation and caching
- **Features**:
  - Multiple provider support (Ollama, OpenAI)
  - Batch processing for efficiency
  - Caching to reduce API calls
  - Vector validation and normalization

**Provider Integration**:
```go
type EmbeddingProvider interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
    GetDimension() int
    GetModelName() string
    Initialize(ctx context.Context) error
    Close() error
    HealthCheck(ctx context.Context) error
}
```

**Supported Providers**:

**Ollama Provider Example**:
```go
// Configure Ollama provider
ollamaProvider := ollama.NewEmbeddingProvider(ollama.Config{
    BaseURL: "http://localhost:11434",
    Model:   "all-minilm:l6-v2",
    Timeout: 30,
})

deps := Dependencies{
    EmbeddingProvider: ollamaProvider,
    // ... other dependencies
}
```

**OpenAI Provider Example**:
```go
// Configure OpenAI provider
openaiProvider := openai.NewEmbeddingProvider(openai.Config{
    APIKey: "your-openai-api-key",
    Model:  "text-embedding-ada-002",
    Timeout: 30,
})

deps := Dependencies{
    EmbeddingProvider: openaiProvider,
    // ... other dependencies
}
```

### Search Engine

**Search Engine** (`core/search.go`)
- **Purpose**: Semantic document search with ranking
- **Key Features**:
  - Vector similarity search
  - Document grouping and ranking
  - Context highlight generation
  - Enhanced search with full content
  - Configurable result formatting

### Configuration System

**Complete Configuration Structure** (`module.go`)
The module uses a comprehensive configuration structure:

```go
type DocumentsConfig struct {
    // Core configuration
    ChunkingConfig models.ChunkingConfig `json:"chunking"`
    SearchConfig   models.SearchConfig   `json:"search"`

    // Processing configuration
    MaxWorkers        int `json:"max_workers"`
    ProcessingTimeout int `json:"processing_timeout_seconds"`
    BatchSize         int `json:"batch_size"`
    RetryAttempts     int `json:"retry_attempts"`
    RetryDelay        int `json:"retry_delay_seconds"`

    // Storage configuration
    VectorStoreConfig   map[string]interface{} `json:"vector_store"`
    DocumentStoreConfig map[string]interface{} `json:"document_store"`

    // Cache configuration
    CacheEnabled bool `json:"cache_enabled"`
    CacheTTL     int  `json:"cache_ttl_seconds"`
    CacheMaxSize int  `json:"cache_max_size"`

    // File watching configuration
    FileWatcherEnabled bool     `json:"file_watcher_enabled"`
    WatchPaths         []string `json:"watch_paths"`
    IgnorePatterns     []string `json:"ignore_patterns"`

    // Metrics and monitoring
    MetricsEnabled      bool `json:"metrics_enabled"`
    HealthCheckInterval int  `json:"health_check_interval_seconds"`

    // Rate limiting
    RateLimitEnabled  bool `json:"rate_limit_enabled"`
    RateLimitRequests int  `json:"rate_limit_requests"`
    RateLimitDuration int  `json:"rate_limit_duration_seconds"`

    // Feature flags
    EnableAdvancedSearch  bool `json:"enable_advanced_search"`
    EnableContentAnalysis bool `json:"enable_content_analysis"`
    EnableEventBus        bool `json:"enable_event_bus"`
}
```

## API Documentation

### Main Interface

The primary API is exposed through the `DocumentsModule` interface:

```go
type DocumentsModule interface {
    // Version information
    Version() string
    GetInfo() *ModuleInfo

    // Core document operations
    ProcessFile(ctx context.Context, path string) (*models.ProcessResult, error)
    ProcessFiles(ctx context.Context, paths []string) ([]models.ProcessResult, error)
    DeleteDocument(ctx context.Context, docID string) error
    GetDocument(ctx context.Context, docID string) (*models.Document, error)

    // Search operations
    SearchDocuments(ctx context.Context, query string, topK int) ([]*models.DocumentSearchResult, error)
    SearchDocumentsEnhanced(ctx context.Context, query string, topK int, config models.SearchConfig) ([]*models.EnhancedDocumentSearchResult, error)

    // Batch operations
    DeleteBatch(ctx context.Context, docIDs []string) error

    // Document management
    ListDocuments(ctx context.Context) ([]*models.Document, error)
    GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error)
    UpdateDocumentMetadata(ctx context.Context, docID string, metadata map[string]interface{}) error

    // Health and diagnostics
    HealthCheck(ctx context.Context) (*models.HealthStatus, error)
    GetMetrics(ctx context.Context) (*models.ModuleMetrics, error)
    ValidateConfiguration(ctx context.Context) error

    // Lifecycle management
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error

    // Configuration management
    UpdateConfiguration(ctx context.Context, config map[string]interface{}) error
    GetConfiguration(ctx context.Context) (map[string]interface{}, error)
}
```

### Usage Examples

#### Basic Document Processing

```go
// Initialize module
deps := Dependencies{
    DB:                database,
    EmbeddingProvider: embeddingProvider,
    Logger:           logger,
    Config:           DefaultDocumentsConfig(),
}

module, err := NewDocumentsModule(deps)
if err != nil {
    log.Fatal(err)
}

// Start module
if err := module.Start(ctx); err != nil {
    log.Fatal(err)
}

// Process a single file
result, err := module.ProcessFile(ctx, "/path/to/document.txt")
if err != nil {
    log.Printf("Processing failed: %v", err)
    return
}

fmt.Printf("Processed document: %s (ID: %s)\n",
    result.Document.FilePath, result.Document.ID)
```

#### Batch Processing

```go
// Process multiple files
filePaths := []string{
    "/docs/file1.txt",
    "/docs/file2.md",
    "/docs/file3.json",
}

results, err := module.ProcessFiles(ctx, filePaths)
if err != nil {
    log.Printf("Batch processing failed: %v", err)
    return
}

// Check results
for _, result := range results {
    if result.Success {
        fmt.Printf("✓ Processed: %s\n", result.FilePath)
    } else {
        fmt.Printf("✗ Failed: %s - %v\n", result.FilePath, result.Error)
    }
}
```

#### Document Search

```go
// Basic search
searchResults, err := module.SearchDocuments(ctx, "artificial intelligence", 5)
if err != nil {
    log.Printf("Search failed: %v", err)
    return
}

// Display results
for i, result := range searchResults {
    fmt.Printf("%d. %s (Score: %.3f)\n",
        i+1, result.Document.FilePath, result.BestScore)

    // Show matching chunks
    for _, chunk := range result.Chunks {
        fmt.Printf("   Chunk: %s (Score: %.3f)\n",
            chunk.Content[:100]+"...", chunk.Score)
    }
}
```

#### Enhanced Search with Configuration

```go
// Configure search
searchConfig := models.SearchConfig{
    MaxDocumentSize:    100 * 1024, // 100KB
    MaxHighlights:      5,
    IncludeFullContent: true,
}

// Enhanced search
enhancedResults, err := module.SearchDocumentsEnhanced(ctx,
    "machine learning algorithms", 3, searchConfig)
if err != nil {
    log.Printf("Enhanced search failed: %v", err)
    return
}

// Display enhanced results
for _, result := range enhancedResults {
    fmt.Printf("Document: %s\n", result.Document.FilePath)
    fmt.Printf("Score: %.3f\n", result.BestScore)
    fmt.Printf("Highlights:\n")

    for _, highlight := range result.ContextHighlights {
        fmt.Printf("  - %s\n", highlight)
    }

    if result.IsTruncated {
        fmt.Printf("Content: %s\n", result.ContentPreview)
    }
}
```

#### Concurrent Processing Example

```go
// Process documents concurrently
filePaths := []string{
    "/docs/file1.txt",
    "/docs/file2.md",
    "/docs/file3.json",
}

// Configure for concurrent processing
config := documents.DefaultDocumentsConfig()
config.MaxWorkers = 8 // Increase workers for concurrent processing
config.BatchSize = 50 // Process in larger batches

// Process multiple files
results, err := module.ProcessFiles(ctx, filePaths)
if err != nil {
    log.Printf("Batch processing failed: %v", err)
    return
}

// Check results
for _, result := range results {
    if result.Success {
        fmt.Printf("✓ Processed: %s in %v\n",
            result.FilePath, result.ProcessedAt)
    } else {
        fmt.Printf("✗ Failed: %s - %v\n",
            result.FilePath, result.Error)
    }
}
```

## Configuration Guide

### Basic Configuration

Default configuration for development:

```go
config := documents.DocumentsConfig{
    ChunkingConfig: models.ChunkingConfig{
        Strategy:     models.ChunkingStrategyFixed,
        MaxChunkSize: 512,
        ChunkOverlap: 50,
    },
    SearchConfig: models.SearchConfig{
        MaxDocumentSize:    50 * 1024, // 50KB
        MaxHighlights:      3,
        IncludeFullContent: true,
    },
    MaxWorkers:        4,
    ProcessingTimeout: 300, // 5 minutes
    BatchSize:         100,
    RetryAttempts:     3,
    RetryDelay:        5,

    // Use memory stores for development
    VectorStoreConfig: map[string]interface{}{
        "type": "memory",
    },
    DocumentStoreConfig: map[string]interface{}{
        "type": "memory",
    },

    // Disable advanced features for simplicity
    FileWatcherEnabled:    false,
    EnableContentAnalysis: false,
    MetricsEnabled:        false,
}
```

### Production Configuration

Optimized for production environments:

```go
config := documents.DocumentsConfig{
    // Core configuration
    ChunkingConfig: models.ChunkingConfig{
        Strategy:     models.ChunkingStrategyFixed,
        MaxChunkSize: 512,
        ChunkOverlap: 50,
    },
    SearchConfig: models.SearchConfig{
        MaxDocumentSize:    100 * 1024, // 100KB
        MaxHighlights:      5,
        IncludeFullContent: true,
    },

    // Processing
    MaxWorkers:        16,
    ProcessingTimeout: 600, // 10 minutes
    BatchSize:         500,
    RetryAttempts:     5,
    RetryDelay:        10,

    // Storage
    VectorStoreConfig: map[string]interface{}{
        "type":       "qdrant",
        "dimension":  384,
        "batch_size": 1000,
        "timeout":    60,
    },
    DocumentStoreConfig: map[string]interface{}{
        "type":       "sql",
        "table_name": "documents",
        "batch_size": 1000,
        "timeout":    30,
    },

    // Cache configuration
    CacheEnabled: true,
    CacheTTL:     3600, // 1 hour
    CacheMaxSize: 50000,

    // Monitoring
    MetricsEnabled:      true,
    HealthCheckInterval: 30,

    // File watching
    FileWatcherEnabled: true,
    WatchPaths:         []string{"/data/documents"},
    IgnorePatterns:     []string{".git", "*.tmp", ".DS_Store"},

    // Performance and security
    RateLimitEnabled:     true,
    RateLimitRequests:    10000,
    RateLimitDuration:    3600, // per hour

    // Feature flags
    EnableAdvancedSearch:  true,
    EnableContentAnalysis: true,
    EnableEventBus:        true,
}
```

### Feature Flags

Control advanced features:

```go
config := documents.DocumentsConfig{
    // Advanced features
    EnableAdvancedSearch:   true,  // Enhanced search capabilities
    EnableContentAnalysis: true,  // Content analysis and extraction
    EnableEventBus:        true,  // Event publishing

    // File watching
    FileWatcherEnabled: true,
    WatchPaths:         []string{"/watched/docs"},
    IgnorePatterns:     []string{".git", "*.tmp", ".DS_Store"},

    // Enable metrics and caching
    MetricsEnabled: true,
    CacheEnabled:   true,
    CacheTTL:       1800, // 30 minutes
    CacheMaxSize:   10000,

    // Rate limiting for API protection
    RateLimitEnabled:  true,
    RateLimitRequests: 1000,
    RateLimitDuration: 3600,
}
```

## Performance Considerations

### Optimization Guidelines

1. **Batch Processing**: Use batch operations for multiple documents
2. **Caching**: Enable embedding cache to reduce API calls
3. **Worker Pools**: Configure appropriate worker count for CPU cores
4. **Chunk Size**: Balance between context and processing speed
5. **Vector Store**: Use Qdrant for production, memory for development

### Performance Benchmarks

**Typical Performance Metrics**:
- **Small Documents** (< 10KB): 50-100 docs/minute
- **Medium Documents** (10KB-100KB): 20-50 docs/minute
- **Large Documents** (100KB-1MB): 5-20 docs/minute
- **Search Latency**: < 100ms for typical queries
- **Memory Usage**: 50-200MB base + 1-5MB per 1000 documents

**Optimization Recommendations**:
```go
// High-throughput configuration
config := documents.DocumentsConfig{
    MaxWorkers:        16,              // Scale with CPU cores
    BatchSize:         500,             // Larger batches
    ProcessingTimeout: 300,             // 5 minutes
    RetryAttempts:     3,

    // Enable caching
    CacheEnabled:     true,
    CacheTTL:         3600,             // 1 hour
    CacheMaxSize:     50000,            // 50k entries

    // Optimize storage
    VectorStoreConfig: map[string]interface{}{
        "type":       "qdrant",
        "batch_size": 1000,
        "timeout":    60,
    },
}
```

### Monitoring and Metrics

The module provides comprehensive metrics:

```go
// Get module metrics
metrics, err := module.GetMetrics(ctx)
if err != nil {
    log.Printf("Failed to get metrics: %v", err)
    return
}

fmt.Printf("Total Documents: %d\n", metrics.TotalDocuments)
fmt.Printf("Total Chunks: %d\n", metrics.TotalChunks)
fmt.Printf("Average Chunk Size: %.2f\n", metrics.AverageChunkSize)
fmt.Printf("Error Rate: %.2f%%\n", metrics.ErrorRate*100)
```

### Health Monitoring

```go
// Check module health
health, err := module.HealthCheck(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
    return
}

fmt.Printf("Status: %s\n", health.Status)
for component, status := range health.Components {
    fmt.Printf("  %s: %v\n", component, status)
}
```

## Error Handling

The module provides structured error handling with detailed error types:

```go
// Error types
const (
    ErrFileNotFound         = "file_not_found"
    ErrFileReadError        = "file_read_error"
    ErrUnsupportedFormat    = "unsupported_format"
    ErrDocumentTooBig       = "document_too_big"
    ErrEmbeddingFailed      = "embedding_failed"
    ErrSearchFailed         = "search_failed"
    ErrStorageFailed        = "storage_failed"
)

// Handle errors with context
result, err := module.ProcessFile(ctx, filePath)
if err != nil {
    if docErr, ok := err.(*models.DocumentError); ok {
        switch docErr.Code {
        case models.ErrFileNotFound:
            log.Printf("File not found: %s", docErr.FilePath)
        case models.ErrUnsupportedFormat:
            log.Printf("Unsupported format: %s", docErr.Message)
        case models.ErrEmbeddingFailed:
            log.Printf("Embedding failed for doc %s: %s",
                docErr.DocumentID, docErr.Message)
        default:
            log.Printf("Processing error: %s", docErr.Message)
        }
    }
    return
}
```

## Development and Testing

### Development Setup

```go
// Development configuration
devConfig := documents.DocumentsConfig{
    ChunkingConfig: models.ChunkingConfig{
        Strategy:     models.ChunkingStrategyFixed,
        MaxChunkSize: 256, // Smaller chunks for faster testing
        ChunkOverlap: 25,
    },
    SearchConfig: models.SearchConfig{
        MaxDocumentSize:    10 * 1024, // 10KB for testing
        MaxHighlights:      2,
        IncludeFullContent: false,
    },
    VectorStoreConfig: map[string]interface{}{
        "type": "memory", // Use memory store for development
    },
    DocumentStoreConfig: map[string]interface{}{
        "type": "memory", // Use memory store for development
    },
    MaxWorkers:        2,
    ProcessingTimeout: 60,
    BatchSize:         10,
    RetryAttempts:     1,
    RetryDelay:        1,

    // Disable heavy features
    FileWatcherEnabled:    false,
    EnableContentAnalysis: false,
    MetricsEnabled:        false,
    CacheEnabled:          false,
    RateLimitEnabled:      false,
}
```

### Testing Support

The module supports comprehensive testing:

```go
// Create test module
deps := Dependencies{
    DB:                mockDB,
    EmbeddingProvider: mockEmbeddingProvider,
    Logger:           testLogger,
    Config:           devConfig,
}

module, err := NewDocumentsModule(deps)
// ... test operations
```

## Security Considerations

### File Path Security
- **Path Validation**: All file paths are sanitized to prevent directory traversal attacks
- **Restricted Directories**: System directories are blocked by default
- **Size Limits**: Configurable file size limits prevent resource exhaustion
- **Content Validation**: UTF-8 validation and binary file detection

### Access Control
```go
// Implement access control in your application layer
type SecureDocumentsModule struct {
    module DocumentsModule
    authz  AuthorizationService
}

func (s *SecureDocumentsModule) ProcessFile(ctx context.Context, path string) (*models.ProcessResult, error) {
    if !s.authz.CanProcessFile(ctx, path) {
        return nil, errors.New("unauthorized")
    }
    return s.module.ProcessFile(ctx, path)
}
```

### Data Protection
- **Encryption**: Store sensitive documents with encryption at rest
- **Sanitization**: Remove sensitive information before processing
- **Audit Logging**: Log all document operations for compliance

## Troubleshooting

### Common Issues

**1. Embedding Provider Connection Issues**
```
Error: failed to initialize embedding provider
```
*Solution*:
- Check provider URL and authentication
- Verify network connectivity
- Ensure provider service is running
- Check API quotas and rate limits

**2. Vector Store Connection Issues**
```
Error: failed to create vector store: connection refused
```
*Solution*:
- Verify Qdrant server is running (if using Qdrant)
- Check connection configuration
- Use memory store for development/testing
- Verify network accessibility

**3. High Memory Usage**
*Symptoms*: Module consuming excessive memory
*Solutions*:
- Reduce batch size in configuration
- Enable caching with appropriate TTL
- Implement document size limits
- Use pagination for large document sets

**4. Slow Processing Performance**
*Symptoms*: Document processing takes too long
*Solutions*:
- Increase worker pool size (`MaxWorkers`)
- Use smaller chunk sizes for faster embedding
- Enable batch processing
- Optimize embedding provider settings

**5. Search Results Quality Issues**
*Symptoms*: Poor search relevance
*Solutions*:
- Adjust chunk size and overlap
- Use appropriate embedding model
- Implement content preprocessing
- Fine-tune search configuration

### Debugging Tools

**Enable Debug Logging**:
```go
config := DefaultDocumentsConfig()
// Configure logger for debug level
deps.Logger.SetLevel("debug")
```

**Health Check Diagnostics**:
```go
health, err := module.HealthCheck(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
} else {
    for component, status := range health.Components {
        log.Printf("Component %s: %v", component, status)
    }
}
```

**Performance Metrics**:
```go
metrics, err := module.GetMetrics(ctx)
if err == nil {
    log.Printf("Processing latency: %v", metrics.ProcessingLatency)
    log.Printf("Error rate: %.2f%%", metrics.ErrorRate*100)
}
```

## Glossary

**Chunk**: A segment of text from a document, typically 512 characters with 50 character overlap.

**Embedding**: A numerical vector representation of text content used for semantic similarity.

**Vector Store**: A specialized database for storing and searching high-dimensional vectors (embeddings).

**Document Store**: A database for storing document metadata and content.

**Semantic Search**: Search based on meaning rather than exact keyword matching.

**Cosine Similarity**: A metric used to measure similarity between vectors.

**Top-K Search**: Retrieving the K most similar documents to a query.

**Batch Processing**: Processing multiple documents simultaneously for efficiency.

**Provider**: An external service (like Ollama or OpenAI) that generates embeddings.

**Qdrant**: A vector database optimized for similarity search and storage.

**MCP (Model Context Protocol)**: A protocol for AI model integration.

**Context Window**: The amount of text an embedding model can process at once.

---

## Version Information

- **Version**: 1.0.0
- **Build**: Go 1.21+
- **Dependencies**: See `go.mod` for complete dependency list

## License

Part of the Arcadia application platform. See main project license for details.