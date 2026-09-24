// Command gen extracts character skeletons from the original app's .glb files
// into internal/sg/data/skeletons.json:
//
//	go run ./internal/sg/gen <storyboarder>/src/data/shot-generator/dummies/gltf
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"sb/internal/sg"
)

func round(v float64) float64 { return math.Round(v*1e6) / 1e6 }

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gen <dummies/gltf folder>")
		os.Exit(2)
	}
	out := map[string]*sg.Skeleton{}
	for _, m := range []string{"adult-female", "adult-male", "teen-female", "teen-male", "child", "baby"} {
		sk, err := sg.ParseSkeleton(filepath.Join(os.Args[1], m+".glb"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		sk.OriginalHeight = round(sk.OriginalHeight)
		for i := range sk.Bones {
			b := &sk.Bones[i]
			for k := range b.T {
				b.T[k], b.S[k] = round(b.T[k]), round(b.S[k])
			}
			for k := range b.R {
				b.R[k] = round(b.R[k])
			}
		}
		out[m] = sk
	}
	data, _ := json.MarshalIndent(out, "", " ")
	if err := os.WriteFile("internal/sg/data/skeletons.json", data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
