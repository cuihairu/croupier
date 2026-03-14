// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type BehaviorPathsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为路径
func NewBehaviorPathsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorPathsLogic {
	return &BehaviorPathsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorPathsLogic) BehaviorPaths(req *types.BehaviorPathsRequest) (*types.BehaviorPathsResponse, error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	depth := req.Depth
	if depth <= 0 {
		depth = 5
	}
	if depth > 10 {
		depth = 10
	}

	start, end, err := utils.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.BehaviorEventOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 5000,
		},
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		StartTime: start,
		EndTime:   end,
	}

	events, _, err := l.svcCtx.BehaviorModel.ListEvents(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	paths := buildPaths(events, depth)

	return &types.BehaviorPathsResponse{
		Paths: map[string]interface{}{
			"depth": depth,
			"items": paths,
			"total": len(paths),
			"range": map[string]interface{}{
				"start": utils.FormatTimestamp(start),
				"end":   utils.FormatTimestamp(end),
			},
		},
	}, nil
}

type pathNode struct {
	Count    int
	Children map[string]*pathNode
}

func buildPaths(events []model.BehaviorEvent, depth int) []map[string]interface{} {
	byUser := groupEventsByUser(events)
	root := &pathNode{Children: map[string]*pathNode{}}

	for _, list := range byUser {
		if len(list) == 0 {
			continue
		}
		sequence := make([]string, 0, len(list))
		for _, ev := range list {
			name := strings.TrimSpace(ev.EventType)
			if name == "" {
				continue
			}
			sequence = append(sequence, name)
		}
		if len(sequence) == 0 {
			continue
		}
		addSequence(root, sequence, depth)
	}

	results := make([]map[string]interface{}, 0, 64)
	walkPaths(root, []string{}, &results, depth)

	sort.Slice(results, func(i, j int) bool {
		if results[i]["count"].(int) == results[j]["count"].(int) {
			return strings.Join(results[i]["path"].([]string), ">") < strings.Join(results[j]["path"].([]string), ">")
		}
		return results[i]["count"].(int) > results[j]["count"].(int)
	})

	if len(results) > 50 {
		results = results[:50]
	}
	return results
}

func groupEventsByUser(events []model.BehaviorEvent) map[string][]model.BehaviorEvent {
	grouped := make(map[string][]model.BehaviorEvent)
	for _, ev := range events {
		user := strings.TrimSpace(ev.UserID)
		if user == "" {
			continue
		}
		grouped[user] = append(grouped[user], ev)
	}

	for user := range grouped {
		list := grouped[user]
		sort.Slice(list, func(i, j int) bool {
			return list[i].OccurredAt.Before(list[j].OccurredAt)
		})
		grouped[user] = list
	}
	return grouped
}

func addSequence(root *pathNode, sequence []string, depth int) {
	node := root
	for i := 0; i < len(sequence) && i < depth; i++ {
		step := sequence[i]
		if node.Children == nil {
			node.Children = make(map[string]*pathNode)
		}
		child, ok := node.Children[step]
		if !ok {
			child = &pathNode{Children: map[string]*pathNode{}}
			node.Children[step] = child
		}
		child.Count++
		node = child
	}
}

func walkPaths(node *pathNode, prefix []string, acc *[]map[string]interface{}, depth int) {
	for label, child := range node.Children {
		path := append(prefix, label)
		entry := map[string]interface{}{
			"path":  append([]string{}, path...),
			"count": child.Count,
		}
		*acc = append(*acc, entry)
		if len(path) < depth {
			walkPaths(child, path, acc, depth)
		}
	}
}
