package brunch

import (
	"fmt"
	"strings"
)

func contentPreview(content string) string {
	if len(content) > 25 {
		return content[:25] + "..."
	}
	return content
}

func PrettyPrint(node Node, indent string, isLastChild bool) string {
	var sb strings.Builder
	nodeIndent := indent
	if !isLastChild {
		nodeIndent = indent + "│"
	}

	switch n := node.(type) {
	case *RootNode:
		sb.WriteString(fmt.Sprintf("%s[ROOT] Provider: %s, Model: %s\n", nodeIndent, n.Provider, n.Model))
		sb.WriteString(fmt.Sprintf("%s├── Temperature: %.2f\n", nodeIndent, n.Temperature))
		sb.WriteString(fmt.Sprintf("%s├── MaxTokens: %d\n", nodeIndent, n.MaxTokens))
		sb.WriteString(fmt.Sprintf("%s└── Hash: %s\n", nodeIndent, n.Hash()))
		if len(n.Children) > 0 {
			for i, child := range n.Children {
				isLast := i == len(n.Children)-1
				childIndent := nodeIndent + "    "
				sb.WriteString(PrettyPrint(child, childIndent, isLast))
			}
		}

	case *MessagePairNode:
		prefix := "├──"
		if isLastChild {
			prefix = "└──"
		}
		sb.WriteString(fmt.Sprintf("%s%s [MESSAGE_PAIR] Time: %s\n", nodeIndent, prefix, n.Time.Format("2006-01-02 15:04:05")))
		if n.User != nil {
			if len(n.User.Images) > 0 {
				sb.WriteString(fmt.Sprintf("%s    ├── User (%s): %s\n", nodeIndent, n.User.Role, contentPreview(n.User.UnencodedContent())))
				sb.WriteString(fmt.Sprintf("%s    ├── User Images: %s\n", nodeIndent, strings.Join(n.User.Images, ", ")))
			} else {
				sb.WriteString(fmt.Sprintf("%s    ├── User (%s): %s\n", nodeIndent, n.User.Role, contentPreview(n.User.UnencodedContent())))
			}
		}
		if n.Assistant != nil {
			if len(n.Assistant.Images) > 0 {
				sb.WriteString(fmt.Sprintf("%s    ├── Assistant (%s): %s\n", nodeIndent, n.Assistant.Role, contentPreview(n.Assistant.UnencodedContent())))
				sb.WriteString(fmt.Sprintf("%s    ├── Assistant Images: %s\n", nodeIndent, strings.Join(n.Assistant.Images, ", ")))
			} else {
				sb.WriteString(fmt.Sprintf("%s    ├── Assistant (%s): %s\n", nodeIndent, n.Assistant.Role, contentPreview(n.Assistant.UnencodedContent())))
			}
		}
		sb.WriteString(fmt.Sprintf("%s    └── Hash: %s\n", nodeIndent, n.Hash()))
		if len(n.Children) > 0 {
			for i, child := range n.Children {
				isLast := i == len(n.Children)-1
				childIndent := nodeIndent + "    "
				sb.WriteString(PrettyPrint(child, childIndent, isLast))
			}
		}
	}
	return sb.String()
}

func PrintTree(node Node) string {
	return PrettyPrint(node, "", true)
}

func messageToString(message *MessageData) string {
	return fmt.Sprintf("%s: %s", message.Role, message.UnencodedContent())
}

func messageToStringWithImages(message *MessageData, images []string) string {
	return fmt.Sprintf("%s: %s [%d images]: %s", message.Role, message.UnencodedContent(), len(images), strings.Join(images, ", "))
}

// todo: make this not so bad
func MapTree(node Node) map[string]Node {
	if node == nil {
		return nil
	}

	tree := make(map[string]Node)

	// Add the current node to the map
	hash := node.Hash()
	if hash != "" {
		tree[hash] = node
	}

	// Recursively map children based on node type
	switch n := node.(type) {
	case *RootNode:
		for _, child := range n.Children {
			childMap := MapTree(child)
			for k, v := range childMap {
				tree[k] = v
			}
		}
	case *MessagePairNode:
		for _, child := range n.Children {
			childMap := MapTree(child)
			for k, v := range childMap {
				tree[k] = v
			}
		}
	}

	return tree
}
