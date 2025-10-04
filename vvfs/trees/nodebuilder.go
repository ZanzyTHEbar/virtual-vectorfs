package trees

import (
	"fmt"
	"time"
)

// NodeBuilder provides a fluent interface for building tree nodes
// Added a new field 'path' to store node path.
type NodeBuilder[T TreeNode] struct {
	node     T
	metadata *Metadata
	path     string
	err      error
}

// NewNodeBuilder creates a new builder for the specified node type
func NewNodeBuilder[T TreeNode]() *NodeBuilder[T] {
	return &NodeBuilder[T]{
		metadata: &Metadata{
			CreatedAt: time.Now(),
		},
	}
}

// WithPath sets the node path
func (b *NodeBuilder[T]) WithPath(path string) *NodeBuilder[T] {
	if b.err != nil {
		return b
	}
	if path == "" {
		b.err = fmt.Errorf("path cannot be empty")
		return b
	}
	b.path = path
	return b
}

// WithMetadata sets node metadata
func (b *NodeBuilder[T]) WithMetadata(metadata *Metadata) *NodeBuilder[T] {
	if b.err != nil {
		return b
	}
	if err := metadata.Validate(); err != nil {
		b.err = fmt.Errorf("invalid metadata: %w", err)
		return b
	}
	b.metadata = metadata
	return b
}

// Build creates the final node
func (b *NodeBuilder[T]) Build() (T, error) {
	if b.err != nil {
		return b.node, b.err
	}
	if any(b.node) == nil {
		if b.path == "" {
			b.err = fmt.Errorf("path must be set before building the node")
			return b.node, b.err
		}

		// Handle different TreeNode types properly
		var node TreeNode
		switch any(b.node).(type) {
		case *DirectoryNode:
			dirNode := NewDirectoryNode(b.path, nil)
			dirNode.Metadata = *b.metadata
			node = dirNode
		default:
			// For other TreeNode types, we would need type-specific constructors
			// For now, return an error indicating unsupported type
			b.err = fmt.Errorf("unsupported TreeNode type: %T", b.node)
			return b.node, b.err
		}

		// Type assertion to ensure compatibility
		if typedNode, ok := node.(T); ok {
			b.node = typedNode
		} else {
			b.err = fmt.Errorf("type assertion failed for TreeNode type")
			return b.node, b.err
		}
	}
	return b.node, nil
}
