/**
 * Memory and knowledge base domain types.
 * メモリとナレッジベースのドメインタイプね。
 *
 * Defines KnowledgeBase and Document for RAG pipeline document storage.
 * RAGパイプラインのドキュメント保存用のKnowledgeBaseとDocumentを定義してるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

type KnowledgeBase struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	EmbeddingModel  string `json:"embedding_model"`
	ChunkStrategy   string `json:"chunk_strategy"`
	ChunkSize       int    `json:"chunk_size"`
	ChunkOverlap    int    `json:"chunk_overlap"`
	DocumentCount   int    `json:"document_count"`
	Metadata        string `json:"metadata"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Document struct {
	ID          string `json:"id"`
	KBID        string `json:"kb_id"`
	Filename    string `json:"filename"`
	Content     string `json:"content,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	Chunks      string `json:"chunks"`
	Metadata    string `json:"metadata"`
	CreatedAt   string `json:"created_at"`
}
