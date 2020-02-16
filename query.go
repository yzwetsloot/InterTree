package main

import "fmt"

func shortestPath(domain string, nodes map[string]node) (string, error) {
	v, ok := nodes[domain]

	if ok {
		res := domain
		n := v.parent

		for n != nil {
			res += " <- " + n.domain
			n = n.parent
		}
		return res, nil
	}
	return "", fmt.Errorf("Domain is not present in tree")
}
