package usecase

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	domainnode "omniflow-go/internal/domain/node"
)

// NodeNameConflictPolicy 定义创建节点遇到同级重名时的处理策略。
type NodeNameConflictPolicy string

const (
	// NodeNameConflictError 表示重名时直接返回 409。
	NodeNameConflictError NodeNameConflictPolicy = "error"
	// NodeNameConflictAutoRename 表示重名时自动追加序号。
	NodeNameConflictAutoRename NodeNameConflictPolicy = "auto_rename"

	maxNodeNameLength             = 255
	maxAutoRenameCandidateAttempt = 10000
)

func normalizeNodeNameConflictPolicy(input NodeNameConflictPolicy) (NodeNameConflictPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(string(input))) {
	case "", string(NodeNameConflictError):
		return NodeNameConflictError, nil
	case string(NodeNameConflictAutoRename), "auto-rename", "autorename":
		return NodeNameConflictAutoRename, nil
	default:
		return "", fmt.Errorf("%w: conflictPolicy only supports error or auto_rename", ErrInvalidArgument)
	}
}

func (u *NodeUseCase) resolveCreateNodeName(
	ctx context.Context,
	parentID uint64,
	libraryID uint64,
	name string,
	policy NodeNameConflictPolicy,
) (string, error) {
	normalizedPolicy, err := normalizeNodeNameConflictPolicy(policy)
	if err != nil {
		return "", err
	}
	if normalizedPolicy == NodeNameConflictError {
		return name, nil
	}

	children, err := u.nodes.ListDirectChildren(ctx, parentID, libraryID)
	if err != nil {
		return "", err
	}
	return resolveAutoRenamedNodeName(name, children)
}

func resolveAutoRenamedNodeName(name string, siblings []domainnode.Node) (string, error) {
	existingNames := make(map[string]struct{}, len(siblings))
	for _, sibling := range siblings {
		existingNames[sibling.Name] = struct{}{}
	}

	if _, exists := existingNames[name]; !exists {
		return name, nil
	}
	for index := 1; index <= maxAutoRenameCandidateAttempt; index++ {
		suffix := fmt.Sprintf(" (%d)", index)
		candidate := truncateNodeNameForSuffix(name, suffix) + suffix
		if _, exists := existingNames[candidate]; !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: no available node name after auto rename", ErrConflict)
}

func truncateNodeNameForSuffix(name string, suffix string) string {
	availableRunes := maxNodeNameLength - utf8.RuneCountInString(suffix)
	if availableRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(name) <= availableRunes {
		return name
	}
	runes := []rune(name)
	return string(runes[:availableRunes])
}
