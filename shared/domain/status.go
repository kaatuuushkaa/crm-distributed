package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/samber/lo"
)

type StatusGraph struct {
	Current string
	Graph   map[string][]string
}

func NewStatusGraph(v string) (*StatusGraph, error) {
	if v != "*" {
		i, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("значение статуса должно быть целым числом, получено: %q", v)
		}

		if i < 0 || i > 20 {
			return nil, errors.New("значение статуса должно быть в диапазоне 0..20")
		}
	}

	return &StatusGraph{
		Current: v,
		Graph:   make(map[string][]string),
	}, nil
}

func NewStatusGraphFromJSON(str string) (*StatusGraph, error) {
	var raw map[string][]string
	if err := json.Unmarshal([]byte(str), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal status graph: %w", err)
	}

	return NewStatusGraphFromMap(raw)
}

func NewStatusGraphFromMap(raw map[string][]string) (*StatusGraph, error) {
	graph := make(map[string][]string, len(raw))

	for vertex, children := range raw {
		if _, ok := graph[vertex]; !ok {
			graph[vertex] = []string{}
		}

		for _, child := range children {
			if _, ok := graph[child]; !ok {
				graph[child] = []string{}
			}

			graph[vertex] = append(graph[vertex], child)
		}
	}

	return &StatusGraph{
		Current: "0",
		Graph:   graph,
	}, nil
}

func (s *StatusGraph) AddRoute(idx, child string) {
	if _, ok := s.Graph[idx]; !ok {
		s.Graph[idx] = []string{}
	}

	s.Graph[idx] = lo.Uniq(append(s.Graph[idx], child))
}

func (s *StatusGraph) RemoveRoute(idx, child string) {
	if _, ok := s.Graph[idx]; !ok {
		return
	}

	s.Graph[idx] = lo.Filter(s.Graph[idx], func(v string, _ int) bool {
		return v != child
	})
}

func CheckPathByValue(sg *StatusGraph, current, value string) (bool, []string) {
	if _, ok := sg.Graph[current]; !ok {
		sg.Current = "0"
	}

	if _, ok := sg.Graph[value]; !ok {
		return false, nil
	}

	visited := make(map[string]bool)

	var dfs func(node string, path []string) (bool, []string)
	dfs = func(node string, path []string) (bool, []string) {
		if visited[node] {
			return false, path
		}

		visited[node] = true

		if node == value || node == "*" {
			return true, path
		}

		for _, next := range sg.Graph[node] {
			nextPath := append(path, next) //nolint:gocritic // intentional copy per iteration
			if ok, found := dfs(next, nextPath); ok {
				return true, found
			}
		}

		return false, path
	}

	return dfs(sg.Current, []string{sg.Current})
}
